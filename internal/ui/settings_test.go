package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/preferences"
)

func TestSettingsPreviewCancelAndSingleAnimationClock(t *testing.T) {
	m := demoModel(140, 40)
	defer m.Close()
	m.openSettings()
	m.settingsToggle()
	if m.options.Theme != "graphite" {
		t.Fatal("palette not previewed")
	}
	m.settings.focus = 1
	if m.settingsToggle() == nil || m.options.ReducedMotion || !m.framePending {
		t.Fatal("animation did not start")
	}
	m.settingsToggle() // pause with a timer still in flight
	if m.settingsToggle() != nil {
		t.Fatal("rapid pause/resume created a second frame clock")
	}
	m.settings.focus = 2
	m.settingsToggle()
	if m.options.NoMouse || !m.settings.draft.noMouse {
		t.Fatal("mouse capture changed before apply")
	}
	m.closeSettings(false)
	if m.options.Theme != "warm" || !m.options.ReducedMotion || m.options.NoMouse || m.view != "details" {
		t.Fatal("cancel did not restore original appearance")
	}
	if _, cmd := m.Update(frameMsg(m.now)); cmd != nil || m.framePending {
		t.Fatal("cancel left an animation timer chain running")
	}
}

func TestSettingsSavePreservesDestinationAndGuardsDuplicateSave(t *testing.T) {
	store := preferences.Store{Dir: filepath.Join(t.TempDir(), "preferences")}
	m := New(nil, Options{Theme: "warm", ReducedMotion: true, Preferences: &store})
	defer m.Close()
	m.view = "details"
	m.openSettings()
	m.settingsToggle()
	m.settings.focus = 2
	m.settingsToggle()
	cmd := m.saveSettings()
	if cmd == nil || m.saveSettings() != nil {
		t.Fatal("missing save or duplicate accepted")
	}
	// An independent config command wins for keys the settings form does not edit.
	host := "different@cluster.invalid"
	if err := store.Edit(context.Background(), "host", &host, false); err != nil {
		t.Fatal(err)
	}
	m.Update(cmd())
	v, err := store.Load()
	if err != nil || v.Host == nil || *v.Host != host || *v.Theme != "graphite" || *v.Mouse || !*v.ReducedMotion {
		t.Fatal("save lost a field", v, err)
	}
	if m.view != "details" || !m.options.NoMouse || m.options.Theme != "graphite" {
		t.Fatal("successful save did not apply")
	}
	m.Update(settingsSavedMsg{sequence: m.settingsSequence}) // late duplicate
	if m.view != "details" {
		t.Fatal("late reply changed navigation")
	}
}

func TestSettingsSaveFailureAndSessionOnlyModes(t *testing.T) {
	for _, demo := range []bool{false, true} {
		store := preferences.Store{Dir: filepath.Join(t.TempDir(), "preferences")}
		m := New(nil, Options{Demo: demo, Theme: "warm", ReducedMotion: true, Preferences: &store})
		m.view = "details"
		m.openSettings()
		m.settingsToggle()
		if demo {
			m.saveSettings()
			if _, err := os.Stat(store.Dir); !os.IsNotExist(err) {
				t.Fatal("demo wrote preferences")
			}
			if m.options.Theme != "graphite" || m.view != "details" {
				t.Fatal("demo preview not applied")
			}
		} else {
			os.MkdirAll(store.Dir, 0700)
			os.WriteFile(store.Path(), []byte("broken"), 0600)
			cmd := m.saveSettings()
			m.Update(cmd())
			if m.view != "settings" || m.settings.saving || m.settings.problem == "" {
				t.Fatal("save failure disappeared")
			}
			if raw, _ := os.ReadFile(store.Path()); string(raw) != "broken" {
				t.Fatal("save overwrote malformed preferences")
			}
			m.closeSettings(false)
			if m.options.Theme != "warm" {
				t.Fatal("could not cancel failed preview")
			}
		}
		m.Close()
	}
	m := New(nil, Options{Theme: "warm", ReducedMotion: true})
	defer m.Close()
	m.view = "details"
	m.openSettings()
	if m.settingsButton() != "Use this session" {
		t.Fatal("no-config offered persistence")
	}
	m.settingsToggle()
	m.saveSettings()
	if m.options.Theme != "graphite" {
		t.Fatal("session-only setting not applied")
	}
}

func TestSettingsRowsMatchMouseTargetsAcrossSizes(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {100, 32}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		defer m.Close()
		m.input.SetValue("/settings")
		m.submitInput()
		if m.view != "settings" {
			t.Fatal("slash command unavailable")
		}
		content := ansi.Strip(m.View().Content)
		for _, label := range []string{"Palette", "Chip motion", "Mouse", "Color mode", "Use for this demo"} {
			found := false
			for y, line := range strings.Split(content, "\n") {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("settings overflow", size, line)
				}
				if x := strings.Index(line, label); x >= 0 {
					// Character offset, not UTF-8 byte offset, for terminal cells.
					cell := ansi.StringWidth(line[:x])
					m.mouseClick(tea.Mouse{X: cell, Y: y, Button: tea.MouseLeft})
					found = true
					break
				}
			}
			if !found {
				t.Fatal("missing compact control", size, label, content)
			}
		}
		if m.view != "details" || m.options.Theme != "graphite" || m.options.ReducedMotion || !m.options.NoMouse || m.options.ColorMode != "truecolor" {
			t.Fatal("pointer controls did not apply expected values", size, m.options)
		}
	}
}

func TestSettingsTabNavigationCancelsPreviewAndNoColorIsRespected(t *testing.T) {
	m := demoModel(140, 40)
	defer m.Close()
	m.openSettings()
	m.settingsToggle()
	line := strings.Split(ansi.Strip(m.View().Content), "\n")[2]
	x := strings.Index(line, "2  History")
	if x < 0 {
		t.Fatal("history tab not visible")
	}
	m.mouseClick(tea.Mouse{X: ansi.StringWidth(line[:x]), Y: 2, Button: tea.MouseLeft})
	if m.view != "details" || m.tab != 1 || m.options.Theme != "warm" || m.settings != nil {
		t.Fatal("tab navigation committed or stranded the preview")
	}
	m.options.NoColor = true
	m.openSettings()
	m.settingsToggle()
	content := m.View().Content
	if content != ansi.Strip(content) || strings.Contains(content, "live preview") || !strings.Contains(content, "Color and artwork are disabled") {
		t.Fatal("no-color setting ignored")
	}
}
