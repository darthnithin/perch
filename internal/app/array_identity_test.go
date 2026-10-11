package app

import (
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestArrayIdentityAmbiguitySurvivesDisplayFamilyFiltering(t *testing.T) {
	at := time.Now().UTC()
	q := pendingArray(at, "0-10")
	f := q
	f.ID, f.TaskRange, f.State, f.Submit = "42_[5-15]", "5-15", "FAILED", time.Time{}
	s := NewStore()
	s.Apply(hummingbird.Refresh{At: at, QueueOK: true, Queue: []domain.Job{q}})
	s.Apply(hummingbird.Refresh{At: at.Add(time.Minute), QueueOK: true, HistoryOK: true, Queue: []domain.Job{q}, History: []domain.Job{f}})
	a := s.Arrays()[0]
	if a.SummaryError == "" || a.Known != 0 || a.FailedSpec() != "" {
		t.Fatal("filter discarded evidence of ambiguous overlapping submission", a)
	}
	f.Submit = at
	s.Apply(hummingbird.Refresh{At: at.Add(2 * time.Minute), QueueOK: true, HistoryOK: true, Queue: []domain.Job{q}, History: []domain.Job{f}})
	a = s.Arrays()[0]
	if a.SummaryError != "" || a.Pending != 11 || a.Failed != 5 || a.FailedSpec() != "--array=11-15" {
		t.Fatal("fresh complete evidence did not recover", a)
	}
}
