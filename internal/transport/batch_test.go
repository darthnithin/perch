package transport

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestBatchStderrSpoolBoundAndCleanup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	commands := [][]string{{"sh", "-c", "head -c 4194304 /dev/zero >&2; printf drained"}, {"printf", "continued"}}
	script, err := batchScript(commands, "spool_test")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "sh", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "drained") || !strings.Contains(string(raw), "continued") {
		t.Fatal("capping stderr broke the command or next query")
	}
	if len(raw) > maxStderr+1024 {
		t.Fatal("stderr spool was not capped", len(raw))
	}
	if _, err := parseBatch(string(raw), "spool_test", 2); !IsKind(err, OutputLimit) {
		t.Fatal("truncated stderr became a successful query", err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatal("spool directory leaked", files, err)
	}
}

func TestBatchStderrBoundaryAndEmptyCommand(t *testing.T) {
	for _, size := range []string{"0", "1048576"} {
		command := "true"
		if size != "0" {
			command = "head -c " + size + " /dev/zero >&2"
		}
		script, err := batchScript([][]string{{"sh", "-c", command}, {"true"}}, "boundary")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		raw, err := exec.CommandContext(ctx, "sh", "-c", script).Output()
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		results, err := parseBatch(string(raw), "boundary", 2)
		if err != nil || len(results) != 2 || results[0].ExitCode != 0 || results[1].Stderr != "" {
			t.Fatal("valid boundary failed", size, err)
		}
		if size != "0" && len(results[0].Stderr) != maxStderr {
			t.Fatal("lost bytes")
		}
	}
}

func TestSSHFailureKindsDoNotExposeRawErrors(t *testing.T) {
	for _, tc := range []struct {
		message string
		kind    Kind
	}{
		{"Host key verification failed. secret", HostKey},
		{"REMOTE HOST IDENTIFICATION HAS CHANGED! secret", HostKey},
		{"Permission denied (publickey). secret", Authentication},
		{"Authentication failed. secret", Authentication},
		{"Connection refused. secret", Network},
	} {
		cmd := exec.Command("sh", "-c", `printf '%s' "$1" >&2; exit 255`, "test", tc.message)
		_, err := execute(context.Background(), cmd, true)
		if !IsKind(err, tc.kind) || strings.Contains(err.Error(), "secret") {
			t.Fatal(tc.kind, err)
		}
	}
	cmd := exec.Command("sh", "-c", `printf 'permission denied' >&2; exit 7`)
	if _, err := execute(context.Background(), cmd, true); !IsKind(err, CommandFailed) {
		t.Fatal("remote permission error mistaken for SSH login failure", err)
	}
}

func TestBatchSignalStopsFollowingQueriesAndCleansSpool(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	script, err := batchScript([][]string{{"sh", "-c", `kill -TERM "$PPID"`}, {"printf", "must-not-run"}}, "signal")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, "sh", "-c", script).Output()
	if err == nil || strings.Contains(string(raw), "must-not-run") || ctx.Err() != nil {
		t.Fatal("signal did not stop the batch", err, string(raw))
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatal("signal leaked spool directory", files, err)
	}
}
