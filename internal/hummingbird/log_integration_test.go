package hummingbird

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/transport"
)

// Explicit opt-in: creates and removes only a private temporary directory.
// No scheduler mutations or installation; also works with Linux Local runner.
func TestLogLifecycleIntegration(t *testing.T) {
	host := os.Getenv("PERCH_LOG_TEST_HOST")
	if host == "" {
		t.Skip("set PERCH_LOG_TEST_HOST to a test SSH host or local on Linux")
	}
	var runner transport.Runner = transport.Local{}
	if host != "local" {
		ssh, err := transport.NewSSH(host)
		if err != nil {
			t.Fatal(err)
		}
		defer ssh.Close()
		runner = ssh
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	run := func(args ...string) string {
		t.Helper()
		r, err := runner.Run(ctx, args)
		if err != nil || r.ExitCode != 0 {
			t.Fatalf("test file operation: %v %s", err, r.Stderr)
		}
		return r.Stdout
	}
	dir := strings.TrimSpace(run("mktemp", "-d", "/tmp/perch-log-test-XXXXXXXX"))
	if !strings.HasPrefix(dir, "/tmp/perch-log-test-") || strings.ContainsAny(dir, "\n\r") {
		t.Fatal("unexpected temporary directory")
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if _, err := runner.Run(cleanup, []string{"rm", "-rf", "--", dir}); err != nil {
			t.Errorf("temporary file cleanup: %v", err)
		}
	}()
	// Spaces, quotes, shell syntax and literal Slurm tokens remain filename data.
	path := dir + "/output ' $(echo unsafe) %j.log"
	job := domain.Job{ID: "1", Stdout: path, StdoutExpanded: true}
	client := Client{Runner: runner}
	write := func(script, text string) { run("sh", "-c", script, "test-log", path, text) }
	write(`printf '%s' "$2" > "$1"`, "first\n")
	read := func(previous LogCursor) LogChunk {
		t.Helper()
		chunk, err := client.ReadLog(ctx, job, false, previous)
		if err != nil {
			t.Fatal(err)
		}
		return chunk
	}
	first := read(LogCursor{})
	if !first.Reset || first.Raw != "first\n" {
		t.Fatal("initial read", first)
	}
	write(`printf '%s' "$2" >> "$1"`, "second\n")
	second := read(first.Cursor)
	if second.Reset || second.Raw != "second\n" {
		t.Fatal("append", second)
	}
	write(`printf '%s' "$2" > "$1"`, "x\n")
	truncated := read(second.Cursor)
	if !truncated.Reset || truncated.Raw != "x\n" {
		t.Fatal("truncate", truncated)
	}
	write(`mv -- "$1" "$1.old"; printf '%s' "$2" > "$1"`, "rotated\n")
	rotated := read(truncated.Cursor)
	if !rotated.Reset || rotated.Raw != "rotated\n" || rotated.Cursor.Identity == truncated.Cursor.Identity {
		t.Fatal("rotation", rotated)
	}
	run("sh", "-c", `head -c 300000 /dev/zero | tr '\000' x > "$1"`, "test-log", path)
	large := read(LogCursor{})
	if len(large.Raw) != 256<<10 || large.Cursor.Offset != 300000 {
		t.Fatal("bounded tail", len(large.Raw), large.Cursor)
	}
	run("rm", "--", path)
	if _, err := client.ReadLog(ctx, job, false, large.Cursor); err == nil {
		t.Fatal("missing output was not an error")
	}
	run("mkfifo", "--", path)
	if _, err := client.ReadLog(ctx, job, false, LogCursor{}); err == nil {
		t.Fatal("non-regular output was accepted")
	}
	t.Log("initial/append/truncate/rotate/bounded/missing/FIFO and literal special-character paths passed")
}
