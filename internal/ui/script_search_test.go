package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func receiveScript(m *Model, text string) {
	m.Update(detailMsg{sequence: m.detailSequence, view: "script", text: text})
}

func TestScriptSearchNavigationAndCaller(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		t.Run(fmt.Sprint(size[0]), func(t *testing.T) {
			m := demoModel(size[0], size[1])
			defer m.Close()
			m.search = "parameter"
			m.normalizeSelection()
			id := m.selectedID
			m.action("s")
			original := "#!/bin/sh\n# [ERROR.*] first\n" + logLines(0, 45) + "# [error.*] middle\n" + logLines(45, 45) + "# [Error.*] last\n"
			receiveScript(m, original)
			searchLogs(m, "[error.*]") // Same keyboard sequence in both readers.
			if m.view != "script" || m.search != "parameter" || m.selectedID != id || len(m.readerSearch.matches) != 3 || !m.logFollowing {
				t.Fatal("script search changed job/filter/log follow state")
			}
			if !strings.Contains(ansi.Strip(m.viewport.View()), "[ERROR.*] first") || !strings.Contains(m.readerSearchSummary(), "1 / 3 · submitted script") {
				t.Fatal(m.viewport.View(), m.readerSearchSummary())
			}
			m.action("]")
			m.action("]")
			m.action("]")
			if m.readerSearch.current != 0 {
				t.Fatal("forward wrap")
			}
			m.action("[")
			if m.readerSearch.current != 2 || !strings.Contains(ansi.Strip(m.viewport.View()), "[Error.*] last") {
				t.Fatal("previous wrap")
			}
			m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
			if !strings.Contains(ansi.Strip(m.viewport.View()), "[Error.*] last") || m.viewport.Height() != 6 {
				t.Fatal("resize lost readable match")
			}
			offset := m.viewport.YOffset()
			m.action("f")
			m.readerSearch.input.SetValue("absent")
			m.readerSearchUpdate(tea.KeyPressMsg{})
			if !strings.Contains(m.readerSearchSummary(), "No matches in submitted script") {
				t.Fatal(m.readerSearchSummary())
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			if m.readerSearch.current != 2 || m.readerSearch.query != "[error.*]" || m.viewport.YOffset() != offset {
				t.Fatal("cancel lost old search")
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
			if m.readerSearch == nil || !m.viewport.AtBottom() {
				t.Fatal("script End cleared search")
			}
			m.action("esc")
			if m.readerSearch != nil || m.view != "script" || m.scriptText != original {
				t.Fatal("clear changed script")
			}
			m.action("esc")
			if m.view != "details" || m.selectedID != id || m.search != "parameter" {
				t.Fatal("lost caller")
			}
		})
	}
}

func TestScriptSearchRefreshFailureAndViewBoundaries(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.action("s")
	m.action("f")
	if m.readerSearch != nil {
		t.Fatal("searched before read")
	}
	original := "needle first\n" + logLines(0, 40) + "needle last\n"
	receiveScript(m, original)
	searchLogs(m, "needle")
	m.action("]")
	offset := m.viewport.YOffset()
	receiveScript(m, original)
	if m.readerSearch.current != 1 || m.viewport.YOffset() != offset {
		t.Fatal("unchanged refresh moved match")
	}
	receiveScript(m, original+"needle added\n")
	if m.readerSearch.current != -1 || len(m.readerSearch.matches) != 3 {
		t.Fatal("changed script inherited match ordinal")
	}
	m.action("]")
	if m.readerSearch.current != 0 {
		t.Fatal("next after replacement")
	}
	m.action("f")
	m.Update(detailMsg{sequence: m.detailSequence, view: "script", err: errors.New("script unavailable")})
	if m.readerSearch != nil || m.scriptLoaded || strings.Contains(ansi.Strip(m.viewport.View()), "needle") {
		t.Fatal("failed read retained searchable script")
	}
	m.action("f")
	if m.readerSearch != nil || m.view != "script" || m.searching {
		t.Fatal("failure search opened filter")
	}
	receiveScript(m, "needle recovered")
	searchLogs(m, "needle")
	m.action("l")
	if m.readerSearch != nil || m.scriptLoaded {
		t.Fatal("script search leaked into logs")
	}
	receiveLog(m, "needle log", true)
	searchLogs(m, "needle")
	m.action("s")
	if m.readerSearch != nil {
		t.Fatal("log search leaked into script")
	}
}

func TestScriptSearchLongUnicodeAndActivityReturn(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.activity.returning = true
	m.activity.tab = m.tab
	m.activity.selectedID = m.selectedID
	m.action("s")
	raw := "#!/bin/sh\n" + strings.Repeat("界", (250<<10)/3) + "\tCAFÉ 👩‍💻\n"
	receiveScript(m, raw)
	searchLogs(m, "café")
	if len(m.readerSearch.matches) != 1 || !strings.Contains(ansi.Strip(m.viewport.View()), "CAFÉ") || m.scriptText != raw {
		t.Fatal("long Unicode search lost match or altered source")
	}
	m.action("esc")
	if !m.activity.returning || m.view != "script" {
		t.Fatal("clear skipped reader")
	}
	m.action("esc")
	if m.view != "activity" {
		t.Fatal("lost activity caller")
	}
}
