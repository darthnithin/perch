package notify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"perch/internal/app"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"strings"
	"testing"
	"time"
)

type recorder struct {
	messages []Message
	err      error
}

func (r *recorder) Send(_ context.Context, m Message) error {
	r.messages = append(r.messages, m)
	return r.err
}

var testScope = Scope{Host: "user@cluster", User: "user", Cluster: "cluster"}

func testJob(now time.Time) domain.Job {
	return domain.Job{ID: "21", Name: "training", User: "user", Cluster: "cluster", State: "RUNNING", Submit: now.Add(-time.Hour)}
}
func refresh(now time.Time, j domain.Job) hummingbird.Refresh {
	r := hummingbird.Refresh{At: now, QueueOK: true, HistoryOK: true}
	if j.Terminal() {
		r.History = []domain.Job{j}
	} else {
		r.Queue = []domain.Job{j}
	}
	return r
}

func TestRestartCatchUpAndNoRepeatedDelivery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	now := time.Now()
	j := testJob(now)
	sender := &recorder{}
	first, _, err := Open(dir, testScope, sender)
	if err != nil {
		t.Fatal(err)
	}
	store := app.NewStore()
	events := store.Apply(refresh(now, j))
	if err = first.Process(context.Background(), store.Checkpoint(), events); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 0 {
		t.Fatal("startup notified")
	}
	first.Close()
	second, checkpoint, err := Open(dir, testScope, sender)
	if err != nil {
		t.Fatal(err)
	}
	store = app.NewStore()
	if !store.Restore(checkpoint, now.Add(time.Minute)) {
		t.Fatal("not restored")
	}
	if !store.Snapshot.At.IsZero() || store.Snapshot.Jobs[0].State != "UNKNOWN" {
		t.Fatal("cache masquerades as live")
	}
	j.State = "FAILED"
	events = store.Apply(refresh(now.Add(time.Minute), j))
	if len(events) != 1 {
		t.Fatal(events)
	}
	if err = second.Process(context.Background(), store.Checkpoint(), events); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0].Body, "FAILED") {
		t.Fatal(sender.messages)
	}
	info, _ := os.Stat(second.path)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	second.Close()
	third, checkpoint, err := Open(dir, testScope, sender)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	store = app.NewStore()
	store.Restore(checkpoint, now.Add(2*time.Minute))
	events = store.Apply(refresh(now.Add(2*time.Minute), j))
	if err = third.Process(context.Background(), store.Checkpoint(), events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 || len(sender.messages) != 1 {
		t.Fatal("repeated notification", events, sender.messages)
	}
}

func TestDelayedCollapsedArrayOutcomesSurviveDiskAndGroupOnce(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	now := time.Now()
	pending := domain.Job{ID: "42_[0-99]", ArrayID: 42, TaskRange: "0-99", State: "PENDING", Name: "sweep", User: "user", Cluster: "cluster", Submit: now}
	sender := &recorder{}
	session, _, err := Open(dir, testScope, sender)
	if err != nil {
		t.Fatal(err)
	}
	store := app.NewStore()
	store.Apply(refresh(now, pending))
	for i := 1; i <= 3; i++ {
		events := store.Apply(hummingbird.Refresh{At: now.Add(time.Duration(i) * time.Minute), QueueOK: true, HistoryOK: true})
		if err = session.Process(context.Background(), store.Checkpoint(), events); err != nil {
			t.Fatal(err)
		}
	}
	if len(sender.messages) != 0 {
		t.Fatal("gap generated notification")
	}
	session.Close()
	session, checkpoint, err := Open(dir, testScope, sender)
	if err != nil {
		t.Fatal(err)
	}
	store = app.NewStore()
	if !store.Restore(checkpoint, now.Add(4*time.Minute)) {
		t.Fatal("restore")
	}
	var history []domain.Job
	for _, task := range []int64{1, 2, 3} {
		j := pending
		j.ID = fmt.Sprintf("42_%d", task)
		j.TaskID = &task
		j.TaskRange = ""
		j.State = "COMPLETED"
		history = append(history, j)
	}
	r := hummingbird.Refresh{At: now.Add(5 * time.Minute), QueueOK: true, HistoryOK: true, History: history}
	events := store.Apply(r)
	if len(events) != 3 {
		t.Fatal(events)
	}
	if err = session.Process(context.Background(), store.Checkpoint(), events); err != nil {
		t.Fatal(err)
	}
	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0].Body, "3 tasks finished") {
		t.Fatal(sender.messages)
	}
	session.Close()
	session, checkpoint, err = Open(dir, testScope, sender)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	store = app.NewStore()
	store.Restore(checkpoint, now.Add(6*time.Minute))
	r.At = now.Add(7 * time.Minute)
	events = store.Apply(r)
	if err = session.Process(context.Background(), store.Checkpoint(), events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 || len(sender.messages) != 1 {
		t.Fatal("duplicate catch-up", events, sender.messages)
	}
}

func TestLockIdentityAndUnsafeState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s, _, err := Open(dir, testScope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if other, _, err := Open(dir, testScope, nil); err == nil {
		other.Close()
		t.Fatal("duplicate workspace acquired lock")
	}
	otherScope := testScope
	otherScope.User = "other"
	other, _, err := Open(dir, otherScope, nil)
	if err != nil {
		t.Fatal(err)
	}
	other.Close()
	path := s.path
	s.Close()
	if err = os.WriteFile(path, []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	if s, _, err = Open(dir, testScope, nil); err == nil {
		s.Close()
		t.Fatal("corrupt state accepted")
	}
	os.Remove(path)
	target := filepath.Join(t.TempDir(), "target")
	os.WriteFile(target, []byte("private"), 0600)
	os.Symlink(target, path)
	if s, _, err = Open(dir, testScope, nil); err == nil {
		s.Close()
		t.Fatal("symlink followed")
	}
}

func TestSaveFailureSuppressesDeliveryAndDeliveryFailureIsNotRetried(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	sender := &recorder{err: errors.New("delivery failed")}
	s, _, err := Open(dir, testScope, sender)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	store := app.NewStore()
	j := testJob(now)
	store.Apply(refresh(now, j))
	j.State = "COMPLETED"
	events := store.Apply(refresh(now.Add(time.Minute), j))
	checkpoint := store.Checkpoint()
	os.Mkdir(s.path, 0700)
	if err = s.Process(context.Background(), checkpoint, events); err == nil {
		t.Fatal("expected save error")
	}
	if len(sender.messages) != 0 {
		t.Fatal("sent without persistence")
	}
	os.Remove(s.path)
	if err = s.Process(context.Background(), checkpoint, events); err == nil {
		t.Fatal("expected delivery error")
	}
	if len(sender.messages) != 1 {
		t.Fatal(sender.messages)
	}
	s.Close()
	next, saved, err := Open(dir, testScope, sender)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	restored := app.NewStore()
	restored.Restore(saved, now.Add(2*time.Minute))
	if e := restored.Apply(refresh(now.Add(2*time.Minute), j)); len(e) != 0 {
		t.Fatal("retry after delivery failure", e)
	}
}

func TestOutOfOrderCheckpointCannotOverwriteNewerState(t *testing.T) {
	s, _, err := Open(filepath.Join(t.TempDir(), "state"), testScope, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now()
	if err = s.Process(context.Background(), app.Checkpoint{At: now}, nil); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path)
	if err = s.Process(context.Background(), app.Checkpoint{At: now.Add(-time.Hour)}, nil); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.path)
	if string(before) != string(after) {
		t.Fatal("stale write")
	}
}

func TestGroupingAndUntrustedLabels(t *testing.T) {
	var events []app.Event
	for i := 0; i < 100; i++ {
		events = append(events, app.Event{Key: string(rune(i + 100)), Kind: "failed", ArrayID: 42})
	}
	messages := Messages(events)
	if len(messages) != 1 || !strings.Contains(messages[0].Body, "100 tasks failed") {
		t.Fatal(messages)
	}
	messages = Messages(append(events, events...))
	if len(messages) != 1 || !strings.Contains(messages[0].Body, "100 tasks") {
		t.Fatal(messages)
	}
	for i := 0; i < 5; i++ {
		events = append(events, app.Event{Key: string(rune(i + 300)), JobID: string(rune(i + 300)), Kind: "finished"})
	}
	messages = Messages(events)
	if len(messages) != 1 || !strings.Contains(messages[0].Body, "100 failed, 5 finished") {
		t.Fatal(messages)
	}
	messages = Messages([]app.Event{{Key: "a", JobID: "1", Kind: "failed", Name: "\x1b[31mhello\nworld\x07"}})
	if strings.ContainsAny(messages[0].Body, "\x1b\n\x07") {
		t.Fatal(messages)
	}
}

func TestDesktopCommandKeepsMessageOutOfSource(t *testing.T) {
	message := Message{Title: "Perch", Body: `\" & do shell script \"touch /tmp/not-an-instruction\" & \"`}
	command := desktopCommand(context.Background(), message)
	if len(command.Args) != 6 || command.Args[2] != desktopScript || command.Args[5] != message.Body {
		t.Fatal(command.Args)
	}
	if strings.Contains(command.Args[2], "do shell script") {
		t.Fatal("message interpolated into source")
	}
	// Deliberately do not execute: no actual desktop notifications in tests.
}
