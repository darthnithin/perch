package app

import (
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"testing"
	"time"
)

func TestAccountingLagNeverInventsSuccess(t *testing.T) {
	s := NewStore()
	now := time.Now()
	j := domain.Job{ID: "1", RawID: 1, State: "RUNNING", Submit: now.Add(-time.Hour)}
	s.Apply(hummingbird.Refresh{QueueOK: true, Queue: []domain.Job{j}, At: now})
	events := s.Apply(hummingbird.Refresh{QueueOK: true, At: now.Add(time.Minute)})
	if len(events) != 0 || len(s.Snapshot.Jobs) != 1 || s.Snapshot.Jobs[0].State != "UNKNOWN" {
		t.Fatal(s.Snapshot, events)
	}
	j.State = "COMPLETED"
	events = s.Apply(hummingbird.Refresh{QueueOK: true, HistoryOK: true, History: []domain.Job{j}, At: now.Add(2 * time.Minute)})
	if len(events) != 1 || events[0].Kind != "finished" || events[0].Attempt != j.AttemptKey() || !events[0].At.Equal(now.Add(2*time.Minute)) {
		t.Fatal(events)
	}
	if e := s.Apply(hummingbird.Refresh{QueueOK: true, At: now.Add(3 * time.Minute)}); len(e) != 0 {
		t.Fatal("duplicate completion", e)
	}
}
func TestStartupAndRequeueDoNotNotifyOldOutcomes(t *testing.T) {
	s := NewStore()
	now := time.Now()
	j := domain.Job{ID: "1", State: "COMPLETED", Submit: now.Add(-time.Hour)}
	if e := s.Apply(hummingbird.Refresh{QueueOK: true, HistoryOK: true, History: []domain.Job{j}, At: now}); len(e) != 0 {
		t.Fatal(e)
	}
	j.State = "RUNNING"
	j.Restarts = 1
	s.Apply(hummingbird.Refresh{QueueOK: true, Queue: []domain.Job{j}, At: now})
	j.State = "FAILED"
	if e := s.Apply(hummingbird.Refresh{QueueOK: true, HistoryOK: true, History: []domain.Job{j}, At: now}); len(e) != 1 {
		t.Fatal(e)
	}
}
func TestArrayRangeDoesNotDoubleCountAccountingOrParents(t *testing.T) {
	s := NewStore()
	task := int64(3)
	history := []domain.Job{{ID: "42_3", ArrayID: 42, TaskID: &task, State: "RUNNING"}, {ID: "42", ArrayID: 42, State: "PENDING"}}
	s.Apply(hummingbird.Refresh{QueueOK: true, HistoryOK: true, Queue: []domain.Job{{ID: "42_[0-99]", ArrayID: 42, TaskRange: "0-99", State: "PENDING"}}, History: history, At: time.Now()})
	a := s.Arrays()
	if len(a) != 1 || a[0].Known != 100 || a[0].Pending != 100 {
		t.Fatal(a)
	}
	s.Apply(hummingbird.Refresh{QueueOK: true, HistoryOK: true, Queue: []domain.Job{{ID: "42_3", ArrayID: 42, TaskID: &task, State: "RUNNING"}}, At: time.Now()})
	a = s.Arrays()
	if a[0].Known != 100 || a[0].Running != 1 || a[0].Unresolved != 99 {
		t.Fatal(a)
	}
}
func TestFailedQueuePreservesSnapshot(t *testing.T) {
	s := NewStore()
	s.Apply(hummingbird.Refresh{QueueOK: true, Queue: []domain.Job{{ID: "1", State: "RUNNING"}}, At: time.Now()})
	s.Apply(hummingbird.Refresh{HistoryOK: true})
	if len(s.Snapshot.Jobs) != 1 || s.Snapshot.Jobs[0].State != "RUNNING" {
		t.Fatal(s.Snapshot)
	}
}
