package app

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestMixedRequeueRetainsSiblingAndTotal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := pendingArray(now, "0-1")
	zero := arrayTask(original, 0, "RUNNING")
	zero.RawID = 43
	one := arrayTask(original, 1, "COMPLETED")
	one.RawID = 42
	retry := zero
	retry.Submit = now.Add(time.Minute)
	retry.Restarts = 1
	retry.State = "PENDING"
	old := zero
	old.State = "PENDING" // Slurm retains this duplicate after requeue.
	for _, reverse := range []bool{false, true} {
		retry.State = "PENDING"
		s := NewStore()
		s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{original}})
		s.Apply(hummingbird.Refresh{At: now.Add(time.Second), QueueOK: true, Queue: []domain.Job{zero, pendingArray(now, "1")}, HistoryOK: true, History: []domain.Job{original}})
		if a := s.Arrays()[0]; a.Known != 2 || a.Running != 1 || a.Pending != 1 {
			t.Fatalf("stale accounting range duplicated current queue: %+v", a)
		}
		sibling := pendingArray(now, "1")
		queue := []domain.Job{sibling, retry}
		if reverse {
			queue[0], queue[1] = queue[1], queue[0]
		}
		s.Apply(hummingbird.Refresh{At: retry.Submit, QueueOK: true, Queue: queue, HistoryOK: true, History: []domain.Job{old, retry}})
		if a := s.Arrays()[0]; a.Known != 2 || a.Pending != 2 {
			t.Fatalf("requeue reset total: %+v", a)
		}
		// Restore with the mixed submission identities preserved.
		raw, _ := json.Marshal(s.Checkpoint())
		var checkpoint Checkpoint
		json.Unmarshal(raw, &checkpoint)
		s = NewStore()
		if !s.Restore(checkpoint, retry.Submit.Add(time.Second)) {
			t.Fatal("restore failed")
		}
		s.Apply(hummingbird.Refresh{At: retry.Submit.Add(time.Second), QueueOK: true, Queue: []domain.Job{retry}, HistoryOK: true, History: []domain.Job{old, one, retry}})
		if a := s.Arrays()[0]; a.Known != 2 || a.Completed != 1 || a.Pending != 1 || a.Unresolved != 0 {
			t.Fatalf("sibling hidden after requeue: %+v", a)
		}
		retry.State = "COMPLETED"
		s.Apply(hummingbird.Refresh{At: retry.Submit.Add(time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{old, one, retry}})
		if a := s.Arrays()[0]; a.Known != 2 || a.Completed != 2 || a.Unresolved != 0 {
			t.Fatalf("wrong final outcomes: %+v", a)
		}
		if len(s.ArrayGaps()) != 0 || len(s.arrayDisplays) != 0 {
			t.Fatal("completed array retained unresolved evidence")
		}
	}
	// Joining after the requeue uses duplicate accounting records to find origin.
	retry.State = "PENDING"
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: retry.Submit, QueueOK: true, Queue: []domain.Job{retry}, HistoryOK: true, History: []domain.Job{old, one, retry}})
	if a := s.Arrays()[0]; a.Known != 2 || a.Pending != 1 || a.Completed != 1 {
		t.Fatalf("cold start lost sibling: %+v", a)
	}
}

func TestAccountingBitmapOnlyBecomesUnresolvedCoverage(t *testing.T) {
	now := time.Now().UTC()
	s := NewStore()
	pending := pendingArray(now, "0-9")
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, HistoryOK: true, History: []domain.Job{pending}})
	if len(s.Snapshot.Jobs) != 0 || len(s.Events) != 0 {
		t.Fatal("stale accounting bitmap became a scheduler job")
	}
	if a := s.Arrays()[0]; a.Known != 10 || a.Unresolved != 10 {
		t.Fatal(a)
	}
	if gaps := s.ArrayGaps(); len(gaps) != 1 || gaps[0].AwaitingTasks != 10 {
		t.Fatal(gaps)
	}
}

func TestRequeueLinkRequiresMatchingRawMemberAndScope(t *testing.T) {
	now := time.Now().UTC()
	original := pendingArray(now, "0-9")
	old := arrayTask(original, 0, "RUNNING")
	old.RawID = 43
	for _, mutate := range []func(*domain.Job){
		func(j *domain.Job) { j.RawID = 99 }, func(j *domain.Job) { j.User = "other" }, func(j *domain.Job) { j.Restarts = 0 },
	} {
		next := old
		next.Submit = now.Add(time.Minute)
		next.Restarts = 1
		mutate(&next)
		s := NewStore()
		s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{original, old}})
		s.Apply(hummingbird.Refresh{At: next.Submit, QueueOK: true, Queue: []domain.Job{next}, HistoryOK: true, History: []domain.Job{old}})
		if d := s.arrayDisplays[next.ArrayID]; len(d.Retries) != 0 || !d.Submit.Equal(next.Submit) {
			t.Fatalf("unrelated member linked as retry: %+v", d)
		}
	}
}

// Final records captured from synthetic array 802126, projected and anonymized.
// Intermediate queue rows below are reconstructed, not recorded live JSON.
func TestScrubbedMixedRequeueAccounting(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/hummingbird/mixed-requeue.json")
	if err != nil {
		t.Fatal(err)
	}
	history, err := hummingbird.ParseAccounting(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatal(history)
	}
	var old, retry, sibling domain.Job
	for _, j := range history {
		if j.TaskID != nil && *j.TaskID == 1 {
			sibling = j
		} else if j.Restarts == 1 {
			retry = j
		} else {
			old = j
		}
	}
	if retry.Submit.Equal(sibling.Submit) || !old.Submit.Equal(sibling.Submit) {
		t.Fatal("fixture lost mixed submission times")
	}
	original := old
	original.TaskID = nil
	original.ID = "9500_[0-1]"
	original.TaskRange = "0-1"
	original.State = "PENDING"
	for _, reversed := range []bool{false, true} {
		s := NewStore()
		s.Apply(hummingbird.Refresh{At: old.Submit, QueueOK: true, Queue: []domain.Job{original}})
		queueRetry := retry
		queueRetry.State = "PENDING"
		if reversed {
			history[0], history[2] = history[2], history[0]
		}
		s.Apply(hummingbird.Refresh{At: retry.Submit.Add(time.Minute), QueueOK: true, Queue: []domain.Job{queueRetry}, HistoryOK: true, History: history})
		if a := s.Arrays()[0]; a.Known != 2 || a.Pending != 1 || a.Completed != 1 {
			t.Fatalf("live shape lost sibling: %+v", a)
		}
		s.Apply(hummingbird.Refresh{At: retry.End.Add(time.Minute), QueueOK: true, HistoryOK: true, History: history})
		if a := s.Arrays()[0]; a.Known != 2 || a.Completed != 2 || a.Unresolved != 0 {
			t.Fatal(a)
		}
	}
}

func TestArrayRetryCheckpointBounds(t *testing.T) {
	now := time.Now().UTC()
	base := ArrayDisplay{ID: 42, User: "me", Cluster: "hb", Submit: now.Add(-time.Hour), ObservedAt: now, Minimum: 2}
	for _, retries := range [][]ArrayRetry{
		{{Submit: now, Restarts: 0}}, {{Submit: now.Add(time.Second), Restarts: 1}},
		{{Submit: base.Submit, Restarts: 1}}, {{Submit: now, Restarts: 1}, {Submit: now, Restarts: 1}},
		make([]ArrayRetry, maxArrayRetries+1),
	} {
		d := base
		d.Retries = retries
		if NewStore().Restore(Checkpoint{At: now, Displays: []ArrayDisplay{d}}, now) {
			t.Fatalf("accepted invalid retry identities: %+v", retries)
		}
	}
}
