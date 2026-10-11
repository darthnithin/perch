package ui

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func largeArrayModel(size int) *Model {
	m := New(nil, Options{Demo: true})
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	at := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	var queue, history []domain.Job
	for i := 0; i < size; i++ {
		task := int64(i)
		j := domain.Job{ID: fmt.Sprintf("9000_%d", i), RawID: int64(9001 + i), ArrayID: 9000, TaskID: &task, Name: "parameter-sweep", User: "researcher", Cluster: "hb", Submit: at, State: "COMPLETED", Partition: "128x24", CPUs: 1}
		if i < 4 {
			j.State = "RUNNING"
			queue = append(queue, j)
		} else {
			history = append(history, j)
		}
	}
	m.now = at
	m.Update(refreshMsg{generation: m.generation, result: hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: queue, History: history}})
	return m
}

// Includes the complete animation update and view, not only string rendering.
// Fixture construction and initial refresh happen outside the timed loop.
func BenchmarkLargeArrayAnimation(b *testing.B) {
	for _, size := range []int{1000, 5000} {
		for _, tab := range []int{0, 1} {
			b.Run(fmt.Sprintf("%d/tab%d", size, tab), func(b *testing.B) {
				m := largeArrayModel(size)
				defer m.Close()
				m.tab = tab
				m.normalizeSelection()
				for b.Loop() {
					m.Update(frameMsg(m.now))
					if len(m.View().Content) == 0 {
						b.Fatal("empty view")
					}
				}
			})
		}
	}
}
