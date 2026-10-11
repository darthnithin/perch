package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"perch/internal/app"
	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestJobProjectionKeepsRefreshAndFilterResultsCurrent(t *testing.T) {
	m := New(nil, Options{ReducedMotion: true})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	at := m.now
	job := func(id int64, name, state string) domain.Job {
		return domain.Job{ID: []string{"42_0", "42_1", "42_2"}[id], RawID: 43 + id, ArrayID: 42, TaskID: &id, Name: name, State: state, Submit: at}
	}
	zero, one, two := job(0, "failed-task", "FAILED"), job(1, "running-task", "RUNNING"), job(2, "queued-task", "PENDING")
	refresh := func(queue, history []domain.Job, queueOK bool) {
		m.Update(refreshMsg{generation: m.generation, result: hummingbird.Refresh{At: at, QueueOK: queueOK, HistoryOK: true, Queue: queue, History: history}})
	}
	refresh([]domain.Job{one, two}, []domain.Job{zero}, true)
	if len(m.jobs()) != 2 || m.arraySummaries()[0].FailedSpec() != "--array=0" {
		t.Fatal("initial projection wrong")
	}
	// A frame and resize reuse membership while rendering at the new size.
	m.Update(frameMsg(at))
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 24})
	m.search = "QUEUED"
	if jobs := m.jobs(); len(jobs) != 1 || jobs[0].ID != two.ID {
		t.Fatal("filter reused an old row set", jobs)
	}
	m.tab = 1
	if len(m.jobs()) != 0 {
		t.Fatal("tab reused active rows")
	}
	m.search = "failed"
	if jobs := m.jobs(); len(jobs) != 1 || jobs[0].ID != zero.ID {
		t.Fatal("history filter stale", jobs)
	}
	// A partial queue failure must preserve the old reconciled outcome even
	// though a newer accounting response is retained for the next refresh.
	zero.State = "COMPLETED"
	refresh(nil, []domain.Job{zero}, false)
	if m.arraySummaries()[0].FailedSpec() != "--array=0" {
		t.Fatal("partial refresh invented a confirmed outcome")
	}
	// Same timestamp, different task membership/outcomes and a changed name.
	one.State = "FAILED"
	two.State = "RUNNING"
	two.Name = "new-name"
	refresh([]domain.Job{two}, []domain.Job{zero, one}, true)
	m.tab = 0
	m.search = ""
	if jobs := m.jobs(); len(jobs) != 1 || jobs[0].Name != "new-name" {
		t.Fatal("refresh retained obsolete rows", jobs)
	}
	a := m.arraySummaries()[0]
	if a.Known != 3 || a.Completed != 1 || a.Running != 1 || a.FailedSpec() != "--array=1" {
		t.Fatal("refreshed rows and selection disagree", a)
	}
	m.search = "queued"
	if len(m.jobs()) != 0 {
		t.Fatal("renamed job retained old search text")
	}
	// Repeated lookup is the frame hot path. It must not rebuild large slices.
	m.search = ""
	m.jobs()
	m.arraySummaries()
	if allocs := testing.AllocsPerRun(20, func() { m.jobs(); m.arraySummaries() }); allocs != 0 {
		t.Fatalf("unchanged membership allocated %v times", allocs)
	}
}

func TestJobProjectionNoticesRestoreAndScopeReplacement(t *testing.T) {
	m := New(nil, Options{ReducedMotion: true})
	defer m.Close()
	if len(m.jobs()) != 0 || len(m.arraySummaries()) != 0 {
		t.Fatal("new model nonempty")
	}
	// Restore changes only missing-array evidence: Jobs, its slice identity and
	// Snapshot.At stay unchanged. The store revision must invalidate the cache.
	c := app.Checkpoint{At: m.now, Displays: []app.ArrayDisplay{{ID: 42, Minimum: 10, ObservedAt: m.now}}}
	if !m.store.Restore(c, m.now) {
		t.Fatal("restore rejected")
	}
	if jobs := m.jobs(); len(jobs) != 1 || jobs[0].AwaitingTasks != 10 || m.arraySummaries()[0].Unresolved != 10 {
		t.Fatal("restored evidence absent", jobs)
	}
	bad := c
	bad.At = m.now.Add(time.Hour)
	if m.store.Restore(bad, m.now) || len(m.jobs()) != 1 {
		t.Fatal("rejected restore changed visible evidence")
	}
	m.store = app.NewStore()
	if len(m.jobs()) != 0 || len(m.arraySummaries()) != 0 {
		t.Fatal("connection scope inherited previous array evidence")
	}
}

func TestReturnFromJobViewsPreservesFilterUntilListEscape(t *testing.T) {
	for _, view := range []string{"inspect", "logs", "script", "resources"} {
		m := demoModel(140, 40)
		m.search = "parameter"
		m.normalizeSelection()
		id := m.selectedID
		m.view = view
		if strings.Contains(m.commandHint(), "clears the filter") {
			t.Fatal("incorrect Escape hint", view)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.view != "details" || m.search != "parameter" || m.selectedID != id {
			t.Fatal("lost filtered context", view)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.search != "" || m.selectedID != id {
			t.Fatal("list escape did not clear filter while preserving selection", view)
		}
		m.Close()
	}
}
