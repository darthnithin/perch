package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestNodesReportReturnAndCompactView(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := New(nil, Options{Demo: true, ReducedMotion: true})
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.switchTab(2)
		c := m.action("n")
		if c == nil {
			t.Fatal("no node query")
		}
		m.Update(c())
		if !strings.Contains(m.viewport.GetContent(), "0 B unallocated") || !strings.Contains(m.viewport.GetContent(), "planned") {
			t.Fatal(m.viewport.GetContent())
		}
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "Node allocations") || !strings.Contains(view, "node-06") {
			t.Fatal(view)
		}
		m.action("esc")
		if m.tab != 2 || m.view != "details" {
			t.Fatal("lost origin")
		}
		m.Close()
	}
}

func TestFullHeightReportsScrollFromFirstContentLine(t *testing.T) {
	for _, commandName := range []string{"/nodes", "/priority"} {
		m := New(nil, Options{Demo: true, ReducedMotion: true})
		m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
		c := command(m, commandName)
		m.Update(c())
		before := m.viewport.YOffset()
		m.mouseWheel(tea.Mouse{X: 10, Y: m.layout().bodyTop + 1, Button: tea.MouseWheelDown})
		if m.viewport.YOffset() <= before {
			t.Fatal("first content row did not scroll", commandName)
		}
		m.Close()
	}
}

func TestNodeGPUFilterRetainsScope(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		m.switchTab(2)
		c := m.action("n")
		m.Update(c())
		m.action("g")
		if !m.nodesGPUOnly || !strings.Contains(m.softwareBody, "GPU nodes · 1 of 20") || strings.Contains(m.softwareBody, "node-06") {
			t.Fatal(m.softwareBody)
		}
		v := ansi.Strip(m.View().Content)
		if strings.Contains(v, "opens setup") || !strings.Contains(ansi.Strip(m.commandHint()), "Scheduler assignments") {
			t.Fatal("node report inherited module hints", v)
		}
		if !strings.Contains(v, "p100 · 4 / 4 allocated") {
			t.Fatal(v)
		}
		m.help = true
		m.action("g")
		m.help = false
		if !m.nodesGPUOnly {
			t.Fatal("help toggled hidden filter")
		}
		c = m.action("r")
		m.action("g")
		if !m.nodesGPUOnly {
			t.Fatal("loading toggled filter")
		}
		m.Update(c())
		if !strings.Contains(m.softwareBody, "GPU nodes · 1 of 20") {
			t.Fatal("refresh lost filter")
		}
		m.nodeRows = nil
		m.refreshNodeBody()
		if !strings.Contains(m.softwareBody, "GPU nodes · 0 of 0") || !strings.Contains(m.softwareBody, "No node records reported") {
			t.Fatal(m.softwareBody)
		}
		m.action("esc")
		if m.tab != 2 {
			t.Fatal("lost partition origin")
		}
		c = m.action("n")
		m.Update(c())
		if m.nodesGPUOnly {
			t.Fatal("new visit inherited old filter")
		}
		m.Close()
	}
}
