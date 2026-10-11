package preferences

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

func TestExplicitPreferencesPersistAndUnsetWithoutOtherChanges(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "preferences")}
	v, err := s.Load()
	if err != nil || len(v.Flags()) != 0 {
		t.Fatal(v, err)
	}
	if _, err := os.Stat(s.Dir); !os.IsNotExist(err) {
		t.Fatal("reading defaults created files")
	}
	for key, value := range map[string]string{"host": "researcher@hb.ucsc.edu", "theme": "graphite", "mouse": "false", "reduced-motion": "true"} {
		if err := s.Edit(context.Background(), key, &value, false); err != nil {
			t.Fatal(err)
		}
	}
	v, err = s.Load()
	if err != nil || *v.Host != "researcher@hb.ucsc.edu" || *v.Theme != "graphite" || *v.Mouse || !*v.ReducedMotion {
		t.Fatal(v, err)
	}
	if err := s.Edit(context.Background(), "theme", nil, false); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Load()
	if v.Theme != nil || v.Host == nil || v.Mouse == nil {
		t.Fatal("unset lost unrelated preferences", v)
	}
	for _, path := range []string{s.Path(), filepath.Join(s.Dir, "settings.lock")} {
		st, err := os.Stat(path)
		if err != nil || st.Mode().Perm() != 0600 {
			t.Fatal("non-private file", path, err)
		}
	}
	if err := s.Edit(context.Background(), "", nil, true); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Load()
	if len(v.Flags()) != 0 {
		t.Fatal("reset did not restore defaults")
	}
}

func TestMalformedPreferencesRequireExplicitReset(t *testing.T) {
	for _, raw := range []string{"", "{", `{"schema_version":2}`, `{"schema_version":1,"notify":"desktop"}`, `{"schema_version":1,"host":"-bad"}`, `{"schema_version":1,"theme":"pink"}`, `{"schema_version":1,"color_mode":"bogus"}`, `{"schema_version":1,"time_warnings":"0m"}`, `{"schema_version":1,"time_warnings":[5]}`, `{"schema_version":1,"mouse":"false"}`, `{"schema_version":1,"host":"one","host":"two"}`, `{"schema_version":1} {}`, strings.Repeat(" ", maxBytes+1)} {
		s := Store{Dir: t.TempDir()}
		os.Chmod(s.Dir, 0700)
		os.WriteFile(s.Path(), []byte(raw), 0600)
		if _, err := s.Load(); err == nil {
			t.Fatalf("accepted %q", raw[:min(70, len(raw))])
		}
		value := "warm"
		if err := s.Edit(context.Background(), "theme", &value, false); err == nil {
			t.Fatal("silently replaced bad preferences")
		}
		if after, _ := os.ReadFile(s.Path()); string(after) != raw {
			t.Fatal("failed edit changed data")
		}
		if err := s.Edit(context.Background(), "", nil, true); err != nil {
			t.Fatal("explicit reset failed", err)
		}
		if v, err := s.Load(); err != nil || len(v.Flags()) != 0 {
			t.Fatal(v, err)
		}
	}
}

func TestUnsafeFilesAndInvalidValuesAreRejected(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo", "public", "directory"} {
		t.Run(kind, func(t *testing.T) {
			s := Store{Dir: t.TempDir()}
			os.Chmod(s.Dir, 0700)
			switch kind {
			case "symlink":
				os.Symlink(filepath.Join(t.TempDir(), "target"), s.Path())
			case "fifo":
				unix.Mkfifo(s.Path(), 0600)
			case "public":
				os.WriteFile(s.Path(), []byte(`{"schema_version":1}`), 0644)
			case "directory":
				os.Mkdir(s.Path(), 0700)
			}
			if _, err := s.Load(); err == nil {
				t.Fatal("unsafe read")
			}
			if err := s.Edit(context.Background(), "", nil, true); err == nil {
				t.Fatal("reset bypassed file safety")
			}
		})
	}
	s := Store{Dir: filepath.Join(t.TempDir(), "missing")}
	for key, value := range map[string]string{"notify": "desktop", "local": "true", "submit": "true", "host": "user@host\nother", "mouse": "maybe", "theme": "pink", "color-mode": "bogus"} {
		if err := s.Edit(context.Background(), key, &value, false); err == nil {
			t.Fatal("accepted invalid or action preference", key)
		}
	}
	if _, err := os.Stat(s.Dir); !os.IsNotExist(err) {
		t.Fatal("invalid setting created state")
	}
}

func TestConcurrentEditsAndStagingRecovery(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "preferences")}
	var wg sync.WaitGroup
	for key, value := range map[string]string{"host": "researcher@hb.ucsc.edu", "theme": "graphite", "mouse": "false", "reduced-motion": "true"} {
		wg.Go(func() {
			if err := s.Edit(context.Background(), key, &value, false); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if v, err := s.Load(); err != nil || len(v.Flags()) != 4 {
		t.Fatal("concurrent edit lost settings", v, err)
	}
	os.WriteFile(filepath.Join(s.Dir, ".settings.next"), []byte("interrupted"), 0600)
	if err := s.Edit(context.Background(), "theme", nil, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, ".settings.next")); !os.IsNotExist(err) {
		t.Fatal("staging file retained")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Edit(ctx, "host", nil, false); err == nil {
		t.Fatal("cancelled edit succeeded")
	}
	v, _ := s.Load()
	if v.Host == nil {
		t.Fatal("cancelled edit removed host")
	}
}
