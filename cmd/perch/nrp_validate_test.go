package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNRPValidateRejectsLocalErrorsBeforeCredentials(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	p := filepath.Join(dir, "job.json")
	if err := os.WriteFile(p, []byte(`{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"test","namespace":"research"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"validate", p, "--demo"},
		{"validate", p, "--context", "prod", "--namespace", "other"},
		{"validate", dir, "--context", "prod", "--namespace", "research"},
		{"validate", p, "--namespace", "research"},
	} {
		var out bytes.Buffer
		err := runNRP(args, &out)
		if err == nil || strings.Contains(err.Error(), "kubectl") {
			t.Fatal(args, err)
		}
	}
}
