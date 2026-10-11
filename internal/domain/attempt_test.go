package domain

import (
	"testing"
	"time"
)

func TestAttemptOrderRequiresScopeAndKnownTimes(t *testing.T) {
	old := Job{ID: "42", User: "me", Cluster: "hb", Submit: time.Now()}
	latest := old
	latest.Restarts++
	if !latest.AttemptAfter(old) || old.AttemptAfter(latest) || old.AttemptAfter(old) {
		t.Fatal("restart ordering")
	}
	latest = old
	latest.Submit = latest.Submit.Add(time.Hour)
	if !latest.AttemptAfter(old) {
		t.Fatal("reuse ordering")
	}
	for _, change := range []func(*Job){func(j *Job) { j.Cluster = "other" }, func(j *Job) { j.User = "other" }, func(j *Job) { j.ID = "43" }, func(j *Job) { j.Submit = time.Time{} }} {
		next := latest
		change(&next)
		if next.AttemptAfter(old) || old.AttemptAfter(next) {
			t.Fatal("incomparable attempts ordered")
		}
	}
}
