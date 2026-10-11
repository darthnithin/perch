package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"perch/internal/transport"
)

type fakeRunner struct {
	result transport.Result
	err    error
	input  string
	calls  int
	login  *exec.Cmd
}

func (f *fakeRunner) Run(context.Context, []string) (transport.Result, error) {
	f.calls++
	return f.result, f.err
}
func (f *fakeRunner) Batch(context.Context, [][]string) ([]transport.Result, error) {
	f.calls++
	return []transport.Result{f.result}, f.err
}
func (f *fakeRunner) RunInput(_ context.Context, _ []string, input string) (transport.Result, error) {
	f.calls++
	f.input = input
	return f.result, f.err
}
func (f *fakeRunner) LoginCommand() *exec.Cmd { return f.login }
func newStore(t *testing.T, raw bool) *Store {
	t.Helper()
	s, e := New(filepath.Join(t.TempDir(), "capture"), "test", raw)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func events(t *testing.T, s *Store) []Event {
	t.Helper()
	var out []Event
	for _, name := range []string{"events.previous.jsonl", "events.jsonl"} {
		data, err := os.ReadFile(filepath.Join(s.Path(), name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			var e Event
			if err = json.Unmarshal([]byte(line), &e); err != nil {
				t.Fatal(err)
			}
			out = append(out, e)
		}
	}
	return out
}
func TestMetadataNeverIncludesUserText(t *testing.T) {
	s := newStore(t, false)
	marker := "SECRET-token-user-path-script"
	f := &fakeRunner{result: transport.Result{Stdout: marker, Stderr: marker, ExitCode: 255, Duration: time.Second}, err: &transport.Error{Kind: transport.Authentication, Message: marker}}
	r := &Runner{Base: f, Store: s}
	got, err := r.Run(context.Background(), []string{"sh", "-c", marker})
	if got != f.result || err != f.err {
		t.Fatal("changed outcome")
	}
	r.Run(context.Background(), []string{"squeue", "--json=v0.0.44", "--user=" + marker})
	data, _ := os.ReadFile(filepath.Join(s.Path(), "events.jsonl"))
	if strings.Contains(string(data), marker) {
		t.Fatal("private content in metadata")
	}
	all := events(t, s)
	e := all[1]
	if e.Operation != "other" || e.ErrorKind != "authentication" || e.StdoutBytes != len(marker) || e.DurationMS != 1000 || e.RawCaptured {
		t.Fatal(e)
	}
}
func TestRawScopeTruncationAndSubmission(t *testing.T) {
	s := newStore(t, true)
	f := &fakeRunner{result: transport.Result{Stdout: "\xff\x1b[31mRAW", Stderr: "stderr"}, login: exec.Command("true")}
	r := &Runner{Base: f, Store: s}
	for _, args := range [][]string{{"squeue", "--json=v0.0.44"}, {"sacct", "--json=v0.0.44", "--batch-script"}, {"scontrol", "write", "batch_script", "42", "-"}, {"sh", "-c", "read logs"}, {"sbatch", "--parsable"}} {
		r.Run(context.Background(), args)
	}
	r.RunInput(context.Background(), []string{"sbatch", "--parsable"}, "secret stdin script")
	if f.input != "secret stdin script" || r.LoginCommand() != f.login {
		t.Fatal("capability forwarding lost")
	}
	all := events(t, s)
	if string(all[1].Stdout) != f.result.Stdout || !all[1].RawCaptured {
		t.Fatal("raw bytes not preserved")
	}
	for _, e := range all[2:] {
		if e.RawCaptured || len(e.Stdout) > 0 || len(e.Stderr) > 0 {
			t.Fatal("raw scope expanded", e.Operation)
		}
	}
	f.result.Stdout = strings.Repeat("a", RawLimit+500)
	f.result.Stderr = "hidden beyond limit"
	r.Batch(context.Background(), [][]string{{"sacct", "--json=v0.0.44"}})
	all = events(t, s)
	e := all[len(all)-1]
	if !e.RawTruncated || len(e.Stdout) != RawLimit || len(e.Stderr) != 0 || e.StdoutBytes != RawLimit+500 {
		t.Fatal("bad truncation")
	}
	f.err = &transport.Error{Kind: transport.Protocol, Message: "raw protocol secret"}
	r.Batch(context.Background(), nil)
	all = events(t, s)
	if all[len(all)-1].ErrorKind != "protocol" || all[len(all)-1].RawCaptured {
		t.Fatal("protocol metadata")
	}
}
func TestCaptureFailureDoesNotChangeExecution(t *testing.T) {
	s := newStore(t, true)
	path := filepath.Join(s.Path(), "events.jsonl")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	f := &fakeRunner{result: transport.Result{Stdout: "accepted-ID", ExitCode: 0}}
	r := &Runner{Base: f, Store: s}
	got, err := r.RunInput(context.Background(), []string{"sbatch"}, "script")
	if err != nil || got.Stdout != "accepted-ID" || f.calls != 1 || s.Err() == nil || !strings.Contains(s.Status(), "stopped") {
		t.Fatal("capture failure changed submission outcome")
	}
	r.Run(context.Background(), []string{"squeue", "--json"})
	if f.calls != 2 {
		t.Fatal("capture failure blocked later commands")
	}
}
func TestRotatedRetentionAndMultipleWriters(t *testing.T) {
	s := newStore(t, true)
	second, err := New(s.Path(), "second", true)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var wg sync.WaitGroup
	for _, store := range []*Store{s, second} {
		wg.Add(1)
		go func(s *Store) {
			defer wg.Done()
			r := &Runner{Store: s, Base: &fakeRunner{result: transport.Result{Stdout: strings.Repeat("a", RawLimit)}}}
			for i := 0; i < 18; i++ {
				r.Run(context.Background(), []string{"squeue", "--json"})
			}
		}(store)
	}
	wg.Wait()
	if s.Err() != nil || second.Err() != nil {
		t.Fatal(s.Err(), second.Err())
	}
	all := events(t, s)
	if len(all) == 0 || len(all) >= 36 {
		t.Fatal("did not rotate")
	}
	files, err := os.ReadDir(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, f := range files {
		st, _ := f.Info()
		if st.Mode().Perm() != 0600 {
			t.Fatal("public file")
		}
		if st.Size() > FileLimit {
			t.Fatal("unbounded file")
		}
		total += st.Size()
	}
	if len(files) != 3 || total > 2*FileLimit {
		t.Fatal("retention unbounded", len(files), total)
	}
}
func TestRejectUnsafeExistingFiles(t *testing.T) {
	for _, name := range []string{"capture.lock", "events.jsonl", "events.previous.jsonl"} {
		for _, kind := range []string{"symlink", "fifo", "directory", "public", "hardlink"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "capture")
				os.Mkdir(dir, 0700)
				target := filepath.Join(dir, name)
				switch kind {
				case "symlink":
					os.Symlink(filepath.Join(dir, "missing"), target)
				case "fifo":
					unix.Mkfifo(target, 0600)
				case "directory":
					os.Mkdir(target, 0700)
				case "public":
					os.WriteFile(target, []byte("private-content"), 0644)
				case "hardlink":
					other := filepath.Join(dir, "other")
					os.WriteFile(other, []byte("private-content"), 0600)
					os.Link(other, target)
				}
				if s, err := New(dir, "test", false); err == nil {
					s.Close()
					t.Fatal("accepted unsafe file")
				}
			})
		}
	}
	dir := filepath.Join(t.TempDir(), "public")
	os.Mkdir(dir, 0755)
	if s, err := New(dir, "test", false); err == nil {
		s.Close()
		t.Fatal("accepted public directory")
	}
}
func TestWriterLockHasDeadline(t *testing.T) {
	s := newStore(t, false)
	fd, err := unix.Open(filepath.Join(s.Path(), "capture.lock"), unix.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if err = unix.Flock(fd, unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer unix.Flock(fd, unix.LOCK_UN)
	started := time.Now()
	s.record(Event{Operation: "test"})
	if s.Err() == nil || time.Since(started) > time.Second {
		t.Fatal("lock not bounded")
	}
}

func TestDirectorySymlinkOversizeAndClosedStore(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	os.Mkdir(target, 0700)
	link := filepath.Join(root, "link")
	os.Symlink(target, link)
	if s, err := New(link, "test", false); err == nil {
		s.Close()
		t.Fatal("directory symlink accepted")
	}
	for _, name := range []string{"events.jsonl", "events.previous.jsonl"} {
		dir := filepath.Join(root, name+"-dir")
		os.Mkdir(dir, 0700)
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		f.Truncate(FileLimit + 1)
		f.Close()
		if s, err := New(dir, "test", false); err == nil {
			s.Close()
			t.Fatal("oversize file accepted")
		}
	}
	s := newStore(t, false)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s.record(Event{Operation: "after-close"})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if len(events(t, s)) != 1 {
		t.Fatal("wrote after close")
	}
}
