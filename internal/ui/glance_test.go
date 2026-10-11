package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestGlanceKeepsProgressAndInspectorReachable(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {110, 32}, {96, 24}, {96, 18}, {160, 48}} {
		m := demoModel(size[0], size[1])
		defer m.Close()
		j, _ := m.selected()
		l := m.layout()
		text := ansi.Strip(m.glance(j, l.detailWidth-4, l.bodyHeight-2))
		lines := strings.Split(text, "\n")
		if len(lines) != l.bodyHeight-2 || !strings.Contains(lines[len(lines)-1], "full details") {
			t.Fatalf("%v clipped inspector link: %q", size, text)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > l.detailWidth-4 {
				t.Fatalf("%v overflow: %s", size, line)
			}
		}
		if !strings.Contains(text, "finished") {
			t.Fatalf("%v lost array progress: %s", size, text)
		}
		x, y := renderedTarget(t, m, "full details")
		click(m, x, y)
		if m.view != "inspect" {
			t.Fatalf("%v footer click did not open inspector", size)
		}
		before, _ := m.selected()
		if before.ID != j.ID {
			t.Fatal("inspector link changed selection")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.view != "details" || m.selectedID != j.ID {
			t.Fatal("back lost list selection")
		}
	}
}

func TestGlanceClickObeysMousePreferenceAndBorders(t *testing.T) {
	m := demoModel(140, 40)
	defer m.Close()
	x, y := renderedTarget(t, m, "full details")
	m.options.NoMouse = true
	click(m, x, y)
	if m.view != "details" {
		t.Fatal("disabled mouse still opened inspector")
	}
	m.options.NoMouse = false
	click(m, x, y+1)
	if m.view != "details" {
		t.Fatal("border opened inspector")
	}
}

func TestFullInspectorWrapsLongValuesWithoutLosingText(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	j, _ := m.selected()
	j.Name = strings.Repeat("experiment", 12)
	j.Nodes = strings.Repeat("node", 20)
	text := ansi.Strip(m.details(j, 40))
	for _, line := range strings.Split(text, "\n") {
		if ansi.StringWidth(line) > 40 {
			t.Fatal("unreachable clipped detail", line)
		}
	}
	flattened := strings.ReplaceAll(text, "\n", "")
	if !strings.Contains(flattened, j.Name) || !strings.Contains(flattened, j.Nodes) {
		t.Fatal("full inspector lost long values")
	}
}

func TestGlanceLongIdentityDoesNotHideStatus(t *testing.T) {
	m := demoModel(96, 18)
	defer m.Close()
	j, _ := m.selected()
	j.ID = "42_[" + strings.Repeat("1,3,5,", 20) + "9]"
	text := ansi.Strip(m.glance(j, 32, 4))
	if !strings.Contains(text, "running") || !strings.Contains(text, "full details") {
		t.Fatal(text)
	}
}
