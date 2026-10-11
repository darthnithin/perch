package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"perch/assets/bird"
	"perch/internal/domain"
)

type appearance struct {
	theme, colorMode string
	reduced, noMouse bool
}
type settingsForm struct {
	original, draft appearance
	previousView    string
	focus           int
	saving          bool
	problem         string
}
type settingsSavedMsg struct {
	sequence int
	err      error
}

func (m *Model) openSettings() tea.Cmd {
	if m.view == "settings" {
		return nil
	}
	a := appearance{m.options.Theme, m.options.ColorMode, m.options.ReducedMotion, m.options.NoMouse}
	if a.colorMode == "" {
		a.colorMode = "auto"
	}
	if a.theme == "" {
		a.theme = "warm"
	}
	m.settings = &settingsForm{original: a, draft: a, previousView: m.view}
	m.input.Blur()
	m.input.SetValue("")
	m.inputError = ""
	m.help, m.palette, m.searching = false, false, false
	m.pendingLaunchFocus = ""
	m.invalidateDetail()
	m.view = "settings"
	return nil
}

func (m *Model) settingsButton() string {
	if m.settings.saving {
		return "Saving…"
	}
	if m.options.Demo {
		return "Use for this demo"
	}
	if m.options.Preferences == nil {
		return "Use this session"
	}
	return "Save defaults"
}

func (m *Model) settingsToggle() tea.Cmd {
	f := m.settings
	if f.saving {
		return nil
	}
	f.problem = ""
	switch f.focus {
	case 0:
		if f.draft.theme == "warm" {
			f.draft.theme = "graphite"
		} else {
			f.draft.theme = "warm"
		}
		m.options.Theme = f.draft.theme
	case 1:
		f.draft.reduced = !f.draft.reduced
		m.options.ReducedMotion = f.draft.reduced
		return m.nextFrame()
	case 2:
		// Retain capture while editing so a pointer user can still reach Save.
		f.draft.noMouse = !f.draft.noMouse
	case 3:
		switch f.draft.colorMode {
		case "auto":
			f.draft.colorMode = "truecolor"
		case "truecolor":
			f.draft.colorMode = "256"
		default:
			f.draft.colorMode = "auto"
		}
		m.options.ColorMode = f.draft.colorMode
		return m.applyColorMode()
	case 4:
		return m.saveSettings()
	}
	return nil
}

func (m *Model) closeSettings(keep bool) tea.Cmd {
	f := m.settings
	if f == nil || f.saving {
		return nil
	}
	a := f.original
	if keep {
		a = f.draft
	}
	m.options.Theme, m.options.ReducedMotion, m.options.NoMouse = a.theme, a.reduced, a.noMouse
	m.options.ColorMode = a.colorMode
	m.view = f.previousView
	m.settings = nil
	return tea.Batch(m.nextFrame(), m.fetchDetail(), m.applyColorMode())
}

func (m *Model) saveSettings() tea.Cmd {
	f := m.settings
	if f.saving {
		return nil
	}
	if m.options.Demo || m.options.Preferences == nil {
		m.setNotice("Appearance applied for this session.")
		return m.closeSettings(true)
	}
	f.saving = true
	f.problem = ""
	m.settingsSequence++
	sequence, store, draft := m.settingsSequence, *m.options.Preferences, f.draft
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return settingsSavedMsg{sequence, store.SaveAppearance(ctx, draft.theme, draft.colorMode, draft.reduced, !draft.noMouse)}
	}
}

func (m *Model) settingsSaved(msg settingsSavedMsg) tea.Cmd {
	if m.view != "settings" || m.settings == nil || msg.sequence != m.settingsSequence || !m.settings.saving {
		return nil
	}
	m.settings.saving = false
	if msg.err != nil {
		m.settings.problem = "Save error: " + domain.Label(msg.err.Error()) + ". Inspect perch config show."
		m.setNotice(m.settings.problem)
		return nil
	}
	m.setNotice("Appearance saved · flags can override it on your next launch.")
	return m.closeSettings(true)
}

func (m *Model) settingsKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.settings.saving {
		return nil
	}
	switch msg.String() {
	case "esc":
		return m.closeSettings(false)
	case "q":
		m.Close()
		return tea.Quit
	case "up", "k", "shift+tab":
		m.settings.focus = (m.settings.focus + 4) % 5
	case "down", "j", "tab":
		m.settings.focus = (m.settings.focus + 1) % 5
	case "left", "right", "space", "enter":
		return m.settingsToggle()
	case "s", "ctrl+s":
		return m.saveSettings()
	}
	return nil
}

// Shared with pointer hit testing, including the compact five-row layout.
func settingsRows(height int) (start, stride int) {
	if height >= 14 {
		return 3, 2
	}
	return 0, 1
}

func (m *Model) settingsContent(width, height int) string {
	f := m.settings
	start, stride := settingsRows(height)
	formWidth := width
	preview := width >= 88 && height >= 14 && !m.options.NoColor
	if preview {
		formWidth = width / 2
	}
	var rows []string
	if start > 0 {
		rows = append(rows, ink.Bold(true).Render("Make yourself at home"), muted.Render("Preview here. Esc restores your choices."), "")
	}
	motion, mouse := "Animated", "Capture clicks"
	if f.draft.reduced {
		motion = "Still"
	}
	if f.draft.noMouse {
		mouse = "Native selection"
	}
	colorLabel := map[string]string{"auto": "Auto", "truecolor": "True color", "256": "256 colors"}[f.draft.colorMode]
	values := []string{f.draft.theme, motion, mouse, colorLabel}
	labels := []string{"Palette", "Chip motion", "Mouse", "Color mode", m.settingsButton()}
	for i, label := range labels {
		text := label
		if i < 4 {
			text = joinLR(label, values[i]+"  ‹ ›", formWidth-4)
		}
		prefix := "  "
		style := ink
		if i == f.focus {
			prefix = "› "
			style = accent.Bold(true)
		}
		rows = append(rows, style.Render(prefix+fit(text, formWidth-2)))
		if stride > 1 {
			rows = append(rows, "")
		}
	}
	if start > 0 {
		note := "Mouse capture changes when you apply."
		if m.options.Demo {
			note = "Demo changes are never saved to disk."
		} else if m.options.Preferences == nil {
			note = "--no-config: changes last this session."
		}
		rows = append(rows, muted.Render(note))
	}
	form := strings.Join(rows, "\n")
	if !preview {
		return form
	}
	chip := bird.OctantFrame("running", m.frame, f.draft.reduced)
	right := []string{muted.Render("Chip · live preview"), ""}
	for _, row := range chip {
		right = append(right, "    "+row)
	}
	right = append(right, "", muted.Render("Your terminal controls the font."))
	return lipgloss.JoinHorizontal(lipgloss.Top, form, "    ", strings.Join(right, "\n"))
}

func (m *Model) settingsHint() string {
	if m.settings.problem != "" {
		return yellow.Render(m.settings.problem)
	}
	if m.settings.saving {
		return muted.Render("Saving your appearance preferences…")
	}
	if m.options.NoColor && (m.settings.focus < 2 || m.settings.focus == 3) {
		return muted.Render("Color and artwork are disabled this session.")
	}
	hints := []string{"Warm charcoal or cool graphite.", "Preview Chip's animation or keep him still.", "Mouse mode changes when you apply.", "Choose True color if SSH changes the colors.", fmt.Sprintf("%s · Esc cancels the preview.", m.settingsButton())}
	return muted.Render(hints[m.settings.focus])
}

func (m *Model) settingsMouse(p tea.Mouse, wheel bool) tea.Cmd {
	if m.settings.saving {
		return nil
	}
	l := m.layout()
	if wheel {
		if p.Y <= l.bodyTop || p.Y >= l.bodyTop+l.bodyHeight-1 {
			return nil
		}
		delta := 1
		if p.Button == tea.MouseWheelUp {
			delta = -1
		}
		m.settings.focus = max(0, min(4, m.settings.focus+delta))
		return nil
	}
	if p.Y == 2 {
		x := 2
		for i, label := range tabLabels {
			if p.X >= x && p.X < x+len(label) {
				cancel := m.closeSettings(false)
				return tea.Batch(cancel, m.action(string(rune('1'+i))))
			}
			x += len(label) + l.tabGap
		}
		return nil
	}
	width, height := l.inner-4, l.bodyHeight-2
	if width >= 88 && height >= 14 && !m.options.NoColor {
		width /= 2
	}
	start, stride := settingsRows(height)
	row := p.Y - l.bodyTop - 1 - start
	if p.X < 4 || p.X >= 4+width || row < 0 || row%stride != 0 || row/stride > 4 {
		return nil
	}
	m.settings.focus = row / stride
	return m.settingsToggle()
}
