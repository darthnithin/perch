package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestHelpReflowsWithoutLosingCommands(t *testing.T) {
	m := demoModel(120, 38)
	defer m.Close()
	press(m, '?')
	for _, width := range []int{120, 48, 80, 160, 48} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 38})
		text := ansi.Strip(m.panel.GetContent())
		for _, key := range []string{"/run", "/receipts", "/waits", "/software query", "/module name/version", "Ctrl+k", "Shift+Tab", "--no-mouse"} {
			if !strings.Contains(text, key) {
				t.Fatalf("%d: missing %s", width, key)
			}
		}
		for _, line := range strings.Split(text, "\n") {
			if ansi.StringWidth(line) > width-8 {
				t.Fatalf("%d: overflowing help row %q", width, line)
			}
		}
		if (strings.Index(text, "Work on Hummingbird") < strings.Index(text, "Select a job")) != (width >= 112) {
			t.Fatalf("%d: incorrect column reading order", width)
		}
	}
	m.options.Demo = false
	if strings.Contains(m.helpText(), "demo scenario") {
		t.Fatal("demo-only binding exposed in a live workspace")
	}
}

func TestReverseTabKeepsWorkspaceContext(t *testing.T) {
	m := demoModel(120, 38)
	defer m.Close()
	command(m, "/find parameter")
	m.action("down")
	id := m.selectedID
	press(m, '?')
	if strings.Contains(m.commandHint(), "clears the filter") || !strings.Contains(m.commandHint(), "filter kept") {
		t.Fatal("help advertises clearing a filter that it preserves")
	}
	for _, expected := range []int{3, 2, 1, 0} {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		if m.tab != expected || m.help {
			t.Fatalf("reverse tab selected %d; want %d", m.tab, expected)
		}
	}
	if m.search != "parameter" || m.selectedID != id {
		t.Fatal("reverse navigation lost the job filter or selection")
	}
	// Text-entry modes retain ownership of their own Tab keys.
	m.action("/")
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.tab != 0 || !m.input.Focused() {
		t.Fatal("reverse tab escaped command input")
	}
}

func TestHelpDoesNotHideRequestedJobView(t *testing.T) {
	for key, view := range map[string]string{"enter": "inspect", "l": "logs", "s": "script", "u": "resources", "f": "details"} {
		m := demoModel(120, 38)
		press(m, '?')
		m.action(key)
		if m.help || m.view != view {
			t.Errorf("%s left requested %s hidden behind Help", key, view)
		}
		m.Close()
	}
}
