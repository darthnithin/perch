package bird

import (
	"embed"
	"fmt"
	"image"
	_ "image/png"
	"strings"
	"sync"
	"time"
)

//go:embed *-strip.png
var images embed.FS
var once sync.Once
var frames map[string][]string
var loadErr error
var posters = map[string]int{"idle": 0, "running": 1, "queued": 8, "complete": 11, "attention": 0, "reconnecting": 23}

// FrameInterval controls terminal playback independently of the original web art.
// Running samples a slower, 640 ms stroke at 20 ms intervals.
func FrameInterval(state string) time.Duration {
	if state == "running" {
		return 20 * time.Millisecond
	}
	return 80 * time.Millisecond
}

func terminalSource(state string) string {
	if state == "running" {
		return "terminal-running-strip.png"
	}
	return state + "-strip.png"
}

func Load() error {
	once.Do(func() {
		frames = map[string][]string{}
		for state := range posters {
			f, e := images.Open(terminalSource(state))
			if e != nil {
				loadErr = e
				return
			}
			img, _, e := image.Decode(f)
			_ = f.Close()
			if e != nil {
				loadErr = e
				return
			}
			if img.Bounds().Dx() != 2048 || img.Bounds().Dy() != 32 {
				loadErr = fmt.Errorf("invalid Chip sprite dimensions")
				return
			}
			for frame := 0; frame < 64; frame++ {
				frames[state] = append(frames[state], render(img, frame))
			}
		}
	})
	return loadErr
}
func Frame(state string, index int, static bool) string {
	if Load() != nil {
		return "Chip"
	}
	if _, ok := frames[state]; !ok {
		state = "idle"
	}
	if static {
		index = posters[state]
	}
	return frames[state][index%64]
}
func pixel(img image.Image, x, y int) [3]uint32 {
	r, g, b, a := img.At(x, y).RGBA()
	if a == 0 {
		return [3]uint32{22, 26, 28}
	}
	return [3]uint32{r >> 8, g >> 8, b >> 8}
}
func render(img image.Image, frame int) string {
	var b strings.Builder
	for y := 0; y < 32; y += 2 {
		var previousTop, previousBottom [3]uint32
		for x := 0; x < 32; x++ {
			top, bottom := pixel(img, frame*32+x, y), pixel(img, frame*32+x, y+1)
			if x == 0 || top != previousTop || bottom != previousBottom {
				fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm", top[0], top[1], top[2], bottom[0], bottom[1], bottom[2])
				previousTop, previousBottom = top, bottom
			}
			b.WriteRune('▀')
		}
		b.WriteString("\x1b[0m")
		if y < 30 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
