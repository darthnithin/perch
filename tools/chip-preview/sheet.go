package main

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"io"
	"perch/assets/bird"
	"strings"
)

const sheetCardWidth = 18

// printSheet uses the application's exact frames. Plain scrollback output lets
// reviewers select/copy and screenshot any section without animation advancing.
func printSheet(w io.Writer, selected []string, columns int) error {
	var b strings.Builder
	const accent = "\x1b[38;2;230;173;143m"
	const reset = "\x1b[0m"
	b.WriteString(accent + "Chip · all-frame contact sheet · D / stronger wing shading" + reset + "\n")
	fmt.Fprintf(&b, "%d sprites × 64 frames · labels are state + frame 00–63\n", len(selected))
	b.WriteString("Same 15 × 7 cell artwork as Perch. Use Ioskeley Mono.\n")
	b.WriteString("Scroll back to review. Repeated poses are intentional timing holds.\n")
	b.WriteString("Share a screenshot or a label, e.g. queued #08.\n\n")
	for _, state := range selected {
		fmt.Fprintf(&b, "%s%s · 64 frames%s\n\n", accent, strings.ToUpper(state), reset)
		for start := 0; start < 64; start += columns {
			end := min(start+columns, 64)
			frames := make([][]string, end-start)
			for frame := start; frame < end; frame++ {
				frames[frame-start] = bird.OctantFrame(state, frame, false)
			}
			for frame := start; frame < end; frame++ {
				label := fmt.Sprintf("%s #%02d", state, frame)
				b.WriteString(accent + label + reset + strings.Repeat(" ", sheetCardWidth-ansi.StringWidth(label)))
				if frame < end-1 {
					b.WriteString("  ")
				}
			}
			b.WriteByte('\n')
			for y := 0; y < 7; y++ {
				for i, rows := range frames {
					row := rows[y]
					b.WriteString(row + strings.Repeat(" ", sheetCardWidth-ansi.StringWidth(row)))
					if i < len(frames)-1 {
						b.WriteString("  ")
					}
				}
				b.WriteByte('\n')
			}
			b.WriteByte('\n')
		}
	}
	b.WriteString(accent + "End of sheet" + reset + " · all frames shown; original assets unchanged.\n")
	_, err := io.WriteString(w, b.String())
	return err
}
