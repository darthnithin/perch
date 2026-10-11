package ui

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func (m *Model) displayLogText() {
	plain := cleanLogText(m.logRaw)
	var matches [][]int
	current := -1
	if m.readerSearch != nil {
		plain = m.readerSearch.plain
		matches = m.readerSearch.matches
		current = m.readerSearch.current
	}
	if plain == "" {
		plain = "The output file exists and is empty.\nNew output will appear here."
	}
	content, row := renderReaderText(plain, matches, current, max(1, m.viewport.Width()), !m.options.NoColor)
	offset := m.viewport.YOffset()
	m.viewport.SetContent(content)
	m.viewport.SetYOffset(offset)
	m.logRenderWidth = m.viewport.Width()
	if m.readerSearch != nil {
		m.readerSearch.row = row
	}
}

// Wrap once when text/query/width changes. Bubbles' soft wrapping repeatedly
// cuts from the start of a long logical line; a 256-KiB newline-free log would
// otherwise make a viewport frame quadratic. Each physical line gets its own
// complete highlight sequences, including matches crossing a wrap boundary.
func renderReaderText(plain string, matches [][]int, current, width int, color bool) (string, int) {
	var out strings.Builder
	out.Grow(len(plain))
	column, row, selectedRow := 0, 0, -1
	start, active, match := 0, -1, 0
	// Style whole runs. Lip Gloss's underline-space renderer inserts SGR
	// sequences between runes, splitting joined emoji into separate glyphs.
	hitStyle := ansi.Style{}.ForegroundColor(accent.GetForeground()).Underline(true)
	selectedStyle := ansi.Style{}.ForegroundColor(selected.GetForeground()).BackgroundColor(selected.GetBackground()).Bold().Underline(true)
	flush := func(end int) {
		if end <= start {
			return
		}
		part := plain[start:end]
		if active >= 0 && color {
			style := hitStyle
			if active == current {
				style = selectedStyle
			}
			part = style.Styled(part)
		}
		out.WriteString(part)
		start = end
	}
	for pos := 0; pos < len(plain); {
		cluster, cells := ansi.FirstGraphemeCluster(plain[pos:], ansi.GraphemeWidth)
		end := pos + len(cluster)
		if cluster == "\n" {
			flush(pos)
			out.WriteByte('\n')
			column = 0
			row++
			start = end
			active = -1
			pos = end
			continue
		}
		if column+cells > width {
			flush(pos)
			out.WriteByte('\n')
			column = 0
			row++
		}
		for match < len(matches) && matches[match][1] <= pos {
			match++
		}
		next := -1
		if match < len(matches) && matches[match][0] < end && matches[match][1] > pos {
			next = match
		}
		if current >= 0 && current < len(matches) && matches[current][0] < end && matches[current][1] > pos {
			next = current
		}
		if next != active {
			flush(pos)
			active = next
		}
		if next == current && next >= 0 && selectedRow < 0 {
			selectedRow = row
		}
		column += cells
		pos = end
	}
	flush(len(plain))
	return out.String(), selectedRow
}
