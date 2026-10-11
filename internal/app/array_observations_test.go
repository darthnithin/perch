package app

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func pendingArray(at time.Time, taskRange string) domain.Job {
	return domain.Job{ID: "42_[" + taskRange + "]", ArrayID: 42, TaskRange: taskRange, State: "PENDING", Submit: at, User: "me", Cluster: "hb"}
}
func arrayTask(base domain.Job, task int64, state string) domain.Job {
	base.ID = fmt.Sprintf("%d_%d", base.ArrayID, task)
	base.TaskID = &task
	base.TaskRange = ""
	base.State = state
	return base
}

func TestArrayOutcomesAfterMultipleEmptyPollsAndRestart(t *testing.T) {
	now := time.Now()
	pending := pendingArray(now, "0-99%4")
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	for minute := 1; minute <= 3; minute++ {
		if events := s.Apply(hummingbird.Refresh{At: now.Add(time.Duration(minute) * time.Minute), QueueOK: true, HistoryOK: minute == 2}); len(events) != 0 {
			t.Fatal("invented outcome", events)
		}
	}
	c := s.Checkpoint()
	if len(c.Arrays) != 1 || len(c.Arrays[0].Spans) != 1 {
		t.Fatal(c.Arrays)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var restored Checkpoint
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	s = NewStore()
	if !s.Restore(restored, now.Add(4*time.Minute)) {
		t.Fatal("restore failed")
	}
	history := []domain.Job{arrayTask(pending, 0, "COMPLETED"), arrayTask(pending, 1, "FAILED"), arrayTask(pending, 2, "CANCELLED")}
	r := hummingbird.Refresh{At: now.Add(5 * time.Minute), QueueOK: true, HistoryOK: true, History: history}
	events := s.Apply(r)
	if len(events) != 3 {
		t.Fatal("lost delayed array outcomes", events)
	}
	want := map[string]string{"42_0": "finished", "42_1": "failed", "42_2": "cancelled"}
	for _, event := range events {
		if event.Kind != want[event.JobID] {
			t.Fatal(event)
		}
	}
	c = s.Checkpoint()
	s = NewStore()
	s.Restore(c, now.Add(6*time.Minute))
	r.At = now.Add(7 * time.Minute)
	if events = s.Apply(r); len(events) != 0 {
		t.Fatal("repeated after restart", events)
	}
}

func TestArrayRangeMembershipNeverExpandsToUnobservedTasksOrAttempts(t *testing.T) {
	now := time.Now()
	pending := pendingArray(now, "0-20:2,31,40-50:5")
	for _, test := range []struct {
		name   string
		modify func(*domain.Job)
	}{
		{"odd", func(j *domain.Job) { *j = arrayTask(*j, 3, "COMPLETED") }},
		{"gap", func(j *domain.Job) { *j = arrayTask(*j, 32, "COMPLETED") }},
		{"user", func(j *domain.Job) { j.User = "other" }},
		{"cluster", func(j *domain.Job) { j.Cluster = "other" }},
		{"resubmit", func(j *domain.Job) { j.Submit = j.Submit.Add(time.Second) }},
		{"requeue", func(j *domain.Job) { j.Restarts++ }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := NewStore()
			s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
			s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true})
			j := arrayTask(pending, 4, "COMPLETED")
			test.modify(&j)
			if events := s.Apply(hummingbird.Refresh{At: now.Add(2 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{j}}); len(events) != 0 {
				t.Fatal(events)
			}
		})
	}
}

func TestObservationCompactionAndExpiry(t *testing.T) {
	now := time.Now()
	s := NewStore()
	pending := pendingArray(now, "0-4000000000")
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	for i := 1; i < 200; i++ {
		q := pendingArray(now, fmt.Sprintf("%d-4000000000", i))
		s.Apply(hummingbird.Refresh{At: now.Add(time.Duration(i) * time.Second), QueueOK: true, Queue: []domain.Job{q}})
	}
	if c := s.Checkpoint(); len(c.Arrays) != 1 || len(c.Arrays[0].Spans) != 1 {
		t.Fatal("shrinking range history expanded", c.Arrays)
	}
	at := now.Add(200*time.Second + Retention + time.Second)
	if e := s.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, History: []domain.Job{arrayTask(pending, 0, "COMPLETED")}}); len(e) != 0 {
		t.Fatal("expired membership notified", e)
	}
	if len(s.observedArrays) != 0 {
		t.Fatal("expired observations retained")
	}
}

func TestSpanUnionAgainstEnumeratedMembership(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	for trial := 0; trial < 100; trial++ {
		var spans []TaskSpan
		want := map[int64]bool{}
		for n := 0; n < 30; n++ {
			first := int64(random.Intn(100))
			step := int64(1 + random.Intn(10))
			last := first + int64(random.Intn(15))*step
			spans = unionSpan(spans, TaskSpan{first, last, step})
			for x := first; x <= last; x += step {
				want[x] = true
			}
		}
		for x := int64(0); x < 250; x++ {
			got := false
			for _, span := range spans {
				got = got || span.includes(x)
			}
			if got != want[x] {
				t.Fatalf("union changed membership at %d: %v", x, spans)
			}
		}
	}
}

func TestArrayObservationBoundsAndCheckpointValidation(t *testing.T) {
	now := time.Now()
	s := NewStore()
	for i := 0; i <= maxObservedArrays; i++ {
		j := pendingArray(now, "0-9")
		j.ArrayID = int64(i + 1)
		s.observeArray(j, now)
	}
	s.Snapshot.At = now
	if len(s.observedArrays) != maxObservedArrays || !s.ArrayTrackingLimited() {
		t.Fatal("array tracking unbounded or silently dropped")
	}
	c := s.Checkpoint()
	if !NewStore().Restore(c, now) {
		t.Fatal("valid bounded checkpoint rejected")
	}
	c.Arrays[0].Spans[0].Step = 0
	if NewStore().Restore(c, now) {
		t.Fatal("invalid span accepted")
	}
	s.expireArrayObservations(now.Add(Retention + time.Second))
	s.Snapshot.At = now.Add(Retention + time.Second)
	if s.ArrayTrackingLimited() || len(s.observedArrays) != 0 {
		t.Fatal("limit/observations did not expire")
	}
	for _, value := range []string{"0-100:0", "-1", "4294967294", strings.Repeat("1,", maxArraySpans) + "2"} {
		if _, err := parseSpans(value); err == nil {
			t.Fatal("accepted", value)
		}
	}
	spans, err := parseSpans("[0-9:2,20]%3")
	if err != nil || len(spans) != 2 || spans[0].Last != 8 {
		t.Fatal(spans, err)
	}
}

func TestLegacyCheckpointRangeAndCheckpointIsolation(t *testing.T) {
	now := time.Now()
	pending := pendingArray(now, "0-9")
	legacy := Checkpoint{At: now, Active: []Observation{{ID: pending.ID, Name: pending.Name, Cluster: pending.Cluster, User: pending.User, Submit: now, ArrayID: 42, TaskRange: "0-9"}}}
	s := NewStore()
	if !s.Restore(legacy, now) {
		t.Fatal("legacy restore")
	}
	c := s.Checkpoint()
	c.Arrays[0].Spans[0].Last = 0
	if !s.observedArrayTask(arrayTask(pending, 5, "COMPLETED")) {
		t.Fatal("checkpoint aliases store")
	}
	s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true})
	if events := s.Apply(hummingbird.Refresh{At: now.Add(2 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{arrayTask(pending, 5, "COMPLETED")}}); len(events) != 1 {
		t.Fatal(events)
	}
}

func TestObservedRequeuedRangeCanConfirmNewTaskAttempt(t *testing.T) {
	now := time.Now()
	pending := pendingArray(now, "0-9")
	old := arrayTask(pending, 3, "FAILED")
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, HistoryOK: true, History: []domain.Job{old}})
	pending.Restarts = 1
	s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, Queue: []domain.Job{pending}})
	// Retained old accounting is not covered by this different attempt's range.
	found := false
	for _, j := range s.Snapshot.Jobs {
		if j.ID == old.ID && j.Restarts == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("different attempt was silently covered by queue range")
	}
	arrays := s.Arrays()
	if len(arrays) != 1 || arrays[0].Known != 10 || arrays[0].Pending != 10 || arrays[0].Failed != 0 {
		t.Fatal("older attempt polluted current array progress", arrays)
	}
	s.Apply(hummingbird.Refresh{At: now.Add(2 * time.Minute), QueueOK: true})
	arrays = s.Arrays()
	if len(arrays) != 1 || arrays[0].Failed != 0 || arrays[0].Unresolved != 10 {
		t.Fatal("old failure resurfaced during accounting gap", arrays)
	}
	gaps := s.ArrayGaps()
	if len(gaps) != 1 || gaps[0].AwaitingTasks != 10 {
		t.Fatal("older attempt hid an accounting gap", gaps)
	}
	done := arrayTask(pending, 3, "COMPLETED")
	events := s.Apply(hummingbird.Refresh{At: now.Add(3 * time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{done}})
	if len(events) != 1 || events[0].Kind != "finished" {
		t.Fatal("new observed attempt lost", events)
	}
}

func TestResolvedArrayTasksCannotNotifyAfterDedupCheckpointEviction(t *testing.T) {
	now := time.Now()
	pending := pendingArray(now, "0-10004")
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	var history []domain.Job
	for i := int64(0); i < 10005; i++ {
		history = append(history, arrayTask(pending, i, "COMPLETED"))
	}
	r := hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: history}
	if events := s.Apply(r); len(events) != len(history) {
		t.Fatal(len(events))
	}
	c := s.Checkpoint()
	if len(c.Seen) != 10000 || len(c.Arrays) != 0 {
		t.Fatal("resolved membership retained", len(c.Seen), len(c.Arrays))
	}
	s = NewStore()
	if !s.Restore(c, now.Add(2*time.Minute)) {
		t.Fatal("restore")
	}
	r.At = now.Add(3 * time.Minute)
	if events := s.Apply(r); len(events) != 0 {
		t.Fatal("evicted dedup key replayed", len(events))
	}
}

func TestOnlyUnresolvedStrideMembersRemainEligible(t *testing.T) {
	now := time.Now()
	pending := pendingArray(now, "0-20:2,3-21:3")
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	var history []domain.Job
	for _, id := range []int64{0, 6, 10, 20, 21} {
		history = append(history, arrayTask(pending, id, "COMPLETED"))
	}
	s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: history})
	c := s.Checkpoint()
	c.Seen = nil
	s = NewStore()
	s.Restore(c, now.Add(2*time.Minute))
	for id := int64(0); id <= 22; id++ {
		want := (id%2 == 0 && id <= 20) || (id >= 3 && id%3 == 0 && id <= 21)
		if id == 0 || id == 6 || id == 10 || id == 20 || id == 21 {
			want = false
		}
		if got := s.observedArrayTask(arrayTask(pending, id, "COMPLETED")); got != want {
			t.Fatal("incorrect unresolved membership", id, got, want)
		}
	}
}

func TestHighlyFragmentedOutcomesBoundMemoryAndReportCoverageLimit(t *testing.T) {
	now := time.Now()
	pending := pendingArray(now, "0-9999")
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	var history []domain.Job
	for i := int64(1); i < 3000; i += 2 {
		history = append(history, arrayTask(pending, i, "COMPLETED"))
	}
	events := s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: history})
	if len(events) != len(history) {
		t.Fatal("current confirmed outcomes lost", len(events))
	}
	if !s.ArrayTrackingLimited() || len(s.observedArrays) != 0 {
		t.Fatal("unbounded fragments or silent loss")
	}
}
