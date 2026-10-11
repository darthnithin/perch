package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/app"
	"perch/internal/demo"
	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestActivityNavigationAndGeometry(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		defer m.Close()
		command(m, "/find embedding")
		id := m.selectedID
		command(m, "/activity")
		if !strings.Contains(ansi.Strip(m.View().Content), "No new activity yet") {
			t.Fatal("startup history was activity")
		}
		m.store.Apply(demo.Snapshot(m.now, 1))
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		if len(m.store.Events) != 1 {
			t.Fatal(m.store.Events)
		}
		key := m.activity.key
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.view != "inspect" || m.tab != 1 || m.selectedID != "9100_8" {
			t.Fatal("finished event did not open History")
		}
		m.action("esc")
		m.store.Apply(demo.Snapshot(m.now, 2))
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		if m.activity.key != key || m.activityIndex() == 0 {
			t.Fatal("refresh moved focus to new event")
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("overflow", line)
			}
		}
		if len(strings.Split(m.View().Content, "\n")) != size[1] {
			t.Fatal("height overflow")
		}
		g := m.activityGeometry()
		y := m.layout().bodyTop + 1 + g.header + (m.activityIndex()-g.start)*g.stride
		target := m.mouseTarget(tea.Mouse{X: 5, Y: y})
		if target.kind != "activity" || target.index != m.activityIndex() {
			t.Fatal(target)
		}
		m.options.PointerStyle = "css"
		m.Update(tea.MouseMotionMsg{X: 5, Y: y})
		if m.lastPointer != "pointer" || m.view != "activity" {
			t.Fatal("hover navigated or lacked hand")
		}
		m.mouseClick(tea.Mouse{X: 5, Y: y, Button: tea.MouseLeft})
		if m.view != "inspect" || m.selectedID != "9100_8" || m.tab != 0 {
			t.Fatal("wrong event target", m.selectedID, m.view)
		}
		m.action("esc")
		if m.view != "activity" || m.selectedID != id || m.search != "embedding" || m.activity.key != key {
			t.Fatal("return lost context")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
		if m.activityIndex() != 0 {
			t.Fatal("home")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
		if m.activityIndex() != len(m.store.Events)-1 {
			t.Fatal("end")
		}
		wheel(m, 5, m.layout().bodyTop+1, tea.MouseWheelUp)
		if m.activityIndex() == len(m.store.Events)-1 {
			t.Fatal("wheel")
		}
		m.action("esc")
		if m.view != "details" || m.search != "embedding" {
			t.Fatal("escape lost filter")
		}
	}
}

func TestActivityNeverTargetsHiddenOrRestartedJob(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.store.Apply(demo.Snapshot(m.now, 2))
	m.showActivity()
	m.selectActivity(0)
	e := m.store.Events[len(m.store.Events)-1]
	for _, key := range []string{"l", "s", "u", "a", "shell", "cancel-job", "cancel-array"} {
		if m.jobActionProblem(key) == "" {
			t.Fatal("hidden selection enabled", key)
		}
	}
	j, ok := m.activityJob(e)
	if !ok {
		t.Fatal("missing original attempt")
	}
	j.Restarts++
	m.store = app.NewStore()
	m.store.Apply(hummingbird.Refresh{At: m.now, QueueOK: true, Queue: []domain.Job{j}})
	m.store.Events = []app.Event{e}
	m.openActivityJob()
	if m.view != "activity" || !strings.Contains(m.notice, "no longer loaded") {
		t.Fatal("event followed restart", m.notice, m.view)
	}
	// Explicit navigation must not later bring back an unrelated activity view.
	m.activity.returning = true
	m.action("3")
	m.action("esc")
	if m.view == "activity" || m.activity.returning {
		t.Fatal("stale activity return")
	}
}

func TestActivityScopeResetAndBoundedSelection(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.showActivity()
	for i := 0; i < 120; i++ {
		j := domain.Job{ID: "1", State: "RUNNING", Submit: m.now.Add(time.Duration(i) * time.Hour)}
		m.store.Apply(hummingbird.Refresh{At: m.now, QueueOK: true, Queue: []domain.Job{j}})
		j.State = "FAILED"
		m.store.Apply(hummingbird.Refresh{At: m.now, QueueOK: true, HistoryOK: true, History: []domain.Job{j}})
		if i == 0 {
			m.selectActivity(0)
		}
	}
	if len(m.store.Events) != 100 {
		t.Fatal("unbounded events")
	}
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
	if m.activity.key != m.store.Events[99].Key {
		t.Fatal("evicted selection not repaired")
	}
}

func TestActivityUsesObservedOutcomesFromEveryTab(t *testing.T) {
	for tab := 0; tab < 4; tab++ {
		m := demoModel(140, 40)
		m.store = app.NewStore()
		jobs := []domain.Job{
			{ID: "701", Name: "successful", State: "RUNNING", Submit: m.now},
			{ID: "702", Name: "unsuccessful", State: "RUNNING", Submit: m.now},
			{ID: "703", Name: "stopped", State: "PENDING", Submit: m.now},
		}
		m.store.Apply(hummingbird.Refresh{At: m.now, QueueOK: true, Queue: jobs})
		jobs[0].State, jobs[1].State, jobs[2].State = "COMPLETED", "OUT_OF_MEMORY", "CANCELLED"
		m.store.Apply(hummingbird.Refresh{At: m.now.Add(time.Minute), QueueOK: true, HistoryOK: true, History: jobs})
		m.switchTab(tab)
		m.showActivity()
		m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
		view := ansi.Strip(m.View().Content)
		for _, want := range []string{"Finished · successful", "Failed · unsuccessful", "Cancelled · stopped", "OUT_OF_MEMORY", "CANCELLED"} {
			if !strings.Contains(view, want) {
				t.Fatal("lost scheduler outcome", want, view)
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.view != "inspect" || m.selectedID != "703" || m.tab != 1 {
			t.Fatal("wrong cancelled target")
		}
		m.action("esc")
		if m.view != "activity" || m.tab != tab {
			t.Fatal("lost origin tab")
		}
		m.Close()
	}
}
