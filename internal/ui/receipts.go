package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/launch"
)

func (m *Model) showReceipts() tea.Cmd {
	m.input.Blur()
	m.input.SetValue("")
	m.inputError = ""
	m.help, m.palette, m.searching = false, false, false
	m.view = "receipts"
	m.tab = 0
	m.invalidateDetail()
	m.detailBusy = true
	m.receiptsBody = "Reading saved submissions…"
	m.viewport.SetContent(m.receiptsBody)
	m.viewport.GotoTop()
	seq, journal, destination, demo := m.detailSequence, m.options.Journal, m.options.Host, m.options.Demo
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	m.detailCancel = cancel
	return func() tea.Msg {
		defer cancel()
		var entries []launch.Submission
		var err error
		if demo {
			at := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
			entries = []launch.Submission{{Destination: "demo", Name: "parameter-sweep", Started: at, Receipt: launch.Receipt{At: at, JobID: "9100", Token: "perch-demo-accepted"}}, {Destination: "demo", Name: "training", Started: at.Add(-time.Minute), Receipt: launch.Receipt{At: at, Token: "perch-demo-unconfirmed"}}}
		} else if journal == nil {
			err = fmt.Errorf("submission journal is unavailable")
		} else {
			entries, err = journal.List(ctx, destination)
		}
		return detailMsg{sequence: seq, view: "receipts", text: receiptsContent(entries, demo), err: err}
	}
}

func receiptsContent(entries []launch.Submission, demo bool) string {
	intro := "Local records · not current job state"
	if demo {
		intro = "Demo receipts · no real submissions or saved data"
	}
	rows := []string{muted.Render(intro), ""}
	if len(entries) == 0 {
		rows = append(rows, "No saved submissions for this destination.", "New submissions made here will appear in this view.")
	}
	for _, entry := range entries {
		status := yellow.Render("unconfirmed")
		if entry.Receipt.JobID != "" {
			status = green.Render("accepted · #" + entry.Receipt.JobID)
		}
		rows = append(rows, accent.Bold(true).Render(entry.Name)+"  "+status, muted.Render(entry.Started.Local().Format("Jan 2 · 15:04:05")+" · "+entry.Destination), "Comment: "+entry.Receipt.Token)
		if entry.Receipt.JobID != "" {
			rows = append(rows, muted.Render("/job "+entry.Receipt.JobID+" to inspect if loaded in Jobs or History"))
		}
		rows = append(rows, "")
	}
	rows = append(rows, muted.Render("Accepted means Slurm returned an ID; it does not confirm execution."), muted.Render("For an unconfirmed request, check its comment in squeue/sacct before submitting again."), muted.Render("This view never resubmits. Commands, paths and logs are not saved."))
	return strings.Join(rows, "\n")
}

func (m *Model) syncReceipts() {
	if m.view != "receipts" {
		return
	}
	body := ansi.Wordwrap(m.receiptsBody, max(10, m.width-8), "")
	if body != m.viewport.GetContent() {
		m.viewport.SetContent(body)
	}
}
