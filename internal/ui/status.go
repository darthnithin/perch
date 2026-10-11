package ui

import (
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"time"
)

func (m *Model) setNotice(text string) {
	m.notice = domain.Label(text)
	m.noticeUntil = m.now.Add(8 * time.Second)
}

// Connection health always remains visible. Filters and transient notices must
// not replace it, especially while a retained snapshot is being displayed.
func (m *Model) statusLine(width int) string {
	state := m.statusText()
	style := green
	if !m.connected || m.lastError != "" || m.cachedStore != nil {
		style = yellow
	}
	if m.busy && m.visibleStore().Snapshot.At.IsZero() {
		style = muted
	}
	left := style.Render("● " + state)
	age := muted.Render("  ·  " + m.age())
	if m.view == "warnings" || m.view == "activity" || m.view == "software" || m.search == "" && !m.searching {
		return fit(left+age, width)
	}
	right := accent.Render("Filter: " + domain.Label(m.search))
	// Keep both the state and an identifiable filter at compact widths. Age
	// remains available on Connection; it yields space before either context.
	if ansi.StringWidth(left+age)+ansi.StringWidth(right)+2 <= width {
		left += age
	}
	remaining := max(0, width-ansi.StringWidth(left)-2)
	right = ansi.Truncate(right, remaining, "…")
	return joinLR(left, right, width)
}

func (m *Model) connectionHint() string {
	if m.lastError == "" {
		return ""
	}
	action := "4 connection details"
	if !m.connected {
		action = "c reconnect · 4 details"
	}
	return yellow.Render(action) + muted.Render(" · "+domain.Label(m.lastError))
}
