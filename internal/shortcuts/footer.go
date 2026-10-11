// Package shortcuts maps visible terminal keyboard hints to pointer targets.
package shortcuts

import (
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Footer copy remains the source of its visible geometry. Only complete,
// explicitly recognized shortcuts become targets; prose and clipped hints do
// not. Labels are separated by two spaces or a spaced middle dot.
var footerSeparator = regexp.MustCompile(` {2,}| · `)

// Key returns only supported, explicit keyboard shortcuts.
func Key(key string) (tea.KeyPressMsg, bool) {
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}, true
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}, true
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}, true
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, true
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}, true
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, true
	case "ctrl+c", "^C":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, true
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}, true
	case "ctrl+x":
		return tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}, true
	case "↑":
		return tea.KeyPressMsg{Code: tea.KeyUp}, true
	case "↓":
		return tea.KeyPressMsg{Code: tea.KeyDown}, true
	case "a", "o", "p", "n", "s", "e", "v", "c", "r", "l", "u", "f", "g", "y", "q", "?", "/", "2", "[", "]":
		return tea.KeyPressMsg{Code: rune(key[0]), Text: key}, true
	}
	return tea.KeyPressMsg{}, false
}

type Target struct {
	Key      string
	X, Width int
}

// Targets uses cell coordinates relative to the left edge of the rendered text.
// Width is the available content width, excluding any outer padding.
func Targets(text string, contentWidth int) []Target {
	text = ansi.Strip(text)
	var targets []Target
	start := 0
	ends := append(footerSeparator.FindAllStringIndex(text, -1), []int{len(text), len(text)})
	add := func(key string, x, width int) {
		if _, ok := Key(key); ok {
			targets = append(targets, Target{Key: key, X: x, Width: width})
		}
	}
	for _, end := range ends {
		label := text[start:end[0]]
		x := ansi.StringWidth(text[:start])
		width := ansi.StringWidth(label)
		start = end[1]
		// fit() reserves the last cell for an ellipsis if the full footer clips.
		limit := contentWidth
		if ansi.StringWidth(text) > limit {
			limit--
		}
		if x+width > limit {
			continue
		}
		switch {
		case strings.HasPrefix(label, "↑↓ "):
			add("↑", x, 1)
			add("↓", x+1, 1)
		case strings.HasPrefix(label, "[ ] "):
			add("[", x, 1)
			add("]", x+2, 1)
		default:
			key, _, found := strings.Cut(label, " ")
			if found {
				add(key, x, width)
			}
		}
	}
	return targets
}
