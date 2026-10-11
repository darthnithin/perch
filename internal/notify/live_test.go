package notify

import (
	"context"
	"os"
	"path/filepath"
	"perch/internal/app"
	"perch/internal/hummingbird"
	"perch/internal/launch"
	"perch/internal/transport"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in: submits ONE small workload and uses only a fake notifier.
func TestHummingbirdNotificationIntegration(t *testing.T) {
	host := os.Getenv("PERCH_NOTIFY_LIVE_HOST")
	dir := os.Getenv("PERCH_NOTIFY_LIVE_WORKDIR")
	if host == "" || dir == "" {
		t.Skip("set PERCH_NOTIFY_LIVE_HOST and PERCH_NOTIFY_LIVE_WORKDIR to submit a 15-second, 1-CPU test job")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ssh, err := transport.NewSSH(host)
	if err != nil {
		t.Fatal(err)
	}
	defer ssh.Close()
	client := &hummingbird.Client{Runner: ssh, Host: host}
	report, err := client.Probe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client.User = report.User
	p, err := launch.Build(launch.Spec{Name: "perch-notify-smoke", WorkDir: dir, CPUs: 1, MemoryMiB: 128, Minutes: 1, Command: []string{"/bin/bash", "-c", "sleep 15; printf 'PERCH_NOTIFY_OK\\n'"}})
	if err != nil {
		t.Fatal(err)
	}
	launcher := launch.Client{Runner: ssh}
	if _, err = launcher.Check(ctx, p); err != nil {
		t.Fatal(err)
	}
	receipt, err := launcher.Submit(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("submitted", receipt.JobID)
	sender := &recorder{}
	cache := filepath.Join(t.TempDir(), "state")
	scope := Scope{host, report.User, report.Cluster}
	session, _, err := Open(cache, scope, sender)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { session.Close() }()
	store := app.NewStore()
	observed := false
	for {
		r, err := client.Refresh(ctx, true, false)
		if err != nil {
			t.Fatal(err)
		}
		events := store.Apply(r)
		if err = session.Process(ctx, store.Checkpoint(), events); err != nil {
			t.Fatal(err)
		}
		for _, j := range store.Snapshot.Jobs {
			if j.ID != receipt.JobID {
				continue
			}
			if !observed && !j.Terminal() {
				observed = true
				session.Close()
				var checkpoint app.Checkpoint
				session, checkpoint, err = Open(cache, scope, sender)
				if err != nil {
					t.Fatal(err)
				}
				store = app.NewStore()
				if !store.Restore(checkpoint, time.Now()) {
					t.Fatal("live checkpoint not restored")
				}
				t.Log("observed and restored", j.ID, j.AttemptKey())
			}
			if j.Terminal() {
				if !observed || j.State != "COMPLETED" {
					t.Fatalf("unobserved or unsuccessful: observed=%v state=%s", observed, j.State)
				}
				if len(sender.messages) != 1 || !strings.Contains(sender.messages[0].Body, receipt.JobID) {
					t.Fatal("expected one completion", sender.messages, events)
				}
				// Re-reading the same authoritative outcome must not notify again.
				r, err = client.Refresh(ctx, true, false)
				if err != nil {
					t.Fatal(err)
				}
				events = store.Apply(r)
				if err = session.Process(ctx, store.Checkpoint(), events); err != nil {
					t.Fatal(err)
				}
				if len(sender.messages) != 1 {
					t.Fatal("duplicate", sender.messages)
				}
				t.Log("one fake completion notification; no desktop alerts sent", j.ID, j.State, j.AttemptKey())
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("test job still queued or accounting delayed", receipt.JobID, ctx.Err())
		case <-time.After(5 * time.Second):
		}
	}
}
