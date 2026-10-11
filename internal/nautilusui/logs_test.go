package nautilusui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/nautilus"
)

func filter(m *Model, value string)     { m.input.SetValue("/find " + value); m.submit() }
func press(m *Model, code rune) tea.Cmd { _, cmd := m.Update(tea.KeyPressMsg{Code: code}); return cmd }
func applyCommand(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("missing read command")
	}
	m.Update(cmd())
}

func TestLogsJobPodContainerNavigationPreservesWorkload(t *testing.T) {
	m := ready(t, 80, 24)
	filter(m, "retrying")
	press(m, tea.KeyEnter)
	uid := m.selectedUID[0]
	key(m, "l")
	if m.logs == nil || m.logs.stage != "pods" || len(m.logs.pods) != 2 {
		t.Fatal("Job did not show controller-owned Pods")
	}
	key(m, "j")
	press(m, tea.KeyEnter)
	if m.logs.stage != "containers" || m.logs.pod.Name != "retrying-replacement" {
		t.Fatal("Pod selection lost")
	}
	if m.logs.containers[0].Name != "compute" {
		t.Fatal("primary app is not first")
	}
	applyCommand(t, m, press(m, tea.KeyEnter))
	if m.logs.snapshot == nil || !strings.Contains(m.logs.panel.GetContent(), "Current instance") {
		t.Fatal("current output unavailable")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if m.logs.snapshot != nil || strings.Contains(m.logs.panel.GetContent(), "Current instance") {
		t.Fatal("current output relabelled previous")
	}
	applyCommand(t, m, cmd)
	if !m.logs.snapshot.Previous || !strings.Contains(m.logs.panel.GetContent(), "Previous instance") {
		t.Fatal("previous output unavailable")
	}
	for _, size := range [][2]int{{48, 18}, {80, 24}, {120, 38}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(ansi.Strip(m.View().Content), "\n")
		if len(lines) > size[1] || !strings.Contains(lines[len(lines)-1], "q quit") {
			t.Fatal("log layout clipped", size)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("log width overflow")
			}
		}
	}
	press(m, tea.KeyEscape)
	if m.logs.stage != "containers" {
		t.Fatal("Esc skipped container picker")
	}
	press(m, tea.KeyEscape)
	if m.logs.stage != "pods" || m.logs.index != 1 {
		t.Fatal("Esc lost selected Pod")
	}
	press(m, tea.KeyEscape)
	if m.logs != nil || !m.inspect || m.selectedUID[0] != uid || m.filters[0] != "retrying" {
		t.Fatal("return lost original workload context")
	}
}

func TestLogsLateRepliesModesAndPartialErrors(t *testing.T) {
	m := ready(t, 80, 24)
	key(m, "2")
	filter(m, "retrying-replacement")
	key(m, "l")
	cmd := press(m, tea.KeyEnter)
	old := cmd().(logMsg)
	m.Update(old)
	l := m.logs
	l.snapshot.Text = strings.Repeat("line\n", 100)
	m.syncLogs()
	l.panel.SetYOffset(12)
	// A failed refresh retains the snapshot, and its reading position is not
	// intentionally reset. The normal sync clamps only if content is shorter.
	oldSnapshot := l.snapshot
	m.now = m.now.Add(3 * time.Second)
	m.readLogs(false)
	m.Update(logMsg{l, l.sequence, nautilus.LogSnapshot{}, errors.New("permission denied\x1b[2J")})
	if l.snapshot != oldSnapshot || strings.Contains(l.err, "\x1b") || !strings.Contains(ansi.Strip(m.View().Content), "stale") {
		t.Fatal("failure lost or mislabelled retained output")
	}
	if l.panel.YOffset() != 12 {
		t.Fatal("failed refresh moved the reader")
	}
	_, newCmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.Update(old)
	if l.snapshot != nil || !l.busy {
		t.Fatal("old current reply overwritten previous request")
	}
	m.Update(newCmd())
	closedReply := logMsg{l, l.sequence, *l.snapshot, nil}
	key(m, "1")
	m.Update(closedReply)
	if m.logs != nil || m.tab != 0 {
		t.Fatal("late log reply reopened closed view")
	}
}

func TestLogAuthenticationFailureLatchesUntilReconnect(t *testing.T) {
	m := ready(t, 80, 24)
	key(m, "2")
	filter(m, "retrying-replacement")
	key(m, "l")
	press(m, tea.KeyEnter)
	l := m.logs
	m.Update(logMsg{l, l.sequence, nautilus.LogSnapshot{}, &nautilus.QueryError{Kind: nautilus.Unauthenticated, Message: "sign in"}})
	if !m.authRequired || !m.logAuthRequired {
		t.Fatal("log authentication failure not classified")
	}
	m.refresh()
	m.Update(refreshMsg{m.sequence, m.client.Refresh(context.Background())})
	if !m.authRequired {
		t.Fatal("unrelated namespace success erased log sign-in requirement")
	}
	if m.readLogs(true) != nil {
		t.Fatal("log refresh started without explicit reconnect")
	}
	m.options.Connect = func(context.Context) (*nautilus.Connection, error) { return nil, errors.New("cancelled") }
	m.reconnect()
	if l.busy {
		t.Fatal("reconnect retained in-flight log read")
	}
}
