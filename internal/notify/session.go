package notify

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"perch/internal/app"
)

const maxStateBytes = 4 << 20

type Scope struct{ Host, User, Cluster string }
type diskState struct {
	Version    int
	Scope      Scope
	Checkpoint app.Checkpoint
}

// Session holds an advisory lock for the lifetime of a notifying workspace.
// Only one local Perch instance delivers for a given destination and identity.
type Session struct {
	mu     sync.Mutex
	lock   *os.File
	path   string
	scope  Scope
	sender Sender
	at     time.Time
	closed bool
}

func Open(dir string, scope Scope, sender Sender) (*Session, app.Checkpoint, error) {
	var checkpoint app.Checkpoint
	if scope.Host == "" || scope.User == "" || scope.Cluster == "" {
		return nil, checkpoint, fmt.Errorf("notification tracking needs a verified host, user and cluster")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, checkpoint, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, checkpoint, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, checkpoint, fmt.Errorf("notification state directory must be private (0700)")
	}
	raw, _ := json.Marshal(scope)
	name := fmt.Sprintf("%x", sha256.Sum256(raw))
	lockFD, err := unix.Open(filepath.Join(dir, name+".lock"), unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, checkpoint, err
	}
	lock := os.NewFile(uintptr(lockFD), "notification lock")
	if err = unix.Flock(lockFD, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		lock.Close()
		return nil, checkpoint, fmt.Errorf("another Perch workspace is tracking notifications for this connection")
	}
	s := &Session{lock: lock, path: filepath.Join(dir, name+".json"), scope: scope, sender: sender}
	fd, err := unix.Open(s.path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return s, checkpoint, nil
	}
	if err != nil {
		s.Close()
		return nil, checkpoint, err
	}
	f := os.NewFile(uintptr(fd), s.path)
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		f.Close()
		s.Close()
		return nil, checkpoint, fmt.Errorf("notification state must be a private regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxStateBytes+1))
	f.Close()
	var saved diskState
	if err == nil && len(data) > maxStateBytes {
		err = fmt.Errorf("notification state exceeds size limit")
	}
	if err == nil {
		err = json.Unmarshal(data, &saved)
	}
	if err == nil && (saved.Version != 1 || saved.Scope != scope) {
		err = fmt.Errorf("notification state has an incompatible identity or version")
	}
	if err != nil {
		s.Close()
		return nil, checkpoint, fmt.Errorf("cannot load notification state: %w", err)
	}
	// Invalid/expired checkpoints are safely treated as a new baseline by Store.
	return s, saved.Checkpoint, nil
}

// Process persists deduplication BEFORE attempting delivery. Delivery is at most
// once: a crash or delivery failure can lose a banner, but never triggers an
// automatic retry flood. History remains the authoritative place to inspect jobs.
func (s *Session) Process(ctx context.Context, c app.Checkpoint, events []app.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.at.IsZero() && !c.At.After(s.at) {
		return nil
	}
	raw, err := json.Marshal(diskState{1, s.scope, c})
	if err != nil {
		return err
	}
	if len(raw) > maxStateBytes {
		return fmt.Errorf("notification state exceeds size limit; alerts suppressed")
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".notifications-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), s.path)
	}
	if err == nil {
		// Persist the directory entry too, before attempting external delivery.
		var directory *os.File
		directory, err = os.Open(filepath.Dir(s.path))
		if err == nil {
			err = directory.Sync()
			directory.Close()
		}
	}
	if err != nil {
		return fmt.Errorf("cannot save notification state; alerts suppressed: %w", err)
	}
	s.at = c.At
	if s.sender == nil {
		return nil
	}
	var failures []error
	for _, m := range Messages(events) {
		if err := s.sender.Send(ctx, m); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.lock.Close()
	}
}
