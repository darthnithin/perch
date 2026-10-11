package app

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestArrayGapSurvivesEmptyPollsRestartAndPartialAccounting(t *testing.T) {
	now := time.Now().UTC()
	pending := pendingArray(now, "0-9")
	pending.Name = "sweep"
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	for minute := 1; minute <= 3; minute++ {
		s.Apply(hummingbird.Refresh{At: now.Add(time.Duration(minute) * time.Minute), QueueOK: true, HistoryOK: true})
		gaps := s.ArrayGaps()
		if len(gaps) != 1 || gaps[0].AwaitingTasks != 10 || gaps[0].Name != "sweep" || gaps[0].TaskID != nil || gaps[0].TaskRange != "" || gaps[0].RawID != 0 {
			t.Fatalf("invented task identity or lost gap: %+v", gaps)
		}
		if len(s.Snapshot.Jobs) != 0 || len(s.Events) != 0 || len(s.Seen) != 0 {
			t.Fatal("presentation gap leaked into real jobs or notifications")
		}
		if a := s.Arrays(); len(a) != 1 || a[0].Known != 10 || a[0].Unresolved != 10 {
			t.Fatal(a)
		}
	}
	raw, _ := json.Marshal(s.Checkpoint())
	var checkpoint Checkpoint
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		t.Fatal(err)
	}
	s = NewStore()
	if !s.Restore(checkpoint, now.Add(4*time.Minute)) || len(s.ArrayGaps()) != 1 {
		t.Fatal("lost display after restart")
	}
	queue := []domain.Job{pendingArray(now, "5-9")}
	history := []domain.Job{arrayTask(pending, 0, "COMPLETED"), arrayTask(pending, 1, "FAILED")}
	s.Apply(hummingbird.Refresh{At: now.Add(5 * time.Minute), QueueOK: true, HistoryOK: true, Queue: queue, History: history})
	if gaps := s.ArrayGaps(); len(gaps) != 1 || gaps[0].AwaitingTasks != 3 {
		t.Fatal("queue/history double counted", gaps)
	}
	if a := s.Arrays()[0]; a.Known != 10 || a.Pending != 5 || a.Completed != 1 || a.Failed != 1 || a.Unresolved != 3 {
		t.Fatal(a)
	}
	// A queue failure must not change the gap based on partial history alone.
	s.Apply(hummingbird.Refresh{At: now.Add(6 * time.Minute), HistoryOK: true})
	if s.ArrayGaps()[0].AwaitingTasks != 3 {
		t.Fatal("partial failure changed displayed evidence")
	}
	history = nil
	for i := int64(0); i < 10; i++ {
		history = append(history, arrayTask(pending, i, "COMPLETED"))
	}
	s.Apply(hummingbird.Refresh{At: now.Add(7 * time.Minute), QueueOK: true, HistoryOK: true, History: history})
	if len(s.ArrayGaps()) != 0 || len(s.arrayDisplays) != 0 {
		t.Fatal("fully confirmed array still awaiting")
	}
	s.Apply(hummingbird.Refresh{At: now.Add(8 * time.Minute), QueueOK: true, HistoryOK: true})
	if len(s.Arrays()) != 0 || len(s.ArrayGaps()) != 0 {
		t.Fatal("expired history resurrected a completed array")
	}
}

func TestArrayGapBoundsExpiryAndIDReuse(t *testing.T) {
	now := time.Now().UTC()
	s := NewStore()
	pending := pendingArray(now, "0-4000000000")
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true})
	if gap := s.ArrayGaps()[0]; gap.AwaitingTasks != 4000000001 {
		t.Fatal(gap)
	}
	if raw, _ := json.Marshal(s.Checkpoint()); len(raw) > 2048 {
		t.Fatalf("large array allocated task records: %d bytes", len(raw))
	}
	reused := pendingArray(now.Add(time.Hour), "0-2")
	s.Apply(hummingbird.Refresh{At: now.Add(time.Hour), QueueOK: true, HistoryOK: true, Queue: []domain.Job{reused}, History: []domain.Job{arrayTask(pending, 100, "COMPLETED")}})
	if len(s.ArrayGaps()) != 0 || s.Arrays()[0].Known != 3 {
		t.Fatal("new submission inherited old total", s.Arrays(), s.ArrayGaps())
	}
	s.Apply(hummingbird.Refresh{At: now.Add(time.Hour + time.Minute), QueueOK: true, HistoryOK: true})
	if s.ArrayGaps()[0].AwaitingTasks != 3 {
		t.Fatal("new submission did not retain its own count")
	}
	s.Apply(hummingbird.Refresh{At: now.Add(time.Hour + Retention + time.Second), QueueOK: true, HistoryOK: true})
	if len(s.ArrayGaps()) != 0 || len(s.Arrays()) != 0 {
		t.Fatal("unbounded retention")
	}
}

func TestArrayGapCheckpointValidation(t *testing.T) {
	now := time.Now()
	c := Checkpoint{At: now, Displays: []ArrayDisplay{{ID: 42, Minimum: 10, ObservedAt: now}}}
	for _, modify := range []func(*Checkpoint){
		func(c *Checkpoint) { c.Displays[0].Minimum = -1 },
		func(c *Checkpoint) { c.Displays[0].ObservedAt = now.Add(time.Second) },
		func(c *Checkpoint) { c.Displays = append(c.Displays, c.Displays[0]) },
		func(c *Checkpoint) { c.Displays = make([]ArrayDisplay, maxObservedArrays+1) },
	} {
		bad := c
		bad.Displays = append([]ArrayDisplay(nil), c.Displays...)
		modify(&bad)
		if NewStore().Restore(bad, now.Add(time.Minute)) {
			t.Fatal("invalid display checkpoint accepted", bad)
		}
	}
}

func TestArrayParentDoesNotConfirmAnIndividualMissingTask(t *testing.T) {
	now := time.Now()
	pending := pendingArray(now, "0-9")
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	parent := pending
	parent.ID, parent.TaskRange, parent.State = "42", "", "COMPLETED"
	s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{parent}})
	if a := s.Arrays(); len(a) != 1 || a[0].Known != 10 || a[0].Completed != 0 || a[0].Unresolved != 10 {
		t.Fatal("parent invented task outcome", a)
	}
	if gaps := s.ArrayGaps(); len(gaps) != 1 || gaps[0].AwaitingTasks != 10 || gaps[0].Count() != 0 {
		t.Fatal(gaps)
	}
}

func TestArrayDisplayCapacityIsBoundedAndReported(t *testing.T) {
	now := time.Now()
	s := NewStore()
	var queue []domain.Job
	for i := int64(1); i <= maxObservedArrays+1; i++ {
		j := pendingArray(now, "0-9")
		j.ArrayID, j.ID = i, fmt.Sprintf("%d_[0-9]", i)
		queue = append(queue, j)
	}
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: queue})
	if !s.ArrayDisplayLimited() || len(s.arrayDisplays) != maxObservedArrays || len(s.Snapshot.Jobs) != maxObservedArrays+1 {
		t.Fatal("limit silent, unbounded, or hid current queue records")
	}
	restored := NewStore()
	if !restored.Restore(s.Checkpoint(), now.Add(time.Minute)) || !restored.ArrayDisplayLimited() {
		t.Fatal("coverage warning lost on restart")
	}
	restored.Apply(hummingbird.Refresh{At: now.Add(Retention + time.Minute), QueueOK: true, HistoryOK: true})
	if restored.ArrayDisplayLimited() || len(restored.ArrayGaps()) != 0 {
		t.Fatal("display limit warning or observations did not expire")
	}
}

func TestOverlappingCollapsedRangeDoesNotLeavePhantomTasks(t *testing.T) {
	now := time.Now().UTC()
	pending := pendingArray(now, "0-9,5-14,0-12:2")
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	if a := s.Arrays()[0]; a.Known != 15 || a.Pending != 15 {
		t.Fatalf("duplicate queued tasks: %+v", a)
	}
	s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true})
	if gaps := s.ArrayGaps(); len(gaps) != 1 || gaps[0].AwaitingTasks != 15 {
		t.Fatal("inflated missing-task total", gaps)
	}
	checkpoint := s.Checkpoint()
	s = NewStore()
	if !s.Restore(checkpoint, now.Add(2*time.Minute)) {
		t.Fatal("could not restore observed range")
	}
	var finished []domain.Job
	for i := int64(0); i < 15; i++ {
		finished = append(finished, arrayTask(pending, i, "COMPLETED"))
	}
	s.Apply(hummingbird.Refresh{At: now.Add(3 * time.Minute), QueueOK: true, HistoryOK: true, History: finished})
	if a := s.Arrays()[0]; a.Known != 15 || a.Completed != 15 || a.Unresolved != 0 {
		t.Fatalf("phantom tasks after completion: %+v", a)
	}
	if len(s.ArrayGaps()) != 0 {
		t.Fatal("duplicate indices prevented completion")
	}
}
