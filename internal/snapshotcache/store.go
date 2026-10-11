package snapshotcache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func private(fd int, kind uint32) bool {
	var st unix.Stat_t
	return unix.Fstat(fd, &st) == nil && st.Uid == uint32(os.Geteuid()) && st.Mode&0077 == 0 && uint32(st.Mode)&unix.S_IFMT == kind && (kind != unix.S_IFREG || st.Nlink == 1)
}
func (s Store) open(create bool) (int, error) {
	if !filepath.IsAbs(s.Dir) {
		return -1, fmt.Errorf("cache directory must be absolute")
	}
	if create {
		if err := os.MkdirAll(s.Dir, 0700); err != nil {
			return -1, err
		}
	}
	fd, err := unix.Open(s.Dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err == nil && !private(fd, unix.S_IFDIR) {
		unix.Close(fd)
		return -1, fmt.Errorf("cache directory must be owned and private (0700)")
	}
	return fd, err
}
func read(dir int) (*Entry, error) {
	fd, err := unix.Openat(dir, "view.json", unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "view.json")
	defer f.Close()
	if !private(fd, unix.S_IFREG) {
		return nil, fmt.Errorf("cache must be an owned private regular file with one link")
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxBytes {
		return nil, fmt.Errorf("cache exceeds 8 MiB")
	}
	var e Entry
	if err = json.Unmarshal(raw, &e); err != nil {
		return nil, fmt.Errorf("cache is unreadable")
	}
	return &e, nil
}

// Load creates no files. One most-recent destination is retained, bounded to
// one 8 MiB file. A different host is a cache miss; an expired snapshot returns no view and an
// explanatory error for Connection.
func (s Store) Load(host string, now time.Time) (*Entry, error) {
	fd, err := s.open(false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	e, err := read(fd)
	if err != nil || e == nil {
		return nil, err
	}
	if e.Scope.Host != host {
		return nil, nil
	}
	if err = e.valid(now); err != nil {
		return nil, err
	}
	sanitized, err := Capture(e.Scope, e.Snapshot, e.History)
	if err != nil {
		return nil, err
	}
	return &sanitized, nil
}

// Save uses a bounded cross-process lock and atomic owner-only replacement.
// Older replies cannot overwrite newer snapshots; failed writes keep the old one.
func (s Store) Save(parent context.Context, e Entry, now time.Time) error {
	raw, err := encode(e, now)
	if err != nil {
		return err
	}
	dir, err := s.open(true)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	lock, err := unix.Openat(dir, "view.lock", unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	defer unix.Close(lock)
	if !private(lock, unix.S_IFREG) {
		return fmt.Errorf("cache lock must be an owned private regular file")
	}
	ctx, cancel := context.WithTimeout(parent, 250*time.Millisecond)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err = unix.Flock(lock, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer unix.Flock(lock, unix.LOCK_UN)
	old, err := read(dir)
	// Do not overwrite unsafe or malformed files implicitly. The user can remove
	// their explicit cache file to recover; cache failure never blocks monitoring.
	if err != nil {
		return err
	}
	if old != nil && old.valid(now) == nil && (old.Snapshot.At.After(e.Snapshot.At) || old.Scope == e.Scope && (old.Snapshot.HistoryAt.After(e.Snapshot.HistoryAt) || old.Snapshot.PartitionsAt.After(e.Snapshot.PartitionsAt))) {
		return nil
	}
	// One reserved pending file bounds disk use even after an interrupted save.
	temp := ".view.pending"
	fd, err := unix.Openat(dir, temp, unix.O_WRONLY|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), temp)
	if !private(fd, unix.S_IFREG) {
		f.Close()
		return fmt.Errorf("cache pending file must be an owned private regular file")
	}
	defer unix.Unlinkat(dir, temp, 0)
	if err = f.Truncate(0); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = unix.Renameat(dir, temp, dir, "view.json")
	}
	if err == nil {
		err = unix.Fsync(dir)
	}
	return err
}
