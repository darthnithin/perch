package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestMissingArrayRemainsNavigableWithoutPretendTaskActions(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := New(nil, Options{ReducedMotion: true})
		defer m.Close()
		now := time.Now()
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		j := domain.Job{ID: "42_[0-9]", ArrayID: 42, TaskRange: "0-9", Name: "sweep", State: "PENDING", Submit: now}
		m.Update(refreshMsg{result: hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{j}}})
		m.Update(refreshMsg{result: hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true}})
		if jobs := m.jobs(); len(jobs) != 1 || jobs[0].AwaitingTasks != 10 {
			t.Fatal("array vanished from Jobs", jobs)
		}
		m.input.SetValue("/job 42")
		m.submitInput()
		m.syncPanel()
		if m.view != "inspect" || m.selectedID != "42 [?]" {
			t.Fatal("cannot jump to missing array", m.view, m.selectedID)
		}
		for _, view := range []string{"logs", "script", "resources"} {
			if cmd := m.setView(view); cmd != nil || m.view != "inspect" {
				t.Fatal("synthetic row dispatched a task action", view)
			}
		}
		m.syncPanel()
		content := ansi.Strip(m.View().Content)
		if !strings.Contains(content, "Awaiting accounting") || !strings.Contains(content, "At least 10 tasks") || strings.Contains(content, "CPUs") {
			t.Fatal("missing gap explanation or invented resources", content)
		}
		for _, line := range strings.Split(content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("overflow", line)
			}
		}
		m.switchTab(1)
		if len(m.jobs()) != 0 {
			t.Fatal("gap presented as historical outcome")
		}
	}
}
