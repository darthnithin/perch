package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"perch/internal/preferences"
)

func isolatedPreferences(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
}

func TestConfigCLIAndFlagPrecedence(t *testing.T) {
	isolatedPreferences(t)
	var out bytes.Buffer
	for _, args := range [][]string{{"set", "host", "researcher@hb.ucsc.edu"}, {"set", "theme", "graphite"}, {"set", "color-mode", "truecolor"}, {"set", "mouse", "false"}, {"set", "reduced-motion", "true"}, {"set", "time-warnings", "15m,5m,1m"}} {
		if err := runConfig(args, &out); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{nil, {"--color-mode=auto", "--theme=warm", "--reduced-motion=false", "--no-mouse=false", "--time-warnings=off"}, {"--no-config"}} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		warnings := fs.String("time-warnings", "5m", "")
		host := fs.String("host", "hb.ucsc.edu", "")
		colorMode := fs.String("color-mode", "auto", "")
		theme := fs.String("theme", "warm", "")
		motion := fs.Bool("reduced-motion", false, "")
		mouse := fs.Bool("no-mouse", false, "")
		fs.Bool("no-config", false, "")
		fs.Parse(args)
		if err := applyPreferences(fs); err != nil {
			t.Fatal(err)
		}
		if len(args) == 0 && *warnings != "15m,5m,1m" {
			t.Fatal("saved warnings ignored")
		}
		if len(args) > 1 && *warnings != "off" {
			t.Fatal("explicit warnings ignored")
		}
		if len(args) == 1 && *warnings != "5m" {
			t.Fatal("no-config ignored")
		}
		if len(args) == 0 && (*host != "researcher@hb.ucsc.edu" || *theme != "graphite" || *colorMode != "truecolor" || !*motion || !*mouse) {
			t.Fatal("saved defaults not used")
		}
		if len(args) > 0 && (*theme != "warm" || *colorMode != "auto" || *motion || *mouse) {
			t.Fatal("explicit false or bypass ignored")
		}
		if len(args) == 1 && *host != "hb.ucsc.edu" {
			t.Fatal("bypass retained saved destination")
		}
	}
	out.Reset()
	if err := runWorkload([]string{"--workdir", "/tmp", "--json", "--", "true"}, &out); err != nil {
		t.Fatal(err)
	}
	var result runResult
	json.Unmarshal(out.Bytes(), &result)
	if result.Mode != "preview" || result.Host != "researcher@hb.ucsc.edu" {
		t.Fatal("launch preview ignored host or submitted", out.String())
	}
	out.Reset()
	if err := runConfig([]string{"show", "--json"}, &out); err != nil || !strings.Contains(out.String(), `"mouse": false`) {
		t.Fatal(out.String(), err)
	}
	if err := runConfig([]string{"reset"}, &out); err != nil {
		t.Fatal(err)
	}
	s, _ := preferences.DefaultStore()
	if v, err := s.Load(); err != nil || len(v.Flags()) != 0 {
		t.Fatal(v, err)
	}
}

func TestBadConfigHasOfflineRecoveryAndExplicitBypass(t *testing.T) {
	isolatedPreferences(t)
	s, _ := preferences.DefaultStore()
	os.MkdirAll(s.Dir, 0700)
	os.WriteFile(s.Path(), []byte("{bad"), 0600)
	var out bytes.Buffer
	if err := runWorkload([]string{"--workdir", "/tmp", "--", "true"}, &out); err == nil || !strings.Contains(err.Error(), "--no-config") {
		t.Fatal(err)
	}
	if err := runWorkload([]string{"--no-config", "--workdir", "/tmp", "--", "true"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := runConfig([]string{"path"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"version"}); err != nil {
		t.Fatal("broken config blocked version", err)
	}
	if err := runConfig([]string{"reset"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"set", "notify", "desktop"}, {"set", "host"}, {"unset"}, {"reset", "extra"}, {"path", "--json"}, {"get"}} {
		if err := runConfig(args, &out); err == nil {
			t.Fatal("accepted invalid command", args)
		}
	}
}
