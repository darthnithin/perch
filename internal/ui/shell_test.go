package ui

import (
	"errors"
	"perch/internal/transport"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func previewShell(t *testing.T, m *Model) shellPreparedMsg {
	t.Helper()
	cmd := m.action("shell")
	if cmd == nil {
		t.Fatal("no preview request", m.notice)
	}
	msg := cmd().(shellPreparedMsg)
	m.Update(msg)
	if m.view != "shell" || m.shellForm.stage != "ready" {
		t.Fatal("not ready")
	}
	return msg
}

func TestShellPreviewPreservesContextAndFits(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		defer m.Close()
		command(m, "/find parameter")
		press(m, 'j')
		press(m, 'u')
		id := m.selectedID
		original := m.view
		previewShell(t, m)
		if m.input.Focused() || m.palette {
			t.Fatal("modal did not own focus")
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("overflow", line)
			}
		}
		if len(strings.Split(m.View().Content, "\n")) > size[1] {
			t.Fatal("height overflow")
		}
		x, y := footerPoint(t, m, "enter open shell")
		m.options.PointerStyle = "css"
		m.Update(tea.MouseMotionMsg{X: x, Y: y})
		if m.lastPointer != "pointer" || m.shellForm.stage != "ready" {
			t.Fatal("hover activated shell")
		}
		footerClick(t, m, "esc back")
		if m.view != original || m.selectedID != id || m.search != "parameter" || m.shellForm != nil {
			t.Fatal("lost caller context")
		}
	}
}

func TestShellCancelsStaleRepliesAndDuplicateOpen(t *testing.T) {
	m := demoModel(100, 32)
	defer m.Close()
	cmd := m.action("shell")
	old := cmd().(shellPreparedMsg)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(old)
	if m.view == "shell" || m.shellForm != nil {
		t.Fatal("late preview reopened modal")
	}
	previewShell(t, m)
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || m.shellForm.stage != "opening" {
		t.Fatal("did not begin revalidation")
	}
	if _, again := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); again != nil {
		t.Fatal("duplicate activation")
	}
	msg := cmd().(shellPreparedMsg)
	m.Update(msg)
	if m.view == "shell" || !strings.Contains(m.notice, "Demo preview only") {
		t.Fatal("demo should return without execution")
	}
	m.Update(msg)
	if m.view == "shell" {
		t.Fatal("old open result replayed")
	}
}

func TestShellErrorsAndKeysStayInPreview(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	previewShell(t, m)
	seq := m.shellForm.sequence
	m.Update(shellPreparedMsg{sequence: seq, err: errors.New("\x1b[2Jallocation disappeared")})
	if m.shellForm.stage != "error" || strings.Contains(m.shellForm.problem, "\x1b") {
		t.Fatal("error not sanitized")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatal("error opened a shell")
	}
	m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if m.view != "shell" {
		t.Fatal("modal key leaked")
	}
	before := m.generation
	m.nextRefresh = m.now.Add(-time.Second)
	m.Update(clockMsg(m.now))
	if m.generation != before || m.busy {
		t.Fatal("preview continued scheduler polling")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil || m.shellForm.stage != "checking" {
		t.Fatal("retry unavailable")
	}
	m.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	m.Update(cmd())
	if m.view != "details" || m.tab != 2 || m.shellForm != nil {
		t.Fatal("navigation failed to cancel preview")
	}
}

func TestShellNodeCycleAndReturn(t *testing.T) {
	m := demoModel(100, 32)
	defer m.Close()
	previewShell(t, m)
	m.shellForm.plan.Nodes = []string{"node-14", "node-15"}
	m.shellForm.plan.Node = "node-14"
	if !strings.Contains(m.shellFooter(), "n node") {
		t.Fatal("missing node navigation")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if cmd == nil {
		t.Fatal("node cycle did not request fresh preview")
	}
	// A stale return cannot dismiss a newer preview.
	m.Update(shellReturnedMsg{sequence: m.shellForm.sequence - 1})
	if m.shellForm == nil {
		t.Fatal("stale return closed modal")
	}
	m.Update(cmd())
	m.pointerSuspended = true
	m.Update(shellReturnedMsg{sequence: m.shellForm.sequence})
	if m.pointerSuspended || m.shellForm != nil || m.view != "details" {
		t.Fatal("did not restore workspace")
	}
}

func TestShellRejectsDisconnectedAndNonRunningSelection(t *testing.T) {
	m := demoModel(100, 32)
	defer m.Close()
	m.options.Demo = false
	if cmd := m.action("shell"); cmd != nil || m.shellForm != nil {
		t.Fatal("disconnected shell accepted")
	}
	m.options.Demo = true
	command(m, "/find embedding")
	if cmd := m.action("shell"); cmd != nil {
		t.Fatal("pending shell accepted")
	}
}

func TestShellPrimaryButtonAndInterruptQuit(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	previewShell(t, m)
	point := tea.Mouse{X: 5, Y: m.height - 5, Button: tea.MouseLeft}
	if got := m.mouseTarget(point); got.kind != "shell-open" {
		t.Fatal("primary button has no target", got)
	}
	m.options.NoMouse = true
	if cmd := m.mouseClick(point); cmd != nil {
		t.Fatal("disabled mouse opened shell")
	}
	m.options.NoMouse = false
	m.Update(tea.WindowSizeMsg{Width: 47, Height: 18})
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatal("unrenderable preview opened shell")
	}
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
	if cmd := m.mouseClick(point); cmd == nil || m.shellForm.stage != "opening" {
		t.Fatal("primary action did not revalidate")
	}
	_, cmd := m.Update(shellReturnedMsg{sequence: m.shellForm.sequence, err: transport.ErrForegroundInterrupted})
	if cmd == nil {
		t.Fatal("termination resumed UI instead of quitting")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit")
	}
}
