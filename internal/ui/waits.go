package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/query"
)

func (m *Model) showWaits() tea.Cmd {
	m.input.Blur()
	m.input.SetValue("")
	m.inputError = ""
	m.help, m.palette, m.searching = false, false, false
	m.pendingLaunchFocus = ""
	m.tab = 0
	m.view = "waits"
	m.invalidateDetail()
	m.panel.GotoTop()
	return nil // Use the normal coordinator's accounting snapshot; no extra polling.
}

func (m *Model) waitsContent(width int) string {
	at := m.visibleStore().Snapshot.HistoryAt
	if at.IsZero() {
		return "Queue-wait history is not available yet.\n\nPress r to refresh or 4 for connection details."
	}
	if !at.Equal(m.waitsAt) {
		m.waitsAt = at
		m.waitsReport = query.Waits(m.visibleStore().History, at)
	}
	report := m.waitsReport
	compact := width < 60 || m.height <= 20
	rows := []string{muted.Render("Your starts · last 7 days · not a forecast"), muted.Render("Accounting snapshot: " + at.Local().Format("Jan 2 15:04:05")), ""}
	if compact {
		rows = []string{muted.Render("7-day starts · median / P90")}
	}
	summary := waitSummary
	if compact {
		summary = func(d query.WaitDistribution) string {
			return domain.Duration(time.Duration(d.MedianSeconds*float64(time.Second))) + " / " + domain.Duration(time.Duration(d.P90Seconds*float64(time.Second)))
		}
	}
	submitLabel, eligibleLabel := "Submit → start   ", "Eligible → start "
	if compact {
		submitLabel, eligibleLabel = "Submit   ", "Eligible "
	}
	if m.now.Sub(at) > 2*time.Minute {
		rows = append(rows, yellow.Render("Older snapshot · r to refresh"), "")
	}
	for _, p := range report.Partitions {
		rows = append(rows, accent.Bold(true).Render(p.Partition)+muted.Render(fmt.Sprintf(" · %d starts", p.SubmitToStart.Samples)))
		rows = append(rows, submitLabel+summary(p.SubmitToStart))
		if p.EligibleToStart == nil {
			rows = append(rows, muted.Render("Eligible → start unavailable"))
		} else {
			rows = append(rows, eligibleLabel+summary(*p.EligibleToStart), muted.Render(fmt.Sprintf("%d eligible samples · %d array tasks", p.EligibleToStart.Samples, p.ArrayTasks)))
		}
		if p.EligibleToStart == nil && p.ArrayTasks > 0 {
			rows = append(rows, muted.Render(fmt.Sprintf("%d array tasks", p.ArrayTasks)))
		}
		if p.SubmitToStart.Samples < 10 {
			rows = append(rows, yellow.Render("Small sample · treat as anecdotal"))
		}
		rows = append(rows, "")
	}
	if len(report.Partitions) == 0 {
		rows = append(rows, "No usable started jobs in this window.", "")
	}
	if compact {
		rows = append(rows, muted.Render("Accounting snapshot: "+at.Local().Format("Jan 2 15:04:05")), "")
	}
	rows = append(rows, muted.Render(fmt.Sprintf("%d accounting records excluded", report.Excluded)), "", ink.Bold(true).Render("Reading the numbers"), muted.Render("Median: middle wait. P90: 90% waited no longer."), "", muted.Render("Submit includes holds and dependencies."), muted.Render("Eligible starts when eligible; other delays can remain."), "", muted.Render("Array tasks count separately and may dominate the sample."), muted.Render("Resource requests differ. These are observations, not predictions."))
	return ansi.Wordwrap(strings.Join(rows, "\n"), width, " ")
}
func waitSummary(d query.WaitDistribution) string {
	format := func(seconds float64) string { return domain.Duration(time.Duration(seconds * float64(time.Second))) }
	return ink.Render(format(d.MedianSeconds)) + muted.Render(" median · ") + ink.Render(format(d.P90Seconds)) + muted.Render(" P90")
}
