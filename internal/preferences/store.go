package preferences

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

const maxBytes = 16 << 10

type Store struct{ Dir string }

func private(fd int, kind uint32) bool {
	var st unix.Stat_t
	return unix.Fstat(fd, &st) == nil && st.Uid == uint32(os.Geteuid()) && st.Mode&0077 == 0 && uint32(st.Mode)&unix.S_IFMT == kind
}

func (s Store) open(create bool) (int, error) {
	if !filepath.IsAbs(s.Dir) {
		return -1, fmt.Errorf("preferences directory must be absolute")
	}
	if create {
		if err := os.MkdirAll(s.Dir, 0700); err != nil {
			return -1, err
		}
	}
	fd, err := unix.Open(s.Dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err == nil && !private(fd, unix.S_IFDIR) {
		unix.Close(fd)
		return -1, fmt.Errorf("preferences directory must be owned by you and private (0700)")
	}
	return fd, err
}

func read(dir int, reset bool) ([]byte, error) {
	fd, err := unix.Openat(dir, "settings.json", unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), "settings.json")
	defer f.Close()
	if !private(fd, unix.S_IFREG) {
		return nil, fmt.Errorf("preferences must be an owned private regular file (0600)")
	}
	if reset {
		return nil, nil
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if len(raw) > maxBytes {
		return nil, fmt.Errorf("preferences exceed 16 KiB")
	}
	if len(raw) == 0 && err == nil {
		return nil, fmt.Errorf("preferences file is empty; use config reset to restore defaults")
	}
	return raw, err
}

// Load is read-only and creates neither a directory nor a lock file.
func (s Store) Load() (Values, error) {
	fd, err := s.open(false)
	if errors.Is(err, os.ErrNotExist) {
		return Values{Version: 1}, nil
	}
	if err != nil {
		return Values{}, err
	}
	defer unix.Close(fd)
	raw, err := read(fd, false)
	if err != nil {
		return Values{}, err
	}
	return decode(raw)
}

// Edit serializes local changes and atomically replaces the private file.
// Reset is explicit and can replace malformed JSON, but not unsafe file types.
func (s Store) Edit(ctx context.Context, key string, value *string, reset bool) error {
	probe := Values{Version: 1}
	if !reset {
		var err error
		if value == nil {
			err = probe.Unset(key)
		} else {
			err = probe.Set(key, *value)
		}
		if err != nil {
			return err
		}
	}
	return s.update(ctx, reset, func(v *Values) error {
		if value == nil {
			return v.Unset(key)
		}
		return v.Set(key, *value)
	})
}

// SaveAppearance changes all display preferences in one transaction while
// preserving the latest destination, including edits made by another process.
func (s Store) SaveAppearance(ctx context.Context, theme, colorMode string, reducedMotion, mouse bool) error {
	probe := Values{Version: 1}
	if err := probe.Set("theme", theme); err != nil {
		return err
	}
	if err := probe.Set("color-mode", colorMode); err != nil {
		return err
	}
	return s.update(ctx, false, func(v *Values) error {
		v.ColorMode = &colorMode
		v.Theme, v.ReducedMotion, v.Mouse = &theme, &reducedMotion, &mouse
		return nil
	})
}

func (s Store) update(ctx context.Context, reset bool, change func(*Values) error) error {
	dir, err := s.open(true)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	flags := unix.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	lock, err := unix.Openat(dir, "settings.lock", flags|unix.O_CREAT|unix.O_EXCL, 0600)
	if errors.Is(err, unix.EEXIST) {
		lock, err = unix.Openat(dir, "settings.lock", flags, 0)
	}
	if err != nil {
		return err
	}
	defer unix.Close(lock)
	if !private(lock, unix.S_IFREG) {
		return fmt.Errorf("preferences lock must be an owned private regular file")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("preferences busy or cancelled: %w", err)
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
		case <-time.After(20 * time.Millisecond):
		}
	}
	raw, err := read(dir, reset)
	// A reset may recover corrupt/oversized/empty data only after checking the
	// destination is a private regular file; read errors must not bypass that.
	if err != nil {
		return err
	}
	v := Values{Version: 1}
	if !reset {
		v, err = decode(raw)
		if err != nil {
			return err
		}
		err = change(&v)
		if err != nil {
			return err
		}
	}
	raw, err = json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	const temp = ".settings.next"
	// A fixed staging name bounds residue even after process interruption.
	stale, err := unix.Openat(dir, temp, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err == nil {
		ok := private(stale, unix.S_IFREG)
		unix.Close(stale)
		if !ok {
			return fmt.Errorf("unsafe preferences staging file")
		}
		if err = unix.Unlinkat(dir, temp, 0); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fd, err := unix.Openat(dir, temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	defer unix.Unlinkat(dir, temp, 0)
	f := os.NewFile(uintptr(fd), temp)
	_, err = f.Write(raw)
	if err == nil {
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
		err = unix.Renameat(dir, temp, dir, "settings.json")
	}
	if err == nil {
		err = unix.Fsync(dir)
	}
	return err
}
