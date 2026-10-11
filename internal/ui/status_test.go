package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/demo"
	"perch/internal/hummingbird"
)

func TestTransientNoticesExpireWithoutHidingStatus(t *testing.T) {
	m := demoModel(80, 24)
	m.setNotice("Copied failed tasks: --array=7")
	if !strings.Contains(m.commandHint(), "Copied failed tasks") || !strings.Contains(m.statusLine(76), "demo") {
		t.Fatal("notice hid connection status")
	}
	m.Update(clockMsg(m.now.Add(9 * time.Second)))
	if m.notice != "" || !m.noticeUntil.IsZero() {
		t.Fatal("notice did not expire")
	}
}

func TestConnectionShowsWrappedErrorBeforeCapabilities(t *testing.T) {
	m := demoModel(48, 24)
	m.lastError = "Long remote query failure with enough words to require wrapping inside the connection panel"
	m.connected = false
	m.Update(tea.KeyPressMsg{Code: '4', Text: "4"})
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Long remote query failure") || !strings.Contains(view, "connection panel") {
		t.Fatal(view)
	}
	for _, line := range strings.Split(m.panel.GetContent(), "\n") {
		if ansi.StringWidth(line) > 40 {
			t.Fatal("connection prose clipped", line)
		}
	}
}

func TestHistoryWithoutSuccessfulAccountingIsNotEmptySuccess(t *testing.T) {
	m := demoModel(80, 24)
	m.store.Snapshot.Jobs = nil
	m.store.Snapshot.HistoryAt = time.Time{}
	m.tab = 1
	if !strings.Contains(m.jobList(70, 12), "History is not available") {
		t.Fatal("unavailable accounting reported empty history")
	}
	m.store.Snapshot.HistoryAt = m.now
	if !strings.Contains(m.jobList(70, 12), "No recent completed jobs") {
		t.Fatal("verified empty history reported failure")
	}
	m.tab = 0
	m.search, m.busy = "no-match", true
	if !strings.Contains(m.jobList(70, 12), "No jobs match") {
		t.Fatal("background refresh replaced filter result with connecting")
	}
}

func TestFilterCannotHideConnectionHealth(t *testing.T) {
	for _, width := range []int{48, 80, 140} {
		for _, searching := range []bool{false, true} {
			m := demoModel(width, 24)
			m.search, m.searching = strings.Repeat("é", 100), searching
			m.notice = "Finished · old notice"
			m.Update(refreshMsg{generation: m.generation, err: errors.New("network unavailable")})
			line := ansi.Strip(m.statusLine(width - 4))
			if !strings.Contains(line, "disconnected") || !strings.Contains(line, "Filter:") || ansi.StringWidth(line) > width-4 {
				t.Fatalf("lost state/filter at width %d: %s", width, line)
			}
			view := ansi.Strip(m.View().Content)
			if !strings.Contains(view, "c reconnect") || !strings.Contains(view, "4 details") {
				t.Fatal(view)
			}
			m.action("ctrl+k")
			m.searching = false
			if !strings.Contains(ansi.Strip(m.View().Content), "disconnected") {
				t.Fatal("palette hid disconnection")
			}
		}
	}
}

func TestRefreshFailureRetainsSnapshotAndRecoveryClearsError(t *testing.T) {
	m := demoModel(140, 40)
	m.search = "parameter"
	m.action("down")
	id, at, count := m.selectedID, m.store.Snapshot.At, len(m.store.Snapshot.Jobs)
	for failure := 1; failure <= 6; failure++ {
		m.Update(refreshMsg{generation: m.generation, err: errors.New("network unavailable")})
		if m.connected || m.busy || m.selectedID != id || !m.store.Snapshot.At.Equal(at) || len(m.store.Snapshot.Jobs) != count {
			t.Fatal("failed refresh erased or relabelled retained data")
		}
		want := time.Duration(min(120, 5*(1<<min(failure, 5)))) * time.Second
		if got := m.nextRefresh.Sub(m.now); got != want {
			t.Fatalf("retry %d: %s, want %s", failure, got, want)
		}
	}
	m.now = m.now.Add(2 * time.Minute)
	m.Update(refreshMsg{generation: m.generation, result: demo.Snapshot(m.now, 0)})
	if !m.connected || m.lastError != "" || m.failures != 0 || m.selectedID != id || m.search != "parameter" {
		t.Fatal("recovery lost context or retained failure")
	}
	if m.nextRefresh.Sub(m.now) != 30*time.Second {
		t.Fatal("normal cadence not restored")
	}
}

func TestPartialRefreshIsVisibleDespiteFilterAndNotice(t *testing.T) {
	m := demoModel(80, 24)
	m.search, m.notice = "parameter", "Finished · old notice"
	at, count := m.store.Snapshot.At, len(m.store.Snapshot.Jobs)
	m.Update(refreshMsg{generation: m.generation, result: hummingbird.Refresh{At: m.now.Add(time.Minute), Errors: map[string]error{"jobs": errors.New("scheduler unavailable")}}})
	if !m.connected || !m.store.Snapshot.At.Equal(at) || len(m.store.Snapshot.Jobs) != count {
		t.Fatal("partial refresh discarded data")
	}
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"partial refresh", "Filter:", "4 connection details", "scheduler unavailable"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q: %s", want, view)
		}
	}
}

func TestReportAgeDoesNotFollowQueueRefresh(t *testing.T) {
	m := demoModel(80, 24)
	for _, name := range []string{"/nodes", "/priority", "/software cuda"} {
		c := command(m, name)
		m.Update(c())
		at := m.reportAt
		if at.IsZero() {
			t.Fatal("missing report observation")
		}
		m.now = at.Add(3 * time.Minute)
		m.store.Snapshot.At = m.now
		if m.age() != "report 3m ago" {
			t.Fatal("queue refresh relabelled old report", m.age())
		}
		m.detailBusy = true
		if m.age() != "reading report…" {
			t.Fatal(m.age())
		}
		m.detailBusy = false
		m.detailError = "query failed"
		if m.age() != "report read failed" {
			t.Fatal(m.age())
		}
		m.action("esc")
	}
	m.Close()
}
