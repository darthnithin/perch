package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/app"
)

func emptyModel(width, height int) *Model {
	m := demoModel(width, height)
	m.store = app.NewStore()
	m.store.Snapshot.At, m.store.Snapshot.HistoryAt = m.now, m.now
	m.normalizeSelection()
	return m
}

func TestEmptyWorkspaceActionsFollowTheRenderedLayout(t *testing.T) {
	for _, size := range [][2]int{{160, 44}, {110, 36}, {80, 24}, {48, 24}, {48, 18}} {
		for _, action := range []string{"new", "history", "partitions"} {
			m := emptyModel(size[0], size[1])
			view := ansi.Strip(m.View().Content)
			if !strings.Contains(view, "No active jobs.") || strings.Contains(view, "Select a job") {
				t.Fatal(view)
			}
			for _, row := range strings.Split(view, "\n") {
				if ansi.StringWidth(row) > size[0] {
					t.Fatal("empty workspace overflowed", size, row)
				}
			}
			labels := map[string]string{"new": "n Prepare a job", "history": "2 Recent history", "partitions": "3 Cluster partitions"}
			if m.layout().bodyHeight-2 < 10 {
				labels = map[string]string{"new": "n New job", "history": "2 History", "partitions": "3 Partitions"}
			}
			x, y := renderedTarget(t, m, labels[action])
			// The compact History/Partitions labels also occur in the top tabs.
			// Locate this action within the body rather than accepting the tab.
			for row, line := range strings.Split(view, "\n") {
				if row > m.layout().bodyTop {
					if i := strings.Index(line, labels[action]); i >= 0 {
						x, y = ansi.StringWidth(line[:i]), row
						break
					}
				}
			}
			click(m, x+2, y)
			switch action {
			case "new":
				if m.view != "launch" || m.launchForm.stage != "edit" {
					t.Fatal("did not open a draft", size)
				}
			case "history":
				if m.tab != 1 {
					t.Fatal("did not open history", size)
				}
			case "partitions":
				if m.tab != 2 {
					t.Fatal("did not open partitions", size)
				}
			}
			m.Close()
		}
	}
}

func TestEmptyWorkspaceDoesNotClaimAStaleOrUnobservedQueueIsClear(t *testing.T) {
	m := emptyModel(80, 24)
	defer m.Close()
	m.lastError = "scheduler unavailable"
	if text := ansi.Strip(m.View().Content); !strings.Contains(text, "No jobs at last refresh.") || strings.Contains(text, "No active jobs.") {
		t.Fatal(text)
	}
	m.lastError = ""
	m.store.Snapshot.At = time.Time{}
	if m.emptyWorkspace() {
		t.Fatal("unobserved queue presented as empty")
	}
	m.store.Snapshot.At = m.now
	for _, filter := range []bool{true, false} {
		m.searching = filter
		if !filter {
			m.search = "unmatched"
		}
		if m.emptyWorkspace() {
			t.Fatal("filter result became empty workspace")
		}
	}
}

func TestNewJobShortcutIsScopedAndPreservesDraft(t *testing.T) {
	m := emptyModel(80, 24)
	defer m.Close()
	press(m, 'n')
	if m.view != "launch" || m.launchForm.stage != "edit" {
		t.Fatal("n did not prepare")
	}
	m.launchForm.fields[0].input.SetValue("/scratch/researcher/project")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	press(m, 'n')
	if m.launchForm.fields[0].input.Value() != "/scratch/researcher/project" {
		t.Fatal("draft lost")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	for _, view := range []string{"inspect", "logs", "resources", "software"} {
		m.view = view
		m.action("n")
		if m.view != view {
			t.Fatal("n hijacked panel", view)
		}
	}
}
