package transport

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Opt in to harmless short shell queries only. No scheduler job, configuration,
// user file or credential mutation; SSH manages its own private control socket.
func TestHummingbirdTransportIntegration(t *testing.T) {
	host := os.Getenv("PERCH_TRANSPORT_LIVE_HOST")
	if host == "" {
		t.Skip("set PERCH_TRANSPORT_LIVE_HOST for live transport checks")
	}
	s, err := NewSSH(host)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	literal := "quotes ' and spaces\n$(printf injected) `printf substituted` 雪"
	r, err := s.Run(ctx, []string{"printf", "%s", literal})
	if err != nil || r.Stdout != literal {
		t.Fatal("remote argv round trip", err)
	}
	if info, err := os.Stat(s.socket); err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatal("private control socket unavailable", err)
	}
	if info, err := os.Stat(s.directory); err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("control directory permissions", err)
	}
	check, err := exec.CommandContext(ctx, "ssh", "-S", s.socket, "-O", "check", "--", host).CombinedOutput()
	match := regexp.MustCompile(`Master running \(pid=([0-9]+)\)`).FindSubmatch(check)
	if err != nil || len(match) != 2 {
		t.Fatal("cannot identify this private SSH master", err)
	}
	masterPID, err := strconv.Atoi(string(match[1]))
	if err != nil || masterPID <= 0 {
		t.Fatal("invalid master PID")
	}
	r, err = s.RunInput(ctx, []string{"cat"}, literal)
	if err != nil || r.Stdout != literal {
		t.Fatal("remote stdin round trip", err)
	}
	results, err := s.Batch(ctx, [][]string{
		{"sh", "-c", `p=$(readlink /proc/self/fd/2); printf '%s\n' "$p"; stat -c '%a' -- "${p%/*}"`},
		{"sh", "-c", "printf partial >&2; exit 7"},
		{"printf", "after failure\n"},
	})
	if err != nil || len(results) != 3 {
		t.Fatal("live batch", err)
	}
	spool := strings.Split(strings.TrimSpace(results[0].Stdout), "\n")
	if len(spool) != 2 || spool[1] != "700" || !filepath.IsAbs(spool[0]) || filepath.Base(spool[0]) != "stderr" {
		t.Fatal("remote spool permissions or framing")
	}
	if results[1].Stderr != "partial" || results[1].ExitCode != 7 || results[2].Stdout != "after failure\n" {
		t.Fatal("partial failure lost sibling result")
	}
	if _, err = s.Run(ctx, []string{"test", "!", "-e", filepath.Dir(spool[0])}); err != nil {
		t.Fatal("remote spool not removed", err)
	}
	if _, err = s.Batch(ctx, [][]string{{"sh", "-c", "head -c 4194304 /dev/zero >&2"}}); !IsKind(err, OutputLimit) {
		t.Fatal("remote oversized stderr accepted", err)
	}
	short, stop := context.WithTimeout(ctx, time.Second)
	r, err = s.Run(short, []string{"sh", "-c", "printf started; exec sleep 3"})
	stop()
	if !IsKind(err, Timeout) || r.Stdout != "started" {
		t.Fatal("remote cancellation was not exercised after command start", err)
	}
	r, err = s.Run(ctx, []string{"printf", "recovered"})
	if err != nil || r.Stdout != "recovered" {
		t.Fatal("query after cancellation", err)
	}
	// Observe a batch's own spool before cancelling its client. An SSH client
	// deadline alone does not establish remote-shell cleanup without a PTY.
	script, err := batchScript([][]string{{"sh", "-c", "readlink /proc/self/fd/2; exec sleep 3"}}, "cancel_spool")
	if err != nil {
		t.Fatal(err)
	}
	short, stop = context.WithTimeout(ctx, time.Second)
	r, err = s.Run(short, []string{"sh", "-c", script})
	stop()
	parts := strings.Split(r.Stdout, "\n")
	if !IsKind(err, Timeout) || len(parts) < 2 || parts[0] != "cancel_spool:0:OUT" || !regexp.MustCompile(`^/tmp/tmp\.[A-Za-z0-9]+/stderr$`).MatchString(parts[1]) {
		t.Fatal("cancelled batch was not observed after spool creation", err)
	}
	cancelledDir := filepath.Dir(parts[1])
	spoolCleaned := false
	defer func() {
		if spoolCleaned {
			return
		}
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		// Exact private directory observed from this synthetic batch only.
		_, _ = s.Run(cleanup, []string{"rm", "-rf", "--", cancelledDir})
	}()
	cleanupStarted := time.Now()
	cleanupDeadline := cleanupStarted.Add(5 * time.Second)
	for {
		_, err = s.Run(ctx, []string{"test", "!", "-e", cancelledDir})
		if err == nil {
			spoolCleaned = true
			break
		}
		if time.Now().After(cleanupDeadline) {
			t.Fatal("cancelled batch leaked its remote spool")
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Logf("Remote batch spool disappeared %.2fs after client cancellation", time.Since(cleanupStarted).Seconds())
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(s.directory); !os.IsNotExist(err) {
		t.Fatal("private socket directory leaked", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for syscall.Kill(masterPID, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if err := syscall.Kill(masterPID, 0); err != syscall.ESRCH {
		t.Fatal("private SSH master remains after Close", err)
	}
	t.Log("Live argv/stdin, private multiplexing, framed partial errors, capped remote stderr, spool cleanup, cancellation, recovery and socket cleanup passed")
}
