package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func footerPoint(t *testing.T, m *Model, label string) (int, int) {
	t.Helper()
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	line := lines[len(lines)-1]
	i := strings.Index(line, label)
	if i < 0 {
		t.Fatalf("footer lacks %q: %q", label, line)
	}
	return ansi.StringWidth(line[:i]), len(lines) - 1
}

func footerClick(t *testing.T, m *Model, label string) {
	t.Helper()
	x, y := footerPoint(t, m, label)
	if m.mouseTarget(tea.Mouse{X: x, Y: y}).kind != "footer" {
		t.Fatalf("visible %q lacks target", label)
	}
	click(m, x, y)
}

func TestFooterMouseRoundTripPreservesSelectionAndFilter(t *testing.T) {
	for _, size := range [][2]int{{160, 44}, {100, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		command(m, "/find parameter")
		m.action("down")
		id := m.selectedID
		m.options.PointerStyle = "css"
		x, y := footerPoint(t, m, "enter inspect")
		m.Update(tea.MouseMotionMsg{X: x, Y: y})
		if m.lastPointer != "pointer" || m.view != "details" {
			t.Fatal("hover activated inspector or lacks pointer")
		}
		footerClick(t, m, "enter inspect")
		if m.view != "inspect" {
			t.Fatal("did not inspect", size)
		}
		footerClick(t, m, "l output")
		if m.view != "logs" {
			t.Fatal("did not open output", size)
		}
		footerClick(t, m, "e stdout/stderr")
		if !m.logErr {
			t.Fatal("did not switch stream")
		}
		footerClick(t, m, "esc back")
		if m.view != "details" || m.selectedID != id || m.search != "parameter" {
			t.Fatal("lost list context", size)
		}
		m.Close()
	}
}

func TestFooterRespectsInputAndPaletteOwnership(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.action("/")
	m.input.SetValue("/find embedding")
	footerClick(t, m, "enter run")
	if m.input.Focused() || m.search != "embedding" || m.view != "details" {
		t.Fatal("footer Enter bypassed command input")
	}
	m.action("f")
	m.search = "parameter"
	footerClick(t, m, "enter keep")
	if m.searching || m.search != "parameter" {
		t.Fatal("footer Enter did not keep filter")
	}
	m.action("ctrl+k")
	m.paletteQuery = "resources"
	footerClick(t, m, "enter run")
	if m.palette || m.view != "resources" {
		t.Fatal("footer Enter bypassed palette")
	}
	footerClick(t, m, "esc back")
	m.action("?")
	footerClick(t, m, "esc back")
	if m.help || m.search != "parameter" {
		t.Fatal("help back cleared filter")
	}
}

func TestFooterOnlyVisibleActionsAreInteractive(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	for _, view := range []string{"details", "inspect", "logs", "script", "resources", "software", "waits", "receipts"} {
		m.view = view
		for _, target := range m.footerTargets() {
			if target.x < 2 || target.x+target.width > m.width-2 || target.y != m.height-1 {
				t.Fatal("target exceeds rendered footer", target)
			}
		}
	}
	m.view = "details"
	x, y := footerPoint(t, m, "enter inspect")
	m.options.NoMouse = true
	click(m, x, y)
	if m.view != "details" {
		t.Fatal("disabled mouse activated footer")
	}
	m.options.NoMouse = false
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft, Mod: tea.ModShift})
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseRight})
	if m.view != "details" {
		t.Fatal("modified or right click activated footer")
	}
	// Arrow keys have separate targets; the explanatory text and gaps don't.
	x, y = footerPoint(t, m, "↑↓ select")
	if m.mouseTarget(tea.Mouse{X: x + 3, Y: y}).kind != "" {
		t.Fatal("scroll prose should not guess a direction")
	}
	before := m.selectedID
	click(m, x+1, y)
	if m.selectedID == before {
		t.Fatal("down arrow did not select next job")
	}
	click(m, x, y)
	if m.selectedID != before {
		t.Fatal("up arrow did not return")
	}
}

func TestLaunchFooterReflowsAndUsesModalKeys(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.openLaunch()
	footerClick(t, m, "tab next")
	if m.launchForm.focus != 1 {
		t.Fatal("footer Tab left the form")
	}
	for _, stage := range []string{"edit", "review", "ready", "checking", "submitting", "submitted", "uncertain"} {
		m.launchForm.stage = stage
		text := ansi.Strip(m.footer())
		if ansi.StringWidth(text) > 44 {
			t.Fatal("launch footer clipped", stage, text)
		}
		if stage == "checking" || stage == "submitting" {
			if len(m.footerTargets()) != 0 {
				t.Fatal("waiting footer has actions")
			}
		} else if !strings.Contains(text, "esc close") {
			t.Fatal("launch lost back action", stage)
		}
	}
	m.launchForm.stage = "edit"
	footerClick(t, m, "esc close")
	if m.view == "launch" {
		t.Fatal("footer Escape did not close form")
	}
}
