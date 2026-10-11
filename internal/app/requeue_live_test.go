package app_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"perch/internal/app"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/transport"
)

// Opt-in only. Two serial 1-CPU/128-MiB tasks; requeue task zero once.
// All mutations target only the newly returned array ID. Output goes to /dev/null.
func TestHummingbirdMixedArrayRequeue(t *testing.T) {
	host := os.Getenv("PERCH_REQUEUE_LIVE_HOST")
	if host == "" {
		t.Skip("set PERCH_REQUEUE_LIVE_HOST to submit and requeue a tiny synthetic array")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
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
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	token := "perch-requeue-" + hex.EncodeToString(random[:])
	script := "#!/bin/bash\nset -eu\nif [[ ${SLURM_ARRAY_TASK_ID} == 0 && ${SLURM_RESTART_COUNT:-0} == 0 ]]; then sleep 60; else sleep 3; fi\n"
	args := []string{"bash", "-lc", `umask 077; for key in ${!SBATCH_@}; do unset "$key" || exit 70; done; exec sbatch "$@"`, "perch-requeue-test", "--array=0-1%1", "--cpus-per-task=1", "--mem=128M", "--time=00:02:00", "--job-name=perch-requeue-evidence", "--requeue", "--output=/dev/null", "--error=/dev/null", "--chdir=/tmp", "--comment=" + token}
	if r, e := ssh.RunInput(ctx, append(append([]string{}, args...), "--test-only"), script); e != nil || r.ExitCode != 0 {
		t.Fatalf("validation: %v %s", e, r.Stderr)
	}
	r, err := ssh.RunInput(ctx, append(args, "--parsable"), script)
	if err != nil || r.ExitCode != 0 {
		t.Fatalf("submission unconfirmed; do not retry; comment %s: %v", token, err)
	}
	id := strings.Split(strings.TrimSpace(r.Stdout), ";")[0]
	if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(id) {
		t.Fatalf("unrecognized submission reply; inspect comment %s, do not retry", token)
	}
	parent, _ := strconv.ParseInt(id, 10, 64)
	t.Log("synthetic array", id, "comment", token)
	terminal := false
	defer func() {
		if !terminal {
			cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
			defer stop()
			if r, e := ssh.Run(cleanup, []string{"scancel", id}); e != nil || r.ExitCode != 0 {
				t.Errorf("cleanup of ONLY synthetic array %s: %v %s", id, e, r.Stderr)
			}
		}
	}()
	store := app.NewStore()
	requeued := false
	var original time.Time
	last := ""
	for {
		results, e := ssh.Batch(ctx, [][]string{{"squeue", "--json=" + hummingbird.Parser, "--jobs=" + id}, {"sacct", "--json=" + hummingbird.Parser, "--jobs=" + id, "--duplicates", "--starttime=now-1days"}})
		if e != nil || len(results) != 2 {
			t.Fatalf("refresh %s: %v", id, e)
		}
		for _, r := range results {
			if r.ExitCode != 0 {
				t.Fatalf("query %s: %s", id, r.Stderr)
			}
		}
		queue, e := hummingbird.ParseQueue(results[0].Stdout, time.Now())
		if e != nil {
			t.Fatal(e)
		}
		history, e := hummingbird.ParseAccounting(results[1].Stdout)
		if e != nil {
			t.Fatal(e)
		}
		refresh := hummingbird.Refresh{At: time.Now(), QueueOK: true, HistoryOK: true, Queue: queue, History: history}
		store.Apply(refresh)
		var lines []string
		for source, jobs := range map[string][]domain.Job{"queue": queue, "accounting": history} {
			for _, j := range jobs {
				if j.ArrayID != parent && j.ID != id {
					t.Fatal("unexpected job in scoped response")
				}
				lines = append(lines, fmt.Sprintf("%s %s state=%s restarts=%d submit=%s start=%s", source, j.ID, j.State, j.Restarts, j.Submit.UTC().Format(time.RFC3339), j.Start.UTC().Format(time.RFC3339)))
			}
		}
		rendered := strings.Join(lines, "\n")
		if rendered != last {
			t.Log("observations\n" + rendered)
			arrays := store.Arrays()
			t.Logf("current summaries: %+v", arrays)
			if len(queue)+len(history) > 0 && (len(arrays) != 1 || arrays[0].Known != 2) {
				t.Errorf("intermediate array total changed: %+v", arrays)
			}
			// Inspect only numeric/time identity fields of our synthetic accounting jobs.
			var raw struct{ Jobs []map[string]json.RawMessage }
			json.Unmarshal([]byte(results[1].Stdout), &raw)
			for _, j := range raw.Jobs {
				for key, value := range j {
					if strings.Contains(strings.ToLower(key), "sluid") || strings.Contains(strings.ToLower(key), "db_index") {
						t.Log(key, string(value))
					}
				}
			}
			last = rendered
		}
		if !requeued {
			for _, j := range queue {
				if j.ID == id+"_0" && j.Running() {
					if j.User != monitor.User || j.Name != "perch-requeue-evidence" {
						t.Fatal("refusing to requeue unrelated job")
					}
					original = j.Submit
					out, e := ssh.Run(ctx, []string{"scontrol", "requeue", j.ID})
					if e != nil || out.ExitCode != 0 {
						t.Fatalf("requeue %s: %v %s", j.ID, e, out.Stderr)
					}
					requeued = true
					t.Log("requeued only", j.ID)
					break
				}
			}
		}
		latest := map[string]domain.Job{}
		for _, j := range history {
			if old, ok := latest[j.ID]; !ok || j.AttemptAfter(old) {
				latest[j.ID] = j
			}
		}
		zero, one := latest[id+"_0"], latest[id+"_1"]
		if zero.Terminal() && one.Terminal() && len(queue) == 0 {
			terminal = true
			if !requeued || zero.Restarts != 1 || zero.State != "COMPLETED" || one.State != "COMPLETED" {
				t.Fatalf("unexpected final outcomes: zero=%s/%d one=%s", zero.State, zero.Restarts, one.State)
			}
			t.Logf("FINAL array=%s original_submit=%s retried_submit=%s sibling_submit=%s", id, original.UTC().Format(time.RFC3339), zero.Submit.UTC().Format(time.RFC3339), one.Submit.UTC().Format(time.RFC3339))
			arrays := store.Arrays()
			t.Logf("FINAL summaries: %+v gaps: %+v", arrays, store.ArrayGaps())
			if len(arrays) != 1 || arrays[0].Known != 2 || arrays[0].Completed != 2 || arrays[0].Unresolved != 0 {
				t.Fatalf("mixed-requeue summary incorrect: %+v", arrays)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("test deadline; cleanup will cancel only %s", id)
		case <-time.After(15 * time.Second):
		}
	}
}
