package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
)

func arrayFinished(a domain.ArraySummary) int64 {
	return a.Completed + a.Failed + a.Cancelled
}

// Finished measures progress through the workload, not success. Textual counts
// accompany the colored segments so the distinction survives no-color mode.
func arrayProgress(a domain.ArraySummary, width int) []string {
	if a.SummaryError != "" {
		rows := strings.Split(ansi.Wrap("Array summary unavailable: "+a.SummaryError, max(1, width), ""), "\n")
		for i := range rows {
			rows[i] = yellow.Render(rows[i])
		}
		return rows
	}
	label := fmt.Sprintf("%d / %d observed tasks finished", arrayFinished(a), a.Known)
	if ansi.StringWidth(label) > width {
		label = fmt.Sprintf("%d/%d finished · observed", arrayFinished(a), a.Known)
	}
	lines := []string{arrayProgressBar(a, width), muted.Render(label)}
	// Failure actions and unknown outcomes take precedence in a short glance.
	if a.Failed > 0 {
		lines = append(lines, red.Render(fmt.Sprintf("%d failed · y copies IDs", a.Failed)))
	}
	if a.Unresolved > 0 {
		lines = append(lines, yellow.Render(fmt.Sprintf("%d awaiting accounting", a.Unresolved)))
	}
	var outcomes []string
	if a.Completed > 0 {
		outcomes = append(outcomes, green.Render(fmt.Sprintf("%d succeeded", a.Completed)))
	}
	if a.Cancelled > 0 {
		outcomes = append(outcomes, muted.Render(fmt.Sprintf("%d cancelled", a.Cancelled)))
	}
	if len(outcomes) > 0 {
		lines = append(lines, strings.Join(outcomes, muted.Render(" · ")))
	}
	var active []string
	if a.Running > 0 {
		active = append(active, green.Render(fmt.Sprintf("%d running", a.Running)))
	}
	if a.Pending > 0 {
		active = append(active, yellow.Render(fmt.Sprintf("%d queued", a.Pending)))
	}
	if len(active) > 0 {
		lines = append(lines, strings.Join(active, muted.Render(" · ")))
	}
	return lines
}

func arrayProgressBar(a domain.ArraySummary, width int) string {
	width = max(1, width)
	if a.Known <= 0 {
		return border.Render(strings.Repeat("─", width))
	}
	// Cumulative boundaries keep the bar exactly width cells, with no rounding
	// gaps. A tiny segment can be sub-cell; its exact count remains in the text.
	finished := min(a.Known, max(int64(0), arrayFinished(a)))
	complete := min(finished, max(int64(0), a.Completed))
	failed := min(finished, complete+max(int64(0), a.Failed))
	cell := func(n int64) int { return int(float64(n) / float64(a.Known) * float64(width)) }
	c, f, done := cell(complete), cell(failed), cell(finished)
	return green.Render(strings.Repeat("━", c)) + red.Render(strings.Repeat("━", f-c)) +
		muted.Render(strings.Repeat("━", done-f)) + border.Render(strings.Repeat("─", width-done))
}
