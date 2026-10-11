package transport

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Exercise exec.Cmd's pipe copying, including its io.Copy fast paths. Calling
// limitedBuffer.Write directly cannot catch an accidentally promoted ReadFrom.
func TestOutputLimitHelperProcess(t *testing.T) {
	mode := os.Getenv("PERCH_OUTPUT_LIMIT_HELPER")
	if mode == "" {
		return
	}
	w, limit := os.Stdout, MaxOutput
	if mode == "stderr" {
		w, limit = os.Stderr, 1<<20
	}
	chunk := bytes.Repeat([]byte("x"), 32<<10)
	for remaining := limit + 1; remaining > 0; {
		n, err := w.Write(chunk[:min(remaining, len(chunk))])
		if err != nil {
			os.Exit(2)
		}
		remaining -= n
	}
	os.Exit(0)
}

func TestSubprocessOutputLimits(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			t.Setenv("PERCH_OUTPUT_LIMIT_HELPER", stream)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result, err := (Local{}).Run(ctx, []string{exe, "-test.run=^TestOutputLimitHelperProcess$"})
			if !IsKind(err, OutputLimit) || result.ExitCode != 0 {
				t.Fatalf("exit=%d error=%v; want bounded-output failure from successful process", result.ExitCode, err)
			}
			if stream == "stdout" && (len(result.Stdout) != MaxOutput || result.Stderr != "") || stream == "stderr" && (len(result.Stderr) != 1<<20 || result.Stdout != "") {
				t.Fatalf("retained lengths stdout=%d stderr=%d", len(result.Stdout), len(result.Stderr))
			}
		})
	}
}

func TestQuoteRoundTrip(t *testing.T) {
	values := []string{"", "hello world", "a'b", "$(touch /tmp/should-not-exist)", "a\nb", "--help", "`whoami`", "雪"}
	for _, value := range values {
		got, err := exec.Command("sh", "-c", QuoteArgs([]string{"printf", "%s", value})).Output()
		if err != nil || string(got) != value {
			t.Errorf("%q: %q %v", value, got, err)
		}
	}
}
func TestBatchPreservesOutputAndPartialFailure(t *testing.T) {
	commands := [][]string{{"printf", "a\n\n"}, {"sh", "-c", "printf bad >&2; exit 7"}, {"printf", "tail"}}
	script, err := batchScript(commands, "test_nonce")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := exec.Command("sh", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	result, err := parseBatch(string(raw), "test_nonce", 3)
	if err != nil {
		t.Fatal(err)
	}
	if result[0].Stdout != "a\n\n" || result[1].ExitCode != 7 || result[1].Stderr != "bad" || result[2].Stdout != "tail" {
		t.Fatalf("%#v", result)
	}
	if _, e := parseBatch(string(raw)+"extra", "test_nonce", 3); !IsKind(e, Protocol) {
		t.Fatal(e)
	}
	if _, e := parseBatch(string(raw[:len(raw)/2]), "test_nonce", 3); !IsKind(e, Protocol) {
		t.Fatal(e)
	}
}
func TestTimeoutAndLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, err := (Local{}).Run(ctx, []string{"sleep", "3"})
	if !IsKind(err, Timeout) {
		t.Fatal(err)
	}
	b := limitedBuffer{limit: 4}
	n, err := b.Write([]byte("123456"))
	if n != 6 || err != nil || !b.overflow || b.String() != "1234" {
		t.Fatal(b, n, err)
	}
}
func TestSSHPrivateSocketAndOptions(t *testing.T) {
	s, err := NewSSH("example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(s.directory)
	info, err := os.Stat(s.directory)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal(info, err)
	}
	if len(s.socket) > 90 {
		t.Fatal("socket path too long")
	}
	opts := strings.Join(s.options(false), " ")
	if !strings.Contains(opts, "BatchMode=yes") || !strings.Contains(opts, "ControlPath="+s.socket) {
		t.Fatal(opts)
	}
	login := strings.Join(s.LoginCommand().Args, " ")
	if !strings.Contains(login, "BatchMode=no") || !strings.HasSuffix(login, " example true") {
		t.Fatal(login)
	}
	for _, host := range []string{"-oProxyCommand=bad", "host\nother", ""} {
		if s, e := NewSSH(host); e == nil {
			os.RemoveAll(s.directory)
			t.Fatal("accepted invalid host")
		}
	}
}

func TestLocalInputPreservesScriptAndBounds(t *testing.T) {
	input := "quotes '$()' and spaces\nsecond line\n"
	r, e := (Local{}).RunInput(context.Background(), []string{"cat"}, input)
	if e != nil || r.Stdout != input {
		t.Fatal(r, e)
	}
	if _, e = (Local{}).RunInput(context.Background(), []string{"cat"}, strings.Repeat("x", (64<<10)+1)); e == nil {
		t.Fatal("oversized input accepted")
	}
}
