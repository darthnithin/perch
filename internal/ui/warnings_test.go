package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/demo"
	"perch/internal/preferences"
)

func TestWarningsEditorGeometryCancelAndApply(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		defer m.Close()
		command(m, "/find parameter")
		command(m, "/warnings")
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m.warningForm.input.SetValue("15m,5m,1m")
		if m.options.TimeWarnings.String() != "5m" {
			t.Fatal("draft changed policy")
		}
		view := m.View().Content
		if len(strings.Split(view, "\n")) != size[1] || strings.Contains(ansi.Strip(view), "Find a command") {
			t.Fatal("form height or inactive input", ansi.Strip(view))
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("overflow", line)
			}
		}
		m.options.PointerStyle = "css"
		m.Update(tea.MouseMotionMsg{X: m.warningsGeometry().x + 3, Y: m.warningsGeometry().y + 4})
		if m.lastPointer != "text" {
			t.Fatal("input pointer", m.lastPointer)
		}
		footerClick(t, m, "esc back")
		if m.options.TimeWarnings.String() != "5m" || m.view != "details" || m.search != "parameter" {
			t.Fatal("cancel changed policy/context")
		}
		command(m, "/warnings")
		m.warningForm.input.SetValue("bogus")
		m.warningsKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.view != "warnings" || m.warningForm.problem == "" {
			t.Fatal("bad draft accepted")
		}
		m.warningForm.input.SetValue("off")
		footerClick(t, m, "enter apply")
		if m.options.TimeWarnings.String() != "off" || m.view != "details" {
			t.Fatal("apply")
		}
		m.store.Apply(demo.Snapshot(m.now, 2))
		if len(m.store.Events) != 0 {
			t.Fatal("disabled time warnings still emitted")
		}
	}
}

func TestWarningsExplicitSaveAndFailure(t *testing.T) {
	store := preferences.Store{Dir: filepath.Join(t.TempDir(), "preferences")}
	m := New(nil, Options{Preferences: &store})
	defer m.Close()
	m.openWarnings()
	m.warningForm.input.SetValue("1m,15m,5m")
	cmd := m.saveWarnings(true)
	if cmd == nil || m.saveWarnings(true) != nil || m.closeWarnings() != nil {
		t.Fatal("duplicate save or leave during save")
	}
	host := "owner@cluster.invalid"
	if err := store.Edit(context.Background(), "host", &host, false); err != nil {
		t.Fatal(err)
	}
	m.Update(cmd())
	v, err := store.Load()
	if err != nil || v.TimeWarnings == nil || *v.TimeWarnings != "15m,5m,1m" || *v.Host != host {
		t.Fatal(v, err)
	}
	if m.options.TimeWarnings.String() != "15m,5m,1m" {
		t.Fatal("not applied")
	}
	m.openWarnings()
	m.warningForm.input.SetValue("off")
	os.WriteFile(store.Path(), []byte("broken"), 0600)
	cmd = m.saveWarnings(true)
	m.Update(cmd())
	if m.view != "warnings" || m.warningForm.problem == "" || m.options.TimeWarnings.String() != "15m,5m,1m" {
		t.Fatal("failed save changed active policy")
	}
	m.closeWarnings()
	if raw, _ := os.ReadFile(store.Path()); string(raw) != "broken" {
		t.Fatal("bad file silently replaced")
	}
	m.options.Demo = true
	m.openWarnings()
	m.warningForm.input.SetValue("off")
	m.saveWarnings(true)
	if raw, _ := os.ReadFile(store.Path()); string(raw) != "broken" || m.options.TimeWarnings.String() != "off" {
		t.Fatal("demo saved or failed to apply")
	}
}
