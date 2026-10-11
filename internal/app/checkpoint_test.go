package app

import (
	"encoding/json"
	"fmt"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"strings"
	"testing"
	"time"
)

func TestCheckpointPrivacyBoundsAndExpiry(t *testing.T) {
	now := time.Now()
	s := NewStore()
	s.Snapshot.At = now
	s.Snapshot.Jobs = []domain.Job{{ID: "1", State: "RUNNING", Name: "test", Stdout: "private", WorkDir: "private", Steps: []domain.Step{{ID: "secret"}}}}
	for i := 0; i < 11000; i++ {
		s.Seen[fmt.Sprint(i)] = now.Add(-time.Duration(i) * time.Second)
	}
	c := s.Checkpoint()
	raw, _ := json.Marshal(c)
	if len(c.Seen) != 10000 || strings.Contains(string(raw), "private") || strings.Contains(string(raw), "secret") {
		t.Fatal("unbounded or excess metadata")
	}
	for _, at := range []time.Time{now.Add(-time.Minute), now.Add(Retention + time.Second)} {
		restored := NewStore()
		if restored.Restore(c, at) {
			t.Fatal("future/expired checkpoint accepted")
		}
	}
}

func TestShortArrayTasksFinishBetweenPolls(t *testing.T) {
	now := time.Now()
	s := NewStore()
	pending := domain.Job{ID: "42_[1-100]", ArrayID: 42, TaskRange: "1-100", State: "PENDING", Submit: now, User: "me", Cluster: "hb"}
	s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
	task := int64(5)
	finished := pending
	finished.ID = "42_5"
	finished.TaskID = &task
	finished.TaskRange = ""
	finished.State = "COMPLETED"
	pending.TaskRange = "6-100"
	events := s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, Queue: []domain.Job{pending}, History: []domain.Job{finished}})
	if len(events) != 1 || events[0].JobID != "42_5" {
		t.Fatal(events)
	}
	// Job ID reuse and requeues are different attempts, not catch-up outcomes.
	for _, change := range []func(*domain.Job){func(j *domain.Job) { j.Restarts++ }, func(j *domain.Job) { j.Submit = now.Add(time.Hour) }, func(j *domain.Job) { j.User = "other" }} {
		s = NewStore()
		pending.TaskRange = "1-100"
		s.Apply(hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{pending}})
		different := finished
		change(&different)
		if e := s.Apply(hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{different}}); len(e) != 0 {
			t.Fatal("different attempt", e)
		}
	}
}

func TestTimeWarningSurvivesRestartAndChangedLimit(t *testing.T) {
	now := time.Now()
	s := NewStore()
	j := domain.Job{ID: "1", State: "RUNNING", Submit: now.Add(-time.Hour), Limit: 10 * time.Minute, Elapsed: 6 * time.Minute}
	r := hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{j}}
	if e := s.Apply(r); len(e) != 0 {
		t.Fatal("startup flood", e)
	}
	restored := NewStore()
	restored.Restore(s.Checkpoint(), now.Add(time.Minute))
	r.At = now.Add(time.Minute)
	if e := restored.Apply(r); len(e) != 0 {
		t.Fatal("repeat warning after restart", e)
	}
	j.Limit = 20 * time.Minute
	j.Elapsed = 16 * time.Minute
	r.Queue = []domain.Job{j}
	r.At = now.Add(2 * time.Minute)
	if e := restored.Apply(r); len(e) != 1 || e[0].Kind != "time warning" {
		t.Fatal("changed limit", e)
	}
	j.Limit = 0
	r.Queue = []domain.Job{j}
	r.At = now.Add(3 * time.Minute)
	if e := restored.Apply(r); len(e) != 0 {
		t.Fatal("unset limit warns", e)
	}
}
