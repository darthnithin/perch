package app

import (
	"fmt"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestRangeOutcomesCountOnlyObservedConfirmedMembers(t *testing.T) {
	at := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	original := domain.Job{ID: "42_[0-10]", ArrayID: 42, TaskRange: "0-10", Cluster: "hb", User: "me", Submit: at, State: "PENDING"}
	failed := original
	failed.State = "FAILED"
	remaining := original
	remaining.ID = "42_[0-9]"
	remaining.TaskRange = "0-9"
	for _, restart := range []bool{false, true} {
		s := NewStore()
		s.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: []domain.Job{original}})
		e := s.Apply(hummingbird.Refresh{At: at.Add(time.Minute), QueueOK: true, HistoryOK: true, Queue: []domain.Job{remaining}, History: []domain.Job{failed}})
		if len(e) != 1 || e[0].TaskCount != 1 || e[0].Kind != "failed" {
			t.Fatalf("alerted for unconfirmed members: %+v", e)
		}
		firstKey := e[0].Key
		if restart {
			c := s.Checkpoint()
			s = NewStore()
			if !s.Restore(c, at.Add(90*time.Second)) {
				t.Fatal("checkpoint rejected")
			}
		}
		e = s.Apply(hummingbird.Refresh{At: at.Add(2 * time.Minute), QueueOK: true, HistoryOK: true, Queue: []domain.Job{remaining}, History: []domain.Job{failed}})
		if len(e) != 0 {
			t.Fatal("partial outcome repeated", e)
		}
		e = s.Apply(hummingbird.Refresh{At: at.Add(150 * time.Second), QueueOK: true, HistoryOK: false})
		if len(e) != 0 {
			t.Fatal("old accounting confirmed newly missing tasks", e)
		}
		if a := s.Arrays()[0]; a.Unresolved != 10 || a.Failed != 1 || a.FailedSpec() != "--array=10" {
			t.Fatalf("retained accounting resolved newly missing tasks: %+v", a)
		}
		e = s.Apply(hummingbird.Refresh{At: at.Add(3 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{failed}})
		if len(e) != 1 || e[0].TaskCount != 10 || e[0].Key == firstKey {
			t.Fatalf("remaining outcomes lost or repeated: %+v", e)
		}
		// Expanding the same confirmed range into concrete accounting tasks
		// cannot produce another set of notifications, even after restart.
		c := s.Checkpoint()
		s = NewStore()
		if !s.Restore(c, at.Add(4*time.Minute)) {
			t.Fatal("restore")
		}
		var history []domain.Job
		for i := int64(0); i <= 10; i++ {
			j := failed
			j.ID = domainID(i)
			j.TaskRange = ""
			j.TaskID = &i
			history = append(history, j)
		}
		if e = s.Apply(hummingbird.Refresh{At: at.Add(5 * time.Minute), QueueOK: true, HistoryOK: true, History: history}); len(e) != 0 {
			t.Fatal("expanded outcomes duplicated range", e)
		}
	}
}

func domainID(i int64) string { return fmt.Sprintf("42_%d", i) }

func TestContradictoryRangesDoNotGenerateOutcomeEvents(t *testing.T) {
	at := time.Now()
	j := domain.Job{ID: "42_[0-10]", ArrayID: 42, TaskRange: "0-10", State: "PENDING", Cluster: "hb", User: "me", Submit: at}
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: []domain.Job{j}})
	f := j
	f.State = "FAILED"
	c := j
	c.ID = "42_[5-15]"
	c.TaskRange = "5-15"
	c.State = "COMPLETED"
	if e := s.Apply(hummingbird.Refresh{At: at.Add(time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{f, c}}); len(e) != 0 {
		t.Fatal("ambiguous outcome alerted", e)
	}
	if e := s.Apply(hummingbird.Refresh{At: at.Add(2 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{f}}); len(e) != 1 || e[0].TaskCount != 11 {
		t.Fatal("consistent recovery lost observed outcomes", e)
	}
}

func TestBillionTaskRangeAlertsStayCompact(t *testing.T) {
	at := time.Now()
	j := domain.Job{ID: "42_[0-4000000000]", ArrayID: 42, TaskRange: "0-4000000000", State: "PENDING", Cluster: "hb", User: "me", Submit: at}
	f := j
	f.State = "COMPLETED"
	q := j
	q.ID = "42_[0-4000000000:2]"
	q.TaskRange = "0-4000000000:2"
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: []domain.Job{j}})
	e := s.Apply(hummingbird.Refresh{At: at.Add(time.Minute), QueueOK: true, HistoryOK: true, Queue: []domain.Job{q}, History: []domain.Job{f}})
	if len(e) != 1 || e[0].TaskCount != 2000000000 {
		t.Fatal(e)
	}
	c := s.Checkpoint()
	if len(c.Arrays) != 1 || len(c.Arrays[0].Spans) != 1 || c.Arrays[0].Spans[0].Step != 2 {
		t.Fatal("range expanded or wrong members retained", c.Arrays)
	}
	e = s.Apply(hummingbird.Refresh{At: at.Add(2 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{f}})
	if len(e) != 1 || e[0].TaskCount != 2000000001 || len(s.Checkpoint().Arrays) != 0 {
		t.Fatal("remaining membership lost", e)
	}
}
