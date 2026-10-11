// Package diagnostics provides explicit, bounded local troubleshooting capture.
package diagnostics

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const FileLimit = 2 << 20
const RawLimit = 256 << 10

// Raw byte slices encode as base64, preserving invalid UTF-8 and control bytes
// without emitting executable terminal sequences into the JSONL file.
type Event struct {
	Parser       string    `json:"requested_parser,omitempty"`
	Schema       int       `json:"schema_version"`
	Session      string    `json:"session"`
	Version      string    `json:"version"`
	At           time.Time `json:"at"`
	Operation    string    `json:"operation"`
	DurationMS   float64   `json:"duration_ms"`
	ExitCode     int       `json:"exit_code"`
	ErrorKind    string    `json:"error_kind,omitempty"`
	StdoutBytes  int       `json:"stdout_bytes"`
	StderrBytes  int       `json:"stderr_bytes"`
	RawCaptured  bool      `json:"raw_captured"`
	RawTruncated bool      `json:"raw_truncated,omitempty"`
	Stdout       []byte    `json:"stdout_base64,omitempty"`
	Stderr       []byte    `json:"stderr_base64,omitempty"`
}

type Store struct {
	mu                     sync.Mutex
	dir, lock              int
	path, session, version string
	raw                    bool
	err                    error
	closed                 bool
}

func private(fd int, kind uint32) bool {
	var st unix.Stat_t
	return unix.Fstat(fd, &st) == nil && st.Uid == uint32(os.Geteuid()) && st.Mode&0077 == 0 && uint32(st.Mode)&unix.S_IFMT == kind && (kind != unix.S_IFREG || st.Nlink == 1)
}
func New(dir, version string, raw bool) (*Store, error) {
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("diagnostic directory must be absolute")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	fail := func(e error) (*Store, error) { unix.Close(fd); return nil, e }
	if !private(fd, unix.S_IFDIR) {
		return fail(fmt.Errorf("diagnostic directory must be owned by you and private (0700)"))
	}
	lock, err := unix.Openat(fd, "capture.lock", unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if err != nil {
		return fail(err)
	}
	if !private(lock, unix.S_IFREG) {
		unix.Close(lock)
		return fail(fmt.Errorf("diagnostic lock must be an owned private regular file"))
	}
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		unix.Close(lock)
		return fail(err)
	}
	s := &Store{dir: fd, lock: lock, path: dir, session: hex.EncodeToString(id[:]), version: version, raw: raw}
	// Fail before any workload or network action if the sink cannot be written.
	s.record(Event{Operation: "session-start"})
	if err = s.Err(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Path() string { return s.path }
func (s *Store) Err() error   { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
func (s *Store) Status() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	mode := "metadata only"
	if s.raw {
		mode = "metadata + raw scheduler JSON"
	}
	if s.err != nil {
		return "Diagnostics stopped: " + s.err.Error() + " · " + s.path
	}
	return "Diagnostics: " + mode + " · " + s.path
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.err
	}
	s.closed = true
	unix.Close(s.lock)
	unix.Close(s.dir)
	return s.err
}
func (s *Store) record(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.err != nil {
		return
	}
	e.Schema, e.Session, e.Version, e.At = 1, s.session, s.version, time.Now().UTC()
	data, err := json.Marshal(e)
	if err == nil {
		err = s.append(append(data, '\n'))
	}
	if err != nil {
		s.err = err
	}
}
func (s *Store) openFile(name string, create bool) (*os.File, error) {
	flags := unix.O_WRONLY | unix.O_APPEND | unix.O_NONBLOCK | unix.O_NOFOLLOW | unix.O_CLOEXEC
	if create {
		flags |= unix.O_CREAT
	}
	fd, err := unix.Openat(s.dir, name, flags, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	if !private(fd, unix.S_IFREG) {
		f.Close()
		return nil, fmt.Errorf("diagnostic files must be owned private regular files with one link")
	}
	return f, nil
}
func (s *Store) append(data []byte) error {
	if len(data) > FileLimit {
		return fmt.Errorf("diagnostic record exceeds limit")
	}
	deadline := time.Now().Add(100 * time.Millisecond)
	for {
		err := unix.Flock(s.lock, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("diagnostic writer busy; capture stopped")
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer unix.Flock(s.lock, unix.LOCK_UN)
	// Validate both reserved files before any rotation/removal.
	old, err := s.openFile("events.previous.jsonl", false)
	if err == nil {
		st, e := old.Stat()
		old.Close()
		if e != nil {
			return e
		}
		if st.Size() > FileLimit {
			return fmt.Errorf("previous diagnostic file exceeds limit")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := s.openFile("events.jsonl", true)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	if st.Size() > FileLimit {
		f.Close()
		return fmt.Errorf("diagnostic file exceeds limit")
	}
	if st.Size()+int64(len(data)) > FileLimit {
		f.Close()
		if err = unix.Renameat(s.dir, "events.jsonl", s.dir, "events.previous.jsonl"); err != nil {
			return err
		}
		f, err = s.openFile("events.jsonl", true)
		if err != nil {
			return err
		}
	}
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}
