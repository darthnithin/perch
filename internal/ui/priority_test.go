package ui

import (
	tea "charm.land/bubbletea/v2"
	"errors"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/demo"
	"strings"
	"testing"
)

func TestPriorityReportReturnScopeAndRefresh(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		for tab := 0; tab < 4; tab++ {
			m := New(nil, Options{Demo: true, ReducedMotion: true})
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.store.Apply(demo.Snapshot(m.now, 0))
			m.switchTab(tab)
			if tab < 2 {
				m.search = "parameter-sweep"
				m.normalizeSelection()
			}
			search, id := m.search, m.selectedID
			cmd := command(m, "/priority")
			if cmd == nil {
				t.Fatal("missing query")
			}
			m.Update(cmd())
			if !strings.Contains(m.viewport.GetContent(), "10010148") {
				t.Fatal("missing weighted score")
			}
			text := ansi.Strip(m.View().Content)
			if !strings.Contains(text, "Queue priority") || !strings.Contains(text, "10010148") {
				t.Fatal(size, text)
			}
			for _, action := range []string{"enter", "l", "s", "u", "shell", "cancel-job", "cancel-array"} {
				if m.jobActionProblem(action) == "" {
					t.Fatal("hidden job action enabled", action)
				}
			}
			cmd = m.action("r")
			if cmd == nil {
				t.Fatal("refresh unavailable")
			}
			m.Update(cmd())
			m.action("esc")
			if m.tab != tab || m.search != search || m.selectedID != id || m.view != "details" {
				t.Fatal("lost return context", tab, m.tab, m.search)
			}
			m.Close()
		}
	}
}
func TestPriorityLateReplyDoesNotReplaceNavigation(t *testing.T) {
	m := New(nil, Options{Demo: true, ReducedMotion: true})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	cmd := command(m, "/priority")
	m.action("3")
	m.Update(cmd())
	if m.tab != 2 || m.view != "details" || strings.Contains(m.viewport.GetContent(), "10010148") {
		t.Fatal("late report replaced destination")
	}
}

func TestPriorityFailureIsNotAnEmptySuccessfulReport(t *testing.T) {
	m := New(nil, Options{Demo: true, ReducedMotion: true})
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
	cmd := command(m, "/priority")
	msg := cmd().(detailMsg)
	msg.err = errors.New("priority service unavailable")
	m.Update(msg)
	text := ansi.Strip(m.viewport.GetContent())
	if !strings.Contains(text, "priority service unavailable") || strings.Contains(text, "No priority records") {
		t.Fatal("error became empty data", text)
	}
	cmd = m.action("r")
	m.Update(cmd())
	if m.detailError != "" || !strings.Contains(m.viewport.GetContent(), "10010148") {
		t.Fatal("retry did not recover")
	}
	m.action("l")
	if !strings.Contains(ansi.Strip(m.View().Content), "Choose a job in Jobs or History first.") {
		t.Fatal("compact action feedback was clipped")
	}
	m.Close()
}
