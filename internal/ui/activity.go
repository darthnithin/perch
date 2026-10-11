package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"perch/internal/app"
	"perch/internal/domain"
)

// Activity is session evidence, never reconstructed from cached job history.
// A key holds selection steady while fresh events arrive ahead of it.
type activityState struct {
	key                string
	returning          bool
	tab                int
	search, selectedID string
}

func (m *Model) showActivity() tea.Cmd {
	m.leaveInput()
	m.input.SetValue("")
	m.help, m.palette = false, false
	m.pendingLaunchFocus = ""
	m.activity.returning = false
	m.view = "activity"
	m.invalidateDetail()
	return nil
}

func (m *Model) activityIndex() int {
	for i := len(m.store.Events) - 1; i >= 0; i-- {
		if m.store.Events[i].Key == m.activity.key {
			return len(m.store.Events) - 1 - i
		}
	}
	return 0
}

func (m *Model) selectActivity(index int) {
	n := len(m.store.Events)
	if n == 0 {
		m.activity.key = ""
		return
	}
	index = max(0, min(n-1, index))
	m.activity.key = m.store.Events[n-1-index].Key
}

type activityLayout struct{ header, stride, visible, start int }

func (m *Model) activityGeometry() activityLayout {
	h := m.layout().bodyHeight - 2
	g := activityLayout{header: 1, stride: 2}
	if h >= 12 {
		g.header, g.stride = 2, 3
	}
	g.visible = max(1, (h-g.header-1)/g.stride)
	g.start = max(0, m.activityIndex()-g.visible+1)
	return g
}

func (m *Model) activityContent() string {
	w, h := m.width-8, m.layout().bodyHeight-2
	rows := make([]string, h)
	rows[0] = muted.Render("Observed this session · latest 100")
	n := len(m.store.Events)
	if n == 0 {
		rows[1] = ink.Render("No new activity yet.")
		if h > 5 {
			rows[3] = muted.Render("Finishes, failures, cancellations and time warnings appear here.")
			rows[4] = muted.Render("Existing history is not new activity.")
		}
		rows[h-1] = muted.Render("Watching through normal refreshes")
		if !m.connected {
			rows[h-1] = muted.Render("Connect to start observing jobs")
		}
		return strings.Join(rows, "\n")
	}
	g := m.activityGeometry()
	for i := g.start; i < min(n, g.start+g.visible); i++ {
		e := m.store.Events[n-1-i]
		label, color := domain.Label(e.Kind), muted
		switch e.Kind {
		case "finished":
			label, color = "Finished", green
		case "failed":
			label, color = "Failed", red
		case "cancelled":
			label, color = "Cancelled", muted
		case "time warning":
			label, color = "Time limit near", yellow
		}
		prefix := "  "
		if i == m.activityIndex() {
			prefix = "› "
		}
		line := prefix + color.Render(label) + " · " + ink.Render(domain.Label(e.Name))
		if e.TaskCount > 0 {
			noun := "tasks"
			if e.TaskCount == 1 {
				noun = "task"
			}
			line += muted.Render(fmt.Sprintf(" · %d %s", e.TaskCount, noun))
		}
		if i == m.activityIndex() {
			line = paintBackground(selected.Render(fit(line, w)), "\x1b[48;2;72;56;47m")
		}
		y := g.header + (i-g.start)*g.stride
		rows[y] = line
		outcome := domain.Label(e.State)
		if e.Kind == "time warning" && e.WarningThreshold > 0 {
			outcome = domain.Duration(e.Remaining) + " left"
		}
		rows[y+1] = muted.Render("  " + e.At.Local().Format("Jan 2 15:04") + " · #" + domain.Label(e.JobID) + " · " + outcome)
	}
	rows[h-1] = muted.Render(fmt.Sprintf("%d–%d of %d · observed local time", g.start+1, min(n, g.start+g.visible), n))
	return strings.Join(rows, "\n")
}

func (m *Model) activityJob(e app.Event) (domain.Job, bool) {
	if e.Attempt != "" {
		for _, j := range m.displayJobs() {
			if j.AttemptKey() == e.Attempt {
				return j, true
			}
		}
	}
	return domain.Job{}, false
}

func (m *Model) openActivityJob() tea.Cmd {
	if len(m.store.Events) == 0 {
		return nil
	}
	e := m.store.Events[len(m.store.Events)-1-m.activityIndex()]
	j, ok := m.activityJob(e)
	if !ok {
		m.setNotice("That job attempt is no longer loaded. Activity is retained.")
		return nil
	}
	m.activity.tab, m.activity.search, m.activity.selectedID = m.tab, m.search, m.selectedID
	m.activity.returning = true
	target := 0
	if j.Terminal() {
		target = 1
	}
	m.switchTab(target)
	m.search, m.selectedID = "", j.ID
	m.normalizeSelection()
	return m.setView("inspect")
}

func (m *Model) returnToActivity() tea.Cmd {
	m.switchTab(m.activity.tab)
	m.search, m.selectedID = m.activity.search, m.activity.selectedID
	m.normalizeSelection()
	return m.showActivity()
}

func (m *Model) activityKey(key string) (tea.Cmd, bool) {
	i := m.activityIndex()
	switch key {
	case "down", "j":
		i++
	case "up", "k":
		i--
	case "pgdown":
		i += m.activityGeometry().visible
	case "pgup":
		i -= m.activityGeometry().visible
	case "home":
		i = 0
	case "end":
		i = len(m.store.Events) - 1
	case "enter":
		return m.openActivityJob(), true
	default:
		return nil, false
	}
	m.selectActivity(i)
	return nil, true
}
