package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Keep the nearby reading context when an intervening report replaces the panel.
// Text anchors survive changed wrapping above them and changing ANSI styles.
// If a line disappears (for example, an allocation count changes), try an
// earlier line, then fall back to the old offset. The viewport clamps the result.
type panelBookmark struct {
	row   int
	lines []string
}

func panelLines(content string) []string {
	lines := strings.Split(ansi.Strip(content), "\n")
	for i := range lines {
		lines[i] = strings.Join(strings.Fields(lines[i]), " ")
	}
	return lines
}

func bookmarkPanel(content string, row int) panelBookmark {
	lines := panelLines(content)
	row = max(0, min(row, len(lines)-1))
	return panelBookmark{row: row, lines: lines}
}

func (b panelBookmark) offset(content string) int {
	if b.row == 0 {
		return 0
	}
	lines := panelLines(content)
	oldCounts := make(map[string]int, len(b.lines))
	for _, line := range b.lines {
		oldCounts[line]++
	}
	positions := make(map[string]int, len(lines))
	for i, line := range lines {
		if _, exists := positions[line]; exists {
			positions[line] = -1
		} else {
			positions[line] = i
		}
	}
	for i := min(b.row, len(b.lines)-1); i >= 0; i-- {
		anchor := b.lines[i]
		// Blank lines and graphical separators are not meaningful anchors.
		if strings.Trim(anchor, " ─━│╭╮╰╯") == "" {
			continue
		}
		if oldCounts[anchor] != 1 {
			continue
		}
		if match, exists := positions[anchor]; exists && match >= 0 {
			return match + b.row - i
		}
	}
	return b.row
}
