package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"strings"
	"unicode"
)

func (c paletteCommand) shortcut() string {
	if strings.HasPrefix(c.key, "sort:") {
		return "/sort " + strings.TrimPrefix(c.key, "sort:")
	}
	if len(c.key) == 1 || c.key == "enter" {
		return c.key
	}
	switch c.key {
	case "software":
		return "/software"
	}
	for _, slash := range slashCommands {
		if slash.key == c.key {
			return slash.name
		}
	}
	return c.key
}

// The same guard serves keyboard shortcuts, slash commands and the palette.
// A hidden selection on a cluster tab must never act as the visible target.
func (m *Model) jobActionProblem(key string) string {
	if key == "copy-failed" {
		key = "y"
	}
	if m.view == "software" {
		switch key {
		case "enter", "l", "s", "u", "o", "shell", "y", "a", "actions", "cancel-job", "cancel-array":
			return "Choose a job in Jobs or History first."
		}
	}
	if m.view == "activity" {
		switch key {
		case "enter", "l", "s", "u", "o", "shell", "y", "a", "actions", "cancel-job", "cancel-array":
			return "Open an activity entry to choose its job first."
		}
	}
	if key == "cancel-job" || key == "cancel-array" {
		return m.cancellationProblem(key == "cancel-array")
	}
	if key == "cancel-receipt" && m.lastCancellation == nil {
		return "No cancellation request in this session."
	}
	switch key {
	case "enter", "l", "s", "u", "o", "shell", "y", "a", "actions":
	default:
		return ""
	}
	j, ok := m.selected()
	if m.tab > 1 || !ok {
		return "Choose a job in Jobs or History."
	}
	if (key == "l" || key == "s" || key == "u") && j.AwaitingTasks > 0 {
		return "Choose a confirmed task first."
	}
	if key == "o" || key == "shell" {
		if j.State != "RUNNING" || j.TaskRange != "" || j.AwaitingTasks > 0 {
			return "Choose one running job or array task."
		}
		if !m.shellAvailable() {
			return "Reconnect and refresh the queue first."
		}
	}
	if key == "y" {
		if m.cachedStore != nil {
			return "Refresh cached array outcomes first."
		}
		if j.ArrayID == 0 {
			return "Select an array task first."
		}
		for _, a := range m.arraySummaries() {
			if a.ID == j.ArrayID && a.SummaryError != "" {
				return "Cannot copy failed tasks: " + a.SummaryError
			}
			if a.ID == j.ArrayID && a.SummaryError == "" && a.FailedSpec() != "" {
				return ""
			}
		}
		return "No confirmed failed task IDs to copy."
	}
	return ""
}

func (m *Model) activatePalette(index int) tea.Cmd {
	matches := m.paletteCommands()
	if index < 0 || index >= len(matches) {
		return nil
	}
	m.paletteIndex = index
	c := matches[index]
	if m.paletteProblem(c.key) != "" {
		return nil
	}
	m.palette = false
	if c.key == "f" {
		return m.action("filter-jobs")
	}
	return m.action(c.key)
}

// Keep a job menu tied to the row the user opened it for. Refresh can change
// the underlying selection while the menu is visible; it must not retarget a
// click or Enter to that replacement job or attempt.
func (m *Model) paletteProblem(key string) string {
	if m.palette && m.paletteJob != nil {
		j, ok := m.selected()
		if !ok || m.tab > 1 || j.AttemptKey() != m.paletteJob.AttemptKey() {
			return "Job changed. Close actions and choose again."
		}
	}
	return m.jobActionProblem(key)
}

var jobCommands = []paletteCommand{
	{"Read output", "l"},
	{"View submitted script", "s"},
	{"Resources and efficiency", "u"},
	{"Open a shell…", "shell"},
	{"Full job details", "enter"},
	{"Copy failed task IDs", "y"},
	{"Cancel job or task…", "cancel-job"},
	{"Cancel whole array…", "cancel-array"},
}

func (m *Model) paletteSelection() (paletteCommand, bool) {
	matches := m.paletteCommands()
	if m.paletteIndex < 0 || m.paletteIndex >= len(matches) {
		return paletteCommand{}, false
	}
	return matches[m.paletteIndex], true
}

func (m *Model) paletteHint() string {
	c, ok := m.paletteSelection()
	if !ok {
		return muted.Render("Esc returns to your workspace")
	}
	if problem := m.paletteProblem(c.key); problem != "" {
		return yellow.Render(problem)
	}
	if strings.HasPrefix(c.key, "sort:") {
		return muted.Render("Selection kept · unknown values last")
	}
	switch c.key {
	case "enter", "l", "s", "u", "shell", "y", "copy-failed", "cancel-job", "cancel-array":
		j, _ := m.selected()
		return muted.Render("Selected job · ") + accent.Render(domain.Label(j.ID)) + muted.Render(" · "+domain.Label(j.Name))
	case "launch":
		return muted.Render("Review the command and resources first.")
	case "f":
		return muted.Render("Filter by name, ID, state or partition.")
	case "software":
		return muted.Render("Find installed software and load commands.")
	}
	for _, slash := range slashCommands {
		if slash.key == c.key {
			return muted.Render(slash.description)
		}
	}
	return muted.Render("Esc returns to your workspace")
}

func (m *Model) paletteRow(c paletteCommand, width int) string {
	shortcut := c.shortcut()
	label := c.label
	if m.paletteProblem(c.key) != "" {
		shortcut = "—"
		label = muted.Render(label)
	}
	// Reserve the shortcut even at 48 columns, instead of dropping it when the
	// label is long. The hit target uses this same fixed row width.
	return fit(label, width-ansi.StringWidth(shortcut)-2) + "  " + muted.Render(shortcut)
}

func (m *Model) paletteCommands() []paletteCommand {
	words := strings.Fields(strings.ToLower(m.paletteQuery))
	var matches []paletteCommand
	available := commands
	if m.paletteJob != nil {
		available = jobCommands
	}
	for _, c := range available {
		searchable := c.label + " " + c.key
		for _, slash := range slashCommands {
			if slash.key == c.key {
				searchable += " " + slash.name + " " + slash.description
			}
		}
		searchable = strings.ToLower(searchable)
		match := true
		for _, word := range words {
			if !strings.Contains(searchable, word) {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, c)
		}
	}
	return matches
}

func (m *Model) paletteKey(msg tea.KeyPressMsg) tea.Cmd {
	matches := m.paletteCommands()
	switch msg.String() {
	case "esc", "ctrl+k":
		m.palette = false
	case "up":
		m.paletteIndex = max(0, m.paletteIndex-1)
	case "down":
		m.paletteIndex = max(0, min(len(matches)-1, m.paletteIndex+1))
	case "enter":
		return m.activatePalette(m.paletteIndex)
	case "backspace":
		r := []rune(m.paletteQuery)
		if len(r) > 0 {
			m.paletteQuery = string(r[:len(r)-1])
		}
		m.paletteIndex = 0
	default:
		if msg.Text != "" {
			m.appendPaletteText(msg.Text)
		}
	}
	return nil
}

func (m *Model) appendPaletteText(text string) {
	m.paletteQuery = appendSearchText(m.paletteQuery, text)
	m.paletteIndex = 0
}

func appendSearchText(current, text string) string {
	// Preserve word boundaries when the terminal sends one key at a time, while
	// keeping pasted control sequences and line breaks out of the one-line UI.
	clean := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, domain.Clean(text))
	value := []rune(current + clean)
	return string(value[:min(100, len(value))])
}

func (m *Model) switchTab(tab int) {
	if tab < 0 || tab > 3 || tab == m.tab {
		return
	}
	if m.tab < 2 {
		m.tabSelections[m.tab] = m.selectedID
		m.tabFilters[m.tab] = m.search
	}
	m.tab = tab
	m.selection = 0
	m.selectedID = ""
	m.search = ""
	if tab < 2 {
		m.selectedID = m.tabSelections[tab]
		m.search = m.tabFilters[tab]
	}
	m.normalizeSelection()
}
