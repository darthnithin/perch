// Package transport runs bounded operations locally or through the
// user's system OpenSSH. Remote argument quoting lives only in this package.
package transport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const MaxOutput = 32 << 20
const maxStderr = 1 << 20

type Result struct {
	Stdout, Stderr string
	ExitCode       int
	Duration       time.Duration
}

type Kind string

const (
	Authentication Kind = "authentication"
	HostKey        Kind = "host-key"
	Network        Kind = "network"
	Timeout        Kind = "timeout"
	CommandFailed  Kind = "command"
	OutputLimit    Kind = "output-limit"
	Protocol       Kind = "protocol"
)

type Error struct {
	Kind    Kind
	Message string
}

func (e *Error) Error() string         { return e.Message }
func IsKind(err error, kind Kind) bool { var e *Error; return errors.As(err, &e) && e.Kind == kind }

type Runner interface {
	Run(context.Context, []string) (Result, error)
	Batch(context.Context, [][]string) ([]Result, error)
}

// Limited buffers continue consuming output after the limit, so writers cannot
// deadlock. No partial response is ever returned as a successful query.
type limitedBuffer struct {
	// Keep ReadFrom unexposed so process-output copies always pass through Write.
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Len() int       { return b.buffer.Len() }
func (b *limitedBuffer) String() string { return b.buffer.String() }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := max(0, b.limit-b.Len())
	if n > left {
		b.overflow = true
	}
	_, _ = b.buffer.Write(p[:min(n, left)])
	return n, nil
}

func execute(ctx context.Context, cmd *exec.Cmd, ssh bool) (Result, error) {
	started := time.Now()
	out := &limitedBuffer{limit: MaxOutput}
	stderr := &limitedBuffer{limit: maxStderr}
	cmd.Stdout = out
	cmd.Stderr = stderr
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	r := Result{Stdout: out.String(), Stderr: stderr.String(), Duration: time.Since(started)}
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	} else if err != nil {
		r.ExitCode = -1
	}
	if ctx.Err() != nil {
		return r, &Error{Timeout, "Query cancelled or timed out"}
	}
	if out.overflow || stderr.overflow {
		return r, &Error{OutputLimit, "Query exceeded its output limit; narrow the query"}
	}
	if err == nil {
		return r, nil
	}
	if ssh && r.ExitCode == 255 {
		lower := strings.ToLower(r.Stderr)
		switch {
		case strings.Contains(lower, "host key verification failed"), strings.Contains(lower, "remote host identification has changed"):
			return r, &Error{HostKey, "SSH host-key verification failed; review the host in your terminal"}
		case strings.Contains(lower, "permission denied"), strings.Contains(lower, "authentication failed"):
			return r, &Error{Authentication, "SSH authentication is required; reconnect to sign in"}
		default:
			return r, &Error{Network, "SSH connection failed; check the host and network, then reconnect"}
		}
	}
	return r, &Error{CommandFailed, fmt.Sprintf("Command failed (exit %d)", r.ExitCode)}
}

type Local struct{}

func (Local) Run(ctx context.Context, args []string) (Result, error) {
	if err := validateArgs(args); err != nil {
		return Result{}, err
	}
	return execute(ctx, exec.CommandContext(ctx, args[0], args[1:]...), false)
}

// RunInput sends a bounded script over stdin rather than placing its contents
// in process arguments or creating an unreviewed remote file.
func (Local) RunInput(ctx context.Context, args []string, input string) (Result, error) {
	if err := validateArgs(args); err != nil {
		return Result{}, err
	}
	if len(input) > 64<<10 {
		return Result{}, fmt.Errorf("input exceeds 64 KiB")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(input)
	return execute(ctx, cmd, false)
}
func (l Local) Batch(ctx context.Context, commands [][]string) ([]Result, error) {
	var out []Result
	for _, args := range commands {
		r, e := l.Run(ctx, args)
		out = append(out, r)
		if e != nil && !IsKind(e, CommandFailed) {
			return out, e
		}
	}
	return out, nil
}

type SSH struct{ Host, directory, socket string }

func NewSSH(host string) (*SSH, error) {
	if host == "" || strings.HasPrefix(host, "-") || strings.ContainsAny(host, "\x00\r\n\t ") {
		return nil, fmt.Errorf("invalid SSH host; use a hostname or SSH config alias")
	}
	// /tmp keeps ControlPath below Unix socket limits even with long macOS TMPDIR.
	dir, err := os.MkdirTemp("/tmp", "perch-")
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(dir, 0700); err != nil {
		_ = os.Remove(dir)
		return nil, err
	}
	return &SSH{Host: host, directory: dir, socket: filepath.Join(dir, "ssh")}, nil
}
func (s *SSH) options(interactive bool) []string {
	batch := "yes"
	if interactive {
		batch = "no"
	}
	return []string{"-T", "-o", "ControlMaster=auto", "-o", "ControlPath=" + s.socket, "-o", "ControlPersist=120", "-o", "BatchMode=" + batch, "-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=2", "-o", "ClearAllForwardings=yes", "-o", "RequestTTY=no"}
}
func (s *SSH) LoginCommand() *exec.Cmd {
	args := append(s.options(true), "--", s.Host, "true")
	return exec.Command("ssh", args...)
}
func (s *SSH) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", "-S", s.socket, "-O", "exit", "--", s.Host)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
	return os.RemoveAll(s.directory)
}
func (s *SSH) Run(ctx context.Context, args []string) (Result, error) {
	if err := validateArgs(args); err != nil {
		return Result{}, err
	}
	argv := append(s.options(false), "--", s.Host, QuoteArgs(args))
	return execute(ctx, exec.CommandContext(ctx, "ssh", argv...), true)
}

func (s *SSH) RunInput(ctx context.Context, args []string, input string) (Result, error) {
	if err := validateArgs(args); err != nil {
		return Result{}, err
	}
	if len(input) > 64<<10 {
		return Result{}, fmt.Errorf("input exceeds 64 KiB")
	}
	argv := append(s.options(false), "--", s.Host, QuoteArgs(args))
	cmd := exec.CommandContext(ctx, "ssh", argv...)
	cmd.Stdin = strings.NewReader(input)
	return execute(ctx, cmd, true)
}

func validateArgs(args []string) error {
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("empty command")
	}
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return fmt.Errorf("NUL in command argument")
		}
	}
	return nil
}
func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func QuoteArgs(args []string) string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = Quote(arg)
	}
	return strings.Join(out, " ")
}

// Batch uses a cryptographically random delimiter, one SSH invocation and a
// private, short-lived stderr file. Exit codes belong to individual queries.
// The nonce is generated locally and never comes from job names or output.
func (s *SSH) Batch(ctx context.Context, commands [][]string) ([]Result, error) {
	if len(commands) == 0 {
		return nil, nil
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	nonce := "PERCH_" + hex.EncodeToString(random[:])
	script, err := batchScript(commands, nonce)
	if err != nil {
		return nil, err
	}
	r, err := s.Run(ctx, []string{"sh", "-c", script})
	if err != nil {
		return nil, err
	}
	results, err := parseBatch(r.Stdout, nonce, len(commands))
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Duration = r.Duration
	}
	return results, nil
}
func batchScript(commands [][]string, nonce string) (string, error) {
	var b strings.Builder
	b.WriteString("umask 077\nd=$(mktemp -d) || exit 70\ntrap 'rm -rf -- \"$d\"' EXIT\ntrap 'exit 129' HUP\ntrap 'exit 130' INT\ntrap 'exit 143' TERM\nmkfifo \"$d/stderr\" || exit 70\n")
	for i, args := range commands {
		if err := validateArgs(args); err != nil {
			return "", err
		}
		// Keep one extra byte as an overflow sentinel, then drain the rest.
		// A single FIFO descriptor spans both reads, avoiding a broken pipe
		// when head reaches the cap. Nothing beyond the sentinel hits disk.
		fmt.Fprintf(&b, "{ head -c %d >\"$d/err\" || exit 70; cat >/dev/null; } <\"$d/stderr\" &\nreader=$!\nprintf '%%s\\n' '%s:%d:OUT'\n%s 2>\"$d/stderr\"\nrc=$?\nwait \"$reader\" || exit 70\nprintf '\\n%%s\\n' '%s:%d:ERR'\ncat -- \"$d/err\"\nprintf '\\n%%s:%%s\\n' '%s:%d:EXIT' \"$rc\"\n", maxStderr+1, nonce, i, QuoteArgs(args), nonce, i, nonce, i)
	}
	b.WriteString("exit 0\n")
	return b.String(), nil
}
func parseBatch(raw, nonce string, count int) ([]Result, error) {
	var results []Result
	bad := func() ([]Result, error) { return nil, &Error{Protocol, "Invalid framed SSH response"} }
	for i := 0; i < count; i++ {
		start := fmt.Sprintf("%s:%d:OUT\n", nonce, i)
		if !strings.HasPrefix(raw, start) {
			return bad()
		}
		raw = raw[len(start):]
		out, rest, ok := strings.Cut(raw, fmt.Sprintf("\n%s:%d:ERR\n", nonce, i))
		if !ok {
			return bad()
		}
		stderr, rest, ok := strings.Cut(rest, fmt.Sprintf("\n%s:%d:EXIT:", nonce, i))
		if !ok {
			return bad()
		}
		if len(stderr) > maxStderr {
			return nil, &Error{OutputLimit, "Query exceeded its stderr limit; narrow the query"}
		}
		code, rest, ok := strings.Cut(rest, "\n")
		if !ok {
			return bad()
		}
		n, err := strconv.Atoi(code)
		if err != nil || n < 0 || n > 255 {
			return bad()
		}
		results = append(results, Result{Stdout: out, Stderr: stderr, ExitCode: n})
		raw = rest
	}
	if raw != "" {
		return bad()
	}
	return results, nil
}
