package nautilusui

import "perch/internal/shortcuts"

func (m *Model) footer() string {
	if m.input.Focused() {
		return "enter run  esc back  ctrl+c quit"
	}
	if m.help {
		return "↑↓ scroll  r refresh  esc back  q quit"
	}
	if m.logs != nil {
		if m.logs.metrics && m.logs.stage == "output" {
			return "↑↓ scroll  r read  esc back  q quit"
		}
		if m.logs.stage != "output" {
			return "↑↓ choose  enter open  esc back  q quit"
		}
		if m.width < 80 {
			return "p mode  r read  esc back  q quit"
		}
		return "↑↓ scroll  p current/previous  r read  esc back  q quit"
	}
	if m.inspect && m.tab < 2 {
		return "l logs  u resources  esc back  q quit"
	}
	if m.tab == 3 {
		if m.width < 64 {
			return "c connect  r refresh  esc back  q quit"
		}
		return "↑↓ scroll  c connect  r refresh  esc back  q quit"
	}
	if m.tab >= 2 {
		return "↑↓ scroll  r refresh  esc back  q quit"
	}
	if m.width < 64 {
		return "enter inspect  l logs  ? help  q quit"
	}
	return "↑↓ move  enter inspect  l logs  u resources  ? help  q quit"
}

func (m *Model) footerTargets() []mouseTarget {
	var targets []mouseTarget
	for _, t := range shortcuts.Targets(m.footer(), m.width-4) {
		targets = append(targets, mouseTarget{kind: "footer", key: t.Key, x: t.X + 2, y: m.height - 1, width: t.Width})
	}
	return targets
}
