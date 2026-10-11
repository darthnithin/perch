package launch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"perch/internal/transport"
)

func spec() Spec {
	return Spec{Name: "test-job", WorkDir: "/scratch/test", CPUs: 2, MemoryMiB: 512, Minutes: 5, Command: []string{"printf", "%s\\n", "hello; $(not-a-command)", "single'quote", ""}}
}
func TestScriptPreservesLiteralArguments(t *testing.T) {
	p, e := Build(spec())
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(p.Script)
	if output, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(output))
	}
	cmd = exec.Command("bash")
	cmd.Stdin = strings.NewReader(p.Script)
	output, e := cmd.Output()
	if e != nil {
		t.Fatal(e)
	}
	if string(output) != "hello; $(not-a-command)\nsingle'quote\n\n" {
		t.Fatalf("arguments changed: %q", output)
	}
	if strings.Contains(p.Script, "#SBATCH") {
		t.Fatal("resources should be explicit arguments")
	}
}
func TestSpecRejectsInjectionAndDangerousDefaults(t *testing.T) {
	for _, change := range []func(*Spec){
		func(s *Spec) { s.Name = "job\n#SBATCH --array=1-100" },
		func(s *Spec) { s.WorkDir = "relative" }, func(s *Spec) { s.WorkDir = "/tmp\nx" },
		func(s *Spec) { s.Partition = "x --exclusive" }, func(s *Spec) { s.MemoryMiB = 0 },
		func(s *Spec) { s.CPUs = 0 }, func(s *Spec) { s.Minutes = 0 }, func(s *Spec) { s.GPUs = -1 },
		func(s *Spec) { s.Modules = []string{"x;echo"} }, func(s *Spec) { s.Command = nil },
		func(s *Spec) { s.Command = []string{"-c", "echo"} }, func(s *Spec) { s.Command = []string{"echo", "\x1b[2J"} },
	} {
		s := spec()
		change(&s)
		if _, e := Build(s); e == nil {
			t.Fatalf("accepted %+v", s)
		}
	}
	s := spec()
	s.WorkDir = "/scratch/a b/100%done"
	s.Modules = []string{"gnu14/14.2.0", "openmpi5/5.0.10"}
	s.GPUs = 1
	p, e := Build(s)
	if e != nil {
		t.Fatal(e)
	}
	if p.Stdout != "/scratch/a b/100%%done/perch-%j.out" {
		t.Fatal(p.Stdout)
	}
	if !strings.Contains(p.Script, "module load 'gnu14/14.2.0' 'openmpi5/5.0.10'") {
		t.Fatal("module order lost")
	}
}
func TestUnitsAreFiniteAndUnambiguous(t *testing.T) {
	for _, input := range []string{"0G", "1.5G", "1T", "999999999999999G"} {
		if _, e := MemoryMiB(input); e == nil {
			t.Fatal(input)
		}
	}
	if n, e := MemoryMiB("2G"); e != nil || n != 2048 {
		t.Fatal(n, e)
	}
	for _, input := range []string{"0", "-2m", "30s", "1m1s", "unlimited"} {
		if _, e := Minutes(input); e == nil {
			t.Fatal(input)
		}
	}
	if n, e := Minutes("1h30m"); e != nil || n != 90 {
		t.Fatal(n, e)
	}
}

type runner struct {
	calls  int
	result transport.Result
	err    error
	args   []string
	script string
}

func (r *runner) Run(context.Context, []string) (transport.Result, error) {
	return transport.Result{}, nil
}
func (r *runner) RunInput(_ context.Context, args []string, input string) (transport.Result, error) {
	r.calls++
	r.args = args
	r.script = input
	return r.result, r.err
}
func TestSubmissionParsesReceiptAndNeverRetries(t *testing.T) {
	p, _ := Build(spec())
	for _, raw := range []string{"42\n", "42;hummingbird\n"} {
		r := &runner{result: transport.Result{Stdout: raw}}
		receipt, e := (Client{Runner: r}).Submit(context.Background(), p)
		if e != nil || receipt.JobID != "42" || !strings.HasPrefix(receipt.Token, "perch-") || r.calls != 1 {
			t.Fatal(receipt, e, r.calls)
		}
		if !strings.Contains(strings.Join(r.args, " "), "--comment="+receipt.Token) || !strings.Contains(r.script, "exec 'printf'") {
			t.Fatal("missing exact script/token")
		}
	}
	for _, r := range []*runner{{err: errors.New("network dropped")}, {result: transport.Result{Stdout: "garbled"}}, {result: transport.Result{ExitCode: 1, Stderr: "denied"}}} {
		receipt, e := (Client{Runner: r}).Submit(context.Background(), p)
		if e == nil || r.calls != 1 || receipt.Token == "" || !strings.Contains(e.Error(), "before retrying") {
			t.Fatal(receipt, e, r.calls)
		}
	}
}
func TestCheckCannotSubmitOrUseTamperedPlan(t *testing.T) {
	p, _ := Build(spec())
	p.Arguments = []string{"--array=1-999"}
	p.Script = "unreviewed"
	r := &runner{result: transport.Result{Stderr: "estimate"}}
	if _, e := (Client{Runner: r}).Check(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	args := strings.Join(r.args, " ")
	if !strings.Contains(args, "--test-only") || strings.Contains(args, "--array") || r.script == "unreviewed" {
		t.Fatal(args)
	}
}

func TestSbatchWrapperClearsInheritedOptions(t *testing.T) {
	dir := t.TempDir()
	tool := dir + "/sbatch"
	// Use a non-login shell for this isolated transport test so the test PATH is
	// retained. Production login setup is covered by the live cluster smoke job.
	if e := os.WriteFile(tool, []byte("#!/bin/bash\nif [[ -n ${SBATCH_ARRAY_INX:-} ]]; then exit 9; fi\nprintf '%s\\n' \"$@\"\ncat\n"), 0700); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command("bash", "-c", sbatchShell, "perch-sbatch", "--test-only", "--job-name=literal")
	cmd.Env = append(os.Environ(), "PATH="+dir+":/usr/bin:/bin", "SBATCH_ARRAY_INX=1-999")
	cmd.Stdin = strings.NewReader("#!/bin/bash\ntrue\n")
	output, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatal(e, string(output))
	}
	if !strings.Contains(string(output), "--job-name=literal\n#!/bin/bash\ntrue") {
		t.Fatal(string(output))
	}
}
