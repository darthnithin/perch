package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
)

func TestJobActionsEntryAndReturn(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		defer m.Close()
		command(m, "/find parameter")
		m.action("down")
		id := m.selectedID
		m.options.PointerStyle = "css"
		x, y := footerPoint(t, m, "a actions")
		m.Update(tea.MouseMotionMsg{X: x, Y: y})
		if m.lastPointer != "pointer" || m.palette {
			t.Fatal("hover opened actions")
		}
		footerClick(t, m, "a actions")
		if !m.palette || m.paletteJob == nil || m.paletteJob.ID != id {
			t.Fatal("wrong menu target")
		}
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "Job actions · #"+id) {
			t.Fatal(view)
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("menu overflow", line)
			}
		}
		if len(m.paletteCommands()) != 8 || m.paletteCommands()[0].key != "l" {
			t.Fatal("job actions mixed with global commands")
		}
		if len(strings.Split(view, "\n")) != size[1] || strings.Contains(view, "Find a command") {
			t.Fatal("job menu retained inactive input or changed height")
		}
		g := m.paletteGeometry()
		if g.visible < 5 {
			t.Fatal("compact menu wasted action space")
		}
		for row := 0; row < g.visible; row++ {
			target := m.mouseTarget(tea.Mouse{X: g.x + 3, Y: g.y + 3 + row})
			if target.kind != "palette" || target.index != row {
				t.Fatal("card row disagrees with pointer geometry", row, target)
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.palette || m.selectedID != id || m.search != "parameter" {
			t.Fatal("menu lost list context")
		}
		m.action("ctrl+k")
		if m.paletteJob != nil || m.paletteCommands()[0].key != "launch" {
			t.Fatal("global palette retained job scope")
		}
	}
}

func TestJobActionsCannotFollowReplacementSelection(t *testing.T) {
	for _, change := range []string{"different job", "attempt", "empty"} {
		t.Run(change, func(t *testing.T) {
			m := demoModel(80, 24)
			defer m.Close()
			m.action("a")
			id := m.paletteJob.ID
			switch change {
			case "different job":
				m.selectedID = "9231"
			case "attempt":
				jobs := append([]domain.Job(nil), m.store.Snapshot.Jobs...)
				for i := range jobs {
					if jobs[i].ID == id {
						jobs[i].Restarts++
					}
				}
				m.store.Snapshot.Jobs = jobs
			case "empty":
				m.store.Snapshot.Jobs = nil
			}
			m.normalizeSelection()
			for i := range m.paletteCommands() {
				if cmd := m.activatePalette(i); cmd != nil || !m.palette {
					t.Fatal("action followed changed target")
				}
			}
			if !strings.Contains(m.paletteHint(), "Job changed") || m.paletteJob.ID != id {
				t.Fatal("target change not explained")
			}
		})
	}
}

func TestJobActionsAvailabilityAndReview(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.action("3")
	m.action("a")
	if m.palette {
		t.Fatal("actions opened for hidden selection")
	}
	m.action("1")
	command(m, "/find embedding")
	command(m, "/actions")
	m.paletteQuery = "shell"
	if cmd := m.activatePalette(0); cmd != nil || !m.palette || !strings.Contains(m.paletteHint(), "running") {
		t.Fatal("pending shell not explained")
	}
	m.paletteQuery = "cancel job"
	cmd := m.activatePalette(0)
	if cmd == nil || m.palette || m.cancelForm == nil || m.cancelForm.stage != "checking" {
		t.Fatal("action did not open read-only review")
	}
	m.Update(cmd())
	if m.cancelForm.stage != "ready" || m.lastCancellation != nil {
		t.Fatal("action sent cancellation")
	}
}

func TestJobActionsPreservePausedOutput(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.view, m.logFollowing = "logs", false
	m.viewport.SetContent(strings.Repeat("line\n", 100))
	m.viewport.SetYOffset(30)
	m.action("a")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.view != "logs" || m.logFollowing || m.viewport.YOffset() != 30 {
		t.Fatal("actions reset paused output")
	}
}
