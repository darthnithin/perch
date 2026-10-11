package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

// DetectPointerStyle only enables OSC 22 where its shape vocabulary is known.
// Unknown terminals and multiplexers still get in-app hover feedback.
func DetectPointerStyle(getenv func(string) string) string {
	if getenv("TMUX") != "" || getenv("STY") != "" {
		return ""
	}
	switch strings.ToLower(getenv("TERM_PROGRAM")) {
	case "ghostty":
		return "css"
	case "iterm.app":
		return "x11"
	}
	switch getenv("TERM") {
	case "xterm-kitty", "foot", "foot-extra":
		return "css"
	}
	return ""
}

type mouseTarget struct {
	kind, key   string
	index       int
	x, y, width int
}

// Both clicks and hover use this hit test. It has no navigation side effects.
func (m *Model) mouseTarget(p tea.Mouse) mouseTarget {
	none := mouseTarget{}
	if !m.mouseEnabled() || p.Mod != 0 || p.X < 2 || p.X >= m.width-2 {
		return none
	}
	l := m.layout()
	hit := func(kind, key string, index, x, y, width int) mouseTarget {
		if p.Y == y && p.X >= x && p.X < x+width {
			return mouseTarget{kind, key, index, x, y, width}
		}
		return none
	}
	if m.view == "warnings" && m.warningForm.saving {
		return none
	}
	if m.view == "settings" && m.settings.saving {
		return none
	}
	if m.view == "cancel" && m.cancelForm.stage == "sending" {
		return none
	}
	if p.Y == m.height-1 {
		for _, t := range m.footerTargets() {
			if p.X >= t.x && p.X < t.x+t.width {
				return t
			}
		}
		return none
	}
	if p.Y == 2 {
		if m.view == "launch" && m.launchForm.stage == "submitting" {
			return none
		}
		x := 2
		for i, label := range l.tabs {
			if t := hit("tab", string(rune('1'+i)), i, x, 2, len(label)); t.kind != "" {
				return t
			}
			x += len(label) + l.tabGap
		}
		return none
	}
	if m.view == "cancel" {
		if m.cancellationButton() != "" {
			return hit("cancel-button", "", 0, 4, m.height-3, m.cancellationButtonWidth())
		}
		return none
	}
	if m.view == "shell" {
		if m.shellForm.stage == "ready" {
			return hit("shell-open", "", 0, 4, m.height-5, ansi.StringWidth("Open shell  ↵"))
		}
		return none
	}
	if m.view == "warnings" {
		g := m.warningsGeometry()
		return hit("warning-input", "", 0, g.x+2, g.y+4, g.width-4)
	}
	if m.view == "settings" {
		width, height := l.inner-4, l.bodyHeight-2
		if width >= 88 && height >= 14 && !m.options.NoColor {
			width /= 2
		}
		start, stride := settingsRows(height)
		row := p.Y - l.bodyTop - 1 - start
		if row >= 0 && row%stride == 0 && row/stride <= 4 {
			return hit("setting", "", row/stride, 4, p.Y, width)
		}
		return none
	}
	if m.view == "launch" {
		if m.launchForm.stage != "edit" || p.Y <= l.bodyTop || p.Y >= l.bodyTop+l.bodyHeight-1 {
			return none
		}
		start, visible := m.launchVisible()
		row := p.Y - l.bodyTop - 3
		if row >= 0 && row < visible*3 && row%3 != 2 && start+row/3 < len(m.launchForm.fields) {
			return hit("field", "", start+row/3, 4, p.Y, m.width-8)
		}
		return none
	}
	if (!m.palette || m.paletteJob == nil) && !m.compactTextReading() {
		if t := hit("input", "", 0, 2, m.height-5, m.width-4); t.kind != "" {
			return t
		}
	}
	if p.Y <= l.bodyTop || p.Y >= l.bodyTop+l.bodyHeight-1 {
		return none
	}
	if m.palette {
		matches := m.paletteCommands()
		g := m.paletteGeometry()
		row := p.Y - g.y - 3
		if row >= 0 && row < g.visible && g.start+row < len(matches) {
			return hit("palette", matches[g.start+row].key, g.start+row, g.x+2, p.Y, 2+g.rowWidth)
		}
		return none
	}
	if !m.help && m.view == "activity" {
		g := m.activityGeometry()
		row := p.Y - l.bodyTop - 1 - g.header
		if row >= 0 && row < g.visible*g.stride && row%g.stride < 2 && g.start+row/g.stride < len(m.store.Events) {
			return hit("activity", "", g.start+row/g.stride, 4, p.Y, m.width-8)
		}
		return none
	}
	if !m.help && m.view == "resources" && m.tab < 2 {
		if j, ok := m.selected(); ok && j.Running() && len(m.resourceStepIDs()) > 1 {
			y := l.bodyTop + 2 - m.panel.YOffset()
			if t := hit("action", "[", 0, 4, y, 10); t.kind != "" {
				return t
			}
			if t := hit("action", "]", 0, 17, y, 6); t.kind != "" {
				return t
			}
		}
	}
	if m.emptyWorkspace() {
		_, actions := m.emptyQueueContent(l.bodyHeight - 2)
		for _, a := range actions {
			if t := hit("action", a.key, 0, 4+a.x, l.bodyTop+1+a.y, len(a.key)+1+len(a.label)); t.kind != "" {
				return t
			}
		}
		return none
	}
	if !m.help && m.tab < 2 && m.view == "details" && l.detailWidth > 0 {
		if _, ok := m.selected(); ok {
			if t := hit("action", "enter", 0, l.jobsWidth+5, l.bodyTop+l.bodyHeight-2, len("enter / click full details")); t.kind != "" {
				return t
			}
		}
	}
	if m.catalogReady() && !m.help && !m.palette {
		g := m.catalogGeometry()
		row := p.Y - l.bodyTop - 1 - g.header
		if row >= 0 && row < g.visible*g.stride && g.start+row/g.stride < len(m.catalog.items) {
			return hit("module", "", g.start+row/g.stride, 4, p.Y, l.inner-4)
		}
		return none
	}
	if m.help || m.tab > 1 || m.view != "details" || len(m.jobs()) == 0 {
		return none
	}
	g := jobGeometry(l.jobsWidth-4, l.bodyHeight-2, m.selection)
	headerY := l.bodyTop + 1
	if g.roomy {
		headerY += 3
	}
	for _, c := range m.jobColumns(l.jobsWidth-4, g.compact) {
		if c.key != "" {
			if t := hit("sort", c.key, 0, 4+c.x, headerY, ansi.StringWidth(c.label)); t.kind != "" {
				return t
			}
		}
	}
	row := p.Y - l.bodyTop - 1 - g.header
	if row < 0 || row >= g.visible*g.rowHeight || g.roomy && row%g.rowHeight != 0 {
		return none
	}
	index := g.start + row/g.rowHeight
	if index >= len(m.jobs()) {
		return none
	}
	return hit("job", "", index, 4, p.Y, l.jobsWidth-4)
}

func (m *Model) hoverTarget() mouseTarget {
	if !m.hoverKnown || m.pointerSuspended {
		return mouseTarget{}
	}
	return m.mouseTarget(m.hoverMouse)
}
func (m *Model) pointerShape() string {
	shape := "default"
	switch m.hoverTarget().kind {
	case "input", "field", "warning-input":
		shape = "text"
	case "":
	default:
		shape = "pointer"
	}
	if m.options.PointerStyle == "x11" {
		switch shape {
		case "pointer":
			return "hand2"
		case "text":
			return "xterm"
		default:
			return "xterm"
		}
	}
	return shape
}

// PointerReset is emitted after Bubble Tea releases the terminal, including
// signal/error exits. Reconnect also emits it before yielding to interactive SSH.
func (m *Model) PointerReset() string {
	if m.lastPointer == "" {
		return ""
	}
	switch m.options.PointerStyle {
	case "css":
		return ansi.SetPointerShape("default")
	case "x11":
		return ansi.SetPointerShape("xterm")
	}
	return ""
}
func (m *Model) syncPointer() tea.Cmd {
	if m.options.PointerStyle == "" || m.pointerSuspended || m.options.NoMouse && m.lastPointer == "" {
		return nil
	}
	shape := m.pointerShape()
	if shape == m.lastPointer {
		return nil
	}
	m.lastPointer = shape
	return tea.Raw(ansi.SetPointerShape(shape))
}
func (m *Model) hoverView(content string) string {
	t := m.hoverTarget()
	if t.kind == "" || t.kind == "input" || t.kind == "field" || t.kind == "warning-input" || m.options.NoColor {
		return content
	}
	lines := strings.Split(content, "\n")
	if t.y < 0 || t.y >= len(lines) {
		return content
	}
	line := lines[t.y]
	segment := ansi.Cut(line, t.x, t.x+t.width)
	// Highlight without changing selection, color hierarchy or layout.
	highlight := "\x1b[4m" + strings.NewReplacer("\x1b[m", "\x1b[4m", "\x1b[0m", "\x1b[0;4m").Replace(segment) + "\x1b[24m"
	if t.kind == "job" || t.kind == "palette" {
		if t.kind == "job" && t.index == m.selection || t.kind == "palette" && t.index == m.paletteIndex {
			return content
		}
		bg := "\x1b[48;2;48;41;35m"
		if m.options.Theme == "graphite" {
			bg = "\x1b[48;2;36;43;50m"
		}
		highlight = paintBackground(segment, bg)
	}
	lines[t.y] = ansi.Cut(line, 0, t.x) + highlight + ansi.Cut(line, t.x+t.width, m.width)
	return strings.Join(lines, "\n")
}
