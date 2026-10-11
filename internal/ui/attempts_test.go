package ui

import (
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"strings"
	"testing"
	"time"
)

func TestAttemptChangeInvalidatesInFlightDetails(t *testing.T) {
	for _, view := range []string{"logs", "script", "resources"} {
		t.Run(view, func(t *testing.T) {
			m := demoModel(120, 36)
			defer m.Close()
			selected, _ := m.selected()
			old := m.setView(view)()
			m.logRaw = "old attempt output"
			m.viewport.SetContent("old attempt output")
			m.live = []hummingbird.LiveSample{{At: time.Now()}}
			oldSequence := m.detailSequence
			selected.Restarts++
			m.Update(refreshMsg{generation: m.generation, result: hummingbird.Refresh{At: m.now, QueueOK: true, Queue: []domain.Job{selected}}})
			if m.detailSequence == oldSequence || m.logRaw != "" || len(m.live) > 0 {
				t.Fatal("attempt change retained details")
			}
			m.Update(old)
			if strings.Contains(m.viewport.GetContent(), "Loading experiment") || strings.Contains(m.viewport.GetContent(), "#!/bin/bash") || len(m.live) > 0 {
				t.Fatal("late result from previous attempt accepted")
			}
		})
	}
}
