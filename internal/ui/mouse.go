package ui

import tea "charm.land/bubbletea/v2"

func (m *Model) mouseEnabled() bool {
	return !m.options.NoMouse && m.width >= 48 && m.height >= 18
}

func (m *Model) leaveInput() {
	m.input.Blur()
	m.inputError = ""
	m.searching = false
}

func (m *Model) mouseClick(p tea.Mouse) tea.Cmd {
	if p.Button != tea.MouseLeft {
		return nil
	}
	t := m.mouseTarget(p)
	switch t.kind {
	case "sort":
		m.leaveInput()
		return m.toggleJobSort(t.key)
	case "cancel-button":
		if m.cancelForm.stage == "ready" {
			if !m.panel.AtBottom() {
				m.panel.GotoBottom()
				return nil
			}
			return m.confirmCancellation()
		}
		if m.cancelForm.stage == "receipt" {
			return m.closeCancellation()
		}
	case "shell-open":
		_, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
		return cmd
	case "footer":
		// Retain modal focus: Enter in command input, filter, launch review or
		// palette must mean exactly what the visible keyboard hint promises.
		if key, ok := footerKey(t.key); ok {
			_, cmd := m.update(key)
			return cmd
		}
	case "tab":
		var cancel tea.Cmd
		if m.view == "cancel" {
			cancel = m.closeCancellation()
		}
		if m.view == "shell" {
			cancel = m.closeShellPreview()
		}
		if m.view == "warnings" {
			cancel = m.closeWarnings()
		}
		if m.view == "settings" {
			cancel = m.closeSettings(false)
		}
		if m.view == "launch" {
			m.launchKey(tea.KeyPressMsg{Code: tea.KeyEscape})
		}
		m.leaveInput()
		m.help, m.palette = false, false
		return tea.Batch(cancel, m.action(t.key))
	case "warning-input":
		return m.warningForm.input.Focus()
	case "setting":
		m.settings.focus = t.index
		return m.settingsToggle()
	case "module":
		m.selectModule(t.index)
		cmd, _ := m.catalogKey("enter")
		return cmd
	case "field":
		f := m.launchForm
		f.fields[f.focus].input.Blur()
		f.focus = t.index
		return f.fields[t.index].input.Focus()
	case "input":
		if m.textReader() && m.readerSearch != nil && !m.help && !m.palette {
			return m.startReaderSearch()
		}
		m.searching = false
		m.help, m.palette = false, false
		m.inputError = ""
		return m.input.Focus()
	case "palette":
		m.leaveInput()
		return m.activatePalette(t.index)
	case "action":
		m.leaveInput()
		return m.action(t.key)
	case "activity":
		m.leaveInput()
		m.selectActivity(t.index)
		return m.openActivityJob()
	case "job":
		m.leaveInput()
		return m.selectJob(t.index)
	}
	return nil
}

func (m *Model) selectJob(index int) tea.Cmd {
	m.pendingLaunchFocus = ""
	if index == m.selection {
		return nil
	}
	m.selection = index
	m.selectedID = ""
	m.normalizeSelection()
	m.invalidateDetail()
	return m.fetchDetail()
}

func (m *Model) mouseWheel(p tea.Mouse) tea.Cmd {
	if !m.mouseEnabled() || p.Mod != 0 {
		return nil
	}
	delta := 0
	switch p.Button {
	case tea.MouseWheelUp:
		delta = -3
	case tea.MouseWheelDown:
		delta = 3
	default:
		return nil
	}
	if m.view == "launch" {
		return m.launchMouse(p, true)
	}
	if m.view == "warnings" {
		return nil
	}
	if m.view == "settings" {
		return m.settingsMouse(p, true)
	}
	l := m.layout()
	if p.X <= 2 || p.X >= m.width-3 || p.Y <= l.bodyTop || p.Y >= l.bodyTop+l.bodyHeight-1 {
		return nil
	}
	switch {
	case m.palette:
		m.paletteIndex = max(0, min(len(m.paletteCommands())-1, m.paletteIndex+delta))
	case m.catalogReady() && !m.help:
		g := m.catalogGeometry()
		if p.Y >= l.bodyTop+1+g.header && p.Y < l.bodyTop+l.bodyHeight-2 {
			m.selectModule(m.catalog.index + delta)
		}
	case m.view == "activity" && !m.help:
		m.selectActivity(m.activityIndex() + delta)
	case m.view == "cancel" || m.view == "shell" || m.help || m.tab > 1 || m.view == "inspect" || m.view == "resources" || m.view == "waits":
		if delta < 0 {
			m.panel.ScrollUp(-delta)
		} else {
			m.panel.ScrollDown(delta)
		}
	case m.view == "logs" || m.view == "script" || m.view == "software" || m.view == "receipts":
		header := 3
		if m.view == "software" {
			header = 1 // Full-height reports have no fixed header inside the box.
		}
		if m.view != "receipts" && p.Y < l.bodyTop+header {
			return nil
		}
		if delta < 0 {
			m.viewport.ScrollUp(-delta)
		} else {
			m.viewport.ScrollDown(delta)
		}
		if m.view == "logs" {
			m.logFollowing = m.readerSearch == nil && m.viewport.AtBottom()
		}
	default:
		if p.X >= l.jobsWidth || len(m.jobs()) == 0 {
			return nil
		}
		return m.selectJob(max(0, min(len(m.jobs())-1, m.selection+delta)))
	}
	return nil
}
