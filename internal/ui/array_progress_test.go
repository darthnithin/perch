package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestArrayProgressDistinguishesFinishedFromSuccessful(t *testing.T) {
	a := domain.ArraySummary{Known: 10, Completed: 4, Failed: 2, Cancelled: 1, Running: 1, Pending: 1, Unresolved: 1}
	text := ansi.Strip(strings.Join(arrayProgress(a, 60), "\n"))
	for _, want := range []string{"7 / 10 observed tasks finished", "4 succeeded", "2 failed · y copies IDs", "1 cancelled", "1 running", "1 queued", "1 awaiting accounting"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s:\n%s", want, text)
		}
	}
	bar := arrayProgressBar(a, 10)
	if ansi.Strip(bar) != "━━━━━━━───" {
		t.Fatal(bar)
	}
	if bar != green.Render("━━━━")+red.Render("━━")+muted.Render("━")+border.Render("───") {
		t.Fatal("outcome segments lost their distinct colors")
	}
	for _, width := range []int{1, 2, 3, 17, 40, 100} {
		if ansi.StringWidth(arrayProgressBar(a, width)) != width {
			t.Fatalf("bar overflow at %d", width)
		}
	}
}

func TestArraySummaryExplanationFitsWithoutLosingWords(t *testing.T) {
	reason := "ambiguous array identity: overlapping members mix known and missing submission times"
	for _, width := range []int{32, 44, 55, 80} {
		rows := arrayProgress(domain.ArraySummary{SummaryError: reason}, width)
		for _, row := range rows {
			if ansi.StringWidth(row) > width {
				t.Fatalf("explanation will be clipped at width %d: %s", width, row)
			}
		}
		text := strings.Join(strings.Fields(ansi.Strip(strings.Join(rows, " "))), " ")
		if text != "Array summary unavailable: "+reason {
			t.Fatal("explanation lost words", text)
		}
	}
}

func TestFailedOrCancelledArrayCanBeFinishedWithoutSuccess(t *testing.T) {
	for _, a := range []domain.ArraySummary{{Known: 20, Failed: 20}, {Known: 20, Cancelled: 20}, {Known: 20, Unresolved: 20}} {
		text := ansi.Strip(strings.Join(arrayProgress(a, 40), "\n"))
		if strings.Contains(text, "succeeded") {
			t.Fatal("invented successes", text)
		}
		bar := ansi.Strip(arrayProgressBar(a, 40))
		if a.Unresolved > 0 {
			if strings.Contains(bar, "━") || !strings.Contains(text, "0 / 20") {
				t.Fatal("unconfirmed outcomes filled progress", text)
			}
		} else if bar != strings.Repeat("━", 40) || !strings.Contains(text, "20 / 20") {
			t.Fatal("finished tasks did not fill progress", text)
		}
	}
}

func TestShortGlancePrioritizesFailureAndUnknownCounts(t *testing.T) {
	m := demoModel(140, 40)
	defer m.Close()
	j, _ := m.selected()
	for _, height := range []int{4, 7, 9, 10, 11, 12, 18} {
		text := ansi.Strip(m.glance(j, 52, height))
		if !strings.Contains(text, "1 failed") || !strings.Contains(text, "finished") {
			t.Fatalf("height %d hid outcomes:\n%s", height, text)
		}
	}
	// A summary error at the smallest size must not reappear as 0/0 finished.
	old := domain.Job{ID: "42_[0-4000000000]", ArrayID: 42, TaskRange: "0-4000000000", State: "FAILED"}
	newer := old
	newer.ID, newer.TaskRange, newer.Restarts, newer.State = "42_[0-4000000000:100000]", "0-4000000000:100000", 1, "PENDING"
	m.store.Apply(hummingbird.Refresh{At: m.now, QueueOK: true, HistoryOK: true, Queue: []domain.Job{newer}, History: []domain.Job{old}})
	text := ansi.Strip(m.glance(newer, 52, 4))
	if !strings.Contains(text, "Array summary unavailable") || strings.Contains(text, "0/0") {
		t.Fatal(text)
	}
}
