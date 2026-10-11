package app

import (
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestCurrentQueueRangeWinsOverAccountingRange(t *testing.T) {
	at := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	queued := domain.Job{ID: "42_[0-9]", ArrayID: 42, TaskRange: "0-9", User: "me", Cluster: "hb", State: "PENDING", Submit: at}
	failed := queued
	failed.ID, failed.TaskRange, failed.State = "42_[0-10]", "0-10", "FAILED"
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: []domain.Job{queued}, History: []domain.Job{failed}})
	a := s.Arrays()
	if len(a) != 1 || a[0].Known != 11 || a[0].Pending != 10 || a[0].Failed != 1 || a[0].FailedSpec() != "--array=10" {
		t.Fatalf("current tasks became failures: %+v", a)
	}
	// A failed queue read retains its earlier, explicitly stale snapshot.
	s.Apply(hummingbird.Refresh{At: at.Add(time.Minute), HistoryOK: true, History: []domain.Job{failed}})
	if a := s.Arrays()[0]; a.Pending != 10 || a.FailedSpec() != "--array=10" {
		t.Fatalf("queue failure changed membership: %+v", a)
	}
	// A successful empty queue releases precedence to confirmed accounting.
	s.Apply(hummingbird.Refresh{At: at.Add(2 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{failed}})
	if a := s.Arrays()[0]; a.Pending != 0 || a.Failed != 11 || a.FailedSpec() != "--array=0-10" {
		t.Fatalf("old queue precedence survived a fresh empty queue: %+v", a)
	}
}

func TestSteppedQueueRangeWinsWithoutExpandingTasks(t *testing.T) {
	at := time.Now()
	queued := domain.Job{ID: "42_[0-4000000000:2]", ArrayID: 42, TaskRange: "0-4000000000:2", State: "PENDING", Submit: at}
	failed := queued
	failed.ID, failed.TaskRange, failed.State = "42_[0-4000000000]", "0-4000000000", "FAILED"
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: []domain.Job{queued}, History: []domain.Job{failed}})
	a := s.Arrays()[0]
	if a.Known != 4000000001 || a.Pending != 2000000001 || a.Failed != 2000000000 || a.FailedSpec() != "--array=1-3999999999:2" {
		t.Fatalf("stepped current membership lost: %+v", a)
	}
}
