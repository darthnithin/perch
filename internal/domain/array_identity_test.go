package domain

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestOverlappingArrayMembersCannotOrderMissingSubmission(t *testing.T) {
	a := Job{ID: "42_[0-10]", ArrayID: 42, TaskRange: "0-10", State: "FAILED", Submit: time.Now()}
	b := a
	b.ID, b.TaskRange, b.State, b.Submit = "42_[5-15]", "5-15", "COMPLETED", time.Time{}
	for _, jobs := range [][]Job{{a, b}, {b, a}} {
		s := Arrays(jobs)[0]
		if s.SummaryError == "" || s.Known != 0 || s.FailedSpec() != "" {
			t.Fatalf("ordered incomplete attempt identity: %+v", s)
		}
	}
}

func TestMissingArraySubmissionDisjointStridesAndScopes(t *testing.T) {
	a := Job{ID: "42_[evens]", ArrayID: 42, TaskRange: "0-4000000000:2", State: "FAILED", Submit: time.Now(), User: "me", Cluster: "hb"}
	b := a
	b.ID, b.TaskRange, b.State, b.Submit = "42_[odds]", "1-3999999999:2", "COMPLETED", time.Time{}
	if s := Arrays([]Job{a, b})[0]; s.SummaryError != "" || s.Failed != 2000000001 || s.Completed != 2000000000 {
		t.Fatal("disjoint missing-time records require no ordering", s)
	}
	b.TaskRange = a.TaskRange
	b.Cluster = "other"
	if errors := ArraySubmissionConflicts([]Job{a, b}); len(errors) != 0 {
		t.Fatal("different scopes collided", errors)
	}
	b.Cluster, b.ArrayID = a.Cluster, 43
	if errors := ArraySubmissionConflicts([]Job{a, b}); len(errors) != 0 {
		t.Fatal("different arrays collided", errors)
	}
	b.ArrayID, b.Submit = a.ArrayID, a.Submit.Add(time.Hour)
	if errors := ArraySubmissionConflicts([]Job{a, b}); len(errors) != 0 {
		t.Fatal("known retries are ordered", errors)
	}
}

func TestArraySubmissionConflictsMatchIndependentMembership(t *testing.T) {
	rng := rand.New(rand.NewSource(62840))
	at := time.Now()
	for trial := 0; trial < 400; trial++ {
		var jobs []Job
		known, missing := map[int]bool{}, map[int]bool{}
		for i := 0; i < 7; i++ {
			first, step := rng.Intn(60), rng.Intn(6)+1
			last := first + rng.Intn(8)*step
			j := Job{ID: fmt.Sprint(i), ArrayID: 42, TaskRange: fmt.Sprintf("%d-%d:%d", first, last, step), Submit: at}
			members := known
			if rng.Intn(2) == 0 {
				j.Submit, members = time.Time{}, missing
			}
			if i%3 == 0 {
				task := int64(first)
				j.TaskID, j.TaskRange, last = &task, "", first
			}
			for n := first; n <= last; n += step {
				members[n] = true
			}
			jobs = append(jobs, j)
		}
		conflict := false
		for task := range known {
			conflict = conflict || missing[task]
		}
		rng.Shuffle(len(jobs), func(i, j int) { jobs[i], jobs[j] = jobs[j], jobs[i] })
		if got := ArraySubmissionConflicts(jobs); (got[42] != "") != conflict {
			t.Fatalf("trial %d: expected conflict %t, got %v", trial, conflict, got)
		}
	}
}
