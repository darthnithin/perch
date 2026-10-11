package ui

import (
	"cmp"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
)

// Sorting belongs to each visible list, not to the reconciled scheduler store.
// The zero value preserves the existing running-first order and array gaps.
type jobSort struct {
	key        string
	descending bool
}

func defaultJobSort(key string) jobSort {
	if key == "default" {
		return jobSort{}
	}
	return jobSort{key: key, descending: key != "name" && key != "state"}
}
func parseJobSort(arg string) (jobSort, bool) {
	fields := strings.Fields(arg)
	if len(fields) < 1 || len(fields) > 2 {
		return jobSort{}, false
	}
	switch fields[0] {
	case "default", "name", "state", "elapsed", "cpus", "memory", "submitted":
	default:
		return jobSort{}, false
	}
	order := defaultJobSort(fields[0])
	if len(fields) == 2 {
		if order.key == "" || fields[1] != "asc" && fields[1] != "desc" {
			return jobSort{}, false
		}
		order.descending = fields[1] == "desc"
	}
	return order, true
}
func (s jobSort) label() string {
	if s.key == "" {
		return "default order"
	}
	arrow := " ↑"
	if s.descending {
		arrow = " ↓"
	}
	return s.key + arrow
}
func (m *Model) currentJobSort() jobSort {
	if m.tab < 0 || m.tab > 1 {
		return jobSort{}
	}
	return m.jobSorts[m.tab]
}
func (m *Model) applyJobSort(order jobSort) tea.Cmd {
	if m.tab > 1 {
		m.switchTab(0)
	}
	if j, ok := m.selected(); ok {
		m.selectedID = j.ID
	}
	m.jobSorts[m.tab] = order
	m.pendingLaunchFocus = ""
	m.normalizeSelection()
	m.activity.returning = false
	m.help, m.palette = false, false
	return m.setView("details")
}
func (m *Model) toggleJobSort(key string) tea.Cmd {
	order := defaultJobSort(key)
	if current := m.currentJobSort(); current.key == key {
		order.descending = !current.descending
	}
	return m.applyJobSort(order)
}

// Missing values always follow known ones, in either direction. Array rows
// keep their compact representation; requested resources are per record.
func sortJobRows(rows []domain.Job, order jobSort) {
	if order.key == "" {
		return
	}
	known := func(j domain.Job) bool {
		if j.AwaitingTasks > 0 {
			return false
		}
		switch order.key {
		case "name":
			return j.Name != ""
		case "state":
			return j.State != "" && j.State != "UNKNOWN"
		case "elapsed":
			return j.Elapsed >= 0 && j.State != "" && j.State != "PENDING" && j.State != "UNKNOWN"
		case "cpus":
			return j.CPUs > 0
		case "memory":
			return j.RequestedMemory != nil && *j.RequestedMemory >= 0
		case "submitted":
			return !j.Submit.IsZero()
		}
		return false
	}
	sort.SliceStable(rows, func(i, k int) bool {
		a, b := rows[i], rows[k]
		ak, bk := known(a), known(b)
		if ak != bk {
			return ak
		}
		if !ak {
			return false
		}
		var comparison int
		switch order.key {
		case "name":
			comparison = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case "state":
			comparison = strings.Compare(a.State, b.State)
		case "elapsed":
			comparison = cmp.Compare(a.Elapsed, b.Elapsed)
		case "cpus":
			comparison = cmp.Compare(a.CPUs, b.CPUs)
		case "memory":
			comparison = cmp.Compare(*a.RequestedMemory, *b.RequestedMemory)
		case "submitted":
			comparison = a.Submit.Compare(b.Submit)
		}
		if order.descending {
			return comparison > 0
		}
		return comparison < 0
	})
}

type jobColumn struct {
	key, label string
	x, width   int
}

func (m *Model) jobColumns(width int, compact bool) []jobColumn {
	label := func(key, text string) string {
		if m.currentJobSort().key == key {
			if m.currentJobSort().descending {
				return text + " ↓"
			}
			return text + " ↑"
		}
		return text
	}
	if compact {
		name, state := label("name", "JOB / NAME"), label("state", "STATE")
		return []jobColumn{{"name", name, 0, ansi.StringWidth(name)}, {"state", state, width - ansi.StringWidth(state), ansi.StringWidth(state)}}
	}
	nameWidth := max(10, width-42)
	key, title := "elapsed", "ELAPSED"
	switch m.currentJobSort().key {
	case "cpus":
		key, title = "cpus", "CPUS"
	case "memory":
		key, title = "memory", "REQ MEM"
	}
	return []jobColumn{{"", "JOB", 2, 14}, {"name", label("name", "NAME"), 16, nameWidth}, {"state", label("state", "STATE"), 16 + nameWidth, 13}, {key, label(key, title), 29 + nameWidth, 9}}
}

func (m *Model) jobMetric(j domain.Job, compact bool) string {
	if j.AwaitingTasks > 0 {
		return "—"
	}
	switch m.currentJobSort().key {
	case "cpus":
		if j.CPUs <= 0 {
			return "—"
		}
		if compact {
			return fmt.Sprintf("%d CPUs", j.CPUs)
		}
		return fmt.Sprint(j.CPUs)
	case "memory":
		if j.RequestedMemory == nil || *j.RequestedMemory < 0 {
			return "—"
		}
		value := domain.Bytes(*j.RequestedMemory)
		if compact {
			value += " requested"
		}
		return value
	}
	if j.State == "PENDING" || j.State == "UNKNOWN" {
		return "—"
	}
	return domain.Duration(j.Elapsed)
}
func (m *Model) jobHeading(width int, compact bool) string {
	var out strings.Builder
	column := 0
	for _, c := range m.jobColumns(width, compact) {
		out.WriteString(strings.Repeat(" ", max(0, c.x-column)))
		text := fit(c.label, c.width)
		if c.key != "" && c.key == m.currentJobSort().key {
			out.WriteString(accent.Render(text))
		} else {
			out.WriteString(muted.Render(text))
		}
		column = c.x + c.width
	}
	return out.String()
}
