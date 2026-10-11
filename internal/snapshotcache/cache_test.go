package snapshotcache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"perch/internal/domain"
)

func fixture() (Entry, time.Time) {
	now := time.Now()
	at := now.Add(-time.Minute)
	e := Entry{Version: 1, Scope: Scope{"test-host", "researcher", "cluster"}, Snapshot: domain.Snapshot{At: at, HistoryAt: at, PartitionsAt: at, User: "researcher", Cluster: "cluster"}}
	j := domain.Job{ID: "100", Name: "test job", User: "researcher", Cluster: "cluster", State: "RUNNING", Submit: at, Stdout: "PRIVATE-STDOUT", Stderr: "PRIVATE-STDERR", WorkDir: "PRIVATE-DIR", Steps: []domain.Step{{ID: "PRIVATE-STEP"}}}
	e.Snapshot.Jobs = []domain.Job{j}
	e.History = []domain.Job{j}
	return e, now
}

func TestGPURequestMetadataCopiedWithoutLosingScope(t *testing.T) {
	in := []domain.Job{{GPURequests: []domain.GPURequest{{Scope: "node", Type: "p100", Count: 1}}, GPURequestNote: "Note"}}
	out := jobs(in)
	in[0].GPURequests[0].Count = 9
	if len(out[0].GPURequests) != 1 || out[0].GPURequests[0].Count != 1 || out[0].GPURequests[0].Scope != "node" || out[0].GPURequestNote != "Note" {
		t.Fatal(out)
	}
}

func TestCachedArrayRetainsPresentationPrecedence(t *testing.T) {
	e, now := fixture()
	queued := e.Snapshot.Jobs[0]
	queued.ID, queued.ArrayID, queued.TaskRange, queued.State = "42_[0-9]", 42, "0-9", "PENDING"
	queued.QueueObserved = true
	failed := queued
	failed.ID, failed.TaskRange, failed.State, failed.QueueObserved = "42_[0-10]", "0-10", "FAILED", false
	e.Snapshot.Jobs = []domain.Job{failed, queued}
	e.History = []domain.Job{failed}
	s := Store{Dir: filepath.Join(privateDir(t), "cache")}
	if err := s.Save(context.Background(), e, now); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load(e.Scope.Host, now)
	if err != nil || loaded == nil {
		t.Fatalf("load: %v", err)
	}
	a := domain.Arrays(loaded.Snapshot.Jobs)
	if len(a) != 1 || a[0].Pending != 10 || a[0].Failed != 1 || a[0].FailedSpec() != "--array=10" {
		t.Fatalf("cached display changed task classification: %+v", a)
	}
	queued.QueueObserved, queued.AwaitingAccounting, queued.State = false, true, "UNKNOWN"
	e.Snapshot.Jobs = []domain.Job{failed, queued}
	if err := s.Save(context.Background(), e, now); err != nil {
		t.Fatal(err)
	}
	loaded, err = s.Load(e.Scope.Host, now)
	if err != nil || loaded == nil {
		t.Fatalf("load unconfirmed snapshot: %v", err)
	}
	a = domain.Arrays(loaded.Snapshot.Jobs)
	if len(a) != 1 || a[0].Unresolved != 10 || a[0].Failed != 1 || a[0].FailedSpec() != "--array=10" {
		t.Fatalf("cache promoted unresolved tasks to failures: %+v", a)
	}
}
func TestRoundTripWhitelistScopeRetentionAndOrdering(t *testing.T) {
	s := Store{Dir: filepath.Join(privateDir(t), "cache")}
	e, now := fixture()
	if got, err := s.Load(e.Scope.Host, now); got != nil || err != nil {
		t.Fatal(got, err)
	}
	if _, err := os.Stat(s.Dir); !os.IsNotExist(err) {
		t.Fatal("load created state")
	}
	if err := s.Save(context.Background(), e, now); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(s.Path())
	if strings.Contains(string(raw), "PRIVATE") {
		t.Fatal("sensitive fields persisted")
	}
	got, err := s.Load(e.Scope.Host, now)
	if err != nil || got == nil || got.Snapshot.Jobs[0].Name != "test job" {
		t.Fatal(got, err)
	}
	if got, err := s.Load("another-host", now); got != nil || err != nil {
		t.Fatal("cross-host cache")
	}
	if got, err := s.Load(e.Scope.Host, now.Add(Retention)); got != nil || err == nil {
		t.Fatal("expired cache")
	}
	for _, name := range []string{s.Dir, s.Path(), filepath.Join(s.Dir, "view.lock")} {
		st, err := os.Stat(name)
		if err != nil || st.Mode().Perm()&0077 != 0 {
			t.Fatal("permissions", name)
		}
	}
	e.Snapshot.At = e.Snapshot.At.Add(-time.Second)
	e.Snapshot.Jobs[0].Name = "older"
	if err := s.Save(context.Background(), e, now); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Load(e.Scope.Host, now)
	if got.Snapshot.Jobs[0].Name != "test job" {
		t.Fatal("older writer won")
	}
	// A crashed pending file is reused under the lock, not multiplied.
	if err := os.WriteFile(filepath.Join(s.Dir, ".view.pending"), []byte("interrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	e.Snapshot.At = now.Add(-time.Second)
	e.Snapshot.Jobs[0].Name = "newer"
	if err := s.Save(context.Background(), e, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, ".view.pending")); !os.IsNotExist(err) {
		t.Fatal("temporary survived")
	}
	got, _ = s.Load(e.Scope.Host, now)
	if got.Snapshot.Jobs[0].Name != "newer" {
		t.Fatal("newer writer lost")
	}
}
func TestInvalidAndOversizedViewsDoNotReplaceGoodCache(t *testing.T) {
	s := Store{Dir: privateDir(t)}
	base, now := fixture()
	if err := s.Save(context.Background(), base, now); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.Path())
	for _, change := range []func(*Entry){
		func(e *Entry) { e.Version = 2 }, func(e *Entry) { e.Scope.User = "wrong" }, func(e *Entry) { e.Snapshot.At = now.Add(time.Hour) },
		func(e *Entry) { e.Snapshot.HistoryAt = time.Time{} }, func(e *Entry) { e.Snapshot.Jobs[0].ID = "\x1b]52;c;bad\x07" },
		func(e *Entry) { e.Snapshot.Jobs[0].TaskRange = "1-invalid" }, func(e *Entry) { e.Snapshot.Jobs = make([]domain.Job, maxJobs+1) },
	} {
		e, _ := fixture()
		change(&e)
		if err := s.Save(context.Background(), e, now); err == nil {
			t.Fatal("invalid cache accepted")
		}
		after, _ := os.ReadFile(s.Path())
		if string(before) != string(after) {
			t.Fatal("invalid cache replaced good")
		}
	}
	e, _ := fixture()
	e.Snapshot.Jobs = make([]domain.Job, maxJobs)
	for i := range e.Snapshot.Jobs {
		e.Snapshot.Jobs[i] = base.Snapshot.Jobs[0]
		e.Snapshot.Jobs[i].Reason = strings.Repeat("x", 512)
	}
	e.History = e.Snapshot.Jobs
	if err := s.Save(context.Background(), e, now); err == nil {
		t.Fatal("oversized cache accepted")
	}
	after, _ := os.ReadFile(s.Path())
	if string(before) != string(after) {
		t.Fatal("oversized replaced good")
	}
}
func TestUnsafeFilesAndDirectoryAreRejected(t *testing.T) {
	for _, name := range []string{"view.json", "view.lock", ".view.pending"} {
		for _, kind := range []string{"symlink", "fifo", "hardlink", "public"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				s := Store{Dir: privateDir(t)}
				e, now := fixture()
				path := filepath.Join(s.Dir, name)
				switch kind {
				case "symlink":
					os.Symlink(filepath.Join(privateDir(t), "missing"), path)
				case "fifo":
					unix.Mkfifo(path, 0600)
				case "hardlink":
					other := filepath.Join(privateDir(t), "source")
					os.WriteFile(other, []byte("keep"), 0600)
					os.Link(other, path)
				case "public":
					os.WriteFile(path, []byte("keep"), 0600)
					os.Chmod(path, 0644)
				}
				if err := s.Save(context.Background(), e, now); err == nil {
					t.Fatal("unsafe target accepted")
				}
				if name == "view.json" {
					if _, err := s.Load(e.Scope.Host, now); err == nil {
						t.Fatal("unsafe load accepted")
					}
				}
			})
		}
	}
	s := Store{Dir: filepath.Join(privateDir(t), "link")}
	os.Symlink(privateDir(t), s.Dir)
	e, now := fixture()
	if err := s.Save(context.Background(), e, now); err == nil {
		t.Fatal("symlink directory accepted")
	}
}
func TestCorruptionAndLockCancellation(t *testing.T) {
	s := Store{Dir: privateDir(t)}
	e, now := fixture()
	os.WriteFile(s.Path(), []byte("invalid"), 0600)
	if _, err := s.Load(e.Scope.Host, now); err == nil {
		t.Fatal("corrupt load")
	}
	if err := s.Save(context.Background(), e, now); err == nil {
		t.Fatal("implicit corrupt overwrite")
	}
	os.Remove(s.Path())
	lock, err := unix.Open(filepath.Join(s.Dir, "view.lock"), unix.O_CREAT|unix.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(lock)
	if err := unix.Flock(lock, unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Save(ctx, e, now); err == nil {
		t.Fatal("cancelled writer")
	}
	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Fatal("cancelled write created view")
	}
	if err := s.Save(context.Background(), e, now); err == nil {
		t.Fatal("contended writer did not time out")
	}
}
func TestLoadStripsUnknownContentAndOwnsValues(t *testing.T) {
	e, now := fixture()
	memory := int64(128)
	e.Snapshot.Jobs[0].PeakRSS = &memory
	c, err := Capture(e.Scope, e.Snapshot, e.History)
	if err != nil {
		t.Fatal(err)
	}
	memory = 256
	if *c.Snapshot.Jobs[0].PeakRSS != 128 {
		t.Fatal("capture shares mutable value")
	}
	e.Snapshot.Jobs[0].Name = "name\x1b[31m"
	e.Snapshot.Jobs[0].TaskRange = "1-10"
	s := Store{Dir: privateDir(t)}
	raw, _ := json.Marshal(e)
	os.WriteFile(s.Path(), raw, 0600)
	got, err := s.Load(e.Scope.Host, now)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(got)
	if strings.Contains(string(raw), "PRIVATE") || strings.Contains(got.Snapshot.Jobs[0].Name, "\x1b") {
		t.Fatal("loaded content escaped whitelist")
	}
}

func privateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
