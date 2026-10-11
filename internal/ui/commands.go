package ui

import (
	tea "charm.land/bubbletea/v2"
	"perch/internal/domain"
	"strconv"
	"strings"
)

var slashCommands = []struct{ name, description, key string }{
	{"/run", "Prepare, review and submit a job", "launch"},
	{"/shell", "Preview a shell in the selected running job", "shell"},
	{"/settings", "Preview and save appearance preferences", "settings"},
	{"/receipts", "Saved submissions and unconfirmed requests", "receipts"},
	{"/software", "Search cluster software: /software cuda", ""},
	{"/module", "Prerequisites: /module cuda/12.8.1", ""},
	{"/jobs", "Your active jobs", "1"},
	{"/nodes", "Node CPU, memory and GPU allocations", "nodes"},
	{"/priority", "Your pending-job priority factors", "priority"},
	{"/waits", "Your queue wait history by partition", "waits"},
	{"/history", "Recent outcomes", "2"},
	{"/partitions", "Cluster capacity", "3"},
	{"/connection", "Connection details", "4"},
	{"/connect", "Reconnect through SSH", "c"},
	{"/logs", "Selected job output", "l"},
	{"/script", "Submitted script", "s"},
	{"/resources", "Selected job resources", "u"},
	{"/failed", "Copy failed task IDs for the selected array", "copy-failed"},
	{"/refresh", "Refresh this view", "r"},
	{"/find", "Filter by name, ID, state, or partition", ""},
	{"/sort", "Choose list order; /sort elapsed desc", "sort"},
	{"/job", "Jump to a job ID", ""},
	{"/help", "Keyboard help", "?"},
	{"/quit", "Quit Perch", "q"},
	{"/cancel", "Review cancellation of the selected job or task", "cancel-job"},
	{"/cancel-array", "Review cancellation of every remaining array task", "cancel-array"},
	{"/last-cancel", "Latest cancellation receipt in this session", "cancel-receipt"},
	{"/actions", "Actions for the selected job", "actions"},
	{"/activity", "Changes observed in this session", "activity"},
	{"/warnings", "Choose time-limit warning thresholds", "warnings"},
}

// The command bar dispatches known application actions. It is never a shell.
func (m *Model) submitInput() tea.Cmd {
	value := domain.Label(m.input.Value())
	if value == "" {
		m.input.Blur()
		return nil
	}
	name, arg, _ := strings.Cut(value, " ")
	arg = strings.TrimSpace(arg)
	finish := func() { m.input.Blur(); m.input.SetValue(""); m.inputError = ""; m.help = false; m.palette = false }
	if name == "/sort" {
		if arg == "" {
			finish()
			return m.action("sort")
		}
		order, ok := parseJobSort(arg)
		if !ok {
			m.inputError = "Use /sort default, name, state, elapsed, cpus, memory or submitted; optionally asc/desc."
			return nil
		}
		finish()
		return m.applyJobSort(order)
	}
	if name == "/software" || name == "/module" {
		return m.showSoftware(strings.TrimPrefix(name, "/"), arg)
	}
	if name == "/find" {
		m.activity.returning = false
		finish()
		if m.tab > 1 {
			m.switchTab(0)
		}
		m.search = arg
		m.pendingLaunchFocus = ""
		m.selection = 0
		m.selectedID = ""
		m.normalizeSelection()
		return m.setView("details")
	}
	if name == "/job" || !strings.HasPrefix(name, "/") {
		m.activity.returning = false
		id := arg
		if name != "/job" {
			id = value
		}
		if id == "" {
			m.inputError = "Usage: /job 812046"
			return nil
		}
		for _, j := range m.displayJobs() {
			if j.ID == id || j.ArrayID > 0 && strconv.FormatInt(j.ArrayID, 10) == id {
				finish()
				target := 0
				if j.Terminal() {
					target = 1
				}
				m.switchTab(target)
				m.search = ""
				m.pendingLaunchFocus = ""
				m.selectedID = j.ID
				m.normalizeSelection()
				return m.setView("inspect")
			}
		}
		m.inputError = "No loaded job matches “" + id + "”. Use /find to search by name."
		return nil
	}
	for _, command := range slashCommands {
		if command.name == name && command.key != "" {
			if arg != "" {
				m.inputError = name + " takes no arguments"
				return nil
			}
			finish()
			return m.action(command.key)
		}
	}
	m.inputError = "Unknown command. Tab completes a command; /help shows the keys."
	return nil
}

func (m *Model) commandHint() string {
	if m.view == "warnings" {
		if m.warningForm.problem != "" {
			return yellow.Render(m.warningForm.problem)
		}
		return muted.Render("Limits must be ≥ twice each threshold")
	}
	if m.view == "cancel" {
		if m.options.Demo {
			return muted.Render("Demo only · no real cancellation")
		}
		if m.cancelForm.stage == "ready" {
			return muted.Render("Review the scope · Escape goes back")
		}
		if m.cancelForm.stage == "receipt" {
			return muted.Render("Request receipt · job state comes from Slurm")
		}
		return muted.Render("No automatic retries")
	}
	if m.view == "shell" {
		if m.options.Demo {
			return muted.Render("Demo only · no shell is opened")
		}
		return muted.Render("Shares job resources · opening is explicit")
	}
	if m.view == "settings" {
		return m.settingsHint()
	}
	if m.view == "launch" {
		switch m.launchForm.stage {
		case "edit":
			if m.launchForm.problem != "" {
				return muted.Render("Correct this field · your draft is kept")
			}
			return muted.Render(m.launchForm.fields[m.launchForm.focus].hint)
		case "submitted":
			return muted.Render("Submission receipt retained · n prepares another job")
		case "uncertain":
			return yellow.Render("Check your queue before making another submission")
		case "submitting":
			return muted.Render("Waiting for the scheduler receipt")
		default:
			return muted.Render("A job or task array · nothing runs until you submit")
		}
	}
	if m.inputError != "" {
		return yellow.Render(m.inputError)
	}
	if m.palette {
		return m.paletteHint()
	}
	if m.readerSearchEditing() {
		return muted.Render(m.readerSearchSummary())
	}
	if m.input.Focused() {
		value := strings.TrimSpace(m.input.Value())
		var matches []string
		for _, c := range slashCommands {
			if strings.HasPrefix(c.name, value) || strings.HasPrefix(value, c.name+" ") {
				matches = append(matches, c.name)
				if value == c.name || strings.HasPrefix(value, c.name+" ") {
					return accent.Render(c.name) + muted.Render("  "+c.description+" · Enter to run")
				}
			}
		}
		if len(matches) > 0 {
			return accent.Render(strings.Join(matches, "   "))
		}
		return muted.Render("Enter a job ID to inspect it · Esc returns to your jobs")
	}
	if hint := m.connectionHint(); hint != "" {
		return hint
	}
	if m.notice != "" {
		return ink.Render(domain.Label(m.notice))
	}
	if m.view == "software" && !m.help && !m.palette {
		if m.softwareRequest.Command == "nodes" {
			return muted.Render("Scheduler assignments · on-demand snapshot")
		}
		if m.softwareRequest.Command == "priority" {
			return muted.Render("Reported priority factors · on-demand snapshot")
		}
		if m.catalogReturn != nil {
			return muted.Render("Esc returns to your software results")
		}
		if m.softwareRequest.Command == "module" {
			return muted.Render("Module setup · load prerequisites first")
		}
		return accent.Render("f") + muted.Render(" searches · Enter or click opens setup")
	}
	if m.view == "activity" && !m.help && !m.palette {
		return muted.Render("Session only · existing history is excluded")
	}
	if m.textReader() && !m.help && !m.palette {
		if m.view == "logs" && m.detailError != "" && m.logLoaded {
			return yellow.Render("Last read failed: " + domain.Label(m.detailError))
		}
		if m.readerSearch != nil {
			return muted.Render(m.readerSearchSummary())
		}
		if m.view == "logs" {
			return muted.Render("Recent output · up to 256 KiB retained")
		}
	}
	if m.view == "details" && m.tab < 2 && !m.help && !m.palette && m.currentJobSort().key != "" {
		return muted.Render("Sorted: " + m.currentJobSort().label() + " · /sort changes order")
	}
	if m.search != "" && (m.help || m.palette || m.view != "details") {
		return muted.Render("Esc returns · filter kept")
	}
	if m.search != "" || m.searching {
		return muted.Render("Esc clears the filter · ") + accent.Render("/jobs   /logs   /connect")
	}
	if m.emptyWorkspace() {
		return accent.Render("/run") + muted.Render("   /software   /receipts   /help")
	}
	return accent.Render("/jobs") + muted.Render("   /logs   /partitions   /connect")
}
