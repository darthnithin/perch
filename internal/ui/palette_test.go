package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestPaletteSearchAndKeyboard(t *testing.T) {
	m := demoModel(80, 24)
	m.action("ctrl+k")
	for _, r := range "logs" {
		press(m, r)
	}
	if matches := m.paletteCommands(); len(matches) != 1 || matches[0].key != "l" {
		t.Fatalf("logs alias did not find output: %+v", matches)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.palette || m.view != "logs" {
		t.Fatal("filtered Enter opened wrong view")
	}
	m.action("ctrl+k")
	m.Update(tea.PasteMsg{Content: "no-such-command"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.palette || !strings.Contains(m.View().Content, "No matching commands.") {
		t.Fatal("empty search executed a command")
	}
	m.paletteQuery = "é"
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.paletteQuery != "" {
		t.Fatal("backspace split a rune")
	}
	m.Update(tea.PasteMsg{Content: "new job"})
	if matches := m.paletteCommands(); len(matches) != 1 || matches[0].key != "launch" {
		t.Fatal(matches)
	}
	m.Update(tea.PasteMsg{Content: strings.Repeat("x", 1000)})
	if len([]rune(m.paletteQuery)) != 100 {
		t.Fatal("unbounded search")
	}
}

func TestPaletteUnavailableActionKeepsContext(t *testing.T) {
	for _, tab := range []string{"3", "4"} {
		for _, key := range []string{"enter", "l", "s", "u", "shell", "copy-failed"} {
			m := demoModel(80, 24)
			m.action(tab)
			m.action("ctrl+k")
			found := false
			for i, c := range m.paletteCommands() {
				if c.key == key {
					m.paletteIndex = i
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing command %s", key)
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if !m.palette || m.view != "details" || !strings.Contains(m.commandHint(), "Choose a job") {
				t.Fatalf("tab %s, command %s dismissed palette or lost explanation: %s", tab, key, m.commandHint())
			}
			m.Close()
		}
	}
}

func TestPaletteAvailabilityAndTargetFollowCurrentSelection(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	command(m, "/find embedding")
	m.action("ctrl+k")
	m.appendPaletteText("shell")
	if !strings.Contains(m.commandHint(), "Choose one running") || strings.Contains(m.footer(), "enter run") {
		t.Fatal("pending job advertised an available shell")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.palette || m.shellForm != nil {
		t.Fatal("pending job opened shell")
	}
	m.paletteQuery, m.paletteIndex = "logs", 0
	if !strings.Contains(m.commandHint(), "9231") {
		t.Fatal("target was not visible", m.commandHint())
	}
	// A queue refresh/filter change while the menu is open must invalidate the
	// target immediately, rather than retaining eligibility from when it opened.
	m.search = "no-matching-job"
	m.normalizeSelection()
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.palette || !strings.Contains(m.commandHint(), "Choose a job") {
		t.Fatal("vanished target was activated")
	}
	m.paletteQuery = "filter"
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.palette || !m.searching || m.search != "" {
		t.Fatal("filter could not recover an empty list")
	}
}

func TestUnavailablePaletteMouseAndDirectActions(t *testing.T) {
	for _, size := range [][2]int{{140, 38}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		m.action("4")
		m.action("ctrl+k")
		m.appendPaletteText("logs")
		x, y := renderedTarget(t, m, "Read job output")
		click(m, x, y)
		if !m.palette || m.view != "details" || m.tab != 3 {
			t.Fatal("mouse dispatched unavailable action", size)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		for _, key := range []string{"enter", "l", "s", "u", "o", "y"} {
			if cmd := m.action(key); cmd != nil || m.view != "details" {
				t.Fatalf("direct %s acted on hidden job at %v", key, size)
			}
		}
		command(m, "/logs")
		if m.view != "details" || m.tab != 3 {
			t.Fatal("slash command acted on hidden selection")
		}
		m.Close()
	}
}

func TestPaletteJobReasonsAndCompactShortcuts(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	command(m, "/find embedding")
	if got := m.jobActionProblem("y"); !strings.Contains(got, "array task") {
		t.Fatal(got)
	}
	command(m, "/find parameter")
	if got := m.jobActionProblem("shell"); got != "" {
		t.Fatal(got)
	}
	m.options.Demo = false
	m.connected = false
	if got := m.jobActionProblem("shell"); !strings.Contains(got, "Reconnect") {
		t.Fatal(got)
	}
	for _, c := range commands {
		line := ansi.Strip(m.paletteRow(c, 36))
		if ansi.StringWidth(line) != 36 {
			t.Fatal("row overflow", c, line)
		}
		if m.jobActionProblem(c.key) == "" && !strings.HasSuffix(line, c.shortcut()) {
			t.Fatal("shortcut lost", c, line)
		}
	}
	if got := (paletteCommand{key: "receipts"}).shortcut(); got != "/receipts" {
		t.Fatal("not an actual invocation", got)
	}
}

func TestPaletteFilteredMouseTargets(t *testing.T) {
	for _, size := range [][2]int{{160, 44}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		m.action("ctrl+k")
		m.Update(tea.PasteMsg{Content: "show history"})
		x, y := renderedTarget(t, m, "Show history")
		wheel(m, x, y, tea.MouseWheelDown)
		click(m, x, y)
		if m.palette || m.tab != 1 {
			t.Fatal("filtered mouse target dispatched wrong command", size)
		}
	}
}

func TestSearchTextPreservesWordsAndSanitizesPaste(t *testing.T) {
	m := demoModel(80, 24)
	m.action("f")
	for _, r := range "hello world" {
		press(m, r)
	}
	if m.search != "hello world" {
		t.Fatal(m.search)
	}
	m.Update(tea.PasteMsg{Content: "\n\x1b[31mred\x1b[0m"})
	if m.search != "hello world red" {
		t.Fatal(m.search)
	}
	if got := appendSearchText("", strings.Repeat("é", 200)); len([]rune(got)) != 100 {
		t.Fatal("search cap split UTF-8")
	}
}

func TestTabsRememberIndependentFiltersAndSelections(t *testing.T) {
	m := demoModel(160, 44)
	command(m, "/find parameter")
	m.action("down")
	activeID := m.selectedID
	m.action("2")
	if m.search != "" {
		t.Fatal("jobs filter leaked to history")
	}
	command(m, "/find OUT_OF_MEMORY")
	historyID := m.selectedID
	if historyID == "" {
		t.Fatal("missing history fixture")
	}
	m.action("1")
	if m.search != "parameter" || m.selectedID != activeID {
		t.Fatal("lost active job context")
	}
	m.action("3")
	m.action("2")
	if m.search != "OUT_OF_MEMORY" || m.selectedID != historyID {
		t.Fatal("lost history context")
	}
	m.action("4")
	command(m, "/find embedding")
	if m.tab != 0 || m.search != "embedding" || len(m.jobs()) != 1 {
		t.Fatal("find from connection restored wrong filter")
	}
	command(m, "/job 9100_7")
	m.action("1")
	if m.search != "embedding" {
		t.Fatal("cross-tab jump erased saved filter")
	}
	command(m, "/job 9100")
	j, ok := m.selected()
	if !ok || j.ArrayID != 9100 || m.search != "" || m.view != "inspect" {
		t.Fatal("array parent jump failed", j)
	}
}
