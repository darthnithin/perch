package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Locate targets in rendered text rather than deriving the same coordinates as
// the implementation. This catches drift between drawing and hit testing.
func renderedTarget(t *testing.T, m *Model, text string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if offset := strings.Index(line, text); offset >= 0 {
			return ansi.StringWidth(line[:offset]), y
		}
	}
	t.Fatalf("target %q absent from view", text)
	return 0, 0
}
func click(m *Model, x, y int) {
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}
func wheel(m *Model, x, y int, button tea.MouseButton) {
	m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: button})
}

func TestMouseTargetsMatchRenderedJobsAfterResize(t *testing.T) {
	for _, size := range [][2]int{{160, 44}, {140, 44}, {110, 32}, {80, 24}, {60, 24}, {48, 24}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m := demoModel(160, 44)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			// Ensure there are at least two rows visible at the smallest height.
			jobs := m.jobs()
			x, y := renderedTarget(t, m, jobs[1].ID)
			click(m, x, y)
			if m.selectedID != jobs[1].ID {
				t.Fatalf("selected %s, want %s", m.selectedID, jobs[1].ID)
			}
			// A compact card's second line belongs to the same record.
			if size[0] == 48 {
				click(m, x, y+1)
				if m.selectedID != jobs[1].ID {
					t.Fatal("context row selected different job")
				}
			}
			// Headers and border must not change selection.
			click(m, 4, 4)
			click(m, 2, y)
			if m.selectedID != jobs[1].ID {
				t.Fatal("non-row selected a job")
			}
		})
	}
}
func TestMouseTabsInputAndDisabled(t *testing.T) {
	m := demoModel(160, 44)
	for i, label := range tabLabels {
		x, y := renderedTarget(t, m, label)
		click(m, x, y)
		if m.tab != i {
			t.Fatalf("tab %d did not activate", i)
		}
	}
	x, y := renderedTarget(t, m, "Find a command")
	click(m, x, y)
	if !m.input.Focused() {
		t.Fatal("command input did not focus")
	}
	m.input.SetValue("/find embedding")
	x, y = renderedTarget(t, m, "1  Jobs")
	click(m, x, y)
	if m.input.Focused() || m.tab != 0 {
		t.Fatal("tab did not take focus")
	}
	// Keep the draft so an accidental click does not destroy user input.
	if m.input.Value() != "/find embedding" {
		t.Fatal("draft lost")
	}
	m.options.NoMouse = true
	x, y = renderedTarget(t, m, "2  History")
	click(m, x, y)
	if m.tab != 0 || m.View().MouseMode != tea.MouseModeNone {
		t.Fatal("no-mouse ignored")
	}
	m.options.NoMouse = false
	if m.View().MouseMode != tea.MouseModeAllMotion {
		t.Fatal("mouse capture not enabled")
	}
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	if m.View().MouseMode != tea.MouseModeNone {
		t.Fatal("small window must release mouse capture")
	}
}
func TestMouseWheelIsScopedAndClamped(t *testing.T) {
	m := demoModel(160, 44)
	x, y := renderedTarget(t, m, "› "+m.jobs()[0].ID)
	wheel(m, x, 1, tea.MouseWheelDown)
	wheel(m, 150, y, tea.MouseWheelDown)
	if m.selection != 0 {
		t.Fatal("wheel outside jobs moved selection")
	}
	wheel(m, x, y, tea.MouseWheelDown)
	if m.selection != 3 {
		t.Fatalf("wheel did not move through jobs: selection %d", m.selection)
	}
	for range 10 {
		wheel(m, x, y, tea.MouseWheelDown)
	}
	if m.selection != len(m.jobs())-1 {
		t.Fatal("selection not clamped")
	}
	for range 10 {
		wheel(m, x, y, tea.MouseWheelUp)
	}
	if m.selection != 0 {
		t.Fatal("selection not clamped at top")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	press(m, '?')
	x, y = renderedTarget(t, m, "Move around")
	wheel(m, x, y, tea.MouseWheelDown)
	if m.panel.YOffset() == 0 {
		t.Fatal("help did not scroll")
	}
	press(m, '?')
	press(m, 'l')
	m.viewport.SetContent(strings.Repeat("line\n", 100))
	wheel(m, 8, 7, tea.MouseWheelDown)
	if m.viewport.YOffset() == 0 {
		t.Fatal("log viewport did not scroll")
	}
	offset := m.viewport.YOffset()
	wheel(m, 8, 5, tea.MouseWheelDown)
	if m.viewport.YOffset() != offset {
		t.Fatal("log heading scrolled viewport")
	}
}
func TestMousePaletteUsesVisibleWindowAndDoesNotActivateWhitespace(t *testing.T) {
	m := demoModel(160, 24)
	m.action("ctrl+k")
	click(m, 150, 7)
	if !m.palette {
		t.Fatal("palette whitespace executed action")
	}
	for range 3 {
		wheel(m, 8, 7, tea.MouseWheelDown)
	}
	x, y := renderedTarget(t, m, "Jobs")
	click(m, x, y)
	if m.palette || m.tab != 0 {
		t.Fatal("visible palette command did not execute")
	}
	m.action("ctrl+k")
	x, y = renderedTarget(t, m, "2  History")
	click(m, x, y)
	if m.palette || m.tab != 1 {
		t.Fatal("tab did not dismiss palette")
	}
}
