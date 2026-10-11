package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestAllTabsRemainVisibleAndClickableAtMinimumWidth(t *testing.T) {
	m := demoModel(140, 40)
	defer m.Close()
	for _, width := range []int{48, 60, 80, 140, 48} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 18})
		for i, name := range []string{"Jobs", "History", "Partitions", "Connection"} {
			nav := strings.Split(ansi.Strip(m.View().Content), "\n")[2]
			offset := strings.Index(nav, name)
			if offset < 0 {
				t.Fatalf("%d: tab %s clipped: %s", width, name, nav)
			}
			// Click the final character too: an apparently visible tab must be a
			// target across its full rendered label, including at the right edge.
			click(m, ansi.StringWidth(nav[:offset+len(name)])-1, 2)
			if m.tab != i {
				t.Fatalf("%d: %s click selected tab %d", width, name, m.tab)
			}
		}
	}
}

func TestPanelWrapAndScrollIndicatorSurviveResize(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	press(m, '3')
	if !strings.Contains(ansi.Strip(m.panel.GetContent()), "not\nmeasured utilization.") {
		t.Fatal("partition prose broke mid-word", m.panel.GetContent())
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "1–4 /") {
		t.Fatal("no initial scroll position")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if !m.panel.AtBottom() || m.panel.AtTop() {
		t.Fatal("end did not scroll")
	}
	bottom := strings.Split(ansi.Strip(m.View().Content), "\n")[9]
	if !strings.Contains(bottom, "↑") || strings.Contains(bottom, "↓") {
		t.Fatal(bottom)
	}
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 60})
	if !m.panel.AtTop() || !m.panel.AtBottom() {
		t.Fatal("larger viewport retained obsolete offset")
	}
	if strings.Contains(ansi.Strip(m.View().Content), "1–") {
		t.Fatal("indicator shown with no overflow")
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "128x24") {
		t.Fatal("resize lost top of panel")
	}
}

func TestConnectionLongUnbrokenErrorRemainsReachable(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.lastError = "Transport: " + strings.Repeat("abcde", 30) + ". Done."
	press(m, '4')
	text := ansi.Strip(m.panel.GetContent())
	for _, line := range strings.Split(text, "\n") {
		if ansi.StringWidth(line) > 40 {
			t.Fatal("long value overflow", line)
		}
	}
	if !strings.Contains(strings.ReplaceAll(text, "\n", ""), strings.Repeat("abcde", 30)) {
		t.Fatal("error text lost")
	}
}

func TestCompactFootersKeepRecoveryKeysVisible(t *testing.T) {
	for _, view := range []string{"details", "inspect", "logs", "script", "resources", "software", "waits", "receipts", "settings"} {
		m := demoModel(48, 18)
		m.view = view
		footer := ansi.Strip(m.footer())
		m.Close()
		if ansi.StringWidth(footer) > 44 {
			t.Fatalf("%s clipped footer: %s", view, footer)
		}
		if view == "details" {
			if !strings.Contains(footer, "q quit") {
				t.Fatal("list lost quit hint")
			}
		} else if !strings.Contains(footer, "esc back") {
			t.Fatalf("%s lost back hint: %s", view, footer)
		}
	}
	m := demoModel(48, 18)
	defer m.Close()
	press(m, '/')
	if footer := ansi.Strip(m.footer()); ansi.StringWidth(footer) > 44 || !strings.Contains(footer, "esc back") {
		t.Fatal(footer)
	}
	m.leaveInput()
	m.palette = true
	if footer := ansi.Strip(m.footer()); ansi.StringWidth(footer) > 44 || !strings.Contains(footer, "esc back") {
		t.Fatal(footer)
	}
}
