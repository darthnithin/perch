package allocshell

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"perch/internal/hummingbird"
	"perch/internal/transport"
	fixtures "perch/testdata/hummingbird"
)

type fake struct {
	raw       string
	nodes     string
	commands  [][]string
	queueExit int
}

func (f *fake) Run(_ context.Context, a []string) (transport.Result, error) {
	f.commands = append(f.commands, a)
	switch a[0] {
	case "id":
		return transport.Result{Stdout: "researcher\n"}, nil
	case "squeue":
		return transport.Result{Stdout: f.raw, ExitCode: f.queueExit}, nil
	case "scontrol":
		return transport.Result{Stdout: f.nodes}, nil
	case "timeout":
		return transport.Result{Stdout: "timeout (GNU coreutils) 8.32\n"}, nil
	}
	return transport.Result{}, fmt.Errorf("unexpected operation: %v", a)
}
func (f *fake) Batch(context.Context, [][]string) ([]transport.Result, error) {
	panic("unexpected batch")
}
func newFake(t *testing.T, edit func(map[string]any)) *fake {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal([]byte(fixtures.Queue), &response); err != nil {
		t.Fatal(err)
	}
	job := response["jobs"].([]any)[0].(map[string]any)
	if edit != nil {
		edit(job)
	}
	response["jobs"] = []any{job}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	return &fake{raw: string(raw), nodes: "node-14\n"}
}
func opts() Options { return Options{CPUs: 1, MemoryMiB: 64, Minutes: 10} }
func prepare(t *testing.T, f *fake) Plan {
	t.Helper()
	p, err := Prepare(context.Background(), &hummingbird.Client{Runner: f}, "9100_39", opts())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOwnedArrayTaskUsesConcreteAllocation(t *testing.T) {
	f := newFake(t, nil)
	p := prepare(t, f)
	if p.JobID != "9100_39" || p.AllocationID == 9100 || p.Node != "node-14" {
		t.Fatal(p)
	}
	if !reflect.DeepEqual(f.commands[1], []string{"squeue", "--json=" + hummingbird.Parser, "--user=researcher", "--jobs=9100_39"}) {
		t.Fatal(f.commands)
	}
	args := strings.Join(p.Arguments, " ")
	for _, want := range []string{fmt.Sprintf("--jobid=%d", p.AllocationID), "--nodes=1", "--ntasks=1", "--gres=none", "--overlap", "--exact", "--immediate=5", "--export=ALL"} {
		if !strings.Contains(args, want) {
			t.Fatal(args)
		}
	}
}

func TestExplicitNodeAndResourceScope(t *testing.T) {
	f := newFake(t, func(j map[string]any) {
		j["node_count"] = map[string]any{"set": true, "number": 2}
		j["cpus"] = map[string]any{"set": true, "number": 2}
		j["nodes"] = "node-[14-15]"
	})
	f.nodes = "node-14\nnode-15\n"
	o := opts()
	o.Node = "node-15"
	p, err := Prepare(context.Background(), &hummingbird.Client{Runner: f}, "9100_39", o)
	if err != nil || p.Node != "node-15" || !reflect.DeepEqual(p.Nodes, []string{"node-14", "node-15"}) {
		t.Fatal(p, err)
	}
	o.CPUs = 3
	if _, err = Prepare(context.Background(), &hummingbird.Client{Runner: f}, "9100_39", o); err == nil {
		t.Fatal("CPU overflow accepted")
	}
	o = opts()
	o.MemoryMiB = 8193
	if _, err = Prepare(context.Background(), &hummingbird.Client{Runner: f}, "9100_39", o); err == nil {
		t.Fatal("memory overflow accepted")
	}
}

func TestRejectInvalidOrUnavailableAllocation(t *testing.T) {
	for name, edit := range map[string]func(map[string]any){
		"other owner":        func(j map[string]any) { j["user_name"] = "someone-else" },
		"pending":            func(j map[string]any) { j["job_state"] = []string{"PENDING"} },
		"completing":         func(j map[string]any) { j["job_state"] = []string{"COMPLETING"} },
		"collapsed":          func(j map[string]any) { j["array_task_string"] = "1-20" },
		"array parent":       func(j map[string]any) { j["array_task_id"] = map[string]any{"set": false} },
		"relative directory": func(j map[string]any) { j["current_working_directory"] = "../work" },
		"control directory":  func(j map[string]any) { j["current_working_directory"] = "/work\x1b[2J" },
		"unknown attempt":    func(j map[string]any) { j["submit_time"] = map[string]any{"set": false} },
		"no CPU":             func(j map[string]any) { j["cpus"] = map[string]any{"set": false} },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFake(t, edit)
			if _, err := Prepare(context.Background(), &hummingbird.Client{Runner: f}, "9100_39", opts()); err == nil {
				t.Fatal("accepted invalid allocation")
			}
		})
	}
	f := newFake(t, nil)
	f.queueExit = 1
	if _, err := Prepare(context.Background(), &hummingbird.Client{Runner: f}, "9100_39", opts()); err == nil {
		t.Fatal("accepted failed queue")
	}
	for _, nodes := range []string{"", "node-14\nnode-15\n", "--bad\n"} {
		f = newFake(t, nil)
		f.nodes = nodes
		if _, err := Prepare(context.Background(), &hummingbird.Client{Runner: f}, "9100_39", opts()); err == nil {
			t.Fatal("accepted bad nodes", nodes)
		}
	}
	f = newFake(t, nil)
	o := opts()
	o.Node = "node-999"
	if _, err := Prepare(context.Background(), &hummingbird.Client{Runner: f}, "9100_39", o); err == nil {
		t.Fatal("accepted node outside allocation")
	}
	f = newFake(t, nil)
	if _, err := Prepare(context.Background(), &hummingbird.Client{Runner: f}, "91;touch /tmp/no", opts()); err == nil || len(f.commands) != 0 {
		t.Fatal("invalid request reached transport")
	}
}

func TestRevalidationRefusesChangedAttemptAndDirectory(t *testing.T) {
	f := newFake(t, nil)
	p := prepare(t, f)
	if _, err := Revalidate(context.Background(), &hummingbird.Client{Runner: f}, p); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(map[string]any){
		func(j map[string]any) { j["restart_cnt"] = 1 },
		func(j map[string]any) { j["current_working_directory"] = "/different" },
		func(j map[string]any) { j["start_time"] = map[string]any{"set": true, "number": 1} },
		func(j map[string]any) { j["job_id"] = 9200 },
	} {
		changed := newFake(t, edit)
		if _, err := Revalidate(context.Background(), &hummingbird.Client{Runner: changed}, p); err == nil {
			t.Fatal("changed allocation accepted")
		}
	}
}

func TestCommandKeepsLiteralArgumentsAndClearsInheritedSchedulerOptions(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "do-not-create")
	startup := filepath.Join(dir, "startup")
	if err := os.WriteFile(startup, []byte("touch "+transport.Quote(sentinel)), 0600); err != nil {
		t.Fatal(err)
	}
	literal := "a' $(touch " + sentinel + ")"
	p := Plan{Arguments: []string{"/bin/bash", "--noprofile", "--norc", "-c", `printf '%s\n' "${SLURM_GPUS-unset}" "${SRUN_OUTPUT-unset}" "$PERCH_KEEP" "$1"`, "test", literal}}
	p.Options = opts()
	a := p.Command()
	// The Mac test host need not have GNU timeout. Execute the environment
	// wrapper with the planned timeout prefix removed, retaining literal argv.
	if !reflect.DeepEqual(a[11:16], []string{"timeout", "--foreground", "--signal=TERM", "--kill-after=10s", "600s"}) {
		t.Fatal(a)
	}
	a = append(a[:11], a[16:]...)
	cmd := exec.Command(a[0], a[1:]...)
	cmd.Env = append(os.Environ(), "SLURM_GPUS=8", "SRUN_OUTPUT=/tmp/no", "PERCH_KEEP=kept", "BASH_ENV="+startup)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "unset\nunset\nkept\n"+literal+"\n" {
		t.Fatal(string(out), err)
	}
	if _, err = os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("startup or argument executed", err)
	}
}
