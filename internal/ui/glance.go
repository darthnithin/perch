package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
)

// glance is a bounded summary, not a cropped full inspector. Lifecycle and
// array progress take priority; Enter always reaches the scrollable details.
func (m *Model) glance(j domain.Job, width, height int) string {
	if height < 3 {
		return ""
	}
	status := stateLabel(j)
	id := ansi.Truncate("#"+j.ID, max(1, width-ansi.StringWidth(status)-2), "…")
	lines := []string{ink.Bold(true).Render(j.Name), joinLR(muted.Render(id), status, width)}
	add := func(block ...string) bool {
		if len(lines)+len(block) > height-1 {
			return false
		}
		lines = append(lines, block...)
		return true
	}
	var array *domain.ArraySummary
	for _, a := range m.arraySummaries() {
		if j.ArrayID > 0 && a.ID == j.ArrayID {
			copy := a
			array = &copy
			break
		}
	}
	if height < 10 {
		if array != nil && array.SummaryError != "" {
			add(yellow.Render("Array summary unavailable"))
		} else if array != nil {
			summary := fmt.Sprintf("%d/%d finished", arrayFinished(*array), array.Known)
			style := muted
			if array.Failed > 0 {
				summary = fmt.Sprintf("%d failed · ", array.Failed) + summary
				style = red
			} else if array.Unresolved > 0 {
				summary = fmt.Sprintf("%d unconfirmed · ", array.Unresolved) + summary
				style = yellow
			}
			add(style.Render(summary))
		} else if j.AwaitingTasks > 0 {
			add(yellow.Render(fmt.Sprintf("At least %d tasks unconfirmed", j.AwaitingTasks)))
		} else if j.State == "PENDING" {
			add(yellow.Render(pendingSummary(j.Reason)))
		} else if j.State == "UNKNOWN" {
			add(yellow.Render("Outcome not yet confirmed"))
		} else if j.Running() {
			if left, ok := j.Remaining(); ok {
				style := muted
				if m.options.TimeWarnings.Near(j) {
					style = yellow
				}
				add(style.Render(domain.Duration(left) + " remaining · recorded"))
			}
		}
	} else {
		add("")
		switch {
		case j.AwaitingTasks > 0:
			add(yellow.Render(fmt.Sprintf("At least %d tasks lack current records.", j.AwaitingTasks)))
		case j.State == "PENDING":
			add(yellow.Render(pendingSummary(j.Reason)))
			estimate, _ := pendingEstimate(j, m.now)
			add(muted.Render(estimate))
		case j.State == "UNKNOWN":
			add(yellow.Render("Outcome not yet confirmed"))
		case j.Running():
			if left, ok := j.Remaining(); ok {
				style := muted
				if m.options.TimeWarnings.Near(j) {
					style = yellow
				}
				add(progress(int(j.Elapsed.Seconds()), int(j.Limit.Seconds()), width), style.Render(domain.Duration(left)+" remaining · at last refresh"))
			} else {
				add(muted.Render("Elapsed " + domain.Duration(j.Elapsed)))
			}
		case j.Failed():
			diagnosis := m.jobDiagnosis(j)
			if diagnosis.Summary != "" {
				add(red.Render(diagnosis.Summary))
			}
		case j.Terminal():
			add(muted.Render("Elapsed " + domain.Duration(j.Elapsed)))
		}
		if array != nil {
			rows := arrayProgress(*array, width)
			essential := 4 // blank, heading, bar, total
			if array.Failed > 0 {
				essential++
			}
			if array.Unresolved > 0 {
				essential++
			}
			if array.SummaryError != "" {
				essential = 3
			}
			available := height - 1 - len(lines)
			// Drop decoration before dropping failure or unconfirmed counts.
			if available >= essential {
				add("")
			} else {
				essential--
			}
			if available < essential && array.SummaryError == "" {
				rows = rows[1:]
				essential--
			}
			if available >= essential {
				add(ink.Bold(true).Render(fmt.Sprintf("Array %d", array.ID)))
			}
			for _, line := range rows {
				add(line)
			}
		}
		if j.AwaitingTasks == 0 {
			add("")
			add(muted.Render("Partition    ") + ink.Render(j.Partition))
			cpuLabel := "CPUs"
			if j.CPUs == 1 {
				cpuLabel = "CPU"
			}
			resources := fmt.Sprintf("%d %s", j.CPUs, cpuLabel)
			if j.RequestedMemory != nil {
				resources += " · " + domain.Bytes(*j.RequestedMemory) + " requested"
			}
			add(ink.Render(resources))
			for _, line := range glanceGPURequests(j.GPURequests, width, height-1-len(lines)) {
				add(line)
			}
			if j.GPURequestNote != "" {
				add(yellow.Render("GPU request details unavailable"))
			}
			if j.Restarts > 0 {
				add(muted.Render(fmt.Sprintf("Restarts     %d", j.Restarts)))
			}
			if j.Nodes != "" {
				add(muted.Render("Node         ") + ink.Render(j.Nodes))
			}
			if j.State == "PENDING" && j.Limit > 0 {
				add(muted.Render("Time limit   ") + ink.Render(domain.Duration(j.Limit)))
			}
		}
	}
	for len(lines) < height-1 {
		lines = append(lines, "")
	}
	footer := " / click full details"
	if m.options.NoMouse {
		footer = " full details"
	}
	lines = append(lines, accent.Render("enter")+muted.Render(footer))
	for i := range lines {
		lines[i] = fit(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

// A request's scope must remain visible alongside its count/type. Reserve an
// overflow hint before adding whole wrapped requests; never clip halfway through
// a constraint or silently present a partial list as the complete request.
func glanceGPURequests(requests []domain.GPURequest, width, rows int) []string {
	if len(requests) == 0 || rows <= 0 || width <= 0 {
		return nil
	}
	blocks := make([][]string, len(requests))
	total := 0
	for i, request := range requests {
		blocks[i] = strings.Split(ansi.Wrap(request.Label()+" requested", width, ""), "\n")
		total += len(blocks[i])
	}
	budget := rows
	if total > rows {
		budget--
	}
	var lines []string
	shown := 0
	for _, block := range blocks {
		if len(lines)+len(block) > budget {
			break
		}
		for _, line := range block {
			lines = append(lines, ink.Render(line))
		}
		shown++
	}
	if shown < len(requests) {
		count := len(requests) - shown
		noun := "requests"
		if count == 1 {
			noun = "request"
		}
		prefix := ""
		if shown > 0 {
			prefix = "+"
		}
		lines = append(lines, muted.Render(fmt.Sprintf("%s%d GPU %s · enter", prefix, count, noun)))
	}
	return lines
}
