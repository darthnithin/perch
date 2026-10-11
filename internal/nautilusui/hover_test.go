package nautilusui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/nautilus"
)

func motion(m *Model, x, y int) { m.Update(tea.MouseMotionMsg{X: x, Y: y}) }

func TestHoverSharesClickGeometryWithoutSelection(t *testing.T) {
	for _, size := range [][2]int{{48, 18}, {80, 24}, {120, 38}} {
		m := ready(t, size[0], size[1])
		m.options.PointerStyle = "css"
		before, uid := ansi.Strip(m.View().Content), m.selectedUID
		motion(m, 12, 2)
		if m.lastPointer != "pointer" || m.hoverTarget().kind != "tab" || ansi.Strip(m.View().Content) != before || m.selectedUID != uid || m.tab != 0 {
			t.Fatal("hover changed navigation", size)
		}
		if !strings.Contains(m.View().Content, "\x1b[4m") {
			t.Fatal("no hover highlight")
		}
		if m.syncPointer() != nil {
			t.Fatal("repeated shape emitted")
		}
		m.Update(tea.MouseClickMsg{X: 12, Y: 2, Button: tea.MouseLeft})
		if m.tab != 1 {
			t.Fatal("hover/click disagree")
		}
		// The single visible row at minimum height is selected; larger windows
		// also let us verify that a hover does not move selection to another row.
		y := 7
		if size[1] > 18 {
			y = 8
		}
		beforeUID := m.selectedUID[1]
		motion(m, 8, y)
		target := m.hoverTarget()
		if target.kind != "row" || m.selectedUID[1] != beforeUID {
			t.Fatal("hover selected row")
		}
		m.input.Focus()
		m.Update(tea.MouseClickMsg{X: 8, Y: y, Button: tea.MouseLeft})
		if m.input.Focused() {
			t.Fatal("row click left keyboard in command input")
		}
		if m.selection[1] != target.index {
			t.Fatal("row click differs from hover")
		}
		motion(m, 8, size[1]-5)
		if m.lastPointer != "text" {
			t.Fatal("input cursor")
		}
		for _, p := range [][2]int{{0, 2}, {size[0] - 1, size[1] - 5}, {8, 5}, {8, size[1] - 10}} {
			motion(m, p[0], p[1])
			if m.hoverTarget().kind != "" || m.lastPointer != "default" {
				t.Fatal("decoration is clickable", size, p)
			}
		}
		key(m, "?")
		motion(m, 8, 7)
		if m.hoverTarget().kind != "" {
			t.Fatal("help exposes hidden rows")
		}
	}
}

func TestHoverResetsOnKeyboardBlurResizeAndScopeChanges(t *testing.T) {
	m := ready(t, 80, 24)
	m.options.PointerStyle = "x11"
	for _, msg := range []tea.Msg{tea.KeyPressMsg{Code: '?'}, tea.BlurMsg{}, tea.WindowSizeMsg{Width: 48, Height: 18}, tea.PasteMsg{Content: "test"}} {
		motion(m, 12, 2)
		if m.lastPointer != "hand2" {
			t.Fatal("X11 hand")
		}
		m.Update(msg)
		if m.hoverKnown || m.lastPointer != "xterm" || m.PointerReset() != ansi.SetPointerShape("xterm") {
			t.Fatal("stale hover", msg)
		}
	}
	m.help = false
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	motion(m, 8, 7)
	if m.lastPointer != "hand2" {
		t.Fatal("row missing")
	}
	m.refresh()
	result := m.client.Refresh(context.Background())
	result.Jobs.Collection.Items = nil
	m.Update(refreshMsg{m.sequence, result})
	if m.lastPointer != "xterm" || m.hoverTarget().kind != "" {
		t.Fatal("empty refresh left phantom target")
	}
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	motion(m, 12, 2)
	if m.lastPointer != "xterm" {
		t.Fatal("too-small view target")
	}
}

func TestHoverPickerAndOutputTargets(t *testing.T) {
	m := ready(t, 80, 24)
	m.options.PointerStyle = "css"
	m.input.SetValue("/find retrying")
	m.submit()
	key(m, "l")
	if m.logs == nil || m.logs.stage != "pods" {
		t.Fatal("no picker")
	}
	motion(m, 8, 8)
	if m.hoverTarget().kind != "choice" || m.logs.index != 0 {
		t.Fatal("hover changed picker")
	}
	m.input.Focus()
	m.Update(tea.MouseClickMsg{X: 8, Y: 8, Button: tea.MouseLeft})
	if m.input.Focused() {
		t.Fatal("picker click left keyboard in command input")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.logs.stage != "containers" {
		t.Fatal("no container picker")
	}
	motion(m, 8, 8)
	if m.hoverTarget().kind != "choice" {
		t.Fatal("container target")
	}
	// Output is selectable text, not an action, even if a stale pointer remains.
	m.logs.stage = "output"
	motion(m, 8, 8)
	if m.hoverTarget().kind != "" || m.lastPointer != "default" {
		t.Fatal("output exposed hidden container")
	}
}

func TestHoverDisabledAndModifiedMouse(t *testing.T) {
	m := ready(t, 80, 24)
	motion(m, 12, 2)
	if m.lastPointer != "" || m.PointerReset() != "" {
		t.Fatal("unknown terminal emitted shape")
	}
	if !strings.Contains(m.View().Content, "\x1b[4m") {
		t.Fatal("fallback should retain highlight")
	}
	m.options.NoMouse = true
	m.options.PointerStyle = "css"
	motion(m, 12, 2)
	if m.hoverTarget().kind != "" || m.lastPointer != "" || m.View().MouseMode != tea.MouseModeNone {
		t.Fatal("disabled mouse")
	}
	m.options.NoMouse = false
	m.Update(tea.MouseMotionMsg{X: 12, Y: 2, Mod: tea.ModShift})
	if m.hoverTarget().kind != "" {
		t.Fatal("modified hover")
	}
	m.Update(tea.MouseClickMsg{X: 12, Y: 2, Button: tea.MouseRight})
	if m.tab != 0 {
		t.Fatal("right button navigated")
	}
	m.Update(tea.MouseClickMsg{X: 12, Y: 2, Button: tea.MouseLeft, Mod: tea.ModShift})
	if m.tab != 0 {
		t.Fatal("modified button navigated")
	}
}

func TestLoginResetsPointerBeforeForegroundWork(t *testing.T) {
	m := ready(t, 80, 24)
	m.options.PointerStyle = "css"
	motion(m, 12, 2)
	m.options.Connect = func(context.Context) (*nautilus.Connection, error) { return nil, errors.New("not signed in") }
	cmd := m.reconnect()
	if cmd == nil || m.hoverKnown || m.syncPointer() != nil {
		t.Fatal("login did not suspend hover")
	}
	motion(m, 12, 2)
	if m.hoverTarget().kind != "" {
		t.Fatal("target during login")
	}
	var output bytes.Buffer
	ran := false
	foreground := &loginCommand{reset: m.PointerReset(), run: func() error {
		ran = true
		if output.String() != ansi.SetPointerShape("default") {
			t.Fatal("foreground inherited hand")
		}
		return nil
	}}
	foreground.SetStdout(&output)
	if err := foreground.Run(); err != nil || !ran {
		t.Fatal("foreground", err)
	}
	m.Update(loginMsg{err: errors.New("failure")})
	if m.connecting || m.lastPointer != "default" {
		t.Fatal("hover state after failed login", m.lastPointer)
	}
	// New key input dismisses hover as in the live app.
	key(m, "4")
	if m.lastPointer != "default" {
		t.Fatal("key reset")
	}
}
