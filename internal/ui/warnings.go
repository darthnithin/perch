package ui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"perch/internal/domain"
	"perch/internal/timewarning"
)

type warningForm struct {
	input                 textinput.Model
	previousView, problem string
	saving                bool
}

type warningsSavedMsg struct {
	sequence int
	policy   timewarning.Policy
	err      error
}

func (m *Model) openWarnings() tea.Cmd {
	if m.view == "warnings" {
		return nil
	}
	f := &warningForm{input: textinput.New(), previousView: m.view}
	f.input.SetStyles(m.input.Styles())
	f.input.SetVirtualCursor(true)
	f.input.CharLimit = 64
	f.input.Prompt = "> "
	f.input.SetValue(m.options.TimeWarnings.String())
	f.input.CursorEnd()
	m.warningForm = f
	m.leaveInput()
	m.input.SetValue("")
	m.help, m.palette = false, false
	m.pendingLaunchFocus = ""
	m.invalidateDetail()
	m.view = "warnings"
	return f.input.Focus()
}

func (m *Model) closeWarnings() tea.Cmd {
	if m.warningForm == nil || m.warningForm.saving {
		return nil
	}
	m.view = m.warningForm.previousView
	m.warningForm = nil
	return m.fetchDetail()
}

func (m *Model) applyWarnings(policy timewarning.Policy, saved bool) tea.Cmd {
	m.options.TimeWarnings = policy
	m.store.SetTimeWarnings(policy)
	text := "Time warnings: " + policy.String() + " · this session"
	if saved {
		text = "Time warnings saved: " + policy.String()
	}
	m.setNotice(text)
	return tea.Batch(m.closeWarnings(), m.saveNotifications(nil))
}

func (m *Model) saveWarnings(persist bool) tea.Cmd {
	f := m.warningForm
	if f.saving {
		return nil
	}
	policy, err := timewarning.Parse(strings.TrimSpace(f.input.Value()))
	if err != nil {
		f.problem = err.Error()
		return nil
	}
	if !persist || m.options.Demo || m.options.Preferences == nil {
		return m.applyWarnings(policy, false)
	}
	f.saving, f.problem = true, ""
	m.warningSequence++
	sequence, store := m.warningSequence, *m.options.Preferences
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		value := policy.String()
		return warningsSavedMsg{sequence, policy, store.Edit(ctx, "time-warnings", &value, false)}
	}
}

func (m *Model) warningsSaved(msg warningsSavedMsg) tea.Cmd {
	if m.view != "warnings" || m.warningForm == nil || !m.warningForm.saving || msg.sequence != m.warningSequence {
		return nil
	}
	m.warningForm.saving = false
	if msg.err != nil {
		m.warningForm.problem = "Save not confirmed: " + domain.Label(msg.err.Error()) + ". Check perch config show."
		return nil
	}
	return m.applyWarnings(msg.policy, true)
}

func (m *Model) warningsKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.warningForm.saving {
		return nil
	}
	switch msg.String() {
	case "esc":
		return m.closeWarnings()
	case "enter":
		return m.saveWarnings(false)
	case "ctrl+s":
		return m.saveWarnings(true)
	}
	m.warningForm.problem = ""
	var cmd tea.Cmd
	m.warningForm.input, cmd = m.warningForm.input.Update(msg)
	return cmd
}

func (m *Model) warningsContent(width, height int) string {
	f := m.warningForm
	rows := []string{ink.Bold(true).Render("A heads-up before time runs out"), "", muted.Render("Remaining time · comma-separated"), f.input.View(), "", muted.Render("Whole minutes: 15m,5m,1m · or off"), muted.Render("1–5 thresholds · each 1m–24h"), muted.Render("Enter applies · Ctrl+S saves defaults")}
	if m.options.Demo || m.options.Preferences == nil {
		rows[7] = muted.Render("Applies to this session only")
	}
	if height >= 14 {
		rows = append(rows, "", muted.Render("Warnings use recorded scheduler time."), muted.Render("Already-passed thresholds are not replayed."), muted.Render("Missed thresholds combine into one alert."), muted.Render("Desktop alerts need --notify desktop."))
	}
	return strings.Join(rows, "\n")
}

func (m *Model) warningsFooter() string {
	if m.warningForm.saving {
		return "Saving…"
	}
	if m.options.Demo || m.options.Preferences == nil {
		return "enter apply   esc back"
	}
	return "enter apply   ctrl+s save   esc back"
}

func (m *Model) warningsGeometry() paletteLayout {
	l := m.layout()
	g := paletteLayout{width: min(l.inner, 70), height: min(l.bodyHeight, 18)}
	g.x = 2 + (l.inner-g.width)/2
	g.y = l.bodyTop + (l.bodyHeight-g.height)/3
	return g
}

func (m *Model) warningsView() string {
	l, g := m.layout(), m.warningsGeometry()
	card := box("Time warnings", m.warningsContent(g.width-4, g.height-2), g.width, g.height, true)
	rows := make([]string, l.bodyHeight)
	for i := range rows {
		rows[i] = strings.Repeat(" ", l.inner)
	}
	for i, line := range strings.Split(card, "\n") {
		rows[g.y-l.bodyTop+i] = fit(strings.Repeat(" ", g.x-2)+line, l.inner)
	}
	return strings.Join(rows, "\n")
}
