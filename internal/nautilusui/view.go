package nautilusui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/nautilus"
)

var ink = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECE6DF"))
var muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#A39B93"))
var accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6AD8F"))
var warning = lipgloss.NewStyle().Foreground(lipgloss.Color("#DFC083"))
var selected = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF0E5")).Background(lipgloss.Color("#48382F"))

func stateStyle(state string) lipgloss.Style {
	switch state {
	case "Running", "Complete", "Succeeded":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A9C99A"))
	case "Failed", "Failing":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#DE9485"))
	case "Pending", "Completing", "Deleting":
		return warning
	default:
		return muted
	}
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
func clean(s string) string { return domain.Clean(s) }
func count(n *int64) string {
	if n == nil {
		return "unknown"
	}
	return fmt.Sprint(*n)
}
func flag(b *bool) string {
	if b == nil {
		return "unknown"
	}
	return fmt.Sprint(*b)
}
func (m *Model) tabs() []string {
	if m.width < 64 {
		return []string{"1 Jobs", "2 Pods", "3 Scope", "4 Connect"}
	}
	return []string{"1 Jobs", "2 Pods", "3 Namespace", "4 Connection"}
}
func (m *Model) visibleRows() int { return max(1, m.height-17) }
func (m *Model) listStart() int   { return max(0, m.selection[m.tab]-m.visibleRows()+1) }
func frame(title, body string, w, h int) string {
	title = ansi.Truncate(" "+title+" ", w-4, "")
	lines := []string{accent.Render("╭─" + title + strings.Repeat("─", max(0, w-3-ansi.StringWidth(title))) + "╮")}
	content := strings.Split(body, "\n")
	for i := 0; i < h-2; i++ {
		s := ""
		if i < len(content) {
			s = content[i]
		}
		lines = append(lines, accent.Render("│")+" "+fit(s, w-4)+" "+accent.Render("│"))
	}
	lines = append(lines, accent.Render("╰"+strings.Repeat("─", w-2)+"╯"))
	return strings.Join(lines, "\n")
}
func (m *Model) View() tea.View {
	if m.width < 48 || m.height < 18 {
		v := tea.NewView("perch · Nautilus\n\nResize to at least 48 × 18.\nq to quit")
		v.AltScreen = true
		return v
	}
	w, h := m.width-4, m.height-12
	tabs := m.tabs()
	for i := range tabs {
		if i == m.tab {
			tabs[i] = accent.Bold(true).Render(tabs[i])
		} else {
			tabs[i] = muted.Render(tabs[i])
		}
	}
	title, body := "Namespace workloads", ""
	if m.help {
		title = "Keyboard & commands"
		body = m.panel.View()
	} else if m.logs != nil {
		title, body = m.logBody(w, h)
	} else if m.inspect {
		title = "Object details"
		body = m.panel.View()
	} else if m.tab >= 2 {
		title = []string{"Namespace", "Connection"}[m.tab-2]
		body = m.panel.View()
	} else {
		title = []string{"Namespace Jobs", "Namespace Pods"}[m.tab]
		rows := m.rows(m.tab)
		stateWidth := min(14, w/3)
		lines := []string{muted.Render("  " + fit("NAME", w-6-stateWidth) + "STATE"), ""}
		if len(rows) == 0 {
			text := "No objects in this snapshot."
			if (m.tab == 0 && m.jobs == nil) || (m.tab == 1 && m.pods == nil) {
				text = "Waiting for a successful namespace read."
			}
			if m.filters[m.tab] != "" {
				text = "No matches. Esc clears this filter."
			}
			lines = append(lines, ansi.Wrap(text, w-4, " "))
		}
		start := m.listStart()
		for i := start; i < min(len(rows), start+m.visibleRows()); i++ {
			r := rows[i]
			pointer := "  "
			if i == m.selection[m.tab] {
				pointer = "› "
			}
			line := pointer + fit(r.ref.Name, w-6-stateWidth) + fit(r.state, stateWidth)
			if i == m.selection[m.tab] {
				line = selected.Render(line)
			} else {
				line = pointer + fit(r.ref.Name, w-6-stateWidth) + stateStyle(r.state).Render(fit(r.state, stateWidth))
			}
			lines = append(lines, line)
		}
		for len(lines) < h-3 {
			lines = append(lines, "")
		}
		position := "0 objects"
		if len(rows) > 0 {
			position = fmt.Sprintf("%d–%d of %d · Enter to inspect", start+1, min(len(rows), start+m.visibleRows()), len(rows))
		}
		lines = append(lines, muted.Render(position))
		body = strings.Join(lines, "\n")
	}
	boxed := frame(title, body, w, h)
	panel := &m.panel
	panelVisible := m.help || m.logs == nil && (m.inspect || m.tab >= 2)
	if m.logs != nil && m.logs.stage == "output" && !m.help {
		panel = &m.logs.panel
		panelVisible = true
	}
	if panelVisible && panel.TotalLineCount() > panel.Height() {
		position := fmt.Sprintf(" %d–%d / %d ", panel.YOffset()+1, min(panel.TotalLineCount(), panel.YOffset()+panel.Height()), panel.TotalLineCount())
		if !panel.AtTop() {
			position = " ↑" + position
		}
		if !panel.AtBottom() {
			position += "↓ "
		}
		lines := strings.Split(boxed, "\n")
		lines[len(lines)-1] = accent.Render("╰"+strings.Repeat("─", max(0, w-2-ansi.StringWidth(position)))) + muted.Render(position) + accent.Render("╯")
		boxed = strings.Join(lines, "\n")
	}
	lines := []string{"", accent.Bold(true).Render("perch") + muted.Render("  │  Nautilus · ") + ink.Render(m.scope.Namespace), strings.Join(tabs, "  "), ""}
	lines = append(lines, strings.Split(boxed, "\n")...)
	lines = append(lines, m.status(), "")
	lines = append(lines, strings.Split(frame("", m.input.View(), w, 3), "\n")...)
	hint := m.notice
	if hint == "" {
		hint = "/jobs  /pods  /namespace  /connection"
	}
	if m.tab < 2 && m.filters[m.tab] != "" && m.notice == "" {
		hint = "Filter: " + m.filters[m.tab] + " · Esc clears"
		if m.help || m.inspect {
			hint = "Filter: " + m.filters[m.tab] + " · Esc returns"
		}
	}
	if m.logs != nil && !m.help && m.notice == "" {
		hint = "Choose a Pod · Esc returns"
		if m.logs.stage == "containers" {
			hint = "Choose a container · Esc returns"
		}
		if l := m.logs; l.stage == "output" {
			hint = "Snapshot · up to 256 KiB / 2000 lines"
			if l.metrics {
				hint = m.metricsHint()
			}
			if l.err != "" && (!l.metrics || l.metricsSnapshot == nil) {
				hint = l.err
			}
		}
	}
	lines = append(lines, muted.Render(hint), muted.Render(strings.Repeat("─", w)), muted.Render(m.footer()))

	for i := range lines {
		lines[i] = "  " + fit(lines[i], w) + "  "
	}
	v := tea.NewView(m.hoverView(strings.Join(lines, "\n")))
	v.AltScreen = true
	v.WindowTitle = "Perch · Nautilus"
	v.BackgroundColor = lipgloss.Color("#1C1B1A")
	v.ForegroundColor = lipgloss.Color("#ECE6DF")
	if !m.options.NoMouse {
		v.MouseMode = tea.MouseModeAllMotion
		v.ReportFocus = true
	}
	return v
}

func (m *Model) status() string {
	if m.connecting {
		return warning.Render("Signing in · retained snapshots")
	}
	if m.authRequired {
		return warning.Render("Sign-in required · c reconnect")
	}
	states := []string{}
	for i, name := range []string{"Jobs", "Pods"} {
		err, seen := m.jobsErr, m.jobs != nil
		if i == 1 {
			err, seen = m.podsErr, m.pods != nil
		}
		state := "current"
		if !seen {
			state = "unavailable"
		}
		if err != nil && seen {
			state = "stale"
		}
		states = append(states, name+" "+state)
	}
	text := strings.Join(states, " · ")
	if m.busy {
		text += " · refreshing"
	}
	if m.options.Demo {
		text = "demo · " + text
	}
	if m.quotasErr != nil {
		if m.quotas != nil {
			text += " · quota stale"
		} else {
			text += " · quota unavailable"
		}
	}
	if m.jobsErr != nil || m.podsErr != nil || m.quotasErr != nil {
		if m.width >= 80 {
			text += " · 4 details"
		}
		return warning.Render(text)
	}
	return muted.Render(text)
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return "not observed"
	}
	return t.Format(time.RFC3339)
}

func (m *Model) panelText() string {
	if m.help {
		return "Move around\n\n1 / 2 / 3 / 4  Jobs, Pods, Namespace, Connection\nTab / Shift+Tab  Next / previous tab\n↑ ↓ / j k  Select an object or scroll details\nEnter  Inspect selected object\nl  Choose Pod/container logs\nu  Choose Pod resource metrics\np  Switch current/previous output in logs\nHome / End  First / last row\nf  Filter this resource list\nEsc  Return / clear filter\nClick / wheel  Select / scroll; footer shortcuts are clickable\n\nCommands\n\n/jobs  /pods  /namespace  /connection\n/quotas  Namespace quota usage and limits\n/logs  Choose a container log snapshot\n/metrics  On-demand Pod CPU and memory\n/find words  Search names and state\n/refresh  Reread the open snapshot or namespace\n/connect  Explicit foreground sign-in\n/help  Show this reference\n/quit  Restore the terminal and exit\n\nThis experimental workspace is read-only. Namespace quota is on tab 3. Pod metrics require a served Metrics API and permission. Workload actions are not available yet. Logs are bounded snapshots; r reads again."
	}
	if m.tab == 2 {
		return m.quotaText()
	}
	if m.tab == 3 {
		label := m.options.Context
		if m.options.Demo {
			label = "Offline demo · synthetic data"
		}
		lines := []string{ink.Bold(true).Render(clean(label)), "Namespace: " + m.scope.Namespace, "Cluster identity: " + m.scope.Cluster, "", "Jobs"}
		if m.jobs != nil {
			lines = append(lines, "Observed: "+stamp(m.jobs.ObservedAt), "Resource version: "+m.jobs.ResourceVersion)
		}
		if m.jobsErr != nil {
			lines = append(lines, warning.Render(m.jobsErr.Error()))
		}
		lines = append(lines, "", "Pods")
		if m.pods != nil {
			lines = append(lines, "Observed: "+stamp(m.pods.ObservedAt), "Resource version: "+m.pods.ResourceVersion)
		}
		if m.podsErr != nil {
			lines = append(lines, warning.Render(m.podsErr.Error()))
		}
		lines = append(lines, "", "Quota")
		if m.quotas != nil {
			lines = append(lines, "Observed: "+stamp(m.quotas.ObservedAt), "Resource version: "+m.quotas.ResourceVersion)
		}
		if m.quotasErr != nil {
			lines = append(lines, warning.Render(m.quotasErr.Error()))
		}
		lines = append(lines, "", "Refresh: 15 seconds, backing off to 2 minutes on failures.", "Authentication failures wait for explicit sign-in. Background queries never launch a credential plugin.", "", "r refresh · c reconnect / sign in")
		return strings.Join(lines, "\n")
	}
	if !m.inspect || m.tab > 1 {
		return ""
	}
	uid := m.selectedUID[m.tab]
	if m.tab == 0 && m.jobs != nil {
		for _, j := range m.jobs.Items {
			if j.UID == uid {
				return m.jobDetails(j)
			}
		}
	}
	if m.tab == 1 && m.pods != nil {
		for _, p := range m.pods.Items {
			if p.UID == uid {
				return m.podDetails(p)
			}
		}
	}
	return "Object is no longer in this snapshot."
}

func conditions(items []nautilus.Condition) []string {
	var lines []string
	for _, c := range items {
		lines = append(lines, c.Type+" = "+c.Status+" · "+c.Reason, clean(c.Message))
	}
	return lines
}
func (m *Model) jobDetails(j nautilus.Job) string {
	lines := []string{ink.Bold(true).Render(j.Name), j.State(), "UID: " + j.UID, ""}
	if m.jobsErr != nil {
		lines = append(lines, warning.Render("Stale Job snapshot · "+m.jobsErr.Error()), "")
	}
	lines = append(lines, "Active Pods: "+count(j.Active), "Succeeded Pods: "+count(j.Succeeded), "Failed Pods: "+count(j.Failed), "Parallelism: "+count(j.Parallelism), "Completions requested: "+count(j.Completions), "Completion mode: "+j.CompletionMode)
	if j.CompletedIndexes != "" {
		lines = append(lines, "Completed indexes: "+j.CompletedIndexes)
	}
	if j.FailedIndexes != "" {
		lines = append(lines, "Failed indexes: "+j.FailedIndexes)
	}
	lines = append(lines, "", "Pod counts are not distinct completed indexes.", "", "Controller conditions")
	lines = append(lines, conditions(j.Conditions)...)
	lines = append(lines, "", "Owned Pods · matched by controller UID")
	if m.pods == nil {
		lines = append(lines, "Pod snapshot unavailable.")
	} else {
		if m.podsErr != nil {
			lines = append(lines, warning.Render("Stale Pod snapshot · "+m.podsErr.Error()))
		}
		pods := nautilus.PodsForJob(j, m.pods.Items)
		if len(pods) == 0 {
			lines = append(lines, "No matching Pods observed in this snapshot.")
		}
		for _, p := range pods {
			lines = append(lines, p.Name+" · "+p.Phase)
		}
	}
	return strings.Join(lines, "\n")
}
func (m *Model) podDetails(p nautilus.Pod) string {
	lines := []string{ink.Bold(true).Render(p.Name), p.Phase + " · " + p.Reason, "UID: " + p.UID, "Node: " + p.Node, clean(p.Message), ""}
	if m.podsErr != nil {
		lines = append(lines, warning.Render("Stale Pod snapshot · "+m.podsErr.Error()), "")
	}
	for _, c := range p.Containers {
		lines = append(lines, accent.Bold(true).Render(c.Name+" · "+c.Role), "Image: "+c.Image, "Ready: "+flag(c.Ready)+" · restarts: "+count(c.Restarts), "Current: "+c.State.Phase+" · "+c.State.Reason)
		if c.State.ExitCode != nil {
			lines = append(lines, "Current exit: "+count(c.State.ExitCode))
		}
		if c.State.Message != "" {
			lines = append(lines, clean(c.State.Message))
		}
		lines = append(lines, "Previous: "+c.Last.Phase+" · "+c.Last.Reason)
		if c.Last.ExitCode != nil {
			lines = append(lines, "Previous exit: "+count(c.Last.ExitCode))
		}
		lines = append(lines, "")
	}
	lines = append(lines, "Pod conditions")
	lines = append(lines, conditions(p.Conditions)...)
	return strings.Join(lines, "\n")
}
