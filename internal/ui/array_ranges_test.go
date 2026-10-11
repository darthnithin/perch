package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestArraySelectionErrorIsVisibleAndCannotCopy(t *testing.T) {
	for _, width := range []int{140, 48} {
		m := New(nil, Options{ReducedMotion: true})
		m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
		old := domain.Job{ID: "42_[0-4000000000]", ArrayID: 42, TaskRange: "0-4000000000", State: "FAILED", Name: "complex-array"}
		newer := old
		newer.ID, newer.TaskRange, newer.Restarts, newer.State = "42_[0-4000000000:100000]", "0-4000000000:100000", 1, "PENDING"
		m.Update(refreshMsg{generation: m.generation, result: hummingbird.Refresh{At: m.now, QueueOK: true, HistoryOK: true, Queue: []domain.Job{newer}, History: []domain.Job{old}}})
		m.selectedID = newer.ID
		m.normalizeSelection()
		for _, view := range []string{"details", "inspect"} {
			if width < 100 && view == "details" {
				continue
			} // Compact Jobs shows its list; Enter opens details.
			m.view = view
			m.syncPanel()
			text := ansi.Strip(m.View().Content)
			if !strings.Contains(text, "Array summary unavailable") {
				t.Fatalf("width %d view %s hid error:\n%s", width, view, text)
			}
			if strings.Contains(text, "0 / 0") {
				t.Fatal("invented zero progress")
			}
		}
		if cmd := m.copyFailed(); cmd != nil || !strings.Contains(m.notice, "Cannot copy failed tasks") {
			t.Fatal("copied partial selection", m.notice)
		}
		m.Close()
	}
}
