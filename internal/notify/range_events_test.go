package notify

import (
	"fmt"
	"perch/internal/app"
	"strings"
	"testing"
)

func TestRangeCountsAndIndividualTasksShareOneBanner(t *testing.T) {
	events := []app.Event{{Key: "range", ArrayID: 42, Kind: "failed", TaskCount: 10}, {Key: "individual", ArrayID: 42, Kind: "failed"}}
	m := Messages(events)
	if len(m) != 1 || m[0].Body != "Array 42 · 11 tasks failed" {
		t.Fatal(m)
	}
	events[0].TaskCount = 1
	m = Messages(events[:1])
	if len(m) != 1 || m[0].Body != "Array 42 · 1 task failed" {
		t.Fatal(m)
	}
	for i := int64(1); i <= 4; i++ {
		events = append(events, app.Event{Key: fmt.Sprint(i), ArrayID: i, Kind: "failed", TaskCount: 100})
	}
	m = Messages(events)
	if len(m) != 1 || !strings.Contains(m[0].Body, "402 failed") {
		t.Fatal(m)
	}
}
