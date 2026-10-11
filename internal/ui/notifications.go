package ui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"perch/internal/app"
	"perch/internal/notify"
	"time"
)

type notificationMsg struct {
	session *notify.Session
	err     error
}

func (m *Model) startNotifications() {
	if m.options.Demo || m.options.Notifier == nil {
		return
	}
	scope := notify.Scope{Host: m.options.Host, User: m.report.User, Cluster: m.report.Cluster}
	if m.notifications != nil && m.notificationScope == scope {
		return
	}
	if m.notifications != nil {
		m.notifications.Close()
		m.notifications = nil
		m.store = app.NewStore()
		m.store.SetTimeWarnings(m.options.TimeWarnings)
	}
	session, checkpoint, err := notify.Open(m.options.StateDir, scope, m.options.Notifier)
	if err != nil {
		m.notificationStatus = "Notifications unavailable: " + err.Error()
		m.setNotice("Notifications need attention · see Connection")
		return
	}
	m.notifications = session
	m.notificationScope = scope
	if m.store.Snapshot.At.IsZero() {
		m.store.Restore(checkpoint, m.now)
	}
	m.notificationStatus = "Desktop alerts enabled · macOS controls banner visibility"
}

func (m *Model) saveNotifications(events []app.Event) tea.Cmd {
	if m.notifications == nil || m.store.Snapshot.At.IsZero() {
		return nil
	}
	session, checkpoint := m.notifications, m.store.Checkpoint()
	parent := m.notificationContext
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, 10*time.Second)
		defer cancel()
		return notificationMsg{session: session, err: session.Process(ctx, checkpoint, events)}
	}
}
