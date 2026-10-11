package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
	"time"
)

func TestWaitHistoryNavigation(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		m.input.SetValue("/waits")
		m.submitInput()
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		if m.view != "waits" || !strings.Contains(m.panel.GetContent(), "128x24") || !strings.Contains(m.panel.GetContent(), "Eligible") {
			t.Fatal(m.panel.GetContent())
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("overflow", size, line)
			}
		}
		if size[0] == 48 {
			wheel(m, 5, m.layout().bodyTop+1, tea.MouseWheelDown)
			if m.panel.AtTop() {
				t.Fatal("mouse wheel failed")
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
		if !m.panel.AtBottom() {
			t.Fatal("end failed")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.view != "details" {
			t.Fatal("escape", m.view)
		}
		m.showWaits()
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.store.Snapshot.HistoryAt = time.Time{}
		m.syncPanel()
		if !strings.Contains(m.panel.GetContent(), "not available yet") {
			t.Fatal("missing data shown as zero")
		}
		m.Close()
	}
}

func TestWaitHistoryRefreshAndUnavailableEligibility(t *testing.T) {
	m := demoModel(100, 32)
	defer m.Close()
	m.showWaits()
	m.syncPanel()
	old := m.waitsAt
	m.store.History = nil
	m.store.Snapshot.HistoryAt = old.Add(time.Minute)
	m.syncPanel()
	if !strings.Contains(m.panel.GetContent(), "No usable started jobs") {
		t.Fatal("stale analysis retained")
	}
	m.now = m.store.Snapshot.HistoryAt.Add(3 * time.Minute)
	m.syncPanel()
	if !strings.Contains(m.panel.GetContent(), "Older snapshot") {
		t.Fatal("old snapshot not labelled")
	}
}
