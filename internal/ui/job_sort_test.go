package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/demo"
	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func jobIDs(jobs []domain.Job) []string {
	var ids []string
	for _, j := range jobs {
		ids = append(ids, j.ID)
	}
	return ids
}

func TestJobSortingMissingValuesStayLastAndTiesStable(t *testing.T) {
	small, large, zero := int64(2<<30), int64(8<<30), int64(0)
	rows := []domain.Job{{ID: "unknown"}, {ID: "large", RequestedMemory: &large}, {ID: "zero", RequestedMemory: &zero}, {ID: "small-a", RequestedMemory: &small}, {ID: "small-b", RequestedMemory: &small}, {ID: "gap", AwaitingTasks: 4, RequestedMemory: &large}}
	for _, tc := range []struct {
		arg  string
		want []string
	}{
		{"memory", []string{"large", "small-a", "small-b", "zero", "unknown", "gap"}},
		{"memory asc", []string{"zero", "small-a", "small-b", "large", "unknown", "gap"}},
	} {
		got := append([]domain.Job(nil), rows...)
		order, ok := parseJobSort(tc.arg)
		if !ok {
			t.Fatal(tc.arg)
		}
		sortJobRows(got, order)
		if !reflect.DeepEqual(jobIDs(got), tc.want) {
			t.Fatal(tc.arg, jobIDs(got))
		}
	}
	jobs := []domain.Job{{ID: "pending", State: "PENDING", Elapsed: 999 * time.Hour}, {ID: "short", State: "RUNNING", Elapsed: time.Minute}, {ID: "long", State: "RUNNING", Elapsed: time.Hour}, {ID: "unknown", State: "UNKNOWN"}}
	sortJobRows(jobs, defaultJobSort("elapsed"))
	if !reflect.DeepEqual(jobIDs(jobs), []string{"long", "short", "pending", "unknown"}) {
		t.Fatal(jobIDs(jobs))
	}
	for _, arg := range []string{"", "invalid", "default desc", "memory sideways", "name asc extra"} {
		if _, ok := parseJobSort(arg); ok {
			t.Fatal("accepted", arg)
		}
	}
}

func TestJobSortKeepsSelectionStoreAndTabPreferences(t *testing.T) {
	m := demoModel(140, 40)
	defer m.Close()
	original := jobIDs(m.jobs())
	store := jobIDs(m.store.Snapshot.Jobs)
	selected := m.selectedID
	command(m, "/sort name")
	if m.selectedID != selected || m.currentJobSort().key != "name" || !reflect.DeepEqual(store, jobIDs(m.store.Snapshot.Jobs)) {
		t.Fatal("sorting changed selection/store")
	}
	command(m, "/find parameter")
	id := m.selectedID
	command(m, "/sort elapsed asc")
	if m.search != "parameter" || m.selectedID != id {
		t.Fatal("sorting lost filter/selection")
	}
	for i, j := range m.jobs() {
		if i > 0 && j.Running() && m.jobs()[i-1].Running() && j.Elapsed < m.jobs()[i-1].Elapsed {
			t.Fatal("elapsed order")
		}
	}
	m.action("2")
	command(m, "/sort memory")
	if m.currentJobSort().key != "memory" {
		t.Fatal("history sort")
	}
	m.action("1")
	if m.currentJobSort() != (jobSort{key: "elapsed"}) {
		t.Fatal("history overwrote active sort")
	}
	// New store revision must reorder, without losing the chosen identity.
	m.Update(refreshMsg{generation: m.generation, result: demo.Snapshot(m.now.Add(time.Minute), 0)})
	if m.selectedID != id || m.currentJobSort().key != "elapsed" {
		t.Fatal("refresh lost sort or selection")
	}
	command(m, "/find")
	command(m, "/sort default")
	if !reflect.DeepEqual(original, jobIDs(m.jobs())) {
		t.Fatal("default did not restore normal order", jobIDs(m.jobs()))
	}
	m.action("4")
	command(m, "/sort cpus")
	if m.tab != 0 || m.view != "details" || m.currentJobSort().key != "cpus" {
		t.Fatal("sort from connection did not open jobs")
	}
}

func TestJobSortHeaderGeometryAndPalette(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		id := m.selectedID
		// Header geometry must follow both the compact and roomy table layouts.
		heading := "NAME"
		if size[0] == 48 {
			heading = "JOB / NAME"
		}
		x, y := renderedTarget(t, m, heading)
		target := m.mouseTarget(tea.Mouse{X: x, Y: y})
		if target.kind != "sort" || target.key != "name" {
			t.Fatal(size, target)
		}
		click(m, x, y)
		if m.currentJobSort().key != "name" || m.currentJobSort().descending || m.selectedID != id {
			t.Fatal("header sort", size)
		}
		click(m, x, y)
		if !m.currentJobSort().descending || !strings.Contains(ansi.Strip(m.View().Content), "NAME ↓") {
			t.Fatal("reverse heading", size)
		}
		command(m, "/find no-such-job")
		if target := m.mouseTarget(tea.Mouse{X: x, Y: y}); target.kind == "sort" {
			t.Fatal("invisible heading captured mouse", size)
		}
		command(m, "/find")
		m.selectedID = id
		m.normalizeSelection()
		command(m, "/sort")
		if !m.palette || len(m.paletteCommands()) != 7 {
			t.Fatal("sort choices", size, m.paletteCommands())
		}
		m.appendPaletteText(" memory")
		if len(m.paletteCommands()) != 1 {
			t.Fatal("sort palette filter")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.palette || m.currentJobSort().key != "memory" || m.selectedID != id {
			t.Fatal("palette sort")
		}
		view := ansi.Strip(m.View().Content)
		if size[0] == 48 {
			if !strings.Contains(view, "requested") {
				t.Fatal("compact memory metric", view)
			}
		} else if !strings.Contains(view, "REQ MEM ↓") {
			t.Fatal("memory column", view)
		}
		m.Close()
	}
}

func TestJobSortRefreshReordersWithoutRetargeting(t *testing.T) {
	m := New(nil, Options{Demo: true, ReducedMotion: true})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a := domain.Job{ID: "1", RawID: 1, Name: "alpha", State: "RUNNING", CPUs: 2, Submit: m.now}
	b := domain.Job{ID: "2", RawID: 2, Name: "beta", State: "RUNNING", CPUs: 8, Submit: m.now}
	refresh := func() {
		m.Update(refreshMsg{generation: m.generation, result: hummingbird.Refresh{At: m.now, QueueOK: true, Queue: []domain.Job{a, b}}})
	}
	refresh()
	m.selectedID = a.ID
	m.normalizeSelection()
	command(m, "/sort cpus")
	if m.selection != 1 || m.selectedID != "1" {
		t.Fatal("initial CPU order", m.selection, m.selectedID, jobIDs(m.jobs()))
	}
	a.CPUs = 16
	refresh()
	if m.selection != 0 || m.selectedID != "1" || m.jobs()[0].CPUs != 16 {
		t.Fatal("refresh did not reorder selected identity")
	}
	m.action("enter")
	if j, ok := m.selected(); !ok || j.ID != "1" || m.view != "inspect" {
		t.Fatal("inspection retargeted after sort")
	}
}
