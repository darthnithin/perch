package launch

import (
	"context"
	"fmt"
	"os"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/transport"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in: two six-second array tasks (one at a time) and one dependent
// one-second job. Nothing is submitted by normal test runs.
func TestHummingbirdArrayDependencyIntegration(t *testing.T) {
	host, dir := os.Getenv("PERCH_ARRAY_LIVE_HOST"), os.Getenv("PERCH_ARRAY_LIVE_WORKDIR")
	if host == "" || dir == "" {
		t.Skip("set PERCH_ARRAY_LIVE_HOST and PERCH_ARRAY_LIVE_WORKDIR to submit a small array and dependent job")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ssh, err := transport.NewSSH(host)
	if err != nil {
		t.Fatal(err)
	}
	defer ssh.Close()
	monitor := &hummingbird.Client{Runner: ssh, Host: host}
	if err = monitor.Identify(ctx); err != nil {
		t.Fatal(err)
	}
	launcher := Client{Runner: ssh}
	spec := Spec{Name: "perch-array-smoke", WorkDir: dir, CPUs: 1, MemoryMiB: 128, Minutes: 1, Array: "0-1%1", Command: []string{"/bin/bash", "-c", `printf 'ARRAY=%s\n' "$SLURM_ARRAY_TASK_ID"; sleep 6`}}
	p, err := Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = launcher.Check(ctx, p); err != nil {
		t.Fatal(err)
	}
	parent, err := launcher.Submit(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("array", parent.JobID)
	spec.Name = "perch-afterok-smoke"
	spec.Array = ""
	spec.AfterOK = parent.JobID
	spec.Command = []string{"printf", "AFTER_OK\\n"}
	p, err = Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = launcher.Check(ctx, p); err != nil {
		t.Fatal(err)
	}
	child, err := launcher.Submit(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("dependent", child.JobID)
	parentID, _ := strconv.ParseInt(parent.JobID, 10, 64)
	for {
		r, err := monitor.Refresh(ctx, true, false)
		if err != nil {
			t.Fatal(err)
		}
		if !r.HistoryOK {
			t.Fatal(r.Errors)
		}
		completed := map[string]domain.Job{}
		for _, j := range r.History {
			if (j.ArrayID == parentID && j.TaskID != nil) || j.ID == child.JobID {
				if j.Terminal() {
					if j.State != "COMPLETED" {
						t.Fatalf("job %s %s", j.ID, j.State)
					}
					completed[j.ID] = j
				}
			}
		}
		if len(completed) == 3 {
			zero, one, after := completed[parent.JobID+"_0"], completed[parent.JobID+"_1"], completed[child.JobID]
			if zero.ID == "" || one.ID == "" || after.ID == "" {
				t.Fatal(completed)
			}
			if zero.Start.Before(one.End) && one.Start.Before(zero.End) {
				t.Fatal("concurrency limit exceeded", zero, one)
			}
			if after.Start.Before(zero.End) || after.Start.Before(one.End) {
				t.Fatal("dependency ran before array finished")
			}
			for _, j := range []domain.Job{zero, one, after} {
				log, err := monitor.ReadLog(ctx, j, false, hummingbird.LogCursor{})
				if err != nil {
					t.Fatal(err)
				}
				expected := "AFTER_OK"
				if j.TaskID != nil {
					expected = fmt.Sprintf("ARRAY=%d", *j.TaskID)
				}
				if strings.TrimSpace(log.Raw) != expected {
					t.Fatalf("job %s output %q, expected %q", j.ID, log.Raw, expected)
				}
			}
			t.Log("two serial array tasks and dependent job completed; per-task logs and dependency ordering verified", parent.JobID, child.JobID)
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("jobs still pending or accounting delayed", parent.JobID, child.JobID)
		case <-time.After(5 * time.Second):
		}
	}
}

func TestHummingbirdFailedDependencyIntegration(t *testing.T) {
	host, dir := os.Getenv("PERCH_ARRAY_LIVE_HOST"), os.Getenv("PERCH_ARRAY_LIVE_WORKDIR")
	if host == "" || dir == "" {
		t.Skip("explicit live launch variables required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ssh, err := transport.NewSSH(host)
	if err != nil {
		t.Fatal(err)
	}
	defer ssh.Close()
	monitor := &hummingbird.Client{Runner: ssh, Host: host}
	if err = monitor.Identify(ctx); err != nil {
		t.Fatal(err)
	}
	launcher := Client{Runner: ssh}
	spec := Spec{Name: "perch-dep-fail-smoke", WorkDir: dir, CPUs: 1, MemoryMiB: 128, Minutes: 1, Command: []string{"/bin/bash", "-c", "sleep 15; exit 7"}}
	p, err := Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = launcher.Check(ctx, p); err != nil {
		t.Fatal(err)
	}
	parent, err := launcher.Submit(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("failing prerequisite", parent.JobID)
	spec.Name = "perch-dep-cancel-smoke"
	spec.AfterOK = parent.JobID
	spec.Command = []string{"printf", "SHOULD_NOT_RUN\\n"}
	p, err = Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = launcher.Check(ctx, p); err != nil {
		t.Fatal(err)
	}
	child, err := launcher.Submit(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("dependent", child.JobID)
	for {
		r, err := monitor.Refresh(ctx, true, false)
		if err != nil {
			t.Fatal(err)
		}
		if !r.HistoryOK {
			t.Fatal(r.Errors)
		}
		states := map[string]string{}
		for _, j := range r.History {
			if j.ID == parent.JobID || j.ID == child.JobID {
				if j.Terminal() {
					states[j.ID] = j.State
				}
			}
		}
		if len(states) == 2 {
			if states[parent.JobID] != "FAILED" || states[child.JobID] != "CANCELLED" {
				t.Fatal(states)
			}
			t.Log("failed prerequisite cancelled its dependent job", parent.JobID, child.JobID)
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("dependency outcomes not yet confirmed", parent.JobID, child.JobID)
		case <-time.After(5 * time.Second):
		}
	}
}
