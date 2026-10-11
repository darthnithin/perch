package app

import (
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestRetainedExpandedAccountingCannotFinishDisappearedRange(t *testing.T) {
	at := time.Now().UTC()
	q := pendingArray(at, "0-9")
	var history []domain.Job
	for i := int64(0); i < 10; i++ {
		history = append(history, arrayTask(q, i, "FAILED"))
	}
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: []domain.Job{q}, History: history})
	for n := 1; n <= 2; n++ {
		if events := s.Apply(hummingbird.Refresh{At: at.Add(time.Duration(n) * time.Minute), QueueOK: true}); len(events) != 0 {
			t.Fatal("retained accounting emitted outcomes", events)
		}
		a := s.Arrays()[0]
		if a.Known != 10 || a.Unresolved != 10 || a.Failed != 0 || a.FailedSpec() != "" {
			t.Fatal("retained task rows classified missing members", a)
		}
		if len(s.Checkpoint().Arrays) != 1 {
			t.Fatal("stale outcomes consumed the observation ledger")
		}
	}
	if events := s.Apply(hummingbird.Refresh{At: at.Add(3 * time.Minute), QueueOK: true, HistoryOK: true, History: history}); len(events) != 10 {
		t.Fatal("fresh accounting lost individual events", events)
	}
	if a := s.Arrays()[0]; a.Failed != 10 || a.Unresolved != 0 || a.FailedSpec() != "--array=0-9" {
		t.Fatal(a)
	}
}

func TestRetainedAccountingCannotFinishDisappearedJob(t *testing.T) {
	at := time.Now().UTC()
	q := domain.Job{ID: "42", User: "me", Cluster: "hb", Submit: at, State: "RUNNING"}
	f := q
	f.State = "FAILED"
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: []domain.Job{q}, History: []domain.Job{f}})
	if events := s.Apply(hummingbird.Refresh{At: at.Add(time.Minute), QueueOK: true}); len(events) != 0 || len(s.Snapshot.Jobs) != 1 || s.Snapshot.Jobs[0].State != "UNKNOWN" {
		t.Fatal("cached failure replaced current observation", events, s.Snapshot.Jobs)
	}
	if events := s.Apply(hummingbird.Refresh{At: at.Add(2 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{f}}); len(events) != 1 || events[0].Kind != "failed" {
		t.Fatal("fresh failure not reported", events)
	}
}

func TestUnconfirmedRangeStillYieldsToCurrentQueue(t *testing.T) {
	at := time.Now().UTC()
	q := pendingArray(at, "0-4000000000")
	f := q
	f.State = "FAILED"
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: []domain.Job{q}, History: []domain.Job{f}})
	q.ID, q.TaskRange = "42_[0-4000000000:2]", "0-4000000000:2"
	s.Apply(hummingbird.Refresh{At: at.Add(time.Minute), QueueOK: true, Queue: []domain.Job{q}})
	a := s.Arrays()[0]
	if a.Pending != 2000000001 || a.Unresolved != 2000000000 || a.Failed != 0 || a.FailedSpec() != "" {
		t.Fatal("queue or unresolved membership lost", a)
	}
}
