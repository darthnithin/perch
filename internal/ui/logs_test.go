package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/hummingbird"
)

func logLines(first, count int) string {
	var out strings.Builder
	for i := first; i < first+count; i++ {
		fmt.Fprintf(&out, "iteration %03d: checkpoint written\n", i)
	}
	return out.String()
}

func receiveLog(m *Model, text string, reset bool) {
	m.Update(detailMsg{sequence: m.detailSequence, view: "logs", chunk: hummingbird.LogChunk{Raw: text, Reset: reset}})
}

func TestLogFollowSurvivesResizeAndPausesForReading(t *testing.T) {
	m := demoModel(100, 32)
	defer m.Close()
	m.action("l")
	receiveLog(m, logLines(0, 60), true)
	if !m.viewport.AtBottom() || !strings.Contains(m.logHeading(92), "following") {
		t.Fatal("first read did not show latest output")
	}
	for _, size := range [][2]int{{48, 18}, {140, 50}, {80, 24}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		if !m.viewport.AtBottom() || !strings.Contains(m.viewport.View(), "059") {
			t.Fatal("resize lost follow intent", size)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	if m.logFollowing || !strings.Contains(m.logHeading(72), "paused") {
		t.Fatal("Home did not pause follow")
	}
	receiveLog(m, logLines(60, 10), false)
	if m.viewport.YOffset() != 0 || !strings.Contains(m.viewport.View(), "000") {
		t.Fatal("new output pulled the reader away from earlier lines")
	}
	// A resize that happens to fit all output must not silently opt into follow.
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 100})
	if m.logFollowing || !strings.Contains(m.logHeading(132), "paused") {
		t.Fatal("a larger viewport resumed follow without a scroll action")
	}
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	receiveLog(m, logLines(70, 2), false)
	if !m.logFollowing || !m.viewport.AtBottom() || !strings.Contains(m.viewport.View(), "071") {
		t.Fatal("End did not resume following new output")
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "↑") || !strings.Contains(view, " / ") || !strings.Contains(view, "following") {
		t.Fatal("compact output lost position or follow state", view)
	}
}

func TestLogWheelSwitchAndErrorState(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.action("l")
	receiveLog(m, logLines(0, 60), true)
	wheel(m, 8, 8, tea.MouseWheelUp)
	if m.logFollowing {
		t.Fatal("wheel-up failed to pause")
	}
	offset := m.viewport.YOffset()
	receiveLog(m, "new output\n", false)
	if m.viewport.YOffset() != offset {
		t.Fatal("append changed the paused offset")
	}
	for range 30 {
		wheel(m, 8, 8, tea.MouseWheelDown)
	}
	if !m.logFollowing {
		t.Fatal("scrolling back to the bottom did not resume follow")
	}
	m.Update(detailMsg{sequence: m.detailSequence, view: "logs", err: errors.New("unavailable")})
	if !strings.Contains(m.logHeading(72), "stale") || strings.Contains(m.logHeading(72), "following") {
		t.Fatal("failed reads advertised successful following")
	}
	m.action("e")
	if m.logLoaded || !m.logFollowing || m.logRaw != "" || !strings.Contains(m.logHeading(72), "Standard error") {
		t.Fatal("stdout state leaked into stderr")
	}
	receiveLog(m, "stderr only\n", true)
	if !strings.Contains(m.viewport.View(), "stderr only") {
		t.Fatal("stderr was not rendered")
	}
}

func TestLogFailureRetainsReadableTailCursorAndPausedPosition(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.action("l")
	receiveLog(m, logLines(0, 60), true)
	m.logCursor = hummingbird.LogCursor{Identity: "file-1", Offset: int64(len(m.logRaw))}
	m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	text, cursor, offset := m.viewport.GetContent(), m.logCursor, m.viewport.YOffset()
	m.Update(detailMsg{sequence: m.detailSequence, view: "logs", err: errors.New("permission denied\x1b[2J")})
	if m.viewport.GetContent() != text || m.viewport.YOffset() != offset || m.logCursor != cursor || m.logFollowing {
		t.Fatal("failed refresh replaced the readable tail or changed reading position")
	}
	if !strings.Contains(m.logHeading(72), "stale · r retry") || !strings.Contains(ansi.Strip(m.commandHint()), "Last read failed: permission denied") {
		t.Fatal("retained output was not clearly labelled stale")
	}
	if strings.Contains(m.detailError, "\x1b") {
		t.Fatal("error contained terminal control sequences")
	}
	m.connected = false
	if !strings.Contains(m.logHeading(40), "stale · c reconnect") {
		t.Fatal("disconnected output did not offer reconnect")
	}
	m.connected = true
	receiveLog(m, logLines(60, 1), false)
	if m.detailError != "" || m.viewport.YOffset() != offset || !strings.Contains(m.viewport.GetContent(), "060") || m.logFollowing {
		t.Fatal("recovery did not append while preserving paused reading")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	m.Update(detailMsg{sequence: m.detailSequence, view: "logs", err: errors.New("unavailable")})
	receiveLog(m, logLines(61, 1), false)
	if !m.logFollowing || !m.viewport.AtBottom() || !strings.Contains(m.viewport.View(), "061") {
		t.Fatal("recovery lost follow intent")
	}
	// Rotation or truncation is still authoritative after a failed read.
	m.Update(detailMsg{sequence: m.detailSequence, view: "logs", err: errors.New("unavailable")})
	receiveLog(m, "", true)
	if m.logRaw != "" || !strings.Contains(m.viewport.GetContent(), "exists and is empty") {
		t.Fatal("successful empty reset kept obsolete output")
	}
}

func TestFirstLogReadFailureDoesNotInventRetainedOutput(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.action("l")
	m.Update(detailMsg{sequence: m.detailSequence, view: "logs", err: errors.New("file missing")})
	if m.logLoaded || !strings.Contains(m.logHeading(40), "read failed") || !strings.Contains(m.viewport.GetContent(), "file missing") {
		t.Fatal("initial failure was confused with a cached read")
	}
}
