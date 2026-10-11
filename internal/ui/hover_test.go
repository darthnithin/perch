package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestPointerDetection(t *testing.T) {
	for _, tc := range []struct{ program, term, tmux, want string }{
		{"ghostty", "xterm-256color", "", "css"}, {"iTerm.app", "xterm-256color", "", "x11"}, {"", "xterm-kitty", "", "css"}, {"", "foot-extra", "", "css"}, {"Apple_Terminal", "xterm-256color", "", ""}, {"unknown", "xterm-256color", "", ""}, {"ghostty", "xterm-256color", "/tmp/tmux", ""},
	} {
		env := map[string]string{"TERM_PROGRAM": tc.program, "TERM": tc.term, "TMUX": tc.tmux}
		if got := DetectPointerStyle(func(k string) string { return env[k] }); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
func TestHoverMatchesClickTargetsWithoutChangingSelectionOrText(t *testing.T) {
	for _, size := range [][2]int{{160, 44}, {100, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		m.options.PointerStyle = "css"
		before := ansi.Strip(m.View().Content)
		selected := m.selectedID
		x, y := renderedTarget(t, m, "History")
		m.Update(tea.MouseMotionMsg{X: x, Y: y})
		if m.lastPointer != "pointer" || m.tab != 0 || m.selectedID != selected {
			t.Fatal("hover navigated", m.lastPointer)
		}
		got := m.View().Content
		if ansi.Strip(got) != before || !strings.Contains(got, "\x1b[4m") {
			t.Fatal("hover changed characters/layout", size)
		}
		if m.syncPointer() != nil {
			t.Fatal("unchanged pointer emits redundant output")
		}
		m.Update(tea.MouseMotionMsg{X: 0, Y: 0})
		if m.lastPointer != "default" {
			t.Fatal("non-target has pointer")
		}
		x, y = renderedTarget(t, m, "Find a command")
		m.Update(tea.MouseMotionMsg{X: x, Y: y})
		if m.lastPointer != "text" || m.input.Focused() {
			t.Fatal("hover focused input")
		}
		m.Update(tea.BlurMsg{})
		if m.hoverKnown || m.lastPointer != "default" {
			t.Fatal("blur retained hover")
		}
		m.Update(tea.MouseMotionMsg{X: x, Y: y})
		m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
		if m.hoverKnown || m.lastPointer != "default" {
			t.Fatal("resize retained stale coordinates")
		}
	}
}
func TestHoverDisabledAndPointerVocabulary(t *testing.T) {
	m := demoModel(100, 40)
	m.options.PointerStyle = "x11"
	x, y := renderedTarget(t, m, "History")
	m.Update(tea.MouseMotionMsg{X: x, Y: y})
	if m.lastPointer != "hand2" || m.PointerReset() != ansi.SetPointerShape("xterm") {
		t.Fatal(m.lastPointer, m.PointerReset())
	}
	m.options.NoMouse = true
	m.Update(tea.MouseMotionMsg{X: x, Y: y})
	if m.lastPointer != "xterm" || m.hoverTarget().kind != "" {
		t.Fatal("disabled capture still has hit target")
	}
	m = demoModel(100, 40)
	m.options.NoMouse = true
	m.options.PointerStyle = "css"
	m.Update(tea.MouseMotionMsg{X: x, Y: y})
	if m.lastPointer != "" || m.PointerReset() != "" {
		t.Fatal("untouched pointer was modified")
	}
}
