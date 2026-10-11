package nautilusui

import (
	"perch/internal/shortcuts"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type mouseTarget struct {
	kind, key          string
	index, x, y, width int
}

// Clicks and hover share visible geometry, including scrolled picker rows.
// Merely moving the pointer must never select an object or fetch its logs.
func (m *Model) mouseTarget(p tea.Mouse) mouseTarget {
	if m.options.NoMouse || m.connecting || m.width < 48 || m.height < 18 || p.Mod != 0 || p.X < 2 || p.X >= m.width-2 {
		return mouseTarget{}
	}
	if p.Y == m.height-1 {
		for _, t := range m.footerTargets() {
			if p.X >= t.x && p.X < t.x+t.width {
				return t
			}
		}
		return mouseTarget{}
	}
	if p.Y == 2 {
		x := 2
		for i, label := range m.tabs() {
			if p.X >= x && p.X < x+len(label) {
				return mouseTarget{kind: "tab", index: i, x: x, y: p.Y, width: len(label)}
			}
			x += len(label) + 2
		}
	}
	if p.Y == m.height-5 {
		return mouseTarget{kind: "input", x: 2, y: p.Y, width: m.width - 4}
	}
	if m.help || p.X < 4 || p.X >= m.width-4 || p.Y < 7 || p.Y >= m.height-10 {
		return mouseTarget{}
	}
	if m.logs != nil {
		if m.logs.stage != "output" {
			i := m.logStart() + p.Y - 7
			if i < m.logs.choiceCount() {
				return mouseTarget{kind: "choice", index: i, x: 4, y: p.Y, width: m.width - 8}
			}
		}
		return mouseTarget{}
	}
	if m.tab < 2 && !m.inspect {
		i := m.listStart() + p.Y - 7
		if i < len(m.rows(m.tab)) {
			return mouseTarget{kind: "row", index: i, x: 4, y: p.Y, width: m.width - 8}
		}
	}
	return mouseTarget{}
}

func (m *Model) mouseClick(p tea.Mouse) tea.Cmd {
	if p.Button != tea.MouseLeft {
		return nil
	}
	t := m.mouseTarget(p)
	switch t.kind {
	case "footer":
		if key, ok := shortcuts.Key(t.key); ok {
			return m.update(key)
		}
	case "tab":
		m.switchTab(t.index)
	case "input":
		return m.input.Focus()
	case "choice":
		m.input.Blur()
		m.logs.index = t.index
	case "row":
		m.input.Blur()
		m.selection[m.tab] = t.index
		m.selectedUID[m.tab] = m.rows(m.tab)[t.index].ref.UID
	}
	return nil
}

func (m *Model) hoverTarget() mouseTarget {
	if !m.hoverKnown {
		return mouseTarget{}
	}
	return m.mouseTarget(m.hoverMouse)
}

func (m *Model) pointerShape() string {
	shape := "default"
	switch m.hoverTarget().kind {
	case "input":
		shape = "text"
	case "":
	default:
		shape = "pointer"
	}
	if m.options.PointerStyle == "x11" {
		if shape == "pointer" {
			return "hand2"
		}
		return "xterm"
	}
	return shape
}

func (m *Model) syncPointer() tea.Cmd {
	if m.options.PointerStyle == "" || m.connecting || m.options.NoMouse && m.lastPointer == "" {
		return nil
	}
	shape := m.pointerShape()
	if shape == m.lastPointer {
		return nil
	}
	m.lastPointer = shape
	return tea.Raw(ansi.SetPointerShape(shape))
}

// PointerReset must run after the renderer releases the terminal, on exit and
// before foreground authentication. Unsupported terminals receive no OSC 22.
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

func (m *Model) hoverView(content string) string {
	t := m.hoverTarget()
	if t.kind == "" || t.kind == "input" {
		return content
	}
	lines := strings.Split(content, "\n")
	if t.y >= len(lines) {
		return content
	}
	line := lines[t.y]
	segment := ansi.Cut(line, t.x, t.x+t.width)
	highlight := "\x1b[4m" + strings.NewReplacer("\x1b[m", "\x1b[4m", "\x1b[0m", "\x1b[0;4m").Replace(segment) + "\x1b[24m"
	if t.kind == "row" || t.kind == "choice" {
		// Selection keeps its stronger fill; hovering never replaces it.
		if t.kind == "row" && t.index == m.selection[m.tab] || t.kind == "choice" && t.index == m.logs.index {
			return content
		}
		bg := "\x1b[48;2;48;41;35m"
		highlight = bg + strings.NewReplacer("\x1b[m", "\x1b[m"+bg, "\x1b[0m", "\x1b[0m"+bg).Replace(segment) + "\x1b[m"
	}
	lines[t.y] = ansi.Cut(line, 0, t.x) + highlight + ansi.Cut(line, t.x+t.width, m.width)
	return strings.Join(lines, "\n")
}
