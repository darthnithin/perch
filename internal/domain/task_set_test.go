package domain

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

func enumerateSpans(spans []taskProgression) map[int64]bool {
	out := map[int64]bool{}
	for _, p := range spans {
		for n := p.first; n <= p.last; n += p.step {
			if out[n] {
				panic("overlapping output spans")
			}
			out[n] = true
		}
	}
	return out
}

func TestTaskSubtractionMatchesEnumeratedOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(9091))
	for trial := 0; trial < 5000; trial++ {
		progression := func() taskProgression {
			first, step := int64(rng.Intn(150)), int64(rng.Intn(30)+1)
			return taskProgression{first, first + int64(rng.Intn(30))*step, step}
		}
		a, b := progression(), progression()
		want := enumerateSpans([]taskProgression{a})
		for n := range enumerateSpans([]taskProgression{b}) {
			delete(want, n)
		}
		got, err := subtractProgression(a, b)
		if err != nil || !reflect.DeepEqual(enumerateSpans(got), want) {
			t.Fatalf("%+v minus %+v: %+v %v", a, b, got, err)
		}
	}
}

func TestTaskSetUnionPreservesExactDisjointSelection(t *testing.T) {
	rng := rand.New(rand.NewSource(8925))
	for trial := 0; trial < 500; trial++ {
		s := newTaskSet()
		want := map[int64]bool{}
		for n := 0; n < 20; n++ {
			first, step := int64(rng.Intn(200)), int64(1+rng.Intn(15))
			p := taskProgression{first, first + int64(rng.Intn(20))*step, step}
			if _, err := s.add([]taskProgression{p}); err != nil {
				t.Fatal(err)
			}
			for task := range enumerateSpans([]taskProgression{p}) {
				want[task] = true
			}
		}
		got := enumerateSpans(s.spans)
		expr := formatTaskSelection(s.spans)
		parsed, err := parseTaskProgressions(expr)
		if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(enumerateSpans(parsed), want) {
			t.Fatalf("union or serialized selection differs: %s %v", expr, err)
		}
	}
}

func TestCollapsedFailedSelectionExcludesOtherStatesAndRetries(t *testing.T) {
	at := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	base := Job{ID: "42_[0-10]", ArrayID: 42, TaskRange: "0-10", State: "FAILED", Submit: at}
	jobs := []Job{base, base}
	for _, tc := range []struct {
		id       int64
		state    string
		restarts int
	}{{3, "FAILED", 0}, {3, "RUNNING", 1}, {4, "CANCELLED", 0}, {5, "COMPLETED", 0}, {7, "FAILED", 0}} {
		j := base
		j.ID, j.TaskID, j.TaskRange, j.State, j.Restarts = fmt.Sprintf("42_%d", tc.id), &tc.id, "", tc.state, tc.restarts
		jobs = append(jobs, j)
	}
	pending := base
	pending.ID, pending.TaskRange, pending.State, pending.Restarts = "42_[8-9]", "8-9", "PENDING", 1
	jobs = append(jobs, pending)
	before := append([]Job(nil), jobs...)
	for _, reverse := range []bool{false, true} {
		if reverse {
			for i, k := 0, len(jobs)-1; i < k; i, k = i+1, k-1 {
				jobs[i], jobs[k] = jobs[k], jobs[i]
			}
		}
		a := Arrays(jobs)[0]
		if a.SummaryError != "" || a.Known != 11 || a.Failed != 6 || a.Running != 1 || a.Cancelled != 1 || a.Completed != 1 || a.Pending != 2 || a.FailedSpec() != "--array=0-2,6,7,10" || !reflect.DeepEqual(a.FailedTasks, []int64{7}) {
			t.Fatalf("inexact range/retry selection: %+v spec %s", a, a.FailedSpec())
		}
		if !reverse && !reflect.DeepEqual(jobs, before) {
			t.Fatal("mutated caller's records")
		}
	}
}

func TestLargeFailedSelectionStaysCompact(t *testing.T) {
	base := Job{ID: "42_[0-4000000000]", ArrayID: 42, TaskRange: "0-4000000000", State: "FAILED"}
	pending := base
	pending.ID, pending.TaskRange, pending.State, pending.Restarts = "42_[0-4000000000:2]", "0-4000000000:2", "PENDING", 1
	a := Arrays([]Job{base, pending})[0]
	if a.Known != 4000000001 || a.Pending != 2000000001 || a.Failed != 2000000000 || a.FailedSpec() != "--array=1-3999999999:2" {
		t.Fatalf("%+v %s", a, a.FailedSpec())
	}
	id := int64(42)
	running := Job{ID: "42_42", ArrayID: 42, TaskID: &id, State: "RUNNING", Restarts: 1}
	a = Arrays([]Job{base, running})[0]
	if a.FailedSpec() != "--array=0-41,43-4000000000" || a.Failed != 4000000000 {
		t.Fatal(a)
	}
}

func TestSelectionBoundsNeverReturnPartialCopy(t *testing.T) {
	s := newTaskSet()
	s.work = 0
	if _, err := s.add([]taskProgression{{0, 10, 1}, {5, 15, 1}}); err == nil {
		t.Fatal("work bound ignored")
	}
	if _, err := subtractProgression(taskProgression{0, 4000000000, 1}, taskProgression{0, 4000000000, 100000}); err == nil {
		t.Fatal("fragment bound ignored")
	}
	var parts []string
	for i := 0; i <= maxRangeTerms; i++ {
		parts = append(parts, fmt.Sprint(i*2))
	}
	a := Arrays([]Job{{ID: "42_[many]", ArrayID: 42, TaskRange: strings.Join(parts, ","), State: "FAILED"}})[0]
	if a.SummaryError == "" || a.FailedSpec() != "" || a.Known != 0 {
		t.Fatalf("partial summary leaked: %+v", a)
	}
}

func BenchmarkCollapsedFailureSummary(b *testing.B) {
	jobs := []Job{{ID: "42_[0-4000000000]", ArrayID: 42, TaskRange: "0-4000000000", State: "FAILED"}, {ID: "42_[0-4000000000:2]", ArrayID: 42, TaskRange: "0-4000000000:2", State: "PENDING", Restarts: 1}}
	for b.Loop() {
		if a := Arrays(jobs)[0]; a.Failed != 2000000000 {
			b.Fatal(a)
		}
	}
}

func TestManyIndividualFailuresDoNotRequireRangeArithmetic(t *testing.T) {
	var jobs []Job
	for i := int64(0); i < 20000; i++ {
		jobs = append(jobs, Job{ID: fmt.Sprintf("42_%d", i), ArrayID: 42, TaskID: &i, State: "FAILED"})
	}
	a := Arrays(jobs)[0]
	if a.SummaryError != "" || a.Failed != 20000 || a.FailedSpec() != "--array=0-19999" {
		t.Fatal(a)
	}
}

func TestArrayOutcomesMatchIndependentPerTaskOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(1804))
	states := []string{"FAILED", "COMPLETED", "CANCELLED", "RUNNING", "PENDING", "UNKNOWN"}
	for trial := 0; trial < 300; trial++ {
		var jobs []Job
		type outcome struct {
			attempt int
			state   string
		}
		want := map[int64]outcome{}
		for attempt := 0; attempt < 20; attempt++ {
			first, step := int64(rng.Intn(100)), int64(1+rng.Intn(8))
			last := first + int64(rng.Intn(15))*step
			state := states[rng.Intn(len(states))]
			j := Job{ID: fmt.Sprintf("42_[%d-%d:%d]/%d", first, last, step, attempt), ArrayID: 42, TaskRange: fmt.Sprintf("%d-%d:%d", first, last, step), State: state, Restarts: attempt}
			if first == last {
				j.TaskID = &first
				j.TaskRange = ""
			}
			jobs = append(jobs, j)
			for task := first; task <= last; task += step {
				want[task] = outcome{attempt, state}
			}
		}
		jobs = append(jobs, jobs[3])
		rng.Shuffle(len(jobs), func(i, k int) { jobs[i], jobs[k] = jobs[k], jobs[i] })
		a := Arrays(jobs)[0]
		counts := map[string]int64{}
		failures := map[int64]bool{}
		for id, v := range want {
			counts[v.state]++
			if v.state == "FAILED" {
				failures[id] = true
			}
		}
		if a.SummaryError != "" || a.Known != int64(len(want)) || a.Failed != counts["FAILED"] || a.Completed != counts["COMPLETED"] || a.Cancelled != counts["CANCELLED"] || a.Running != counts["RUNNING"] || a.Pending != counts["PENDING"] || a.Unresolved != counts["UNKNOWN"] {
			t.Fatalf("trial %d: %+v want %+v", trial, a, counts)
		}
		got := map[int64]bool{}
		if a.FailedSpec() != "" {
			spans, err := parseTaskProgressions(strings.TrimPrefix(a.FailedSpec(), "--array="))
			if err != nil {
				t.Fatal(err)
			}
			got = enumerateSpans(spans)
		}
		if !reflect.DeepEqual(got, failures) {
			t.Fatalf("trial %d copied wrong tasks: %s", trial, a.FailedSpec())
		}
	}
}
