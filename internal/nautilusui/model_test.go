package nautilusui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/nautilus"
)

func ready(t *testing.T, w, h int) *Model {
	t.Helper()
	m := New(context.Background(), nautilus.NewDemoClient(), Options{Demo: true})
	t.Cleanup(m.Close)
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.Update(m.refresh()())
	return m
}
func key(m *Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestRefreshPreservesIndependentStaleSnapshotsAndRejectsLateScope(t *testing.T) {
	m := ready(t, 120, 38)
	jobs, pods := m.jobs, m.pods
	if m.refresh() == nil || m.refresh() != nil {
		t.Fatal("refreshes overlap")
	}
	m.Update(refreshMsg{m.sequence - 1, nautilus.Refresh{Scope: m.scope}})
	if !m.busy || m.jobs != jobs {
		t.Fatal("late refresh applied")
	}
	m.Update(refreshMsg{m.sequence, nautilus.Refresh{Scope: nautilus.Scope{Cluster: "different", Namespace: "research"}}})
	if !m.busy {
		t.Fatal("cross-cluster reply applied")
	}
	result := m.client.Refresh(context.Background())
	result.Jobs = nautilus.Result[nautilus.Job]{Error: &nautilus.QueryError{Kind: nautilus.Forbidden, Message: "Job access denied"}}
	m.Update(refreshMsg{m.sequence, result})
	if m.jobs != jobs || m.pods == pods || !strings.Contains(m.status(), "Jobs stale") || m.authRequired {
		t.Fatal("partial refresh lost independent state")
	}
	if !m.nextRefresh.After(m.now.Add(15 * time.Second)) {
		t.Fatal("failure has no backoff")
	}
	result.Pods = nautilus.Result[nautilus.Pod]{Error: &nautilus.QueryError{Kind: nautilus.Unauthenticated}}
	m.refresh()
	m.Update(refreshMsg{m.sequence, result})
	if !m.authRequired {
		t.Fatal("authentication failure did not stop polling")
	}
	seq := m.sequence
	m.Update(clockMsg(m.now.Add(time.Hour)))
	if seq != m.sequence {
		t.Fatal("clock retried expired credentials")
	}
}

func TestRecreatedNameDoesNotInheritInspector(t *testing.T) {
	m := ready(t, 100, 32)
	m.inspect = true
	uid := m.selectedUID[0]
	m.refresh()
	result := m.client.Refresh(context.Background())
	for i := range result.Jobs.Collection.Items {
		if result.Jobs.Collection.Items[i].UID == uid {
			result.Jobs.Collection.Items[i].UID = "new-object-uid"
		}
	}
	m.Update(refreshMsg{m.sequence, result})
	if m.inspect || m.selectedUID[0] == uid || !strings.Contains(m.notice, "absence is not completion") {
		t.Fatal("name reuse inherited old selection identity")
	}
}

func TestResponsiveNavigationAndInspection(t *testing.T) {
	for _, size := range [][2]int{{48, 18}, {80, 24}, {120, 38}} {
		m := ready(t, size[0], size[1])
		if !strings.Contains(ansi.Strip(m.View().Content), "q quit") {
			t.Fatal("list quit hint clipped", size)
		}
		for tab := 0; tab < 4; tab++ {
			key(m, string(rune('1'+tab)))
			if m.tab != tab {
				t.Fatal("tab did not switch")
			}
			if tab < 2 {
				m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
			}
			content := ansi.Strip(m.View().Content)
			lines := strings.Split(content, "\n")
			if len(lines) > size[1] {
				t.Fatal("view exceeds height", size)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("view exceeds width", size, line)
				}
			}
			if !strings.Contains(lines[len(lines)-1], "q quit") {
				t.Fatal("quit hint clipped", size)
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		if m.tab != 2 {
			t.Fatal("reverse navigation failed")
		}
	}
}

func TestFilterMouseAndStaleDetails(t *testing.T) {
	m := ready(t, 80, 24)
	m.input.SetValue("/find retrying")
	m.submit()
	m.Update(clockMsg(m.now))
	if len(m.rows(0)) != 1 {
		t.Fatal("filter did not narrow Jobs")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(m.panel.GetContent(), "retrying-old") {
		t.Fatal("controller-owned Pods missing")
	}
	m.jobsErr = &nautilus.QueryError{Kind: nautilus.Forbidden, Message: "access denied"}
	m.Update(clockMsg(m.now))
	if !strings.Contains(m.panel.GetContent(), "Stale Job snapshot") {
		t.Fatal("stale inspector unlabelled")
	}
	m.Update(tea.MouseClickMsg{X: 12, Y: 2, Button: tea.MouseLeft})
	if m.tab != 1 || m.inspect {
		t.Fatal("Pod tab click failed")
	}
	m.Update(tea.MouseClickMsg{X: 8, Y: 8, Button: tea.MouseLeft})
	if m.selection[1] != 1 {
		t.Fatal("row click geometry disagrees with renderer")
	}
	key(m, "1")
	if m.filters[0] != "retrying" || len(m.rows(0)) != 1 {
		t.Fatal("tab switch lost filter")
	}
	key(m, "4")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.tab != 0 || m.filters[0] != "retrying" {
		t.Fatal("connection escape lost the previous resource list")
	}
	key(m, "23")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.tab != 1 {
		t.Fatal("namespace escape did not return to Pods")
	}
}

func TestSuccessfulEmptyReadReplacesStaleDataAndDuplicateReplyIsIgnored(t *testing.T) {
	m := ready(t, 80, 24)
	m.refresh()
	result := m.client.Refresh(context.Background())
	result.Jobs.Collection.Items = nil
	m.Update(refreshMsg{m.sequence, result})
	if len(m.rows(0)) != 0 || m.jobs == nil || !strings.Contains(ansi.Strip(m.View().Content), "No objects in this snapshot") {
		t.Fatal("successful empty collection was confused with a failed read")
	}
	result = m.client.Refresh(context.Background())
	m.Update(refreshMsg{m.sequence, result})
	if len(m.rows(0)) != 0 {
		t.Fatal("duplicate completed response was applied")
	}
}

func TestReconnectCancelsOldRequestAndDiscardsItsReply(t *testing.T) {
	m := ready(t, 80, 24)
	m.options.Connect = func(context.Context) (*nautilus.Connection, error) { return nil, nil }
	cancelled := false
	m.refresh()
	sequence := m.sequence
	m.requestCancel = func() { cancelled = true }
	if m.reconnect() == nil || !cancelled || !m.connecting || m.busy || !m.authRequired {
		t.Fatal("reconnect did not isolate the old request")
	}
	jobs := m.jobs
	m.Update(refreshMsg{sequence, m.client.Refresh(context.Background())})
	if m.jobs != jobs || !m.connecting {
		t.Fatal("cancelled request changed reconnect state")
	}
	m.Update(loginMsg{})
	if m.connecting || !m.authRequired || !strings.Contains(m.notice, "Sign-in did not complete") {
		t.Fatal("missing credential result did not fail safely")
	}
	m.Update(clockMsg(m.now.Add(24 * time.Hour)))
	if m.busy {
		t.Fatal("failed reconnect restarted background reads")
	}
}
