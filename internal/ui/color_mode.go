package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

func (m *Model) colorProfile() colorprofile.Profile {
	if m.options.NoColor {
		return colorprofile.ASCII
	}
	switch m.options.ColorMode {
	case "truecolor":
		return colorprofile.TrueColor
	case "256":
		return colorprofile.ANSI256
	default:
		return m.detectedColorProfile
	}
}

func (m *Model) applyColorMode() tea.Cmd {
	// The first profile message comes from Bubble Tea's terminal detection.
	// Keep it for Auto; later messages apply the user's live preview.
	if !m.colorProfileKnown {
		return nil
	}
	p := m.colorProfile()
	return tea.Sequence(func() tea.Msg { return tea.ColorProfileMsg{Profile: p} }, tea.ClearScreen)
}
