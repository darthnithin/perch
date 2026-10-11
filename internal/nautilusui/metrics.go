package nautilusui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"perch/internal/domain"
	"perch/internal/nautilus"
)

type metricsSource interface {
	ReadMetrics(context.Context, nautilus.Ref) (nautilus.MetricsSnapshot, error)
}
type metricsMsg struct {
	view     *logView
	sequence uint64
	snapshot nautilus.MetricsSnapshot
	err      error
}

func (m *Model) readMetrics(force bool) tea.Cmd {
	l := m.logs
	if l == nil || !l.metrics || l.stage != "output" || l.busy || m.connecting || m.ctx.Err() != nil {
		return nil
	}
	if m.authRequired {
		l.err = "Sign in with c before reading metrics again."
		return nil
	}
	if !force && m.now.Sub(l.lastRequest) < 2*time.Second {
		return nil
	}
	r, ok := m.client.(metricsSource)
	if !ok {
		l.err = "Pod metrics are unavailable on this connection."
		return nil
	}
	l.busy = true
	l.sequence++
	l.lastRequest = m.now
	sequence, pod := l.sequence, l.pod.Ref
	ctx, cancel := context.WithCancel(m.ctx)
	l.cancel = cancel
	return func() tea.Msg {
		defer cancel()
		s, err := r.ReadMetrics(ctx, pod)
		return metricsMsg{l, sequence, s, err}
	}
}

func (m *Model) receiveMetrics(msg metricsMsg) {
	l := m.logs
	if l == nil || l != msg.view || !l.metrics || l.sequence != msg.sequence || !l.busy {
		return
	}
	l.busy = false
	if msg.err != nil {
		l.err = domain.Label(msg.err.Error())
		var e *nautilus.QueryError
		if errors.As(msg.err, &e) && e.Kind == nautilus.Unauthenticated {
			m.logAuthRequired = true
			m.authRequired = true
		}
		return
	}
	if msg.snapshot.Pod != l.pod.Ref {
		l.err = "Metrics did not match the selected Pod."
		return
	}
	l.metricsSnapshot = &msg.snapshot
	l.err = ""
}

func (m *Model) metricsContent() string {
	l := m.logs
	if l.metricsSnapshot == nil {
		if l.err != "" {
			return "Unable to read Pod metrics.\n\n" + l.err + "\n\nr retries; c reconnects."
		}
		return "Reading Pod metrics…"
	}
	s := l.metricsSnapshot
	lines := []string{}
	if l.err != "" {
		lines = append(lines, warning.Render("Retained sample · read failed"), l.err, "")
	}
	if l.busy {
		lines = append(lines, muted.Render("Retained sample · reading…"), "")
	}
	for _, c := range s.Containers {
		lines = append(lines, accent.Bold(true).Render(c.Name),
			"CPU  "+nautilus.QuotaValue(c.CPUCores)+" cores",
			"RAM  "+nautilus.DisplayQuotaValue("memory", c.MemoryBytes)+" working set")
		if c.Attribution != "current-instance" {
			lines = append(lines, warning.Render(strings.ReplaceAll(c.Attribution, "-", " ")))
		}
		lines = append(lines, "")
	}
	if len(s.Containers) == 0 {
		lines = append(lines, "No container measurements reported.", "")
	}
	window, _ := time.ParseDuration(s.Window)
	age := max(time.Duration(0), m.now.Sub(s.Timestamp))
	if s.Stale || age > max(2*window, 2*time.Minute) {
		lines = append(lines, warning.Render("Stale sample · not current usage"))
	}
	lines = append(lines, muted.Render(fmt.Sprintf("Sample %s · %s ago", s.Timestamp.Local().Format("15:04:05"), age.Round(time.Second))), muted.Render("CPU window "+s.Window), "", muted.Render("On demand · r reads again"), muted.Render("Memory is an estimated working set, not peak usage. Missing values are unknown. No GPU or Job-wide utilization is inferred."))
	return strings.Join(lines, "\n")
}

func (m *Model) metricsHint() string {
	l := m.logs
	if l.metricsSnapshot == nil {
		return "On-demand Pod metrics · Esc returns"
	}
	s := l.metricsSnapshot
	age := max(time.Duration(0), m.now.Sub(s.Timestamp))
	window, _ := time.ParseDuration(s.Window)
	prefix := "Sample "
	if s.Stale || age > max(2*window, 2*time.Minute) {
		prefix = "Stale · sample "
	}
	if l.busy {
		prefix = "Reading · sample "
	} else if l.err != "" {
		prefix = "Read failed · sample "
	}
	return prefix + age.Round(time.Second).String() + " ago · window " + s.Window
}
