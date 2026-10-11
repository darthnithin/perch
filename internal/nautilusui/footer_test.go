package nautilusui

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
		t.Fatalf("missing %q in %q", label, line)
	}
	return ansi.StringWidth(line[:i]), len(lines) - 1
}

func footerClick(t *testing.T, m *Model, label string) tea.Cmd {
	t.Helper()
	x, y := footerPoint(t, m, label)
	if m.mouseTarget(tea.Mouse{X: x, Y: y}).kind != "footer" {
		t.Fatal("visible action has no target", label)
	}
	_, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return cmd
}

func TestFooterInspectLogsRoundTrip(t *testing.T) {
	for _, size := range [][2]int{{48, 18}, {80, 24}, {120, 38}} {
		m := ready(t, size[0], size[1])
		filter(m, "retrying")
		uid := m.selectedUID[0]
		footerClick(t, m, "enter inspect")
		if !m.inspect || !strings.Contains(ansi.Strip(m.View().Content), "Esc returns") {
			t.Fatal("inspection or filter hint disagrees with action")
		}
		footerClick(t, m, "l logs")
		if m.logs == nil || m.logs.stage != "pods" {
			t.Fatal("missing Pod picker")
		}
		footerClick(t, m, "↓")
		footerClick(t, m, "enter open")
		if m.logs.stage != "containers" || m.logs.pod.Name != "retrying-replacement" {
			t.Fatal("wrong Pod")
		}
		applyCommand(t, m, footerClick(t, m, "enter open"))
		if !strings.Contains(m.logs.panel.GetContent(), "Current instance") {
			t.Fatal("missing current output")
		}
		applyCommand(t, m, footerClick(t, m, "p "))
		if !strings.Contains(m.logs.panel.GetContent(), "Previous instance") || !m.logs.previous {
			t.Fatal("mode click failed")
		}
		for range 3 {
			footerClick(t, m, "esc back")
		}
		if m.logs != nil || !m.inspect || m.filters[0] != "retrying" || m.selectedUID[0] != uid {
			t.Fatal("lost original inspection")
		}
		footerClick(t, m, "esc back")
		if m.inspect || m.filters[0] != "retrying" {
			t.Fatal("back cleared list filter")
		}
	}
}

func TestFooterInputFocusAndMouseGuards(t *testing.T) {
	m := ready(t, 48, 18)
	key(m, "/")
	m.input.SetValue("/find retrying")
	if strings.Contains(m.footer(), "inspect") {
		t.Fatal("input advertises inspector")
	}
	footerClick(t, m, "enter run")
	if m.filters[0] != "retrying" || m.input.Focused() || m.inspect {
		t.Fatal("Enter bypassed input")
	}
	x, y := footerPoint(t, m, "enter inspect")
	m.options.PointerStyle = "css"
	motion(m, x, y)
	if m.lastPointer != "pointer" || m.inspect {
		t.Fatal("hover activated inspector or lacks hand")
	}
	for _, mode := range []string{"disabled", "connecting", "modified", "right"} {
		m.options.NoMouse = mode == "disabled"
		m.connecting = mode == "connecting"
		msg := tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
		if mode == "modified" {
			msg.Mod = tea.ModShift
		}
		if mode == "right" {
			msg.Button = tea.MouseRight
		}
		m.Update(msg)
		if m.inspect {
			t.Fatal(mode, "activated footer")
		}
	}
	m.options.NoMouse = false
	m.connecting = false
	key(m, "4")
	if ansi.StringWidth(m.footer()) > 44 {
		t.Fatal("connection recovery clipped")
	}
	footerClick(t, m, "esc back")
	if m.tab != 0 || m.filters[0] != "retrying" {
		t.Fatal("connection back lost list")
	}
}
