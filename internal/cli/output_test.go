package cli

import (
	"bytes"
	"context"
	"errors"
	"perch/internal/query"
	"strings"
	"testing"
	"time"
)

type brokenWriter struct{}

func TestPendingEstimateOutput(t *testing.T) {
	at := time.Unix(1800000000, 0)
	past, future := at.Add(-time.Minute), at.Add(time.Hour)
	for _, tc := range []struct {
		start *time.Time
		want  string
	}{
		{nil, "unavailable"},
		{&future, "tentative; may change"},
		{&past, "past estimate; still queued at observation"},
		{&at, "past estimate; still queued at observation"},
	} {
		var b bytes.Buffer
		j := query.Job{State: "PENDING", EstimatedStart: tc.start}
		if err := Write(&b, query.Response{At: at, Data: j}, false); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "Start estimate") || !strings.Contains(b.String(), tc.want) {
			t.Fatal(b.String())
		}
	}
}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }
func TestOutput(t *testing.T) {
	for _, cmd := range []string{"jobs", "waits", "history", "partitions", "arrays", "job", "logs", "script", "resources"} {
		req := query.Request{Command: cmd}
		if cmd == "job" || cmd == "logs" || cmd == "script" || cmd == "resources" {
			req.ID = "9100_11"
		}
		r, err := (query.Service{Demo: true}).Execute(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		for _, asJSON := range []bool{false, true} {
			var b bytes.Buffer
			if err := Write(&b, r, asJSON); err != nil {
				t.Fatal(err)
			}
			if b.Len() == 0 || strings.Contains(b.String(), "\x1b") {
				t.Fatalf("invalid output for %s", cmd)
			}
			if err := Write(brokenWriter{}, r, asJSON); err == nil {
				t.Fatal("lost write failure")
			}
		}
	}
}

func TestRecordedStepOutput(t *testing.T) {
	tasks, cpu, rss := 2, 0.0, int64(2<<20)
	r := query.Response{Data: query.Resources{Steps: []query.Step{
		{ID: "12.0", State: "COMPLETED", Tasks: &tasks, TotalCPUSeconds: &cpu, PeakRSSBytes: &rss, RSSScope: "largest recorded task peak"},
		{ID: "12.extern", State: "COMPLETED"},
	}}}
	var b bytes.Buffer
	if err := Write(&b, r, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Recorded steps", "peaks are not summed", "12.0", "12.extern", "0.000 s", "2.0 MiB", "unavailable", "largest recorded task peak"} {
		if !strings.Contains(b.String(), want) {
			t.Fatal("missing", want, b.String())
		}
	}
}
