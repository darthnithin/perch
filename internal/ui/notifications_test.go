package ui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"path/filepath"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/notify"
	"perch/internal/transport"
	"strings"
	"testing"
	"time"
)

type notificationRecorder struct{ messages []notify.Message }

func (r *notificationRecorder) Send(_ context.Context, m notify.Message) error {
	r.messages = append(r.messages, m)
	return nil
}

func TestReconnectKeepsNotificationSessionAlive(t *testing.T) {
	ssh, err := transport.NewSSH("example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer ssh.Close()
	sender := &notificationRecorder{}
	m := New(&hummingbird.Client{Runner: ssh}, Options{Host: "hb", Notifier: sender, StateDir: filepath.Join(t.TempDir(), "state"), ReducedMotion: true})
	defer m.Close()
	m.report = domain.Report{User: "me", Cluster: "hb"}
	m.startNotifications()
	session := m.notifications
	if session == nil {
		t.Fatal("notification session missing")
	}
	now := time.Now()
	j := domain.Job{ID: "12", State: "RUNNING", Submit: now, User: "me", Cluster: "hb"}
	_, cmd := m.Update(refreshMsg{result: hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{j}}})
	runNotificationCommand(m, cmd)
	// Construct interactive login, but never run SSH or a desktop notification.
	if m.action("c") == nil {
		t.Fatal("reconnect command missing")
	}
	if err := m.notificationContext.Err(); err != nil {
		t.Fatalf("reconnect cancelled alerts: %v", err)
	}
	m.startNotifications()
	if m.notifications != session {
		t.Fatal("reconnect replaced notification ownership")
	}
	j.State = "COMPLETED"
	_, cmd = m.Update(refreshMsg{generation: m.generation, result: hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{j}}})
	runNotificationCommand(m, cmd)
	if len(sender.messages) != 1 {
		t.Fatalf("completion after reconnect: %+v", sender.messages)
	}
	m.Close()
	if m.notificationContext.Err() == nil {
		t.Fatal("shutdown left alerts alive")
	}
}
func runNotificationCommand(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			runNotificationCommand(m, child)
		}
		return
	}
	m.Update(msg)
}
func TestWorkspaceNotificationLifecycle(t *testing.T) {
	sender := &notificationRecorder{}
	options := Options{Host: "hb", Notifier: sender, StateDir: filepath.Join(t.TempDir(), "state"), ReducedMotion: true}
	report := domain.Report{User: "me", Cluster: "hb"}
	now := time.Now()
	m := New(nil, options)
	m.report = report
	m.startNotifications()
	j := domain.Job{ID: "12", State: "RUNNING", Submit: now, User: "me", Cluster: "hb"}
	_, cmd := m.Update(refreshMsg{result: hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{j}}})
	runNotificationCommand(m, cmd)
	m.Close()
	m = New(nil, options)
	defer m.Close()
	m.report = report
	m.startNotifications()
	if len(m.store.Snapshot.Jobs) != 1 {
		t.Fatal("restore missing")
	}
	j.State = "FAILED"
	_, cmd = m.Update(refreshMsg{result: hummingbird.Refresh{At: now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: []domain.Job{j}}})
	if cmd == nil {
		t.Fatal("no persistence/delivery command")
	}
	runNotificationCommand(m, cmd)
	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0].Body, "FAILED") {
		t.Fatal(sender.messages)
	}
	if !strings.Contains(m.connection(), "Desktop alerts enabled") {
		t.Fatal(m.connection())
	}
}

func TestLimitedArrayNotificationCoverageIsVisible(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.options.Demo = false
	m.options.Notifier = &notificationRecorder{}
	now := time.Now()
	// The queue entry lacks verified attempt identity: retain monitoring, but
	// do not silently promise range-based catch-up for it.
	j := domain.Job{ID: "42_[0-9]", ArrayID: 42, TaskRange: "0-9", State: "PENDING"}
	m.Update(refreshMsg{result: hummingbird.Refresh{At: now, QueueOK: true, Queue: []domain.Job{j}}})
	if !strings.Contains(m.notice, "Array alert coverage limited") {
		t.Fatal(m.notice)
	}
	press(m, '4')
	if !strings.Contains(m.panel.GetContent(), "Some array outcomes may not trigger alerts") {
		t.Fatal(m.panel.GetContent())
	}
}
func TestDemoAndDisabledNeverCreateNotificationState(t *testing.T) {
	for _, options := range []Options{{Demo: true, Notifier: &notificationRecorder{}}, {}} {
		m := New(nil, options)
		m.report = domain.Report{User: "me", Cluster: "hb"}
		m.startNotifications()
		if m.notifications != nil || m.saveNotifications(nil) != nil {
			t.Fatal("side effect for demo/disabled")
		}
		m.Close()
	}
}
