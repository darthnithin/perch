package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/cancellation"
	"perch/internal/domain"
)

func previewCancellation(t *testing.T, m *Model, array bool) cancelPreparedMsg {
	t.Helper()
	key := "cancel-job"
	if array {
		key = "cancel-array"
	}
	cmd := m.action(key)
	if cmd == nil {
		t.Fatal("no preview", m.notice)
	}
	msg := cmd().(cancelPreparedMsg)
	m.Update(msg)
	if m.cancelForm == nil || m.cancelForm.stage != "ready" {
		t.Fatal("not ready", m.cancelForm)
	}
	return msg
}

func TestCancellationReviewFitsAndPreservesContext(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		defer m.Close()
		command(m, "/find parameter")
		press(m, 'j')
		press(m, 'u')
		id, view := m.selectedID, m.view
		previewCancellation(t, m, false)
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("overflow", line)
			}
		}
		if len(strings.Split(m.View().Content, "\n")) != size[1] {
			t.Fatal("height mismatch")
		}
		plain := ansi.Strip(m.View().Content)
		for _, want := range []string{id, "Stops this task", "Sibling tasks continue", "dependencies"} {
			if !strings.Contains(plain, want) {
				t.Fatal(want, plain)
			}
		}
		if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || m.cancelForm.stage != "ready" {
			t.Fatal("Enter confirmed")
		}
		m.options.PointerStyle = "css"
		m.Update(tea.MouseMotionMsg{X: 5, Y: m.height - 3})
		if m.lastPointer != "pointer" || m.cancelForm.stage != "ready" {
			t.Fatal("hover confirmed or pointer absent")
		}
		footerClick(t, m, "esc back")
		if m.view != view || m.selectedID != id || m.search != "parameter" || m.cancelForm != nil {
			t.Fatal("context lost")
		}
	}
}

func TestCancellationScopeDoesNotPromoteTaskToArray(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	msg := previewCancellation(t, m, false)
	if msg.preview.Target.Array || msg.preview.Scope != "task" || msg.preview.Tasks != 1 {
		t.Fatal(msg.preview)
	}
	m.closeCancellation()
	msg = previewCancellation(t, m, true)
	if !msg.preview.Target.Array || msg.preview.Scope != "array" || msg.preview.Tasks <= 1 {
		t.Fatal(msg.preview)
	}
	if msg.preview.Tasks != 32 {
		t.Fatal("demo counted terminal tasks", msg.preview)
	}
	if !strings.Contains(m.cancellationContent(), "Stops all remaining tasks") {
		t.Fatal("scope not visible")
	}
	m.closeCancellation()
	m.tab = 1
	if m.action("cancel-job") != nil {
		t.Fatal("history authorized cancellation")
	}
	m.tab = 2
	if m.action("cancel-array") != nil {
		t.Fatal("hidden selection authorized cancellation")
	}
}

func TestCancellationDuplicateInputAndReceiptRetention(t *testing.T) {
	m := demoModel(100, 32)
	defer m.Close()
	previewCancellation(t, m, false)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if cmd == nil || m.cancelForm.stage != "sending" {
		t.Fatal("not sending")
	}
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: 'x', Mod: tea.ModCtrl}, {Code: tea.KeyEscape}, {Code: '3', Text: "3"}, {Code: 'q', Text: "q"}} {
		_, again := m.Update(key)
		if again != nil || m.view != "cancel" || m.cancelForm.stage != "sending" {
			t.Fatal("duplicate or escaped request", key)
		}
	}
	m.Update(tea.MouseClickMsg{X: 5, Y: 2, Button: tea.MouseLeft})
	if m.view != "cancel" {
		t.Fatal("tab escaped pending request")
	}
	msg := cmd().(cancelSentMsg)
	m.Update(msg)
	if m.cancelForm.stage != "receipt" || m.lastCancellation.Status != "demo" {
		t.Fatal("missing receipt")
	}
	if m.CancellationSummary() != "" {
		t.Fatal("demo printed real receipt")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.action("cancel-receipt")
	if m.cancelForm.stage != "receipt" || !strings.Contains(m.cancellationContent(), "No cancellation was sent") {
		t.Fatal("receipt lost")
	}
	_, again := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if again != nil {
		t.Fatal("receipt retried mutation")
	}
}

func TestCancellationStaleRepliesErrorsAndRefreshPause(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	cmd := m.action("cancel-job")
	old := cmd().(cancelPreparedMsg)
	m.closeCancellation()
	m.Update(old)
	if m.view == "cancel" {
		t.Fatal("late result reopened review")
	}
	previewCancellation(t, m, false)
	m.nextRefresh = m.now.Add(-time.Second)
	generation := m.generation
	m.Update(clockMsg(m.now))
	if m.busy || m.generation != generation {
		t.Fatal("polling during review")
	}
	seq := m.cancelForm.sequence
	m.cancelForm.stage = "sending"
	m.Update(cancelSentMsg{sequence: seq, err: errors.New("attempt changed\x1b[2J")})
	if m.cancelForm.stage != "error" || strings.ContainsRune(m.cancellationContent(), '\x1b') && strings.Contains(m.cancelForm.problem, "\x1b") {
		t.Fatal("error missing or unsafe")
	}
	if !strings.Contains(m.cancelForm.problem, "No cancellation request was sent") {
		t.Fatal(m.cancelForm.problem)
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatal("error confirmed")
	}
	m.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	if m.cancelForm != nil || m.tab != 2 {
		t.Fatal("tab navigation stuck")
	}
}

func TestCancellationUnknownReceiptSurvivesQuitAndScopeIsPinned(t *testing.T) {
	m := demoModel(100, 32)
	defer m.Close()
	previewCancellation(t, m, true)
	m.options.Demo = false
	r := &cancellation.Receipt{Status: "unknown", Target: cancellation.Target{ID: "9100", Array: true}, Detail: "Inspect queue and history before another request."}
	m.cancelForm.stage = "sending"
	m.cancelForm.receipt = r
	if !strings.Contains(m.CancellationSummary(), "9100 · unknown") {
		t.Fatal("inflight receipt absent")
	}
	m.Update(cancelSentMsg{sequence: m.cancelForm.sequence, receipt: r, err: errors.New("lost connection")})
	m.selectedID = "9231"
	m.normalizeSelection()
	if !strings.Contains(m.cancellationContent(), "Whole array · #9100") {
		t.Fatal("receipt followed new selection")
	}
	if !strings.Contains(m.CancellationSummary(), "unknown") {
		t.Fatal("receipt lost")
	}
}

func TestCancellationRequiresVisibleReviewAndMouseOptIn(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	previewCancellation(t, m, false)
	m.cancelForm.preview.Host = strings.Repeat("long-host-", 12)
	m.syncPanel()
	if m.panel.AtBottom() || !strings.Contains(m.cancellationButton(), "Review remaining") {
		t.Fatal("no scroll gate")
	}
	if m.confirmCancellation() != nil {
		t.Fatal("hidden details confirmed")
	}
	m.options.NoMouse = true
	m.mouseClick(tea.Mouse{X: 5, Y: m.height - 3, Button: tea.MouseLeft})
	if m.panel.AtBottom() {
		t.Fatal("no-mouse captured")
	}
	m.options.NoMouse = false
	m.mouseClick(tea.Mouse{X: 5, Y: m.height - 3, Button: tea.MouseLeft})
	if !m.panel.AtBottom() || m.cancelForm.stage != "ready" {
		t.Fatal("review jump confirmed")
	}
	m.width = 47
	if m.confirmCancellation() != nil {
		t.Fatal("undersized view confirmed")
	}
}

func TestCancellationSelectionMustBeCurrent(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	j, _ := m.selected()
	j.State = "COMPLETED"
	m.store.Snapshot.Jobs = []domain.Job{j}
	m.normalizeSelection()
	if m.action("cancel-job") != nil {
		t.Fatal("completed job authorized cancellation")
	}
}

func TestCancellationReturnKeepsScrollUnlessSelectedAttemptChanged(t *testing.T) {
	for _, change := range []string{"unchanged", "different job", "new attempt", "disappeared"} {
		t.Run(change, func(t *testing.T) {
			m := demoModel(80, 24)
			defer m.Close()
			m.view = "logs"
			m.logRaw = "original log"
			m.logLoaded = true
			m.logFollowing = false
			m.viewport.SetContent(strings.Repeat("original log\n", 60))
			m.viewport.SetYOffset(20)
			previewCancellation(t, m, false)
			switch change {
			case "different job":
				m.selectedID = "9231"
			case "new attempt":
				jobs := append([]domain.Job(nil), m.store.Snapshot.Jobs...)
				for i := range jobs {
					if jobs[i].ID == m.selectedID {
						jobs[i].Restarts++
					}
				}
				m.store.Snapshot.Jobs = jobs
			case "disappeared":
				m.store.Snapshot.Jobs = nil
			}
			if change != "unchanged" {
				m.normalizeSelection()
			}
			m.closeCancellation()
			if change != "unchanged" {
				if m.view != "details" || m.logRaw != "" || m.viewport.GetContent() != "" {
					t.Fatal("stale logs followed another job")
				}
			} else if m.view != "logs" || m.viewport.YOffset() != 20 || m.logFollowing {
				t.Fatal("scroll context lost")
			}
		})
	}
}
