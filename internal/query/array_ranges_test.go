package query

import (
	"context"
	"strings"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestArraySelectionComplexityReturnsErrorNotZeroOrPartialData(t *testing.T) {
	// Subtracting this sparse newer attempt would need tens of thousands of
	// fragments. The CLI/MCP shared service must not return plausible zeros or
	// a truncated copy selection when the summary's work bound is exceeded.
	old := domain.Job{ID: "42_[0-4000000000]", ArrayID: 42, TaskRange: "0-4000000000", State: "FAILED"}
	newer := old
	newer.ID, newer.TaskRange, newer.Restarts, newer.State = "42_[0-4000000000:100000]", "0-4000000000:100000", 1, "PENDING"
	b := &backend{refresh: hummingbird.Refresh{At: time.Now(), QueueOK: true, HistoryOK: true, Queue: []domain.Job{newer}, History: []domain.Job{old}}}
	r, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "arrays"})
	if err == nil || !strings.Contains(err.Error(), "fragment limit") || r.Data != nil {
		t.Fatalf("partial data: %+v %v", r, err)
	}
}
