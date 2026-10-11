package ui

import (
	tea "charm.land/bubbletea/v2"
	"perch/internal/shortcuts"
)

func footerKey(key string) (tea.KeyPressMsg, bool) { return shortcuts.Key(key) }

func (m *Model) footerTargets() []mouseTarget {
	var targets []mouseTarget
	for _, t := range shortcuts.Targets(m.footer(), m.width-4) {
		targets = append(targets, mouseTarget{kind: "footer", key: t.Key, x: t.X + 2, y: m.height - 1, width: t.Width})
	}
	return targets
}
