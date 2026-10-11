package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type helpEntry struct{ key, description string }
type helpSection struct {
	title string
	items []helpEntry
}

func (m *Model) helpText() string {
	navigation := helpSection{"Move around", []helpEntry{
		{"↑ ↓ / j k", "Select a job"},
		{"1 2 3 4", "Jobs, history, partitions, connection"},
		{"Tab / Shift+Tab", "Next / previous section"},
		{"Enter", "Inspect the selected job"},
		{"f", "Filter by name, ID, state or partition"},
		{"/sort", "Choose list order; click a column heading to reverse"},
		{"/job ID", "Jump to a loaded job"},
		{"Esc", "Go back / clear filter"},
		{"↑ ↓ / PgUp PgDn", "Scroll an open panel"},
		{"Home / End", "First / last row of a panel"},
		{"Click / wheel", "Select / scroll; footer shortcuts are clickable"},
	}}
	job := helpSection{"Selected job", []helpEntry{
		{"a / /actions", "Browse actions for this job"},
		{"l", "Read output; e switches stdout/stderr"},
		{"f in output", "Search retained output; [ / ] moves between matches"},
		{"s", "View the submitted script"},
		{"f in script", "Search the script; [ / ] moves between matches"},
		{"u", "Live resources and efficiency"},
		{"o / /shell", "Preview a shell in a running allocation"},
		{"/cancel", "Review cancellation of one job or task"},
		{"/cancel-array", "Review cancellation of the whole array"},
		{"/last-cancel", "Latest request receipt this session"},
		{"[ / ]", "Previous / next step in live resources"},
		{"y", "Copy failed array task IDs"},
	}}
	cluster := helpSection{"Work on Hummingbird", []helpEntry{
		{"y in module details", "Copy the module load command"},
		{"n / /run", "Prepare a job (n from Jobs or History)"},
		{"/receipts", "Saved submissions and recovery"},
		{"/waits", "Your queue wait history"},
		{"/nodes", "Node CPU, memory and GPU allocations"},
		{"/priority", "Pending-job scores and weighted factors"},
		{"/activity", "Observed job changes this session"},
		{"/warnings", "Time-limit warning preferences"},
		{"/software query", "Find installed cluster software"},
		{"/module name/version", "Prerequisites and load command"},
		{"r", "Refresh the current view"},
		{"c", "Reconnect / sign in through SSH"},
	}}
	preferences := helpSection{"Make yourself at home", []helpEntry{
		{"/", "Open the command input"},
		{"Ctrl+k", "Search all commands"},
		{"/settings", "Appearance, motion and mouse"},
		{"--no-mouse", "Keep native text selection"},
		{"q / Ctrl+c", "Quit and restore the terminal"},
	}}
	if m.options.Demo {
		preferences.items = append(preferences.items, helpEntry{"d", "Advance the demo scenario"})
	}
	width := max(10, m.width-8)
	if width < 104 {
		return renderHelpSections([]helpSection{navigation, job, cluster, preferences}, width)
	}
	// Keep related sections together as each column wraps. A narrow terminal uses
	// the same reading order in one column; no command is removed to make it fit.
	column := (width - 4) / 2
	left := strings.Split(renderHelpSections([]helpSection{navigation, job}, column), "\n")
	right := strings.Split(renderHelpSections([]helpSection{cluster, preferences}, column), "\n")
	lines := make([]string, max(len(left), len(right)))
	for i := range lines {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines[i] = fit(l, column) + "    " + r
	}
	return strings.Join(lines, "\n")
}

func renderHelpSections(sections []helpSection, width int) string {
	keyWidth := 18
	if width < 48 {
		keyWidth = 15
	}
	var lines []string
	for _, section := range sections {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, ink.Bold(true).Render(section.title))
		for _, item := range section.items {
			if ansi.StringWidth(item.key) > keyWidth {
				lines = append(lines, accent.Render(item.key))
				for _, line := range strings.Split(ansi.Wrap(item.description, width-2, " "), "\n") {
					lines = append(lines, "  "+muted.Render(line))
				}
				continue
			}
			wrapped := strings.Split(ansi.Wrap(item.description, width-keyWidth-2, " "), "\n")
			for i, line := range wrapped {
				key := ""
				if i == 0 {
					key = item.key
				}
				lines = append(lines, accent.Render(fit(key, keyWidth))+"  "+muted.Render(line))
			}
		}
	}
	return strings.Join(lines, "\n")
}
