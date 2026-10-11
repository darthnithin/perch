package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func searchLogs(m *Model, query string) {
	m.action("f")
	m.Update(tea.PasteMsg{Content: query})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}
func TestLogSearchNavigationAndReadingPosition(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		t.Run(string(rune(size[0])), func(t *testing.T) {
			m := demoModel(size[0], size[1])
			defer m.Close()
			m.search = "parameter"
			m.normalizeSelection()
			id := m.selectedID
			m.action("l")
			receiveLog(m, "ERROR alpha\n"+logLines(0, 45)+"error beta\n"+logLines(45, 45)+"Error gamma\n", true)
			searchLogs(m, "error")
			if m.view != "logs" || m.search != "parameter" || m.selectedID != id || m.logFollowing || len(m.readerSearch.matches) != 3 {
				t.Fatal("search changed job or followed logs", m.readerSearch)
			}
			if !strings.Contains(ansi.Strip(m.viewport.View()), "ERROR alpha") || !strings.Contains(m.readerSearchSummary(), "1 / 3") {
				t.Fatal(m.viewport.View(), m.readerSearchSummary())
			}
			m.action("]")
			if !strings.Contains(ansi.Strip(m.viewport.View()), "error beta") {
				t.Fatal("next not visible", m.viewport.View())
			}
			m.action("]")
			m.action("]")
			if m.readerSearch.current != 0 {
				t.Fatal("forward did not wrap")
			}
			m.action("[")
			if m.readerSearch.current != 2 || !strings.Contains(ansi.Strip(m.viewport.View()), "Error gamma") {
				t.Fatal("reverse did not wrap")
			}
			m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
			if !strings.Contains(ansi.Strip(m.viewport.View()), "Error gamma") {
				t.Fatal("resize lost hit")
			}
			offset := m.viewport.YOffset()
			m.action("f")
			m.readerSearch.input.SetValue("absent")
			m.readerSearchUpdate(tea.KeyPressMsg{})
			if len(m.readerSearch.matches) != 0 || !strings.Contains(m.readerSearchSummary(), "No matches in retained") {
				t.Fatal(m.readerSearchSummary())
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			if m.readerSearch.query != "error" || m.readerSearch.input.Value() != "error" || m.readerSearch.current != 2 || m.viewport.YOffset() != offset {
				t.Fatal("cancel lost prior search")
			}
			m.action("esc")
			if m.readerSearch != nil || m.view != "logs" || m.logFollowing {
				t.Fatal("clear unexpectedly left output or resumed follow")
			}
			m.action("esc")
			if m.view != "details" || m.search != "parameter" || m.selectedID != id {
				t.Fatal("return lost job/filter")
			}
		})
	}
}
func TestLogSearchChangesAndFollow(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.action("l")
	receiveLog(m, logLines(0, 40)+"needle one\n", true)
	searchLogs(m, "needle")
	offset := m.viewport.YOffset()
	receiveLog(m, "needle two\n", false)
	if len(m.readerSearch.matches) != 2 || m.readerSearch.current != 0 || m.viewport.YOffset() != offset || m.logFollowing {
		t.Fatal("append stole reading position")
	}
	m.Update(detailMsg{sequence: m.detailSequence, view: "logs", err: errors.New("permission denied")})
	if len(m.readerSearch.matches) != 2 || !strings.Contains(m.logHeading(70), "stale") {
		t.Fatal("failure discarded search")
	}
	receiveLog(m, "needle rotated\n", true)
	if len(m.readerSearch.matches) != 1 || m.readerSearch.current != -1 || strings.Contains(ansi.Strip(m.viewport.GetContent()), "needle one") {
		t.Fatal("rotated text kept old hit identity")
	}
	m.action("[")
	if m.readerSearch.current != 0 {
		t.Fatal("reverse after reset skipped last match")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if m.readerSearch != nil || !m.logFollowing {
		t.Fatal("End did not leave search and resume")
	}
	receiveLog(m, "fresh bottom\n", false)
	if !strings.Contains(m.viewport.View(), "fresh bottom") {
		t.Fatal("follow did not recover")
	}
	searchLogs(m, "needle")
	m.action("e")
	if m.readerSearch != nil || m.logLoaded {
		t.Fatal("stderr inherited stdout search")
	}
}
func TestLogSearchLiteralUnicodeWrapAndLimits(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.action("l")
	receiveLog(m, strings.Repeat("界", 90)+"\t[Error.*] CAFÉ\n"+logLines(0, 40), true)
	searchLogs(m, "[error.*]")
	if len(m.readerSearch.matches) != 1 || !strings.Contains(ansi.Strip(m.viewport.View()), "[Error.*]") {
		t.Fatal("literal/wide/wrapped match not visible", m.viewport.View())
	}
	m.action("esc")
	searchLogs(m, "café")
	if len(m.readerSearch.matches) != 1 || !strings.Contains(ansi.Strip(m.viewport.View()), "CAFÉ") {
		t.Fatal("Unicode case matching failed")
	}
	receiveLog(m, strings.Repeat("hit ", 5000), true)
	m.action("esc")
	searchLogs(m, "hit")
	if len(m.readerSearch.matches) != maxReaderMatches || !strings.Contains(m.readerSearchSummary(), "4096+") {
		t.Fatal("match cap not explicit")
	}
	m.action("f")
	m.Update(tea.PasteMsg{Content: "\x1b[2J" + strings.Repeat("z", 1000) + "\n"})
	if len([]rune(m.readerSearch.query)) > 100 || strings.Contains(m.readerSearch.query, "\x1b") {
		t.Fatal("unsafe or unbounded query")
	}
}
func TestLogSearchEmptyAndCancellation(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.action("l")
	m.action("f")
	if m.readerSearch != nil {
		t.Fatal("search started before output loaded")
	}
	receiveLog(m, "", true)
	searchLogs(m, "exists")
	if len(m.readerSearch.matches) != 0 {
		t.Fatal("searched UI placeholder")
	}
	m.action("esc")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	m.action("f")
	m.Update(tea.PasteMsg{Content: "not found"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.readerSearch != nil || !m.logFollowing {
		t.Fatal("cancel did not restore follow intent")
	}
	// End after a failed first read must not replace the failure with 'empty'.
	m.action("e")
	m.Update(detailMsg{sequence: m.detailSequence, view: "logs", err: errors.New("missing")})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if !strings.Contains(m.viewport.GetContent(), "missing") {
		t.Fatal("End erased initial read failure")
	}
}

func TestLogSearchMouseFocusAndGlobalFilter(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.action("l")
	receiveLog(m, "needle one\n", true)
	m.action("f")
	m.Update(tea.PasteMsg{Content: "needle"})
	// Refocusing an edit must not replace its cancellation checkpoint.
	m.mouseClick(tea.Mouse{X: 10, Y: m.height - 5, Button: tea.MouseLeft})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.readerSearch != nil || !m.logFollowing {
		t.Fatal("clicking the editor changed cancel semantics")
	}
	searchLogs(m, "needle")
	m.action("ctrl+k")
	for i, c := range m.paletteCommands() {
		if c.key == "f" {
			m.activatePalette(i)
			break
		}
	}
	if !m.searching || m.view != "details" || m.readerSearch != nil {
		t.Fatal("global Filter jobs became log search")
	}
}

func TestCompactLogReadingRestoresInputAndGeometry(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.action("l")
	receiveLog(m, logLines(0, 60), true)
	if m.viewport.Height() != 6 || !m.compactTextReading() {
		t.Fatal("compact logs did not gain four rows", m.viewport.Height())
	}
	if target := m.mouseTarget(tea.Mouse{X: 10, Y: m.height - 5}); target.kind == "input" {
		t.Fatal("invisible command input captured log clicks")
	}
	searchLogs(m, "checkpoint")
	if m.viewport.Height() != 6 || !strings.Contains(ansi.Strip(m.View().Content), "find checkpoint") {
		t.Fatal("committed search lost its query or reading room")
	}
	m.action("f")
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.viewport.Height() != 2 || !m.readerSearchEditing() {
		t.Fatal("search editing did not restore input")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.input.Focused() || m.compactTextReading() {
		t.Fatal("slash command input not restored")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.viewport.Height() != 6 || !m.compactTextReading() {
		t.Fatal("command cancellation lost compact reading mode")
	}
}

func BenchmarkLogSearchRetainedTail(b *testing.B) {
	m := demoModel(100, 32)
	defer m.Close()
	m.action("l")
	receiveLog(m, strings.Repeat("checkpoint error data\n", (256<<10)/22), true)
	searchLogs(m, "error")
	b.Run("reindex", func(b *testing.B) {
		for b.Loop() {
			m.indexReaderSearch(false)
		}
	})
	b.Run("frame", func(b *testing.B) {
		for b.Loop() {
			m.Update(frameMsg(m.now))
			_ = m.View()
		}
	})
}

func TestLogSearchPreservesActivityReturn(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.activity.returning = true
	m.activity.tab = m.tab
	m.activity.selectedID = m.selectedID
	m.action("l")
	receiveLog(m, "needle", true)
	searchLogs(m, "needle")
	if !m.activity.returning {
		t.Fatal("local search discarded the activity caller")
	}
	m.action("esc")
	if m.readerSearch != nil || m.view != "logs" || !m.activity.returning {
		t.Fatal("clear search skipped the output view")
	}
	m.action("esc")
	if m.view != "activity" {
		t.Fatal("search lost return to activity")
	}
}

func TestLogSearchRetentionAndNoColor(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.options.NoColor = true
	m.action("l")
	receiveLog(m, "needle old\n"+strings.Repeat("x", (256<<10)-11), true)
	searchLogs(m, "needle")
	if len(m.readerSearch.matches) != 1 || strings.Contains(m.View().Content, "\x1b") {
		t.Fatal("no-color search or initial hit failed")
	}
	receiveLog(m, strings.Repeat("y", 100), false)
	if len(m.logRaw) > 256<<10 || len(m.readerSearch.matches) != 0 || m.readerSearch.current != -1 {
		t.Fatal("retention left a discarded match searchable")
	}
	if !strings.Contains(m.readerSearchSummary(), "No matches in retained output") {
		t.Fatal("missing bounded-search scope")
	}
	for _, line := range strings.Split(m.View().Content, "\n") {
		if ansi.StringWidth(line) > 48 {
			t.Fatal("narrow search view overflowed")
		}
	}
}

func TestLogRenderPreservesCellsAndWrappedHighlights(t *testing.T) {
	for _, plain := range []string{"abcDEFghi", "a界界b  c", "café é́ end", "👨‍👩‍👧‍👦   family\nsecond line\n", "  leading and   repeated spaces  "} {
		matches := [][]int{{0, len(plain)}}
		got, row := renderReaderText(plain, matches, 0, 5, true)
		if ansi.Strip(got) != ansi.Hardwrap(plain, 5, true) || row != 0 {
			t.Fatalf("wrapping changed text: %q", got)
		}
		for _, line := range strings.Split(got, "\n") {
			if ansi.StringWidth(line) > 5 {
				t.Fatalf("wrapped line overflowed: source=%q line=%q width=%d", plain, line, ansi.StringWidth(line))
			}
			if strings.Contains(line, "\x1b") && !strings.HasSuffix(line, "\x1b[m") && !strings.HasSuffix(line, "\x1b[0m") {
				t.Fatal("highlight style leaked between physical lines", line)
			}
		}
	}
	// Multiple literal matches can fall within one grapheme; selecting either
	// still reveals/highlights that whole grapheme, without dropping combining marks.
	text := "prefix\né́"
	pos := strings.Index(text, "́")
	_, row := renderReaderText(text, [][]int{{pos, pos + 2}, {pos + 2, pos + 4}}, 1, 5, true)
	if row != 2 {
		t.Fatal("combining-mark match lost its visible row", row)
	}
}

func BenchmarkLogSearchSingleLine(b *testing.B) {
	m := demoModel(100, 32)
	defer m.Close()
	m.action("l")
	receiveLog(m, strings.Repeat("x", (256<<10)-6)+"needle", true)
	searchLogs(m, "needle")
	b.Run("reindex", func(b *testing.B) {
		for b.Loop() {
			m.indexReaderSearch(false)
		}
	})
	b.Run("frame", func(b *testing.B) {
		for b.Loop() {
			m.Update(frameMsg(m.now))
			_ = m.View()
		}
	})
}
