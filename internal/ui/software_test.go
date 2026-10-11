package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/demo"
	"perch/internal/query"
)

func TestSoftwareCommandsWorkWithoutJobsAndIgnoreStaleResults(t *testing.T) {
	m := New(nil, Options{Demo: true, ReducedMotion: true})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	cmd := command(m, "/software CUDA")
	if cmd == nil || !m.detailBusy {
		t.Fatal("software query did not start")
	}
	m.Update(cmd())
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "cuda/12.8.1") || strings.Contains(view, "Select a job first") {
		t.Fatal(view)
	}
	cmd = command(m, "/module cuda/12.8.1")
	m.Update(cmd())
	if !strings.Contains(m.viewport.GetContent(), "module load cuda/12.8.1") {
		t.Fatal("missing setup command")
	}
	cmd = command(m, "/software cuda")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(cmd())
	if m.view != "details" || m.viewport.GetContent() != "" {
		t.Fatal("stale software result replaced jobs view")
	}
	cmd = command(m, "/software cuda")
	m.Update(cmd())
	m.Update(refreshMsg{generation: m.generation, result: demo.Snapshot(m.now, 0)})
	if !strings.Contains(m.viewport.GetContent(), "cuda/12.8.1") {
		t.Fatal("job refresh cleared software view")
	}
	command(m, "/module ../bad")
	if !m.input.Focused() || m.inputError == "" {
		t.Fatal("invalid module lost input error")
	}
}

func TestSoftwareCatalogNavigationAndReturn(t *testing.T) {
	for _, size := range [][2]int{{48, 18}, {80, 24}, {140, 40}} {
		m := demoModel(size[0], size[1])
		command(m, "/find parameter")
		original := m.selectedID
		m.Update(command(m, "/software")())
		if !m.catalogReady() || len(m.catalog.items) != 5 {
			t.Fatal("missing catalog")
		}
		for i := 0; i < 3; i++ {
			press(m, 'j')
		}
		if m.catalog.selected != "openmpi5/5.0.10" {
			t.Fatal("selection did not move")
		}
		g, l := m.catalogGeometry(), m.layout()
		p := tea.Mouse{X: 5, Y: l.bodyTop + 1 + g.header + (m.catalog.index-g.start)*g.stride, Button: tea.MouseLeft}
		target := m.mouseTarget(p)
		if target.kind != "module" || target.index != 3 {
			t.Fatal("module hit target", target)
		}
		before := m.catalog
		m.Update(softwareKey(m, tea.KeyEnter)())
		if m.softwareRequest.ID != "openmpi5/5.0.10" || m.catalogReturn != before {
			t.Fatal("wrong module or lost catalog")
		}
		if !strings.Contains(m.viewport.GetContent(), "module load openmpi5/5.0.10") {
			t.Fatal("missing instructions")
		}
		softwareKey(m, tea.KeyEscape)
		if m.catalog != before || m.catalog.index != 3 || m.catalogReturn != nil {
			t.Fatal("lost return selection")
		}
		// Re-enter, then cancel before the response. A late result cannot
		// replace the restored catalog or change its highlighted module.
		pending := softwareKey(m, tea.KeyEnter)
		softwareKey(m, tea.KeyEscape)
		m.Update(pending())
		if !m.catalogReady() || m.catalog.index != 3 {
			t.Fatal("late response replaced catalog")
		}
		view := ansi.Strip(m.View().Content)
		if strings.Contains(view, "Filter: parameter") {
			t.Fatal("catalog showed the hidden job-list filter")
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("catalog overflow", line)
			}
		}
		m.action("f")
		if m.input.Value() != "/software " || !m.input.Focused() || m.search != "parameter" {
			t.Fatal("catalog search changed the caller's filter")
		}
		softwareKey(m, tea.KeyEscape)
		softwareKey(m, tea.KeyEscape)
		if m.view != "details" || m.search != "parameter" || m.selectedID != original {
			t.Fatal("lost calling list")
		}
	}
}

func TestSoftwareCatalogRefreshIdentityAndFailure(t *testing.T) {
	m := demoModel(80, 24)
	m.Update(command(m, "/software")())
	m.selectModule(3)
	m.action("r")
	m.Update(detailMsg{sequence: m.detailSequence, view: "software", modules: []query.Software{{Name: "openmpi5/5.0.10"}, {Name: "cuda/12.8.1"}}})
	if m.catalog.index != 0 || m.catalog.selected != "openmpi5/5.0.10" {
		t.Fatal("refresh followed old row number")
	}
	m.action("r")
	m.Update(detailMsg{sequence: m.detailSequence, view: "software", err: errors.New("catalog unavailable")})
	if m.catalogReady() || m.action("enter") != nil {
		t.Fatal("failed read left a clickable stale result")
	}
	m.Update(command(m, "/software nonexistent")())
	softwareKey(m, tea.KeyEnter)
	if m.softwareRequest.Command != "software" || m.view != "software" {
		t.Fatal("empty catalog activated a hidden job")
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "No matching modules") {
		t.Fatal("missing empty state")
	}
}

func softwareKey(m *Model, code rune) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Code: code})
	return cmd
}
