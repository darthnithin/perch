package bird

import (
	"fmt"
	"image"
	"strings"
	"sync"
)

var octantOnce sync.Once
var octantFrames map[string][][]string

// OctantFrame uses the approved detailed sprite silhouette at two source pixels
// across and four down per character. The palette treatment is the user's D
// choice. Only foreground colors are emitted; source eyes become negative space.
// This requires a Unicode 16 block-mosaic font, such as Ioskeley Mono.
func OctantFrame(state string, frame int, static bool) []string {
	octantOnce.Do(func() {
		octantFrames = make(map[string][][]string)
		for name := range posters {
			f, err := images.Open(terminalSource(name))
			if err != nil {
				continue
			}
			source, _, err := image.Decode(f)
			_ = f.Close()
			if err != nil {
				continue
			}
			frames := make([][]string, 64)
			for i := range frames {
				frames[i] = renderOctants(source, name, i)
			}
			octantFrames[name] = frames
		}
	})
	frames, ok := octantFrames[state]
	if !ok {
		state = "idle"
		frames = octantFrames[state]
	}
	if len(frames) == 0 {
		return nil
	}
	if static {
		frame = posters[state]
	}
	frame = (frame%len(frames) + len(frames)) % len(frames)
	return append([]string(nil), frames[frame]...)
}

type spriteRGB [3]uint8

func renderOctants(source image.Image, state string, frame int) []string {
	rows := make([]string, 0, 7)
	for y := 0; y < 28; y += 4 {
		var row strings.Builder
		for x := 2; x < 32; x += 2 {
			mask := 0
			colors := map[spriteRGB]int{}
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					if y+dy >= 26 && state != "running" {
						continue
					} // The panel replaces the original branch.
					r, g, b, a := source.At(frame*32+x+dx, y+dy).RGBA()
					c := spriteRGB{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
					if a == 0 || c == (spriteRGB{48, 42, 42}) {
						continue
					}
					mask |= 1 << (dx + 2*dy)
					weight := 5
					if c == (spriteRGB{168, 111, 89}) {
						weight = 9
					}
					colors[c] += weight
				}
			}
			if mask == 0 {
				row.WriteByte(' ')
				continue
			}
			var chosen spriteRGB
			best := -1
			for c, weight := range colors {
				if weight > best || weight == best && rgbLess(c, chosen) {
					chosen, best = c, weight
				}
			}
			fmt.Fprintf(&row, "\x1b[38;2;%d;%d;%dm%c\x1b[m", chosen[0], chosen[1], chosen[2], octantGlyphs[mask])
		}
		rows = append(rows, row.String())
	}
	return rows
}
func rgbLess(a, b spriteRGB) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
