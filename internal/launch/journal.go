package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const journalLimit = 1000
const journalBytes = 1 << 20

// Submission records acceptance, not execution or completion. Destination is
// the SSH argument used for this request, not a verified permanent identity.
// Deliberately exclude scripts, arguments, paths, output and transport errors.
type Submission struct {
	Destination string    `json:"destination"`
	Name        string    `json:"name"`
	Started     time.Time `json:"started_at"`
	Receipt     Receipt   `json:"receipt"`
}

func (s Submission) Status() string {
	if s.Receipt.JobID != "" {
		return "accepted"
	}
	return "unconfirmed"
}

type Journal struct{ Dir string }
type journalData struct {
	Version int          `json:"schema_version"`
	Entries []Submission `json:"entries"`
}

func DefaultJournal() (*Journal, error) {
	var root string
	if runtime.GOOS == "linux" {
		root = os.Getenv("XDG_STATE_HOME")
		if !filepath.IsAbs(root) {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			root = filepath.Join(home, ".local", "state")
		}
	} else {
		var err error
		root, err = os.UserConfigDir()
		if err != nil {
			return nil, err
		}
	}
	return &Journal{Dir: filepath.Join(root, "perch", "submissions")}, nil
}

var tokenPattern = regexp.MustCompile(`^perch-[a-f0-9]{24}$`)

func validSubmission(s Submission) bool {
	r := s.Receipt
	if !tokenPattern.MatchString(r.Token) || !namePattern.MatchString(s.Name) || s.Started.IsZero() || r.At.IsZero() || s.Destination == "" || len(s.Destination) > 512 || !cleanText(s.Destination) || strings.ContainsAny(s.Destination, " \t") || strings.HasPrefix(s.Destination, "-") {
		return false
	}
	if r.JobID == "" {
		return r.Cluster == ""
	}
	return len(r.JobID) <= 20 && len(r.Cluster) <= 128 && jobResponse.MatchString(r.JobID) && !strings.Contains(r.JobID, ";") && (r.Cluster == "" || regexp.MustCompile(`^[A-Za-z0-9_.-]+$`).MatchString(r.Cluster))
}

// List is offline and does not create a directory merely to read an empty log.
func (j *Journal) List(ctx context.Context, destination string) ([]Submission, error) {
	entries := []Submission{}
	err := j.transaction(ctx, false, func(saved *journalData) error {
		for _, entry := range saved.Entries {
			if destination == "" || entry.Destination == destination {
				entries = append(entries, entry)
			}
		}
		return nil
	})
	sort.Slice(entries, func(a, b int) bool { return entries[a].Started.After(entries[b].Started) })
	return entries, err
}

func (j *Journal) Save(ctx context.Context, s Submission) error {
	if !validSubmission(s) {
		return fmt.Errorf("invalid submission receipt")
	}
	return j.transaction(ctx, true, func(saved *journalData) error {
		for i, prior := range saved.Entries {
			if prior.Receipt.Token == s.Receipt.Token {
				if prior.Destination != s.Destination || prior.Name != s.Name || !prior.Started.Equal(s.Started) || (prior.Receipt.JobID != "" && (prior.Receipt.JobID != s.Receipt.JobID || prior.Receipt.Cluster != s.Receipt.Cluster || !prior.Receipt.At.Equal(s.Receipt.At))) {
					return fmt.Errorf("receipt identity or accepted outcome cannot change")
				}
				saved.Entries[i] = s
				return nil
			}
		}
		if len(saved.Entries) >= journalLimit {
			oldest := -1
			for i, entry := range saved.Entries {
				if entry.Receipt.JobID != "" && (oldest < 0 || entry.Started.Before(saved.Entries[oldest].Started)) {
					oldest = i
				}
			}
			if oldest < 0 {
				return fmt.Errorf("submission journal is full of unconfirmed requests; inspect %s before submitting more", j.Dir)
			}
			saved.Entries = append(saved.Entries[:oldest], saved.Entries[oldest+1:]...)
		}
		saved.Entries = append(saved.Entries, s)
		return nil
	})
}

func privateFile(fd int, directory bool) bool {
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil || st.Uid != uint32(os.Geteuid()) || st.Mode&0077 != 0 {
		return false
	}
	kind := uint32(unix.S_IFREG)
	if directory {
		kind = unix.S_IFDIR
	}
	return uint32(st.Mode)&unix.S_IFMT == kind
}

// Hold a short process-wide advisory lock only around read/modify/write, never
// across SSH. Directory-relative operations prevent path replacement races.
func (j *Journal) transaction(ctx context.Context, write bool, change func(*journalData) error) error {
	if j == nil || !filepath.IsAbs(j.Dir) {
		return fmt.Errorf("submission journal needs an absolute directory")
	}
	if write {
		if err := os.MkdirAll(j.Dir, 0700); err != nil {
			return fmt.Errorf("create submission directory: %w", err)
		}
	}
	dir, err := unix.Open(j.Dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if !write && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open submission directory: %w", err)
	}
	defer unix.Close(dir)
	if !privateFile(dir, true) {
		return fmt.Errorf("submission directory must be owned by you and private (0700)")
	}
	lockFlags := unix.O_RDWR | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	lock, err := unix.Openat(dir, "journal.lock", lockFlags|unix.O_CREAT|unix.O_EXCL, 0600)
	if errors.Is(err, unix.EEXIST) {
		lock, err = unix.Openat(dir, "journal.lock", lockFlags, 0)
	}
	if err != nil {
		return fmt.Errorf("open submission lock: %w", err)
	}
	defer unix.Close(lock)
	if !privateFile(lock, false) {
		return fmt.Errorf("submission lock must be an owned private regular file")
	}
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for {
		if err = lockCtx.Err(); err != nil {
			return fmt.Errorf("submission journal busy or cancelled: %w", err)
		}
		err = unix.Flock(lock, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return err
		}
		select {
		case <-lockCtx.Done():
		case <-time.After(20 * time.Millisecond):
		}
	}
	saved := journalData{Version: 1, Entries: []Submission{}}
	fd, err := unix.Openat(dir, "journal.json", unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err == nil {
		f := os.NewFile(uintptr(fd), "journal.json")
		if !privateFile(fd, false) {
			f.Close()
			return fmt.Errorf("submission journal must be an owned private regular file")
		}
		raw, readErr := io.ReadAll(io.LimitReader(f, journalBytes+1))
		f.Close()
		if readErr != nil {
			return readErr
		}
		if len(raw) > journalBytes {
			return fmt.Errorf("submission journal exceeds size limit")
		}
		if err = json.Unmarshal(raw, &saved); err != nil {
			return fmt.Errorf("cannot read submission journal: %w", err)
		}
		if saved.Version != 1 || len(saved.Entries) > journalLimit {
			return fmt.Errorf("unsupported or oversized submission journal")
		}
		seen := map[string]bool{}
		for _, entry := range saved.Entries {
			if !validSubmission(entry) || seen[entry.Receipt.Token] {
				return fmt.Errorf("invalid or duplicate saved submission")
			}
			seen[entry.Receipt.Token] = true
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = change(&saved); err != nil || !write {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	raw, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if len(raw) > journalBytes {
		return fmt.Errorf("submission journal exceeds size limit")
	}
	// One reserved temporary name under the exclusive lock keeps even crash
	// residue bounded. Only our private regular staging file may be replaced.
	tmp := ".journal.next"
	stale, staleErr := unix.Openat(dir, tmp, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if staleErr == nil {
		owned := privateFile(stale, false)
		unix.Close(stale)
		if !owned {
			return fmt.Errorf("submission staging file must be an owned private regular file")
		}
		if err = unix.Unlinkat(dir, tmp, 0); err != nil {
			return err
		}
	} else if !errors.Is(staleErr, os.ErrNotExist) {
		return staleErr
	}
	fd, err = unix.Openat(dir, tmp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return fmt.Errorf("create submission temporary file: %w", err)
	}
	defer unix.Unlinkat(dir, tmp, 0)
	f := os.NewFile(uintptr(fd), tmp)
	_, err = f.Write(raw)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = unix.Renameat(dir, tmp, dir, "journal.json")
		if err != nil {
			return fmt.Errorf("replace submission journal: %w", err)
		}
	}
	if err == nil {
		err = unix.Fsync(dir)
		if err != nil {
			return fmt.Errorf("sync submission directory: %w", err)
		}
	}
	return err
}
