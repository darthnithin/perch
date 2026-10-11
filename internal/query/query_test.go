package query

import (
	"context"
	"encoding/json"
	"errors"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"strings"
	"testing"
	"time"
)

type backend struct {
	refresh hummingbird.Refresh
	log     string
	calls   int
}

func (b *backend) Refresh(context.Context, bool, bool) (hummingbird.Refresh, error) {
	b.calls++
	return b.refresh, nil
}
func (b *backend) ReadLog(context.Context, domain.Job, bool, hummingbird.LogCursor) (hummingbird.LogChunk, error) {
	return hummingbird.LogChunk{Raw: b.log}, nil
}
func (*backend) Script(context.Context, domain.Job) (string, error) { return "#!/bin/sh\ntrue\n", nil }
func (*backend) LiveResources(context.Context, domain.Job) (hummingbird.LiveSample, error) {
	return hummingbird.LiveSample{At: time.Now(), Scope: "batch"}, nil
}
func TestDemoCommands(t *testing.T) {
	for _, command := range []string{"jobs", "waits", "history", "partitions", "arrays", "job", "logs", "script", "resources"} {
		t.Run(command, func(t *testing.T) {
			req := Request{Command: command}
			if command == "job" || command == "logs" || command == "script" || command == "resources" {
				req.ID = "9100_11"
			}
			result, err := (Service{Demo: true}).Execute(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if result.Source != "demo" || result.SchemaVersion != 1 || result.At.IsZero() {
				t.Fatalf("invalid envelope: %+v", result)
			}
			raw, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "\u001b") {
				t.Fatal("terminal escape in response")
			}
		})
	}
	r, _ := (Service{Demo: true}).Execute(context.Background(), Request{Command: "arrays"})
	for _, a := range r.Data.([]Array) {
		if a.ID == 9100 {
			if a.Observed != 40 || a.Failed != 1 || a.Pending != 28 || a.FailedSpec != "--array=7" {
				t.Fatalf("array counts: %+v", a)
			}
			return
		}
	}
	t.Fatal("missing demo array")
}
func TestFailuresAreNotEmptySuccess(t *testing.T) {
	for _, cmd := range []string{"jobs", "waits", "history", "partitions", "arrays", "job"} {
		b := &backend{refresh: hummingbird.Refresh{Errors: map[string]error{"jobs": errors.New("queue offline"), "history": errors.New("accounting offline"), "partitions": errors.New("partitions offline")}}}
		req := Request{Command: cmd}
		if cmd == "job" {
			req.ID = "12"
		}
		_, err := (Service{Backend: b}).Execute(context.Background(), req)
		if err == nil {
			t.Errorf("%s falsely succeeded", cmd)
		}
	}
}
func TestActiveJobSurvivesAccountingFailureAndOverridesHistory(t *testing.T) {
	for _, historyOK := range []bool{false, true} {
		b := &backend{refresh: hummingbird.Refresh{QueueOK: true, HistoryOK: historyOK, Queue: []domain.Job{{ID: "12", State: "RUNNING"}}, History: []domain.Job{{ID: "12", State: "COMPLETED"}}}}
		r, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "job", ID: "12"})
		if err != nil {
			t.Fatal(err)
		}
		if r.Data.(Job).State != "RUNNING" {
			t.Fatal("used stale accounting")
		}
	}
}
func TestLogsSanitizeAndResourcesStayUnknown(t *testing.T) {
	b := &backend{refresh: hummingbird.Refresh{QueueOK: true, Queue: []domain.Job{{ID: "12", State: "RUNNING"}}}, log: "hello\x1b]52;c;secret\a\x1b[2Jworld\n"}
	r, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "logs", ID: "12"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Data.(Text).Text != "helloworld\n" {
		t.Fatalf("unsafe log: %+v", r.Data)
	}
	r, err = (Service{Backend: b}).Execute(context.Background(), Request{Command: "resources", ID: "12"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r)
	if !strings.Contains(string(raw), `"live_peak_rss_bytes":null`) {
		t.Fatalf("missing metrics became zero: %s", raw)
	}
}
func TestInvalidRequestsNeverReachBackend(t *testing.T) {
	b := &backend{}
	for _, req := range []Request{{Command: "logs", ID: "12;touch /tmp/x"}, {Command: "job", ID: "9100_[0-9]"}, {Command: "jobs", Stderr: true}, {Command: "wat"}} {
		if _, err := (Service{Backend: b}).Execute(context.Background(), req); err == nil {
			t.Fatalf("accepted %+v", req)
		}
	}
	if b.calls != 0 {
		t.Fatal("queried cluster for invalid request")
	}
}
func TestEmptyListIsJSONArray(t *testing.T) {
	b := &backend{refresh: hummingbird.Refresh{QueueOK: true}}
	r, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "jobs"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r)
	if !strings.Contains(string(raw), `"data":[]`) {
		t.Fatalf("empty list is not []: %s", raw)
	}
}

func TestAbsentJobDoesNotInheritStaleRunningState(t *testing.T) {
	b := &backend{refresh: hummingbird.Refresh{QueueOK: true, HistoryOK: true, History: []domain.Job{{ID: "12", State: "RUNNING"}}}}
	r, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "job", ID: "12"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Data.(Job).State != "UNKNOWN" {
		t.Fatalf("stale state: %+v", r.Data)
	}
}
