package app

import (
	"fmt"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/timewarning"
	"testing"
	"time"
)

func TestWarningCheckpointCompactsLargeArraysWithoutRepeating(t *testing.T) {
	p, _ := timewarning.Parse("30m,15m,10m,5m,1m")
	s := NewStore()
	s.SetTimeWarnings(p)
	now := time.Now()
	var queue []domain.Job
	for i := 0; i < 5000; i++ {
		queue = append(queue, domain.Job{ID: fmt.Sprint(i + 1), State: "RUNNING", Limit: time.Hour, Elapsed: 40 * time.Minute, Submit: now.Add(-time.Hour)})
	}
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: queue})
	for i := range queue {
		queue[i].Elapsed = 59 * time.Minute
	}
	if events := s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, Queue: queue}); len(events) != 5000 {
		t.Fatal(len(events))
	}
	c := s.Checkpoint()
	if len(c.Seen) != 5000 {
		t.Fatal("thresholds multiplied checkpoint storage", len(c.Seen))
	}
	r := NewStore()
	r.SetTimeWarnings(p)
	if !r.Restore(c, now.Add(2*time.Minute)) {
		t.Fatal("restore")
	}
	if events := r.Apply(hummingbird.Refresh{At: now.Add(2 * time.Minute), QueueOK: true, Queue: queue}); len(events) != 0 {
		t.Fatal("retained urgent threshold repeated earlier warnings", len(events))
	}
}

func TestWarningThresholdsCatchupDedupAndMeasurements(t *testing.T) {
	s := NewStore()
	p, _ := timewarning.Parse("15m,5m,1m")
	s.SetTimeWarnings(p)
	now := time.Now()
	j := domain.Job{ID: "17", State: "RUNNING", Limit: time.Hour, Submit: now.Add(-time.Hour)}
	apply := func(elapsed time.Duration, queueOK bool) []Event {
		j.Elapsed = elapsed
		now = now.Add(time.Minute)
		return s.Apply(hummingbird.Refresh{At: now, QueueOK: queueOK, Queue: []domain.Job{j}})
	}
	apply(40*time.Minute, true)
	e := apply(58*time.Minute, true)
	if len(e) != 1 || e[0].WarningThreshold != 5*time.Minute || e[0].Remaining != 2*time.Minute {
		t.Fatal("catch-up flood or wrong evidence", e)
	}
	if len(apply(58*time.Minute, true)) != 0 {
		t.Fatal("same measurement repeated")
	}
	if len(apply(59*time.Minute, false)) != 0 {
		t.Fatal("failed queue emitted warning")
	}
	e = apply(59*time.Minute, true)
	if len(e) != 1 || e[0].WarningThreshold != time.Minute {
		t.Fatal(e)
	}
	j.Restarts++
	if len(apply(59*time.Minute, true)) != 1 {
		t.Fatal("new attempt was deduped")
	}
	j.Limit = 2 * time.Hour
	if len(apply(119*time.Minute, true)) != 1 {
		t.Fatal("changed limit was deduped")
	}
	// Clock advances alone never change scheduler elapsed time.
	if len(apply(119*time.Minute, true)) != 0 {
		t.Fatal("local time invented a crossing")
	}
}

func TestChangingWarningPolicyBaselinesWithoutLosingOutcomes(t *testing.T) {
	now := time.Now()
	j := domain.Job{ID: "8", State: "RUNNING", Limit: time.Hour, Elapsed: 50 * time.Minute, Submit: now.Add(-time.Hour)}
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{j}})
	checkpoint := s.Checkpoint()
	p, _ := timewarning.Parse("15m,5m,1m")
	s.SetTimeWarnings(p)
	if len(s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, Queue: []domain.Job{j}})) != 0 {
		t.Fatal("editing replayed threshold")
	}
	// Restore under a changed policy baselines only warnings on the first
	// successful queue refresh; it must still report a confirmed outcome.
	r := NewStore()
	r.SetTimeWarnings(p)
	if !r.Restore(checkpoint, now.Add(time.Minute)) {
		t.Fatal("restore")
	}
	if len(r.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, Queue: []domain.Job{j}})) != 0 {
		t.Fatal("new preference replayed after restart")
	}
	j.Elapsed = 56 * time.Minute
	if e := r.Apply(hummingbird.Refresh{At: now.Add(2 * time.Minute), QueueOK: true, Queue: []domain.Job{j}}); len(e) != 1 || e[0].WarningThreshold != 5*time.Minute {
		t.Fatal(e)
	}
	r = NewStore()
	r.SetTimeWarnings(p)
	r.Restore(checkpoint, now.Add(time.Minute))
	j.State = "COMPLETED"
	if e := r.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{j}}); len(e) != 1 || e[0].Kind != "finished" {
		t.Fatal("policy suppressed outcome", e)
	}
}
