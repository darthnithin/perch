package hummingbird

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"perch/internal/transport"
)

type fakeRunner struct {
	results  []transport.Result
	err      error
	commands [][]string
}

func (f *fakeRunner) Run(ctx context.Context, args []string) (transport.Result, error) {
	f.commands = append(f.commands, args)
	if len(f.results) == 0 {
		return transport.Result{}, f.err
	}
	return f.results[0], f.err
}
func (f *fakeRunner) Batch(ctx context.Context, args [][]string) ([]transport.Result, error) {
	f.commands = args
	return f.results, f.err
}
func TestPartialRefresh(t *testing.T) {
	f := &fakeRunner{results: []transport.Result{{Stdout: fixture(t, "queue")}, {ExitCode: 1, Stderr: "accounting unavailable"}, {Stdout: fixture(t, "partitions")}}}
	c := Client{Runner: f, User: "researcher"}
	r, e := c.Refresh(context.Background(), true, true)
	if e != nil || !r.QueueOK || r.HistoryOK || !r.PartitionsOK || r.Errors["history"] == nil {
		t.Fatal(r, e)
	}
	if len(f.commands) != 3 || !strings.Contains(strings.Join(f.commands[0], " "), "--user=researcher") {
		t.Fatal(f.commands)
	}
	f.err = errors.New("offline")
	if _, e := c.Refresh(context.Background(), true, true); e == nil {
		t.Fatal("network error hidden")
	}
}
func TestReadLogMetadata(t *testing.T) {
	jobs, e := ParseAccounting(fixture(t, "accounting"))
	if e != nil {
		t.Fatal(e)
	}
	f := &fakeRunner{results: []transport.Result{{Stdout: "1:2 13 0 1\nhello\nworld!\n"}}}
	c := Client{Runner: f}
	chunk, e := c.ReadLog(context.Background(), jobs[0], false, LogCursor{})
	if e != nil || !chunk.Reset || chunk.Cursor.Offset != 13 || chunk.Size != 13 {
		t.Fatal(chunk, e)
	}
	f.results[0].Stdout = "1:2 -1 0 1\nbad"
	if _, e = c.ReadLog(context.Background(), jobs[0], false, LogCursor{}); e == nil {
		t.Fatal("invalid metadata accepted")
	}
}
func TestLiveUnits(t *testing.T) {
	if p := parseMemory("125916K"); p == nil || *p != 128937984 {
		t.Fatal(p)
	}
	if p := parseClock("1-02:03:04.5"); p == nil || *p != 93784.5 {
		t.Fatal(p)
	}
	if parseMemory("") != nil || parseClock("") != nil {
		t.Fatal("missing sample became zero")
	}
	for _, raw := range []string{"NaN", "+Inf", "NaNM", "1e100G", "-1K"} {
		if parseMemory(raw) != nil {
			t.Errorf("invalid memory reading %q accepted", raw)
		}
	}
	for _, raw := range []string{"00:NaN", "Inf:00", "NaN-00:00", "1e308-00:00", "1e308:00:00"} {
		if parseClock(raw) != nil {
			t.Errorf("invalid CPU reading %q accepted", raw)
		}
	}
}

// Opt-in integration: existing user jobs only; no submission, cancellation, or
// remote installation. Neither scripts nor log content enter test output.
func TestHummingbirdLive(t *testing.T) {
	host := os.Getenv("PERCH_LIVE_HOST")
	if host == "" {
		t.Skip("set PERCH_LIVE_HOST to run read-only integration")
	}
	s, e := transport.NewSSH(host)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	c := Client{Runner: s, Host: host}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	report, e := c.Probe(ctx)
	if e != nil {
		t.Fatal(report, e)
	}
	if !report.Supported("jobs") || !report.Supported("history") || !report.Supported("partitions") {
		t.Fatal(report)
	}
	c.User = report.User
	r, e := c.Refresh(ctx, true, true)
	if e != nil || !r.QueueOK || !r.HistoryOK || !r.PartitionsOK {
		t.Fatal(r.Errors, e)
	}
	t.Logf("%s / %s: %d queue records, %d accounting records, %d partitions", report.Version, report.Parser, len(r.Queue), len(r.History), len(r.Partitions))
	checked := false
	for _, j := range r.Queue {
		if !j.Running() || j.TaskRange != "" {
			continue
		}
		sample, e := c.LiveResources(ctx, j)
		if e != nil {
			t.Fatal(e)
		}
		chunk, e := c.ReadLog(ctx, j, false, LogCursor{})
		if e != nil {
			t.Fatal(e)
		}
		next, e := c.ReadLog(ctx, j, false, chunk.Cursor)
		if e != nil || next.Reset || next.Cursor.Offset < chunk.Cursor.Offset {
			t.Fatal("incremental log read", e)
		}
		script, e := c.Script(ctx, j)
		if e != nil || len(script) == 0 {
			t.Fatal("script read", e)
		}
		t.Logf("Running job: live RSS available=%t; bounded log=%d bytes; incremental=%d bytes; script=%d bytes", sample.PeakRSS != nil, len(chunk.Raw), len(next.Raw), len(script))
		checked = true
		break
	}
	if !checked {
		t.Log("No running job: live resources and incremental log integration not exercised")
	}
	// A completed job can still verify non-empty output and accounting-backed
	// script retrieval after the controller has forgotten it. Retention varies.
	retained := false
	attempts := 0
	for _, j := range r.History {
		if !j.Terminal() || j.Stdout == "" || j.TaskRange != "" {
			continue
		}
		attempts++
		if attempts > 5 {
			break
		}
		chunk, err := c.ReadLog(ctx, j, false, LogCursor{})
		if err != nil || len(chunk.Raw) == 0 {
			continue
		}
		script, err := c.Script(ctx, j)
		if err != nil || len(script) == 0 {
			continue
		}
		t.Logf("Completed job: retained output=%d bytes; accounting script=%d bytes", len(chunk.Raw), len(script))
		retained = true
		break
	}
	if !retained {
		t.Log("No retained non-empty historical output/script pair found in the bounded sample")
	}
}

func TestHistoryIncludesCancelledBeforeEligibility(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/hummingbird/cancelled-before-start.json")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRunner{results: []transport.Result{{Stdout: fixture(t, "queue")}, {Stdout: string(raw)}}}
	c := Client{Runner: f, User: "researcher"}
	r, err := c.Refresh(context.Background(), true, false)
	if err != nil || !r.HistoryOK || len(r.History) != 1 {
		t.Fatal(r.Errors, err)
	}
	j := r.History[0]
	if j.State != "CANCELLED" || !j.Start.IsZero() || !j.Eligible.IsZero() || j.Elapsed != 0 {
		t.Fatal(j)
	}
	var selection string
	for _, arg := range f.commands[1] {
		if strings.HasPrefix(arg, "--state=") {
			selection = strings.TrimPrefix(arg, "--state=")
		}
	}
	for _, state := range []string{"CANCELLED", "FAILED", "COMPLETED", "PENDING", "RUNNING", "SUSPENDED", "TIMEOUT", "BOOT_FAIL", "NODE_FAIL", "PREEMPTED", "DEADLINE", "OUT_OF_MEMORY"} {
		found := false
		for _, chosen := range strings.Split(selection, ",") {
			if chosen == state {
				found = true
			}
		}
		if !found {
			t.Fatal("accounting query can omit non-eligible outcomes; missing state", state)
		}
	}
}
