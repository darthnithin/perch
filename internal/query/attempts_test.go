package query

import (
	"context"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"testing"
	"time"
)

func TestJobSelectsNewestAccountingAttempt(t *testing.T) {
	old := domain.Job{ID: "42", User: "me", Cluster: "hb", Submit: time.Now(), State: "FAILED", Name: "old"}
	latest := old
	latest.Restarts = 1
	latest.State = "COMPLETED"
	latest.Name = "latest"
	for _, history := range [][]domain.Job{{old, latest}, {latest, old}} {
		b := &backend{refresh: hummingbird.Refresh{At: time.Now(), QueueOK: true, HistoryOK: true, History: history}}
		r, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "job", ID: "42"})
		if err != nil || r.Data.(Job).Name != "latest" {
			t.Fatal("wrong attempt", r, err)
		}
	}
}

func TestArrayQueryUsesCurrentAttemptSummary(t *testing.T) {
	now := time.Now()
	task := int64(3)
	old := domain.Job{ID: "42_3", ArrayID: 42, TaskID: &task, User: "me", Cluster: "hb", Submit: now, State: "FAILED"}
	pending := domain.Job{ID: "42_[0-9]", ArrayID: 42, TaskRange: "0-9", User: "me", Cluster: "hb", Submit: now, Restarts: 1, State: "PENDING"}
	b := &backend{refresh: hummingbird.Refresh{At: now, QueueOK: true, HistoryOK: true, Queue: []domain.Job{pending}, History: []domain.Job{old}}}
	r, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "arrays"})
	if err != nil {
		t.Fatal(err)
	}
	arrays := r.Data.([]Array)
	if len(arrays) != 1 || arrays[0].Observed != 10 || arrays[0].Pending != 10 || arrays[0].Failed != 0 || arrays[0].FailedSpec != "" {
		t.Fatal(arrays)
	}
}
