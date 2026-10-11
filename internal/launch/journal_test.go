package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"perch/internal/transport"
)

func submission(n int) Submission {
	at := time.Now().Add(time.Duration(n) * time.Second)
	return Submission{Destination: "user@cluster", Name: "test", Started: at, Receipt: Receipt{At: at, Token: fmt.Sprintf("perch-%024x", n)}}
}

func TestJournalRestartPrivacyAndImmutableAcceptance(t *testing.T) {
	j := &Journal{Dir: filepath.Join(t.TempDir(), "receipts")}
	ctx := context.Background()
	if entries, err := j.List(ctx, ""); err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
	if _, err := os.Stat(j.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("offline empty read created state", err)
	}
	s := submission(1)
	if err := j.Save(ctx, s); err != nil {
		t.Fatal(err)
	}
	j = &Journal{Dir: j.Dir}
	entries, err := j.List(ctx, "user@cluster")
	if err != nil || len(entries) != 1 || entries[0].Status() != "unconfirmed" {
		t.Fatal(entries, err)
	}
	s.Receipt.JobID = "42"
	if err = j.Save(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err = j.Save(ctx, s); err != nil {
		t.Fatal("idempotent save", err)
	}
	s.Receipt.JobID = ""
	if err = j.Save(ctx, s); err == nil {
		t.Fatal("regressed accepted receipt")
	}
	entries, err = j.List(ctx, "another@cluster")
	if err != nil || len(entries) != 0 {
		t.Fatal("destination filter", entries, err)
	}
	info, _ := os.Stat(filepath.Join(j.Dir, "journal.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	raw, _ := os.ReadFile(filepath.Join(j.Dir, "journal.json"))
	for _, forbidden := range []string{"script", "command", "working_directory", "stderr", "stdout", "password"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatal("journal stored extra data", forbidden)
		}
	}
}

func TestConcurrentJournalWritersKeepAllReceipts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "receipts")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			j := &Journal{Dir: dir}
			if err := j.Save(context.Background(), submission(n)); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	entries, err := (&Journal{Dir: dir}).List(context.Background(), "")
	if err != nil || len(entries) != 20 {
		t.Fatal(len(entries), err)
	}
}

func TestJournalRejectsUnsafeFilesAndCancelledLock(t *testing.T) {
	for _, kind := range []string{"corrupt", "oversized", "public", "link", "fifo", "lock-link", "directory-link", "public-dir"} {
		t.Run(kind, func(t *testing.T) {
			j := &Journal{Dir: filepath.Join(t.TempDir(), "receipts")}
			os.Mkdir(j.Dir, 0700)
			path := filepath.Join(j.Dir, "journal.json")
			switch kind {
			case "corrupt":
				os.WriteFile(path, []byte("bad JSON"), 0600)
			case "oversized":
				os.WriteFile(path, []byte(strings.Repeat("x", journalBytes+1)), 0600)
			case "public":
				os.WriteFile(path, []byte("{}"), 0600)
				os.Chmod(path, 0644)
			case "link":
				os.Symlink(filepath.Join(t.TempDir(), "target"), path)
			case "fifo":
				unix.Mkfifo(path, 0600)
			case "lock-link":
				os.Symlink(filepath.Join(t.TempDir(), "target"), filepath.Join(j.Dir, "journal.lock"))
			case "directory-link":
				os.Remove(j.Dir)
				os.Symlink(t.TempDir(), j.Dir)
			case "public-dir":
				os.Chmod(j.Dir, 0755)
			}
			if err := j.Save(context.Background(), submission(1)); err == nil {
				t.Fatal("unsafe state accepted")
			}
		})
	}
	j := &Journal{Dir: filepath.Join(t.TempDir(), "receipts")}
	j.Save(context.Background(), submission(1))
	lock, err := os.OpenFile(filepath.Join(j.Dir, "journal.lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err = j.List(ctx, ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("lock wait not cancelled", err)
	}
}

func TestJournalBoundNeverEvictsUnconfirmed(t *testing.T) {
	j := &Journal{Dir: filepath.Join(t.TempDir(), "receipts")}
	os.Mkdir(j.Dir, 0700)
	saved := journalData{Version: 1}
	for i := 0; i < journalLimit; i++ {
		saved.Entries = append(saved.Entries, submission(i))
	}
	raw, _ := json.Marshal(saved)
	path := filepath.Join(j.Dir, "journal.json")
	os.WriteFile(path, raw, 0600)
	if err := j.Save(context.Background(), submission(1001)); err == nil {
		t.Fatal("silently dropped unconfirmed request")
	}
	saved.Entries[500].Receipt.JobID = "42"
	raw, _ = json.Marshal(saved)
	os.WriteFile(path, raw, 0600)
	if err := j.Save(context.Background(), submission(1001)); err != nil {
		t.Fatal(err)
	}
	entries, err := j.List(context.Background(), "")
	if err != nil || len(entries) != journalLimit {
		t.Fatal(len(entries), err)
	}
	for _, entry := range entries {
		if entry.Receipt.JobID == "42" {
			t.Fatal("accepted record was not evicted")
		}
	}
}

func TestJournalRecoversInterruptedStagingWithoutLosingCommittedRecords(t *testing.T) {
	j := &Journal{Dir: filepath.Join(t.TempDir(), "receipts")}
	if err := j.Save(context.Background(), submission(1)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(j.Dir, ".journal.next")
	if err := os.WriteFile(path, []byte("partial write"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := j.Save(context.Background(), submission(2)); err != nil {
		t.Fatal(err)
	}
	entries, err := j.List(context.Background(), "")
	if err != nil || len(entries) != 2 {
		t.Fatal(entries, err)
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("staging residue", err)
	}
}

type journalRunner struct {
	runner
	onSubmit func()
}

func (r *journalRunner) RunInput(ctx context.Context, args []string, input string) (transport.Result, error) {
	if r.onSubmit != nil {
		r.onSubmit()
	}
	return r.runner.RunInput(ctx, args, input)
}

func TestJournalPersistsIntentBeforeTransportAndAcceptedReplyAfterCancellation(t *testing.T) {
	j := &Journal{Dir: filepath.Join(t.TempDir(), "receipts")}
	ctx, cancel := context.WithCancel(context.Background())
	r := &journalRunner{runner: runner{result: transport.Result{Stdout: "42\n"}}}
	r.onSubmit = func() {
		entries, err := j.List(context.Background(), "")
		if err != nil || len(entries) != 1 || entries[0].Status() != "unconfirmed" {
			t.Fatal("transport preceded durable intent", entries, err)
		}
		cancel()
	}
	p, _ := Build(spec())
	receipt, err := (Client{Runner: r, Journal: j, Destination: "user@cluster"}).Submit(ctx, p)
	if err != nil || receipt.JobID != "42" || r.calls != 1 {
		t.Fatal(receipt, err, r.calls)
	}
	entries, err := j.List(context.Background(), "")
	if err != nil || len(entries) != 1 || entries[0].Receipt.JobID != "42" {
		t.Fatal(entries, err)
	}
}

func TestJournalSubmissionFailuresRetainTruth(t *testing.T) {
	p, _ := Build(spec())
	for _, failure := range []string{"before", "network", "crash", "after"} {
		t.Run(failure, func(t *testing.T) {
			j := &Journal{Dir: filepath.Join(t.TempDir(), "receipts")}
			r := &journalRunner{runner: runner{result: transport.Result{Stdout: "42\n"}}}
			switch failure {
			case "before":
				os.Mkdir(j.Dir, 0755)
				os.Chmod(j.Dir, 0755)
			case "network":
				r.err = errors.New("connection lost")
			case "crash":
				r.onSubmit = func() { panic("simulated process interruption") }
			case "after":
				r.onSubmit = func() { os.Chmod(j.Dir, 0755) }
			}
			var receipt Receipt
			var err error
			func() {
				if failure == "crash" {
					defer func() {
						if recover() == nil {
							t.Error("did not interrupt")
						}
					}()
				}
				receipt, err = (Client{Runner: r, Journal: j, Destination: "user@cluster"}).Submit(context.Background(), p)
			}()
			os.Chmod(j.Dir, 0700)
			entries, readErr := (&Journal{Dir: j.Dir}).List(context.Background(), "")
			if readErr != nil {
				t.Fatal(readErr)
			}
			if failure == "before" {
				if err == nil || r.calls != 0 || receipt.Token != "" || len(entries) != 0 {
					t.Fatal("submitted without intent", receipt, err, r.calls)
				}
				return
			}
			if len(entries) != 1 || entries[0].Status() != "unconfirmed" {
				t.Fatal(entries)
			}
			if failure == "after" && (receipt.JobID != "42" || err == nil || !strings.Contains(err.Error(), "do not resubmit")) {
				t.Fatal("accepted reply lost", receipt, err)
			}
			if failure == "network" && (err == nil || receipt.Token != entries[0].Receipt.Token || r.calls != 1) {
				t.Fatal("uncertain outcome lost or retried", receipt, err, r.calls)
			}
		})
	}
}
