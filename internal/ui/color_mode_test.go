package ui

import (
	"context"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"perch/internal/preferences"
)

func TestColorModePreviewCancelAndSave(t *testing.T) {
	s := preferences.Store{Dir: filepath.Join(t.TempDir(), "prefs")}
	m := New(nil, Options{ReducedMotion: true, Preferences: &s})
	defer m.Close()
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.ANSI256})
	m.openSettings()
	m.settings.focus = 3
	if m.settingsToggle() == nil || m.colorProfile() != colorprofile.TrueColor {
		t.Fatal("no live truecolor preview")
	}
	// Renderer acknowledgements must not replace the original Auto profile.
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	m.closeSettings(false)
	if m.colorProfile() != colorprofile.ANSI256 {
		t.Fatal("cancel did not restore detected color profile")
	}
	m.openSettings()
	m.settings.focus = 3
	m.settingsToggle()
	m.Update(m.saveSettings()())
	v, err := s.Load()
	if err != nil || v.ColorMode == nil || *v.ColorMode != "truecolor" {
		t.Fatal("color preference not saved", v, err)
	}
	if m.colorProfile() != colorprofile.TrueColor {
		t.Fatal("save lost preview")
	}
	m.openSettings()
	m.settings.focus = 3
	m.settingsToggle()
	if m.colorProfile() != colorprofile.ANSI256 {
		t.Fatal("missing 256-color choice")
	}
	m.settingsToggle()
	if m.options.ColorMode != "auto" || m.colorProfile() != colorprofile.ANSI256 {
		t.Fatal("auto did not restore detection")
	}
	if err := s.Edit(context.Background(), "color-mode", nil, false); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Load()
	if v.ColorMode != nil {
		t.Fatal("unset kept color mode")
	}
}

func TestSavedColorModeOverridesDetectionButNotNoColor(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		m := New(nil, Options{ColorMode: "truecolor", NoColor: noColor})
		_, cmd := m.Update(tea.ColorProfileMsg{Profile: colorprofile.ANSI256})
		want := colorprofile.TrueColor
		if noColor {
			want = colorprofile.ASCII
		}
		if cmd == nil || m.colorProfile() != want {
			t.Fatal("wrong initial profile", noColor)
		}
		m.Close()
	}
}
