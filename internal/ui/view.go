package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
)

const background = "#1C1B1A"

var (
	ink      = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECE6DF"))
	muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A39B93"))
	accent   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6AD8F"))
	green    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8CA9A"))
	yellow   = lipgloss.NewStyle().Foreground(lipgloss.Color("#DFC083"))
	red      = lipgloss.NewStyle().Foreground(lipgloss.Color("#E69D91"))
	border   = lipgloss.NewStyle().Foreground(lipgloss.Color("#57514B"))
	selected = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF0E5")).Background(lipgloss.Color("#48382F"))
)

// Retain an explicit cell background across nested foreground/style resets.
func paintBackground(s, code string) string {
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+code)
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+code)
	return code + s + "\x1b[m"
}
func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = ansi.Truncate(s, width, "…")
	return s + strings.Repeat(" ", max(0, width-ansi.StringWidth(s)))
}
func joinLR(left, right string, width int) string {
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 2 {
		return fit(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}
func box(title, content string, width, height int, focus bool) string {
	if width < 6 || height < 3 {
		return ""
	}
	color := border
	if focus {
		color = accent
	}
	title = " " + title + " "
	title = ansi.Truncate(title, width-4, "")
	top := color.Render("╭─" + title + strings.Repeat("─", max(0, width-3-ansi.StringWidth(title))) + "╮")
	if strings.TrimSpace(title) == "" {
		top = color.Render("╭" + strings.Repeat("─", width-2) + "╮")
	}
	lines := strings.Split(content, "\n")
	out := []string{top}
	for i := 0; i < height-2; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out = append(out, color.Render("│")+" "+fit(line, width-4)+" "+color.Render("│"))
	}
	out = append(out, color.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return strings.Join(out, "\n")
}
func stateLabel(j domain.Job) string {
	switch {
	case j.Running():
		return green.Render("● running")
	case j.State == "PENDING":
		return yellow.Render("◷ queued")
	case j.State == "COMPLETED":
		return muted.Render("✓ done")
	case j.State == "UNKNOWN":
		return yellow.Render("? awaiting")
	case j.State == "CANCELLED":
		return muted.Render("− cancelled")
	case j.Failed():
		return red.Render("! failed")
	}
	return muted.Render(strings.ToLower(j.State))
}

// Keep scroll position on the existing border so short terminals retain every
// content row. The line count includes wrapping, just like keyboard scrolling.
func scrollBox(title string, v viewport.Model, width, height int) string {
	return scrollPosition(box(title, v.View(), width, height, true), v, width)
}

// Also used by output/script panels, whose viewport follows a two-row heading.
func scrollPosition(out string, v viewport.Model, width int) string {
	if out == "" || v.TotalLineCount() <= v.Height() {
		return out
	}
	first, last := v.YOffset()+1, min(v.TotalLineCount(), v.YOffset()+v.Height())
	label := fmt.Sprintf(" %d–%d / %d ", first, last, v.TotalLineCount())
	if !v.AtTop() {
		label = " ↑" + label
	}
	if !v.AtBottom() {
		label += "↓ "
	}
	lines := strings.Split(out, "\n")
	lines[len(lines)-1] = accent.Render("╰"+strings.Repeat("─", max(0, width-4-ansi.StringWidth(label)))) + muted.Render(label) + accent.Render("──╯")
	return strings.Join(lines, "\n")
}

func (m *Model) View() tea.View {
	w, h := m.width, m.height
	if w < 48 || h < 18 {
		content := m.applyTheme(ink.Render("perch\n\nA little more room, please.\nResize to at least 48 × 18.\n\nq to quit"))
		if m.options.NoColor {
			content = ansi.Strip(content)
		}
		v := tea.NewView(content)
		v.AltScreen = true
		return v
	}
	layout := m.layout()
	inner, bodyHeight, chip := layout.inner, layout.bodyHeight, layout.chip
	heading := accent.Bold(true).Render("perch") + border.Render("  │  ") + ink.Render("Hummingbird workspace")
	identity := m.report.User
	if identity != "" {
		identity += " @ "
	}
	identity += strings.TrimPrefix(m.options.Host, m.report.User+"@")
	if m.options.Demo {
		identity = "demo workspace"
	}
	header := joinLR(heading, muted.Render(domain.Label(identity)), inner)
	tabs := layout.tabs
	for i := range tabs {
		if i == m.tab {
			tabs[i] = accent.Bold(true).Render(tabs[i])
		} else {
			tabs[i] = muted.Render(tabs[i])
		}
	}
	nav := strings.Join(tabs, strings.Repeat(" ", layout.tabGap))
	if inner >= 132 && bodyHeight >= 18 {
		nav = joinLR(nav, m.workloadSummary(), inner)
	}
	var body string
	if m.view == "cancel" {
		title := "Cancellation review"
		if m.cancelForm != nil && m.cancelForm.stage == "receipt" {
			title = "Cancellation receipt"
		}
		body = scrollBox(title, m.panel, inner, bodyHeight)
	} else if m.view == "shell" {
		body = scrollBox("Allocation shell", m.panel, inner, bodyHeight)
	} else if m.view == "warnings" {
		body = m.warningsView()
	} else if m.view == "settings" {
		body = box("Appearance", m.settingsContent(inner-4, bodyHeight-2), inner, bodyHeight, true)
	} else if m.view == "launch" {
		body = box("New job", m.launchContent(inner-4, bodyHeight-2), inner, bodyHeight, true)
	} else if m.help {
		body = scrollBox("Keyboard & commands", m.panel, inner, bodyHeight)
	} else if m.palette {
		var rows []string
		g := m.paletteGeometry()
		title := "Commands"
		query := muted.Render("Type to find a command…")
		if m.paletteJob != nil {
			title = "Job actions · #" + domain.Label(m.paletteJob.ID)
			query = muted.Render("Type to find an action…")
		}
		if m.paletteQuery != "" {
			query = accent.Render("› ") + ink.Render(m.paletteQuery) + accent.Render("▏")
		}
		rows = append(rows, query, "")
		matches := m.paletteCommands()
		for i := g.start; i < min(len(matches), g.start+g.visible); i++ {
			c := matches[i]
			line := m.paletteRow(c, g.rowWidth)
			if i == m.paletteIndex {
				style := selected
				if m.paletteProblem(c.key) != "" {
					style = style.Foreground(lipgloss.Color("#A39B93"))
				}
				line = style.Render("› " + ansi.Strip(line))
			} else {
				line = "  " + line
			}
			rows = append(rows, line)
		}
		if len(matches) == 0 {
			rows = append(rows, ink.Render("No matching commands."), muted.Render("Try jobs, logs, software or run."))
			if m.paletteJob != nil {
				rows = append(rows[:len(rows)-2], ink.Render("No matching actions."), muted.Render("Try output, shell or cancel."))
			}
		} else {
			rows = append(rows, muted.Render(fmt.Sprintf("%d of %d", m.paletteIndex+1, len(matches))))
		}
		body = box(title, strings.Join(rows, "\n"), g.width, g.height, true)
		if m.paletteJob != nil {
			placed := make([]string, bodyHeight)
			for i := range placed {
				placed[i] = strings.Repeat(" ", inner)
			}
			for i, line := range strings.Split(body, "\n") {
				placed[g.y-layout.bodyTop+i] = fit(strings.Repeat(" ", g.x-2)+line, inner)
			}
			body = strings.Join(placed, "\n")
		}
	} else if m.view == "activity" {
		body = box("Activity", m.activityContent(), inner, bodyHeight, true)
	} else if m.view == "waits" {
		body = scrollBox("Queue wait history", m.panel, inner, bodyHeight)
	} else if m.view == "receipts" {
		body = scrollBox("Saved submissions", m.viewport, inner, bodyHeight)
	} else if m.view == "software" {
		title := "Software"
		if m.softwareRequest.ID != "" {
			title += " · " + m.softwareRequest.ID
		}
		if m.catalogReady() {
			body = box(title, m.catalogContent(inner-4, bodyHeight-2), inner, bodyHeight, true)
		} else if m.softwareRequest.Command == "nodes" {
			body = scrollBox("Node allocations", m.viewport, inner, bodyHeight)
		} else if m.softwareRequest.Command == "priority" {
			body = scrollBox("Queue priority", m.viewport, inner, bodyHeight)
		} else {
			body = scrollBox(title, m.viewport, inner, bodyHeight)
		}
	} else if m.tab == 2 {
		body = scrollBox("Partitions", m.panel, inner, bodyHeight)
	} else if m.tab == 3 {
		body = scrollBox("Connection", m.panel, inner, bodyHeight)
	} else if m.view == "logs" || m.view == "script" || m.view == "resources" || m.view == "inspect" {
		j, ok := m.selected()
		title := "Job"
		if ok {
			title = j.ID + " · " + j.Name
		}
		content := "Select a job first."
		if ok {
			switch m.view {
			case "inspect":
				content = m.panel.View()
			case "resources":
				content = m.panel.View()
			default:
				kind := "Submitted script"
				if m.view == "logs" {
					kind = "Standard output"
					if m.logErr {
						kind = "Standard error"
					}
				}
				indicator := ""
				if m.detailBusy {
					indicator = " · reading…"
				}
				header := accent.Render(kind) + muted.Render(indicator)
				if m.view == "logs" {
					header = m.logHeading(inner - 4)
				}
				subhead := ""
				if m.compactTextReading() && m.readerSearch != nil {
					subhead = muted.Render("find ") + ink.Render(ansi.Truncate(m.readerSearch.query, inner-9, "…"))
				}
				content = header + "\n" + subhead + "\n" + m.viewport.View()
			}
			if j.AwaitingTasks > 0 {
				content = m.panel.View()
			}
		}
		body = box(title, content, inner, bodyHeight, true)
		if ok && (m.view == "inspect" || m.view == "resources" || j.AwaitingTasks > 0) {
			body = scrollBox(title, m.panel, inner, bodyHeight)
		} else if ok && (m.view == "logs" || m.view == "script") {
			body = scrollPosition(body, m.viewport, inner)
		}
	} else {
		jobsWidth, detailWidth := layout.jobsWidth, layout.detailWidth
		jobsTitle := "Your jobs"
		if m.tab == 1 {
			jobsTitle = "Recent history"
		}
		jobsTitle += fmt.Sprintf(" · %d", len(m.jobs()))
		if jobsWidth-4 >= 58 && bodyHeight >= 16 {
			jobsTitle = "" // Spacious layouts put the heading inside the panel.
		}
		pieces := []string{}
		jobsContent := m.jobList(jobsWidth-4, bodyHeight-2)
		if m.emptyWorkspace() {
			jobsTitle = "My jobs"
			if len(chip) > 0 {
				jobsTitle = "" // Chip's feet occupy the left part of this border.
			}
			jobsContent, _ = m.emptyQueueContent(bodyHeight - 2)
		}
		pieces = append(pieces, box(jobsTitle, jobsContent, jobsWidth, bodyHeight, true))
		if detailWidth > 0 {
			content := "Select a job to take a closer look."
			if j, ok := m.selected(); ok {
				content = m.glance(j, detailWidth-4, bodyHeight-2)
			}
			pieces = append(pieces, box("At a glance", content, detailWidth, bodyHeight, false))
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, intersperse(pieces, " ")...)
	}
	feedback := m.statusLine(inner)
	footer := m.footer()
	content := []string{strings.Repeat(" ", w), "  " + fit(header, inner) + "  ", "  " + fit(nav, inner) + "  "}
	if len(chip) > 1 {
		for _, line := range chip[:len(chip)-1] {
			content = append(content, "      "+line)
		}
		first, rest, _ := strings.Cut(body, "\n")
		feet := chip[len(chip)-1]
		for x := 0; x < ansi.StringWidth(feet); x++ {
			cell := ansi.Cut(feet, x, x+1)
			if strings.TrimSpace(ansi.Strip(cell)) != "" {
				first = ansi.Cut(first, 0, 4+x) + cell + ansi.Cut(first, 5+x, inner)
			}
		}
		body = first + "\n" + rest
	} else {
		content = append(content, "")
	}
	for _, line := range strings.Split(body, "\n") {
		content = append(content, "  "+line+"  ")
	}
	content = append(content, "  "+fit(feedback, inner)+"  ")
	if m.view == "cancel" {
		content = append(content, "    "+fit(m.cancellationButtonView(), inner-2), "  "+fit(m.commandHint(), inner), "  "+fit(footer, inner))
	} else if m.compactTextReading() || m.view == "warnings" || m.palette && m.paletteJob != nil || m.launchEditingProblem() {
		content = append(content, "", "  "+fit(m.commandHint(), inner), "  "+fit(footer, inner))
	} else {
		if m.view != "settings" || h != 18 {
			content = append(content, "")
		}
		inputLine := m.input.View()
		if m.view == "shell" {
			inputLine = muted.Render("Checking allocation…")
			if m.shellForm.stage == "ready" {
				inputLine = accent.Bold(true).Render("Open shell  ↵")
			}
			if m.shellForm.stage == "opening" {
				inputLine = muted.Render("Rechecking before entry…")
			}
			if m.shellForm.stage == "error" {
				inputLine = yellow.Render("Shell not opened · r retries")
			}
		}
		if m.view == "settings" {
			inputLine = accent.Render("Preview  →  Apply")
		}
		if m.view == "launch" {
			inputLine = accent.Render("Prepare  →  Review  →  Submit")
		}
		if m.textReader() && m.readerSearch != nil && !m.input.Focused() && !m.help && !m.palette {
			inputLine = m.readerSearch.input.View()
		}
		if m.searching {
			inputLine = accent.Render("> find ") + ink.Render(m.search) + accent.Render("▏")
		}
		for _, line := range strings.Split(commandBox(inputLine, inner, m.input.Focused() || m.searching || m.readerSearchEditing()), "\n") {
			content = append(content, "  "+line+"  ")
		}
		content = append(content, "  "+fit(m.commandHint(), inner)+"  ", "  "+border.Render(strings.Repeat("─", inner))+"  ", "  "+fit(footer, inner)+"  ")
	}
	for i, line := range content {
		// The canvas comes from View.BackgroundColor. Painting each cell here
		// can obscure glyph edges in terminals whose box strokes overlap rows.
		content[i] = fit(line, w)
	}
	result := strings.Join(content[:min(len(content), h)], "\n")
	result = m.hoverView(m.applyTheme(result))
	if m.options.NoColor {
		result = ansi.Strip(result)
	}
	v := tea.NewView(result)
	v.AltScreen = true
	v.WindowTitle = "Perch · Hummingbird"
	if m.mouseEnabled() {
		v.MouseMode = tea.MouseModeAllMotion
		v.ReportFocus = true
	}
	if !m.options.NoColor {
		bg, fg := m.canvasColors()
		v.BackgroundColor = lipgloss.Color(bg)
		v.ForegroundColor = lipgloss.Color(fg)
	}
	return v
}
func intersperse(parts []string, separator string) []string {
	var out []string
	for i, p := range parts {
		if i > 0 {
			out = append(out, separator)
		}
		out = append(out, p)
	}
	return out
}
func commandBox(content string, width int, focus bool) string {
	style := accent
	if focus {
		style = accent.Bold(true)
	}
	return style.Render("╭"+strings.Repeat("─", width-2)+"╮") + "\n" + style.Render("│") + " " + fit(content, width-4) + " " + style.Render("│") + "\n" + style.Render("╰"+strings.Repeat("─", width-2)+"╯")
}
func (m *Model) jobList(width, height int) string {
	jobs := m.jobs()
	if len(jobs) == 0 {
		if m.busy && m.visibleStore().Snapshot.At.IsZero() {
			return "\nConnecting to your workspace…\n\nYour jobs will appear here."
		}
		if m.lastError != "" && m.visibleStore().Snapshot.At.IsZero() {
			return "\nUnable to load your jobs.\n\nPress c to reconnect.\nPress 4 for connection details."
		}
		if m.search != "" {
			return "\nNo jobs match this filter.\n\nEsc clears the filter."
		}
		if m.tab == 1 {
			if m.visibleStore().Snapshot.HistoryAt.IsZero() {
				return "\nHistory is not available yet.\n\nPress 4 for connection details."
			}
			return "\nNo recent completed jobs.\n\nHistory covers the last seven days."
		}
		return "\nNothing in the queue.\n\nA quiet moment. Chip approves.\n\nPress 2 to browse recent history."
	}
	geometry := jobGeometry(width, height, m.selection)
	compact, roomy := geometry.compact, geometry.roomy
	visible, start := geometry.visible, geometry.start
	nameWidth := max(10, width-42)
	lines := []string{}
	basis := "active records"
	if len(jobs) == 1 {
		basis = "active record"
	}
	for _, j := range jobs {
		if j.AwaitingTasks > 0 {
			basis = "records"
			if len(jobs) == 1 {
				basis = "record"
			}
			break
		}
	}
	if roomy {
		title, count := "My jobs", fmt.Sprintf("%d %s", len(jobs), basis)
		if m.tab == 1 {
			title, count = "Recent history", "Last 7 days"
		}
		lines = append(lines, "", joinLR(ink.Bold(true).Render(title), muted.Render(count), width), "")
	}
	if compact {
		lines = append(lines, m.jobHeading(width, true))
		if geometry.header > 1 {
			lines = append(lines, "")
		}
	} else {
		lines = append(lines, m.jobHeading(width, false), "")
		if !roomy {
			lines = append(lines, "")
		}
	}
	for i := start; i < min(len(jobs), start+visible); i++ {
		j := jobs[i]
		id := j.ID
		if j.TaskRange != "" {
			id = fmt.Sprintf("%d [%d]", j.ArrayID, j.Count())
		}
		pointer := "  "
		if i == m.selection {
			pointer = "› "
		}
		var label string
		if compact {
			label = pointer + joinLR(ink.Bold(i == m.selection).Render(id), stateLabel(j), width-2)
		} else {
			elapsed := m.jobMetric(j, false)
			label = pointer + muted.Render(fit(id, 14)) + ink.Bold(i == m.selection).Render(fit(j.Name, nameWidth)) + fit(stateLabel(j), 13) + muted.Render(elapsed)
		}
		if i == m.selection {
			label = paintBackground(selected.Render(fit(label, width)), "\x1b[48;2;72;56;47m")
		}
		lines = append(lines, label)
		if compact {
			context := j.Partition
			if m.currentJobSort().key == "cpus" || m.currentJobSort().key == "memory" {
				metric := m.jobMetric(j, true)
				left := ansi.Truncate(j.Name+" · "+context, max(1, width-4-ansi.StringWidth(metric)), "…")
				lines = append(lines, "  "+muted.Render(joinLR(left, metric, width-2)))
				continue
			} else if j.State == "PENDING" {
				context += " · " + j.Reason
			} else {
				context += " · " + domain.Duration(j.Elapsed)
			}
			lines = append(lines, "  "+muted.Render(ansi.Truncate(j.Name+" · "+context, width-2, "…")))
		} else if roomy {
			lines = append(lines, "")
		}
	}
	for len(lines) < height-1 {
		lines = append(lines, "")
	}
	if m.tab == 1 {
		basis = "records · last 7 days"
	}
	lines = append(lines, muted.Render(fmt.Sprintf("%d–%d of %d %s", start+1, min(len(jobs), start+visible), len(jobs), basis)))
	return strings.Join(lines, "\n")
}
func (m *Model) workloadSummary() string {
	running, queued, failed := int64(0), int64(0), int64(0)
	for _, j := range m.visibleStore().Snapshot.Jobs {
		if j.ArrayID > 0 {
			continue
		}
		if j.Running() {
			running += j.Count()
		}
		if j.State == "PENDING" {
			queued += j.Count()
		}
		if j.Failed() {
			failed += j.Count()
		}
	}
	for _, a := range m.arraySummaries() {
		if a.SummaryError != "" {
			return yellow.Render("Array totals unavailable")
		}
		running += a.Running
		queued += a.Pending
		failed += a.Failed
	}
	parts := []string{green.Render(fmt.Sprintf("%d running", running)), yellow.Render(fmt.Sprintf("%d queued", queued))}
	if failed > 0 {
		parts = append(parts, red.Render(fmt.Sprintf("%d failed", failed)))
	}
	return strings.Join(parts, muted.Render("   ·   "))
}
func (m *Model) details(j domain.Job, width int) string {
	if j.AwaitingTasks > 0 {
		return m.arrayGapDetails(j, width)
	}
	field := func(label, value string) string { return muted.Render(fit(label, 13)) + ink.Render(value) }
	rule := border.Render(strings.Repeat("─", width))
	lines := []string{ink.Bold(true).Render(j.Name), joinLR(muted.Render("#"+j.ID), stateLabel(j), width), "", rule, ""}
	if j.State == "PENDING" {
		title, explanation := pendingMessage(j.Reason)
		lines = append(lines, yellow.Bold(true).Render(title))
		if explanation != "" {
			lines = append(lines, ansi.Wordwrap(ink.Render(explanation), width, ""))
		}
		if j.Reason != "" && j.Reason != "None" {
			lines = append(lines, muted.Render("Scheduler: "+domain.Label(j.Reason)))
		}
		estimate, note := pendingEstimate(j, m.now)
		lines = append(lines, muted.Render(estimate))
		if note != "" {
			lines = append(lines, muted.Render(ansi.Wordwrap(note, width, " ")))
		}
		lines = append(lines, "")
	}
	if j.State == "UNKNOWN" {
		lines = append(lines, yellow.Render("Outcome not yet confirmed"), muted.Render("Awaiting accounting."), "")
	}
	if j.Nodes != "" {
		lines = append(lines, field("Node", j.Nodes))
	}
	lines = append(lines, field("Partition", j.Partition), field("CPUs", fmt.Sprintf("%d", j.CPUs)))
	if j.Restarts > 0 {
		lines = append(lines, field("Restarts", fmt.Sprint(j.Restarts)))
	}
	if j.RequestedMemory != nil {
		lines = append(lines, field("Requested", domain.Bytes(*j.RequestedMemory)+" memory"))
	}
	for _, request := range j.GPURequests {
		lines = append(lines, field("GPU request", request.Label()))
	}
	if j.GPURequestNote != "" {
		lines = append(lines, yellow.Render(domain.Label(j.GPURequestNote)))
	}
	if j.AllocatedMemory != nil && (j.RequestedMemory == nil || *j.AllocatedMemory != *j.RequestedMemory) {
		lines = append(lines, field("Allocated", domain.Bytes(*j.AllocatedMemory)+" memory"))
	}
	if j.Running() || j.Terminal() {
		lines = append(lines, field("Elapsed", domain.Duration(j.Elapsed)))
	}
	if j.Limit > 0 {
		lines = append(lines, field("Time limit", domain.Duration(j.Limit)))
	}
	if j.Running() && j.Limit > 0 {
		lines = append(lines, "", progress(int(j.Elapsed.Seconds()), int(j.Limit.Seconds()), width))
		if remaining, ok := j.Remaining(); ok {
			style := muted
			if m.options.TimeWarnings.Near(j) {
				style = yellow
			}
			lines = append(lines, style.Render(domain.Duration(remaining)+" remaining · at last refresh"))
		}
	}
	if value, ok := j.Efficiency(); ok {
		label := "CPU efficiency"
		if j.EfficiencyNote() != "" {
			label = "Recorded CPU ratio"
		}
		lines = append(lines, "", rule, "", joinLR(ink.Render(label), green.Bold(true).Render(fmt.Sprintf("%.1f%%", value)), width), progress(int(min(value, 100)*100), 10000, width), muted.Render("Recorded CPU / allocated CPU time"))
	}
	if note := j.EfficiencyNote(); note != "" && j.Terminal() {
		lines = append(lines, muted.Render(ansi.Wordwrap(note, width, " ")))
	}
	if j.PeakRSS != nil {
		lines = append(lines, "", joinLR(ink.Render("Recorded peak RSS"), ink.Bold(true).Render(domain.Bytes(*j.PeakRSS)), width), muted.Render(ansi.Wordwrap(domain.Label(j.RSSScope), width, " ")))
	}
	if diagnosis := m.jobDiagnosis(j); diagnosis.Summary != "" {
		lines = append(lines, "", rule, "", red.Bold(true).Render(diagnosis.Summary), muted.Render(diagnosis.Evidence))
	}
	if j.ExitCode != nil {
		lines = append(lines, muted.Render(fmt.Sprintf("Exit code %d", *j.ExitCode)))
	}
	if j.ArrayID > 0 {
		for _, a := range m.arraySummaries() {
			if a.ID == j.ArrayID {
				lines = append(lines, "", rule, "", ink.Bold(true).Render(fmt.Sprintf("Array %d", a.ID)))
				lines = append(lines, arrayProgress(a, width)...)
				break
			}
		}
	}
	if m.view != "resources" {
		lines = append(lines, "", rule, "", accent.Render("l")+muted.Render(" output   ")+accent.Render("s")+muted.Render(" script   ")+accent.Render("u")+muted.Render(" resources"))
	}
	if width < 64 {
		compact := lines[:0]
		for _, line := range lines {
			if line != "" && line != rule {
				compact = append(compact, line)
			}
		}
		lines = compact
	}
	return ansi.Wrap(strings.Join(lines, "\n"), width, " ")
}

func (m *Model) arrayGapDetails(j domain.Job, width int) string {
	var lines []string
	if m.view != "inspect" {
		lines = append(lines, ink.Bold(true).Render(j.Name), muted.Render(fmt.Sprintf("Array %d", j.ArrayID)), "")
	}
	lines = append(lines, yellow.Bold(true).Render("Awaiting accounting"), "")
	lines = append(lines, ink.Render(fmt.Sprintf("At least %d tasks lack current records.", j.AwaitingTasks)), "", muted.Render("Leaving the queue does not confirm success or failure. Individual task details will appear when available."))
	for _, a := range m.arraySummaries() {
		if a.ID == j.ArrayID {
			lines = append(lines, "")
			lines = append(lines, arrayProgress(a, width)...)
			break
		}
	}
	lines = append(lines, "", muted.Render("Perch checks again automatically.\nOpen History for confirmed outcomes.\nSelect a task to read its output."))
	return ansi.Wordwrap(strings.Join(lines, "\n"), max(1, width), " ")
}

func progress(value, total, width int) string {
	if total <= 0 {
		return muted.Render(strings.Repeat("─", max(1, width)))
	}
	width = max(1, width)
	filled := min(width, max(0, int(float64(value)/float64(total)*float64(width))))
	return accent.Render(strings.Repeat("━", filled)) + border.Render(strings.Repeat("─", width-filled))
}
func (m *Model) partitions(width int) string {
	lines := []string{muted.Render("Allocation is scheduler capacity, not measured utilization."), ""}
	if m.visibleStore().Snapshot.PartitionsAt.IsZero() {
		return strings.Join(append(lines, "Partition data is not available yet."), "\n")
	}
	for _, p := range m.visibleStore().Snapshot.Partitions {
		lines = append(lines, joinLR(accent.Bold(true).Render(p.Name), green.Render(strings.ToLower(p.State)), width), muted.Render(fmt.Sprintf("%d nodes · %d idle nodes", p.NodeCount, p.IdleNodes)), progress(p.AllocatedCPUs, p.CPUs, min(width, 60)), ink.Render(fmt.Sprintf("%d / %d CPUs allocated · %d idle", p.AllocatedCPUs, p.CPUs, p.IdleCPUs)))
		if p.GRES != "" {
			lines = append(lines, muted.Render("GPU resources: "+p.GRES), muted.Render("Allocated: "+p.GRESUsed))
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
func (m *Model) connection() string {
	host := m.options.Host
	if m.options.Demo {
		host = "Offline demo · synthetic workloads"
	}
	lines := []string{accent.Bold(true).Render(host), "", ink.Render(m.statusText()), muted.Render(m.age())}
	if m.lastError != "" {
		lines = append(lines, "", yellow.Render(domain.Clean(m.lastError)))
	}
	if m.options.Demo {
		lines = append(lines, "", muted.Render("c restarts the demo connection."))
	} else {
		lines = append(lines, "", muted.Render("c reconnects through your system SSH client."), muted.Render("Perch does not collect or store passwords."))
		if m.options.Host != "local" {
			lines = append(lines, muted.Render("To change destination, quit and run perch setup."))
		}
	}
	lines = append(lines, "", muted.Render("User: "+m.report.User), muted.Render("Scheduler: "+m.report.Version), "")
	lines = append(lines, muted.Render(m.cacheDescription()), "")
	if m.options.Diagnostics != nil {
		lines = append(lines, muted.Render(domain.Label(m.options.Diagnostics())), "")
	}
	if m.store.ArrayDisplayLimited() {
		lines = append(lines, yellow.Render("Some missing arrays may not remain in Jobs."), muted.Render("Perch retains earlier totals for up to 512 arrays. Current queue and accounting records are still shown."), "")
	}
	if m.options.Demo {
		lines = append(lines, muted.Render("Notifications stay off in the demo."), "")
	} else if m.options.Notifier == nil {
		lines = append(lines, muted.Render("Notifications off · enable with --notify desktop"), "")
	} else {
		lines = append(lines, muted.Render(domain.Label(m.notificationStatus)), muted.Render("Only while Perch runs; recent observed jobs can catch up on restart."), "")
		if m.store.ArrayTrackingLimited() {
			lines = append(lines, yellow.Render("Some array outcomes may not trigger alerts."), muted.Render("Observation limits or missing task identity reduced array tracking; inspect History for confirmed outcomes."), "")
		}
	}
	for _, c := range m.report.Capabilities {
		status := muted.Render(c.Status)
		if c.Status == "supported" {
			status = green.Render(c.Status)
		}
		lines = append(lines, fit(c.Name, 24)+status, muted.Render("  "+c.Detail))
	}
	return strings.Join(lines, "\n")
}
func (m *Model) resources(j domain.Job, width int) string {
	if !j.Running() {
		return m.details(j, width) + "\n\n" + recordedSteps(j.Steps, width)
	}
	lines := []string{accent.Bold(true).Render("Live step measurements"), accent.Render("[ previous   ] next"), ""}
	if m.detailError != "" {
		lines = append(lines, yellow.Render(m.detailError), muted.Render("r retries this query."), "")
	}
	if len(m.live) == 0 {
		return ansi.Wordwrap(strings.Join(append(lines, "Waiting for the first sample…"), "\n"), width, " ")
	}
	if m.resourceStep == "" {
		return ansi.Wordwrap(strings.Join(append(lines, "No running step measurements reported."), "\n"), width, " ")
	}
	samples := m.resourceSamples()
	last := samples[len(samples)-1]
	lines = append(lines, accent.Bold(true).Render("Step "+m.resourceStep), muted.Render("Last sample: "+last.At.Local().Format("15:04:05")), "")
	if last.PeakRSS != nil {
		lines = append(lines, ink.Render(domain.Bytes(*last.PeakRSS))+muted.Render("  largest task peak RSS"))
	} else {
		lines = append(lines, muted.Render("RSS unavailable · no measurement reported"))
	}
	if last.CPUSeconds != nil {
		lines = append(lines, ink.Render(fmt.Sprintf("%.1f s", *last.CPUSeconds))+muted.Render("  average task CPU time"))
	} else {
		lines = append(lines, muted.Render("CPU time unavailable · no measurement reported"))
	}
	chartWidth := min(width, 80)
	lines = append(lines, "", ink.Bold(true).Render("Task peak RSS · MiB"), chart(memoryPoints(samples), chartWidth, 5), chartRange(samples, chartWidth), "", ink.Bold(true).Render("Average task CPU time · seconds"), chart(cpuPoints(samples), chartWidth, 5), chartRange(samples, chartWidth), "", muted.Render(sampleCount(samples)), muted.Render("Gaps indicate missing readings. Peak RSS is a high-water mark."), muted.Render("Steps are measured separately; peaks are not summed."), muted.Render("Samples are collected while this view is open."))
	return ansi.Wordwrap(strings.Join(lines, "\n"), width, " ")
}
func (m *Model) footer() string {
	if m.view == "cancel" {
		return muted.Render(m.cancellationFooter())
	}
	hint := func(wide, compact string) string {
		if ansi.StringWidth(wide) > m.width-4 {
			return muted.Render(compact)
		}
		return muted.Render(wide)
	}

	if m.view == "shell" {
		return muted.Render(m.shellFooter())
	}
	if m.view == "warnings" {
		return muted.Render(m.warningsFooter())
	}
	if m.view == "settings" {
		return muted.Render("↑↓ move  space change  s apply  esc back")
	}
	if m.view == "launch" {
		return muted.Render(m.launchFooter())
	}
	if m.readerSearchEditing() {
		return muted.Render("enter keep  esc cancel  ctrl+c quit")
	}
	if m.input.Focused() {
		return hint("enter run   tab complete   esc return   ctrl+c quit", "enter run  tab complete  esc back  ^C quit")
	}
	if m.searching {
		return hint("enter keep filter   esc clear   ctrl+c quit", "enter keep  esc clear  ctrl+c quit")
	}
	if m.palette {
		if m.paletteJob != nil {
			c, ok := m.paletteSelection()
			if !ok || m.paletteProblem(c.key) != "" {
				return muted.Render("↑↓ choose   esc back")
			}
			verb := "open"
			if c.key == "shell" || c.key == "cancel-job" || c.key == "cancel-array" {
				verb = "review"
			} else if c.key == "y" {
				verb = "copy"
			}
			return muted.Render("↑↓ choose   enter " + verb + "   esc back")
		}
		if c, ok := m.paletteSelection(); !ok || m.paletteProblem(c.key) != "" {
			return hint("type to search   ↑↓ choose   esc close", "type to search  ↑↓ move  esc back")
		}
		return hint("type to search   ↑↓ choose   enter run   esc close", "type to search  ↑↓ move  enter run  esc back")
	}
	if m.help {
		return hint("↑↓ scroll   / commands   esc back   q quit", "↑↓ scroll  / commands  esc back  q quit")
	}
	if m.view == "activity" {
		if len(m.store.Events) == 0 {
			return muted.Render("r refresh   esc back")
		}
		return muted.Render("↑↓ choose   enter inspect   esc back")
	}
	if m.emptyWorkspace() {
		return hint("n new job   / commands   ? help   q quit", "n new job  / commands  ? help  q quit")
	}
	if j, ok := m.selected(); ok && m.tab < 2 && j.AwaitingTasks > 0 {
		if m.view == "inspect" {
			return hint("↑↓ scroll   r refresh   2 history   esc back", "↑↓ scroll  r refresh  2 history  esc back")
		}
		return hint("↑↓ select   enter inspect   2 history   / commands", "↑↓ select  enter inspect  2 history  / more")
	}
	if m.tab == 3 && m.view == "details" {
		if m.width < 64 {
			return muted.Render("↑↓ scroll  c reconnect  / commands  q quit")
		}
		return muted.Render("↑↓ scroll   c reconnect   r refresh   / commands   q quit")
	}
	if m.tab == 2 && m.view == "details" {
		return hint("↑↓ scroll   n nodes   r refresh   / commands   q quit", "↑↓ scroll  n nodes  r refresh  q quit")
	}
	if m.view == "logs" {
		if m.readerSearch != nil {
			return hint("[ previous   ] next   f edit   end latest   esc clear", "[ prev  ] next  f edit  esc clear")
		}
		return hint("↑↓ scroll   f search   end latest   e stdout/stderr   r refresh   esc back", "f search  e stdout/stderr  esc back  q quit")
	}
	if m.view == "waits" {
		return muted.Render("↑↓ scroll · r refresh · esc back")
	}
	if m.view == "software" && m.softwareRequest.Command == "nodes" {
		if m.nodesGPUOnly {
			return hint("↑↓ scroll   g all nodes   r refresh   esc back", "g all nodes  r refresh  esc back  q quit")
		}
		return hint("↑↓ scroll   g GPU nodes   r refresh   esc back", "g GPUs  r refresh  esc back  q quit")
	}
	if m.view == "software" && m.softwareRequest.Command == "priority" {
		return hint("↑↓ scroll   r refresh   esc back   q quit", "↑↓ scroll  r refresh  esc back  q quit")
	}
	if m.catalogReady() {
		return hint("↑↓ select   enter setup   f search   r refresh   esc back   q quit", "↑↓ select  enter setup  esc back  q quit")
	}
	if m.view == "software" && m.softwareRequest.Command == "module" {
		return hint("↑↓ scroll   y copy load command   r refresh   esc back", "y copy load  r refresh  esc back  q quit")
	}
	if m.view == "software" {
		return hint("↑↓ scroll   / search or inspect   r refresh   esc back   q quit", "↑↓ scroll  / commands  esc back  q quit")
	}
	if m.view == "receipts" {
		return muted.Render("↑↓ scroll · r reload receipts · esc back")
	}
	if m.view == "script" {
		if m.detailError != "" && !m.cacheBlocked {
			return muted.Render("↑↓ scroll   r retry   esc back   q quit")
		}
		if m.readerSearch != nil {
			return hint("[ previous   ] next   f edit   esc clear", "[ prev  ] next  f edit  esc clear")
		}
		return hint("↑↓ scroll   f search   / commands   esc back   q quit", "f search  / commands  esc back  q quit")
	}
	if m.view == "resources" {
		return hint("[ ] step   ↑↓ scroll   r refresh   esc back   q quit", "[ ] step  ↑↓ scroll  esc back  q quit")
	}
	if m.view == "inspect" {
		if m.shellAvailable() {
			return hint("↑↓ scroll   l output   s script   u resources   o shell   esc back", "↑↓ scroll  l output  o shell  esc back")
		}
		return hint("↑↓ scroll   l output   s script   u resources   esc back   q quit", "↑↓ scroll  l output  / commands  esc back")
	}
	if m.width < 85 {
		return hint("↑↓ select   enter inspect   a actions   / commands   q quit", "↑↓ select  enter inspect  a actions  q quit")
	}
	return hint("↑↓ select   enter inspect   l output   a actions   / commands   f filter   ? help   q quit", "↑↓ select  enter inspect  a actions  / commands  q quit")
}
