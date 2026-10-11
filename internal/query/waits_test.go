package query

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestWaitStatisticsAndEligibility(t *testing.T) {
	at := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	var jobs []domain.Job
	for i, seconds := range []int{0, 60, 120, 180, 240, 300, 360, 420, 480, 540} {
		submit := at.Add(-time.Hour)
		task := int64(i)
		jobs = append(jobs, domain.Job{ID: fmt.Sprintf("42_%d", i), ArrayID: 42, TaskID: &task, Partition: "cpu", State: "COMPLETED", Submit: submit, Eligible: submit.Add(time.Duration(seconds/2) * time.Second), Start: submit.Add(time.Duration(seconds) * time.Second)})
	}
	// Missing and invalid eligibility don't discard valid submit-to-start observations.
	jobs[0].Eligible = time.Time{}
	jobs[1].Eligible = at
	r := Waits(jobs, at)
	if len(r.Partitions) != 1 || r.Excluded != 0 {
		t.Fatal(r)
	}
	p := r.Partitions[0]
	if p.ArrayTasks != 10 || p.SubmitToStart.Samples != 10 || p.SubmitToStart.MedianSeconds != 270 || p.SubmitToStart.P90Seconds != 480 {
		t.Fatal(p)
	}
	if p.EligibleToStart == nil || p.EligibleToStart.Samples != 8 || p.EligibleToStart.MedianSeconds != 165 || p.EligibleToStart.P90Seconds != 270 {
		t.Fatal(p.EligibleToStart)
	}
}
func TestWaitExclusions(t *testing.T) {
	at := time.Now()
	valid := domain.Job{ID: "42", Partition: "cpu", State: "COMPLETED", Submit: at.Add(-time.Hour), Eligible: at.Add(-30 * time.Minute), Start: at.Add(-time.Minute)}
	for name, change := range map[string]func(*domain.Job){
		"restart":          func(j *domain.Job) { j.Restarts = 1 },
		"future":           func(j *domain.Job) { j.Start = at.Add(time.Hour) },
		"old":              func(j *domain.Job) { j.Start = at.Add(-8 * 24 * time.Hour); j.Submit = j.Start.Add(-time.Hour) },
		"reversed":         func(j *domain.Job) { j.Submit = at },
		"unstarted":        func(j *domain.Job) { j.Start = time.Time{} },
		"missing submit":   func(j *domain.Job) { j.Submit = time.Time{} },
		"pending estimate": func(j *domain.Job) { j.State = "PENDING" },
		"parent":           func(j *domain.Job) { j.ArrayID = 42 },
		"range":            func(j *domain.Job) { j.TaskRange = "0-4294967293" },
		"step":             func(j *domain.Job) { j.ID = "42.batch" },
		"partition":        func(j *domain.Job) { j.Partition = "" },
	} {
		t.Run(name, func(t *testing.T) {
			j := valid
			change(&j)
			r := Waits([]domain.Job{j}, at)
			if len(r.Partitions) != 0 || r.Excluded != 1 {
				t.Fatal(r)
			}
		})
	}
	if r := Waits([]domain.Job{valid, valid}, at); len(r.Partitions) != 0 || r.Excluded != 2 {
		t.Fatal("duplicate", r)
	}
	if r := Waits(nil, at); r.Partitions == nil {
		t.Fatal("null list")
	}
}
func TestWaitsRequiresAccountingButNotQueue(t *testing.T) {
	b := &backend{refresh: hummingbird.Refresh{At: time.Now(), HistoryOK: true, Errors: map[string]error{"jobs": errors.New("queue unavailable")}}}
	if _, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "waits"}); err != nil {
		t.Fatal(err)
	}
	b.refresh.HistoryOK = false
	if _, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "waits"}); err == nil {
		t.Fatal("accounting failure became empty success")
	}
	if err := Validate(Request{Command: "waits", ID: "42"}); err == nil {
		t.Fatal("accepted extra ID")
	}
}

func TestWaitMissingEligibilityAndSanitization(t *testing.T) {
	at := time.Now()
	j := domain.Job{ID: "42", Partition: "cpu\x1b[31m", State: "COMPLETED", Submit: at.Add(-time.Hour), Start: at.Add(-time.Minute)}
	r := Waits([]domain.Job{j}, at)
	if len(r.Partitions) != 1 || r.Partitions[0].Partition != "cpu" || r.Partitions[0].EligibleToStart != nil {
		t.Fatal(r)
	}
	j.Eligible = j.Submit.Add(-time.Second)
	r = Waits([]domain.Job{j}, at)
	if r.Partitions[0].EligibleToStart != nil {
		t.Fatal("eligible before submit accepted")
	}
}
