package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestCompactInspectorReturnsSpaceToDetailsUntilCommandEntry(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.view != "inspect" || !m.compactTextReading() || m.panel.Height() != 8 {
		t.Fatalf("compact inspector has %d rows in %s", m.panel.Height(), m.view)
	}
	selected := m.selectedID
	if target := m.mouseTarget(tea.Mouse{X: 6, Y: m.height - 5}); target.kind == "input" {
		t.Fatal("hidden command box still intercepts the inspector")
	}
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.input.Focused() || m.compactTextReading() || m.panel.Height() != 4 {
		t.Fatal("command entry did not restore the normal command layout")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.input.Focused() || m.panel.Height() != 8 || m.selectedID != selected {
		t.Fatal("cancelled command lost inspector space or identity")
	}
}

func TestPendingInspectorPreservesSchedulerEvidence(t *testing.T) {
	m := demoModel(120, 40)
	defer m.Close()
	for _, reason := range []string{"BeginTime", "Dependency", "DependencyNeverSatisfied", "JobArrayTaskLimit", "Priority", "Resources", "JobHeldUser", "JobHeldAdmin", "InvalidAccount", "InvalidQOS", "PartitionDown", "PartitionInactive", "PartitionTimeLimit", "BadConstraints", "ReqNodeNotAvail", "SiteSpecificReason"} {
		j := domain.Job{ID: "123", Name: "queued", State: "PENDING", Reason: reason}
		text := ansi.Strip(m.details(j, 60))
		if !strings.Contains(text, "Scheduler: "+reason) || strings.Contains(text, "Estimated start:") {
			t.Fatal(text)
		}
	}
	title, detail := pendingMessage("SiteSpecificReason")
	if title != "Queued" || detail != "" {
		t.Fatal("unknown reason should not invent an explanation")
	}
}

func TestPendingGuidanceKeepsHoldsAndUnavailableResourcesDistinct(t *testing.T) {
	m := demoModel(120, 40)
	defer m.Close()
	for _, tc := range []struct{ reason, evidence string }{
		{"JobHeldUser", "coordinator"},
		{"JobHeldAdmin", "administrator"},
		{"ReqNodeNotAvail", "busy, reserved or out of service"},
		{"InvalidAccount", "account access"},
		{"InvalidQOS", "allowed QoS"},
		{"PartitionTimeLimit", "current limit"},
	} {
		j := domain.Job{ID: "123", Name: "queued", State: "PENDING", Reason: tc.reason}
		for _, width := range []int{40, 60} {
			text := ansi.Strip(m.details(j, width))
			flat := strings.Join(strings.Fields(text), " ")
			if !strings.Contains(flat, tc.evidence) || !strings.Contains(flat, "Scheduler: "+tc.reason) || !strings.Contains(flat, "No start estimate reported") {
				t.Fatal(text)
			}
			for _, line := range strings.Split(text, "\n") {
				if ansi.StringWidth(line) > width {
					t.Fatalf("clipped guidance: %q", line)
				}
			}
		}
		if j.State != "PENDING" || !j.EstimatedStart.IsZero() {
			t.Fatal("guidance fabricated a transition or estimate")
		}
	}
}

func TestUnknownPendingReasonVisibleInGlanceWithoutGuessing(t *testing.T) {
	m := demoModel(120, 40)
	defer m.Close()
	j := domain.Job{ID: "123", Name: "queued", State: "PENDING", Reason: "SiteSpecificReason"}
	for _, height := range []int{4, 15} {
		text := ansi.Strip(m.glance(j, 50, height))
		if !strings.Contains(text, "Queued · SiteSpecificReason") || !strings.Contains(text, "full details") {
			t.Fatal(text)
		}
	}
	for _, reason := range []string{"", "None"} {
		if pendingSummary(reason) != "Queued" {
			t.Fatal("missing reason should remain neutral")
		}
	}
}

func TestPendingEstimatesRemainTentativeAndExpiredEstimatesStayQueued(t *testing.T) {
	m := demoModel(120, 40)
	defer m.Close()
	for _, tc := range []struct {
		start       time.Time
		label, note string
	}{
		{time.Time{}, "No start estimate reported", ""},
		{m.now.Add(time.Hour), "Tentative start:", "timing can change"},
		{m.now.Add(-time.Minute), "Past estimate:", "Still queued"},
	} {
		j := domain.Job{ID: "12", State: "PENDING", Reason: "Priority", EstimatedStart: tc.start}
		for _, width := range []int{40, 60} {
			text := ansi.Strip(m.details(j, width))
			if !strings.Contains(text, tc.label) || !strings.Contains(text, tc.note) || !strings.Contains(text, "Scheduler: Priority") {
				t.Fatal(text)
			}
			glance := ansi.Strip(m.glance(j, width, 15))
			if !strings.Contains(glance, tc.label) {
				t.Fatal(glance)
			}
		}
		if j.State != "PENDING" || !j.Start.IsZero() {
			t.Fatal("presentation changed execution state")
		}
	}
}
