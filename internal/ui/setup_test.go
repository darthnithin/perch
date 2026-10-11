package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/preferences"
)

func setupTarget(t *testing.T, m *SetupModel, label string) (int, int) {
	t.Helper()
	for y, row := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if x := strings.Index(row, label); x >= 0 {
			return ansi.StringWidth(row[:x]), y
		}
	}
	t.Fatalf("missing %q in %s", label, ansi.Strip(m.View().Content))
	return 0, 0
}

func TestSetupResponsiveMouseAndKeyboard(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			store := preferences.Store{Dir: filepath.Join(t.TempDir(), "preferences")}
			m := NewSetup(Options{Preferences: &store, PointerStyle: "css"})
			defer m.Close()
			m.Init()
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, row := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(row) != size[0] {
					t.Fatal("bad row width")
				}
			}
			if len(strings.Split(m.View().Content, "\n")) != size[1] {
				t.Fatal("bad height")
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if m.Host != "" || m.problem == "" {
				t.Fatal("empty destination accepted")
			}
			m.Update(tea.PasteMsg{Content: "alice@hb.ucsc.edu\nother"})
			if m.input.Value() != "" {
				t.Fatal("multiline paste accepted")
			}
			m.Update(tea.PasteMsg{Content: "alice@hb.ucsc.edu"})
			_, _, width := m.geometry()
			if ansi.StringWidth(m.input.View()) > width-4 {
				t.Fatal("short SSH destination clipped the text input")
			}
			x, y := setupTarget(t, m, "Remember this destination")
			m.Update(tea.MouseMotionMsg{X: x, Y: y})
			if m.pointer != "pointer" || m.remember {
				t.Fatal("hover activated checkbox")
			}
			m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			if !m.remember || m.focus != 1 {
				t.Fatal("checkbox click missed")
			}
			x, y = setupTarget(t, m, "Save & open workspace")
			_, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			if cmd == nil || !m.saving || m.Host != "" {
				t.Fatal("save did not wait for disk result")
			}
			if m.submit() != nil {
				t.Fatal("duplicate save")
			}
			m.Update(cmd())
			if m.Host != "alice@hb.ucsc.edu" || m.saving {
				t.Fatal("saved target not accepted")
			}
			v, err := store.Load()
			if err != nil || v.Host == nil || *v.Host != m.Host {
				t.Fatal(v, err)
			}
			if m.PointerReset() != ansi.SetPointerShape("default") {
				t.Fatal("pointer not restored")
			}
		})
	}
}

func TestSetupSessionOnlySaveFailureAndExistingPreferences(t *testing.T) {
	store := preferences.Store{Dir: filepath.Join(t.TempDir(), "preferences")}
	for key, value := range map[string]string{"host": "old-alias", "theme": "graphite"} {
		if err := store.Edit(context.Background(), key, &value, false); err != nil {
			t.Fatal(err)
		}
	}
	m := NewSetup(Options{Host: "new-alias", Preferences: &store})
	defer m.Close()
	m.Init()
	m.submit()
	v, _ := store.Load()
	if m.Host != "new-alias" || *v.Host != "old-alias" {
		t.Fatal("session-only changed defaults")
	}
	m.Host = ""
	m.remember = true
	cmd := m.submit()
	m.Update(cmd())
	v, _ = store.Load()
	if *v.Host != "new-alias" || *v.Theme != "graphite" {
		t.Fatal("save replaced unrelated choices", v)
	}
	m.Host = ""
	m.saving = true
	m.Update(setupSavedMsg{host: "bad", err: errors.New("disk full")})
	if m.Host != "" || m.saving || !strings.Contains(m.problem, "Save failed") {
		t.Fatal("save error opened workspace")
	}
	m.remember = false
	m.submit()
	if m.Host != "new-alias" {
		t.Fatal("could not continue without save")
	}
}

func TestSetupCancelNoMouseAndMinimumSize(t *testing.T) {
	m := NewSetup(Options{Host: "an-alias", NoMouse: true, NoColor: true})
	defer m.Close()
	m.Init()
	m.Update(tea.MouseClickMsg{X: 6, Y: 14, Button: tea.MouseLeft})
	if m.Host != "" || m.remember || strings.Contains(m.View().Content, "\x1b") {
		t.Fatal("no-mouse/no-color ignored")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focus != 2 {
		t.Fatal("session-only setup focused unavailable saving")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.focus != 0 {
		t.Fatal("backward navigation")
	}
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Host != "" {
		t.Fatal("hidden button activated")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.Host != "" {
		t.Fatal("cancel accepted a destination")
	}
}
