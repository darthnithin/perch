package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func command(m *Model, value string) tea.Cmd {
	press(m, '/')
	m.input.SetValue(value)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return cmd
}
func TestCommandInputNavigationAndFiltering(t *testing.T) {
	m := demoModel(160, 44)
	command(m, "/history")
	if m.tab != 1 || m.input.Focused() {
		t.Fatal("history command did not navigate")
	}
	command(m, "/jobs")
	command(m, "/find embedding")
	if len(m.jobs()) != 1 || m.search != "embedding" {
		t.Fatal("filter command failed")
	}
	command(m, "/job 9100_7")
	j, ok := m.selected()
	if !ok || j.ID != "9100_7" || m.tab != 1 || m.view != "inspect" {
		t.Fatal("jump failed", j)
	}
	command(m, "9100_9")
	j, ok = m.selected()
	if !ok || j.ID != "9100_9" || m.tab != 0 {
		t.Fatal("bare job ID failed", j)
	}
}
func TestCommandInputValidationCompletionAndEscape(t *testing.T) {
	m := demoModel(80, 24)
	command(m, "/unknown")
	if !m.input.Focused() || m.inputError == "" {
		t.Fatal("invalid command lost feedback")
	}
	m.input.SetValue("/part")
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.input.Value() != "/partitions " {
		t.Fatal(m.input.Value())
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.input.Focused() || m.input.Value() != "" {
		t.Fatal("Escape failed")
	}
}
func TestWorkspaceHasCommandBarAndNoMascotPanel(t *testing.T) {
	m := demoModel(160, 44)
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"Hummingbird workspace", "1  Jobs", "2  History", "3  Partitions", "4  Connection", "My jobs", "Find a command or jump to a job", "/jobs"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(view, "╭─ Chip") || strings.Contains(view, "╭─ Workspace") {
		t.Fatal("oversized mascot returned")
	}
}

func TestApprovedOctantChipYieldsToCompactViews(t *testing.T) {
	m := demoModel(160, 44)
	view := ansi.Strip(m.View().Content)
	if !hasChipGlyph(view) || !strings.Contains(view, "q quit") {
		t.Fatal("approved Chip or footer missing")
	}
	for _, settings := range []struct {
		width, height int
		noColor       bool
	}{{80, 24, false}, {160, 44, true}} {
		m := demoModel(settings.width, settings.height)
		m.options.NoColor = settings.noColor
		if hasChipGlyph(ansi.Strip(m.View().Content)) {
			t.Fatal("Chip must yield space in compact/no-color mode")
		}
	}
	m.help = true
	if hasChipGlyph(ansi.Strip(m.View().Content)) {
		t.Fatal("Chip belongs to jobs, not help")
	}
}

func hasChipGlyph(s string) bool {
	for _, r := range s {
		if r >= 0x1cd00 && r <= 0x1ceaf || r >= 0x2580 && r <= 0x259f {
			return true
		}
	}
	return false
}
