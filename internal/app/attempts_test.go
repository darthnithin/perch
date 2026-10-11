package app

import (
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"testing"
	"time"
)

func TestNewestAccountingAttemptIndependentOfOrder(t *testing.T) {
	now := time.Now()
	old := domain.Job{ID: "42", User: "me", Cluster: "hb", State: "FAILED", Submit: now.Add(-time.Hour)}
	for _, advance := range []func(*domain.Job){func(j *domain.Job) { j.Submit = now }, func(j *domain.Job) { j.Restarts = 1 }} {
		latest := old
		advance(&latest)
		latest.State = "COMPLETED"
		for _, history := range [][]domain.Job{{old, latest}, {latest, old}} {
			s := NewStore()
			s.Apply(hummingbird.Refresh{At: now, QueueOK: true, HistoryOK: true, History: history})
			if len(s.Snapshot.Jobs) != 1 || s.Snapshot.Jobs[0].AttemptKey() != latest.AttemptKey() || s.Snapshot.Jobs[0].State != "COMPLETED" {
				t.Fatal("selected older history", s.Snapshot.Jobs)
			}
		}
	}
}
func TestOldAccountingCannotReplaceObservedNewAttempt(t *testing.T) {
	now := time.Now()
	old := domain.Job{ID: "42", User: "me", Cluster: "hb", State: "COMPLETED", Submit: now.Add(-time.Hour)}
	for _, advance := range []func(*domain.Job){func(j *domain.Job) { j.Submit = now }, func(j *domain.Job) { j.Restarts = 1 }} {
		active := old
		advance(&active)
		active.State = "RUNNING"
		s := NewStore()
		s.Apply(hummingbird.Refresh{At: now, QueueOK: true, HistoryOK: true, Queue: []domain.Job{active}, History: []domain.Job{old}})
		for i := 1; i <= 3; i++ {
			events := s.Apply(hummingbird.Refresh{At: now.Add(time.Duration(i) * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{old}})
			if len(events) != 0 || len(s.Snapshot.Jobs) != 1 || s.Snapshot.Jobs[0].AttemptKey() != active.AttemptKey() || s.Snapshot.Jobs[0].State != "UNKNOWN" {
				t.Fatal("old outcome replaced current attempt", s.Snapshot.Jobs, events)
			}
		}
		active.State = "COMPLETED"
		events := s.Apply(hummingbird.Refresh{At: now.Add(4 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{active, old}})
		if len(events) != 1 || events[0].Key != active.AttemptKey()+"/outcome" {
			t.Fatal("new attempt completion lost", events)
		}
	}
}

func TestConfirmedNewAttemptDoesNotRollBackButCanAgeOut(t *testing.T) {
	now := time.Now()
	old := domain.Job{ID: "42", Submit: now, State: "FAILED"}
	latest := old
	latest.Restarts = 1
	latest.State = "COMPLETED"
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, HistoryOK: true, History: []domain.Job{latest}})
	s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{old}})
	if got := s.Snapshot.Jobs[0]; got.Restarts != 1 || got.State != "COMPLETED" {
		t.Fatal("old attempt resurrected", got)
	}
	latest.State = "RUNNING"
	s.Apply(hummingbird.Refresh{At: now.Add(2 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{latest}})
	if s.Snapshot.Jobs[0].State != "COMPLETED" {
		t.Fatal("terminal confirmation regressed")
	}
	s.Apply(hummingbird.Refresh{At: now.Add(8 * 24 * time.Hour), QueueOK: true, HistoryOK: true})
	if len(s.Snapshot.Jobs) != 0 {
		t.Fatal("terminal history never expired")
	}
}
