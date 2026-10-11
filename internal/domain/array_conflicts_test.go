package domain

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestConflictingRangeOutcomesNeverChooseByLabel(t *testing.T) {
	a := Job{ID: "42_[0-10]", ArrayID: 42, TaskRange: "0-10", State: "FAILED"}
	b := Job{ID: "42_[5-15]", ArrayID: 42, TaskRange: "5-15", State: "COMPLETED"}
	for _, jobs := range [][]Job{{a, b}, {b, a}} {
		s := Arrays(jobs)[0]
		if s.SummaryError == "" || s.Known != 0 || s.FailedSpec() != "" {
			t.Fatalf("invented overlapping outcomes: %+v", s)
		}
	}
	b.Restarts = 1
	if s := Arrays([]Job{a, b})[0]; s.SummaryError != "" || s.Completed != 11 || s.Failed != 5 {
		t.Fatalf("different attempts were treated as a conflict: %+v", s)
	}
}

func TestDisjointStrideOutcomesRemainExact(t *testing.T) {
	a := Job{ID: "42_[evens]", ArrayID: 42, TaskRange: "0-4000000000:2", State: "FAILED"}
	b := Job{ID: "42_[odds]", ArrayID: 42, TaskRange: "1-3999999999:2", State: "COMPLETED"}
	s := Arrays([]Job{a, b})[0]
	if s.SummaryError != "" || s.Known != 4000000001 || s.Failed != 2000000001 || s.Completed != 2000000000 {
		t.Fatal(s)
	}
}

func TestRangeConflictsMatchEnumeratedTaskEvidence(t *testing.T) {
	rng := rand.New(rand.NewSource(71021))
	states := []string{"FAILED", "COMPLETED", "CANCELLED"}
	for trial := 0; trial < 300; trial++ {
		var jobs []Job
		members := map[int]string{}
		conflict := false
		for i := 0; i < 6; i++ {
			first, step := rng.Intn(60), rng.Intn(5)+1
			last := first + rng.Intn(10)*step
			state := states[rng.Intn(len(states))]
			expr := fmt.Sprintf("%d-%d:%d", first, last, step)
			jobs = append(jobs, Job{ID: "42_[" + expr + "]", ArrayID: 42, TaskRange: expr, State: state})
			for task := first; task <= last; task += step {
				if previous, ok := members[task]; ok && previous != state {
					conflict = true
				}
				members[task] = state
			}
		}
		rng.Shuffle(len(jobs), func(i, j int) { jobs[i], jobs[j] = jobs[j], jobs[i] })
		a := Arrays(jobs)[0]
		if (a.SummaryError != "") != conflict {
			t.Fatalf("trial %d: conflict=%t, %+v", trial, conflict, a)
		}
		if conflict {
			if a.Known != 0 || a.FailedSpec() != "" {
				t.Fatal("partial ambiguous summary escaped", a)
			}
			continue
		}
		counts := map[string]int64{}
		for _, state := range members {
			counts[state]++
		}
		if a.Known != int64(len(members)) || a.Failed != counts["FAILED"] || a.Completed != counts["COMPLETED"] || a.Cancelled != counts["CANCELLED"] {
			t.Fatalf("trial %d: %+v, want %+v", trial, a, counts)
		}
	}
}
