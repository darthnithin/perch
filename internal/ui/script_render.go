package ui

import "github.com/charmbracelet/x/ansi"

// Scripts preserve whitespace and grapheme boundaries. Wrapping once avoids
// repeated prefix scans when the viewport displays a very long logical line.
func (m *Model) displayScriptText() {
	offset := m.viewport.YOffset()
	content := ""
	if s := m.readerSearch; s != nil {
		content, s.row = renderReaderText(s.plain, s.matches, s.current, max(1, m.viewport.Width()), !m.options.NoColor)
	} else {
		content = ansi.Hardwrap(cleanLogText(m.scriptText), max(1, m.viewport.Width()), true)
	}
	m.viewport.SetContent(content)
	m.viewport.SetYOffset(offset)
	m.scriptRenderedWidth = m.viewport.Width()
}

func (m *Model) displayScriptError() {
	offset := m.viewport.YOffset()
	body := "Unable to read this view.\n\n" + m.detailError
	m.viewport.SetContent(ansi.Wrap(body, max(1, m.viewport.Width()), " "))
	m.viewport.SetYOffset(offset)
	m.scriptRenderedWidth = m.viewport.Width()
}
