package cancellation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/transport"
	fixtures "perch/testdata/hummingbird"
)

type fake struct {
	raw         string
	user        string
	calls       [][]string
	mutation    transport.Result
	mutationErr error
}

func (f *fake) Run(_ context.Context, args []string) (transport.Result, error) {
	f.calls = append(f.calls, args)
	if args[0] == "id" {
		return transport.Result{Stdout: f.user}, nil
	}
	if args[0] != "env" || len(args) < 12 {
		return transport.Result{}, fmt.Errorf("unexpected command %v", args)
	}
	switch args[11] {
	case "squeue":
		return transport.Result{Stdout: f.raw}, nil
	case "scancel":
		return f.mutation, f.mutationErr
	}
	return transport.Result{}, fmt.Errorf("unexpected command %v", args)
}
func (f *fake) Batch(context.Context, [][]string) ([]transport.Result, error) {
	panic("unexpected batch")
}
func (f *fake) mutations() [][]string {
	var calls [][]string
	for _, a := range f.calls {
		if len(a) > 11 && a[11] == "scancel" {
			calls = append(calls, a[11:])
		}
	}
	return calls
}
func client(f *fake) *hummingbird.Client {
	return &hummingbird.Client{Host: "fixture.invalid", Runner: f}
}
func queue(t *testing.T, edits ...func(map[string]any)) *fake {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(fixtures.Queue), &data); err != nil {
		t.Fatal(err)
	}
	base := data["jobs"].([]any)[0]
	var jobs []any
	for _, edit := range edits {
		b, _ := json.Marshal(base)
		var j map[string]any
		_ = json.Unmarshal(b, &j)
		j["het_job_id"] = map[string]any{"set": false, "infinite": false, "number": 0}
		j["het_job_offset"] = map[string]any{"set": true, "number": 0}
		for _, key := range []string{"het_job_id_set", "federation_origin", "federation_siblings_active", "federation_siblings_viable"} {
			j[key] = ""
		}
		if edit != nil {
			edit(j)
		}
		jobs = append(jobs, j)
	}
	if jobs == nil {
		jobs = []any{}
	}
	data["jobs"] = jobs
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return &fake{raw: string(b), user: "researcher"}
}
func ordinary(j map[string]any) {
	j["job_id"] = 42
	j["array_job_id"] = map[string]any{"set": false}
	j["array_task_id"] = map[string]any{"set": false}
}
func grouped(j map[string]any) {
	j["job_id"] = 9100
	j["array_task_id"] = map[string]any{"set": false}
	j["array_task_string"] = "0-40"
	j["job_state"] = []string{"PENDING"}
}
func prepared(t *testing.T, f *fake, target Target) *Plan {
	t.Helper()
	p, err := Prepare(context.Background(), client(f), target)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestScopePreservesLogicalTaskEvenWhenAllocationEqualsParent(t *testing.T) {
	f := queue(t, func(j map[string]any) { j["job_id"] = 9100 })
	if _, err := Prepare(context.Background(), client(f), Target{ID: "9100"}); err == nil {
		t.Fatal("bare parent accepted")
	}
	p := prepared(t, f, Target{ID: "9100_39"})
	if len(f.mutations()) != 0 {
		t.Fatal("preview mutated")
	}
	r, err := Execute(context.Background(), client(f), p)
	if err != nil || r.Status != "requested" {
		t.Fatal(r, err)
	}
	a := strings.Join(f.mutations()[0], " ")
	if a != "scancel --ctld --clusters=hummingbird --user=researcher --state=RUNNING -- 9100_39" {
		t.Fatal(a)
	}
	if _, err := Execute(context.Background(), client(f), p); err == nil || len(f.mutations()) != 1 {
		t.Fatal("review reused")
	}
}

func TestOrdinaryAndGroupedAndWholeArrayScopes(t *testing.T) {
	for _, state := range []string{"PENDING", "RUNNING", "SUSPENDED"} {
		f := queue(t, func(j map[string]any) { ordinary(j); j["job_state"] = []string{state} })
		p := prepared(t, f, Target{ID: "42"})
		if p.View().Scope != "job" {
			t.Fatal(p.View())
		}
		if _, err := Execute(context.Background(), client(f), p); err != nil {
			t.Fatal(err)
		}
	}
	f := queue(t, grouped)
	p := prepared(t, f, Target{ID: "9100_0"})
	if p.View().Tasks != 1 || p.View().Scope != "task" {
		t.Fatal(p.View())
	}
	if _, err := Execute(context.Background(), client(f), p); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), client(f), Target{ID: "9100_99"}); err == nil {
		t.Fatal("absent task accepted")
	}
	f = queue(t, func(j map[string]any) { grouped(j); j["array_task_string"] = "0-38" }, nil)
	p = prepared(t, f, Target{ID: "9100", Array: true})
	if p.View().Tasks != 40 || strings.Join(p.View().States, ",") != "1 RUNNING,39 PENDING" {
		t.Fatal(p.View())
	}
	if _, err := Execute(context.Background(), client(f), p); err != nil {
		t.Fatal(err)
	}
	if a := strings.Join(f.mutations()[0], " "); a != "scancel --ctld --clusters=hummingbird --user=researcher -- 9100" {
		t.Fatal(a)
	}
	// Huge ranges stay compact and count distinct indices within one record.
	f = queue(t, func(j map[string]any) { grouped(j); j["array_task_string"] = "0-4000000000,1-200" })
	p = prepared(t, f, Target{ID: "9100", Array: true})
	if p.View().Tasks != 4000000001 {
		t.Fatal(p.View())
	}
}

func TestRefusalsNeverMutate(t *testing.T) {
	for name, edit := range map[string]func(map[string]any){
		"foreign":           func(j map[string]any) { j["user_name"] = "other" },
		"sanitized owner":   func(j map[string]any) { j["user_name"] = "researcher\x1b" },
		"unknown submit":    func(j map[string]any) { j["submit_time"] = map[string]any{"set": false} },
		"missing restarts":  func(j map[string]any) { delete(j, "restart_cnt") },
		"negative restarts": func(j map[string]any) { j["restart_cnt"] = -1 },
		"unknown scope":     func(j map[string]any) { delete(j, "het_job_id") },
		"heterogeneous":     func(j map[string]any) { j["het_job_id"] = map[string]any{"set": true, "number": 42} },
		"heterogeneous set": func(j map[string]any) { j["het_job_id_set"] = "42-43" },
		"federated":         func(j map[string]any) { j["federation_origin"] = "hummingbird" },
		"remote sibling":    func(j map[string]any) { j["federation_siblings_active"] = "elsewhere" },
		"wrong cluster":     func(j map[string]any) { j["cluster"] = "elsewhere" },
		"control cluster":   func(j map[string]any) { j["cluster"] = "hummingbird\x1b" },
		"completed":         func(j map[string]any) { j["job_state"] = []string{"COMPLETED"} },
		"state flags":       func(j map[string]any) { j["job_state"] = []string{"RUNNING", "COMPLETING"} },
		"missing start":     func(j map[string]any) { j["start_time"] = map[string]any{"set": false} },
	} {
		t.Run(name, func(t *testing.T) {
			f := queue(t, edit)
			if _, err := Prepare(context.Background(), client(f), Target{ID: "9100_39"}); err == nil {
				t.Fatal("accepted")
			}
			if len(f.mutations()) != 0 {
				t.Fatal("mutated")
			}
		})
	}
	for _, target := range []Target{{ID: "0"}, {ID: "42.0"}, {ID: "42+0"}, {ID: "42_[0-9]"}, {ID: "42;true"}, {ID: "42_01"}, {ID: "42_0", Array: true}, {ID: "4294967295"}} {
		f := queue(t, nil)
		if _, err := Prepare(context.Background(), client(f), target); err == nil || len(f.calls) != 0 {
			t.Fatal(target, err)
		}
	}
	f := queue(t, grouped, nil)
	if _, err := Prepare(context.Background(), client(f), Target{ID: "9100", Array: true}); err == nil {
		t.Fatal("overlap accepted")
	}
	if _, err := Prepare(context.Background(), client(f), Target{ID: "9100_39"}); err == nil {
		t.Fatal("ambiguous task accepted")
	}
}

func TestRevalidateAttemptScopeAndHost(t *testing.T) {
	for _, edit := range []func(map[string]any){
		func(j map[string]any) { j["restart_cnt"] = 1 },
		func(j map[string]any) { j["submit_time"] = map[string]any{"set": true, "number": 1791599999} },
		func(j map[string]any) { j["job_id"] = 9109 },
		func(j map[string]any) { j["name"] = "changed" },
		func(j map[string]any) { j["job_state"] = []string{"SUSPENDED"} },
	} {
		f := queue(t, nil)
		p := prepared(t, f, Target{ID: "9100_39"})
		f.raw = queue(t, edit).raw
		if _, err := Execute(context.Background(), client(f), p); err == nil || len(f.mutations()) != 0 {
			t.Fatal("changed attempt accepted", err)
		}
	}
	for _, mode := range []string{"host", "user", "expired", "missing", "array-change"} {
		f := queue(t, grouped)
		p := prepared(t, f, Target{ID: "9100", Array: true})
		c := client(f)
		switch mode {
		case "host":
			c.Host = "other"
		case "user":
			f.user = "other"
		case "expired":
			p.preview.Expires = time.Now().Add(-time.Second)
		case "missing":
			f.raw = queue(t).raw
		case "array-change":
			f.raw = queue(t, func(j map[string]any) { grouped(j); j["array_task_string"] = "0-39" }).raw
		}
		if _, err := Execute(context.Background(), c, p); err == nil || len(f.mutations()) != 0 {
			t.Fatal(mode, err)
		}
	}
}

func TestDuplicateConfirmationAndDetachedView(t *testing.T) {
	f := queue(t, nil)
	p := prepared(t, f, Target{ID: "9100_39"})
	v := p.View()
	v.Target.ID = "42"
	v.States[0] = "1 PENDING"
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { _, _ = Execute(context.Background(), client(f), p) })
	}
	wg.Wait()
	if len(f.mutations()) != 1 || !strings.Contains(strings.Join(f.mutations()[0], " "), "--state=RUNNING -- 9100_39") {
		t.Fatal(f.calls)
	}
}

func TestReceiptsDoNotInventSchedulerStateOrRetry(t *testing.T) {
	for _, tc := range []struct {
		err    error
		code   int
		status string
	}{
		{nil, 0, "requested"}, {nil, 1, "unconfirmed"}, {&transport.Error{Kind: transport.CommandFailed, Message: "failed"}, 1, "unconfirmed"}, {context.DeadlineExceeded, 0, "unknown"}, {&transport.Error{Kind: transport.Network, Message: "lost"}, 255, "unknown"},
	} {
		f := queue(t, nil)
		p := prepared(t, f, Target{ID: "9100_39"})
		f.mutationErr = tc.err
		f.mutation = transport.Result{ExitCode: tc.code, Stderr: "bad\x1b[2J"}
		r, err := Execute(context.Background(), client(f), p)
		if r == nil || r.Status != tc.status || (err == nil) != (tc.status == "requested") || len(f.mutations()) != 1 || strings.ContainsRune(r.Detail, '\x1b') {
			t.Fatal(r, err)
		}
	}
}

func TestEnvironmentCannotChangeScopeAndArgumentsRemainLiteral(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "must-not-exist")
	startup := filepath.Join(dir, "startup")
	if err := os.WriteFile(startup, []byte("touch "+transport.Quote(sentinel)), 0600); err != nil {
		t.Fatal(err)
	}
	literal := "$(touch " + sentinel + ")"
	a := clean([]string{"/bin/bash", "--noprofile", "--norc", "-c", `printf '%s\n' "${SCANCEL_SIGNAL-unset}" "${SCANCEL_HURRY-unset}" "${SQUEUE_STATES-unset}" "${SLURM_CLUSTERS-unset}" "$1"`, "test", literal})
	cmd := exec.Command(a[0], a[1:]...)
	cmd.Env = append(os.Environ(), "SCANCEL_SIGNAL=KILL", "SCANCEL_HURRY=1", "SQUEUE_STATES=RUNNING", "SLURM_CLUSTERS=other", "BASH_ENV="+startup)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "unset\nunset\nunset\nunset\n"+literal+"\n" {
		t.Fatal(string(out), err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("argument or startup executed", err)
	}
}

func TestIncompleteOrInconsistentIdentityRefused(t *testing.T) {
	for name, edit := range map[string]func(map[string]any){
		"array field missing":  func(j map[string]any) { delete(j, "array_job_id") },
		"array infinite":       func(j map[string]any) { j["array_job_id"] = map[string]any{"set": true, "infinite": true} },
		"task field missing":   func(j map[string]any) { delete(j, "array_task_id") },
		"offset missing":       func(j map[string]any) { delete(j, "het_job_offset") },
		"heterogeneous offset": func(j map[string]any) { j["het_job_offset"] = map[string]any{"set": true, "number": 1} },
		"ordinary with task":   func(j map[string]any) { j["array_task_id"] = map[string]any{"set": true, "number": 0} },
	} {
		t.Run(name, func(t *testing.T) {
			f := queue(t, func(j map[string]any) { ordinary(j); edit(j) })
			if _, err := Prepare(context.Background(), client(f), Target{ID: "42"}); err == nil || len(f.mutations()) != 0 {
				t.Fatal(err)
			}
		})
	}
}

func TestIncompleteEnvelopeAndUnverifiedGroupedVersionRefused(t *testing.T) {
	for _, mode := range []string{"warning", "parser", "cluster", "release", "errors"} {
		f := queue(t, grouped)
		var data map[string]any
		_ = json.Unmarshal([]byte(f.raw), &data)
		meta := data["meta"].(map[string]any)
		switch mode {
		case "warning":
			data["warnings"] = []any{map[string]any{"description": "Partial response"}}
		case "parser":
			meta["plugin"].(map[string]any)["data_parser"] = "data_parser/v0.0.45"
		case "cluster":
			meta["slurm"].(map[string]any)["cluster"] = "other"
		case "release":
			meta["slurm"].(map[string]any)["release"] = "26.05.0"
		case "errors":
			data["errors"] = []any{map[string]any{"description": "error"}}
		}
		b, _ := json.Marshal(data)
		f.raw = string(b)
		if _, err := Prepare(context.Background(), client(f), Target{ID: "9100_0"}); err == nil || len(f.mutations()) != 0 {
			t.Fatal(mode, err)
		}
	}
	f := queue(t)
	var data map[string]any
	_ = json.Unmarshal([]byte(f.raw), &data)
	data["warnings"] = []any{map[string]any{"description": "Zero jobs to dump"}}
	b, _ := json.Marshal(data)
	f.raw = string(b)
	if _, err := Prepare(context.Background(), client(f), Target{ID: "42"}); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatal(err)
	}
}

func TestCapturedProductionScopes(t *testing.T) {
	b, err := os.ReadFile("../../testdata/hummingbird/cancellation-scopes.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Case, Stage, Parent string
			Response            json.RawMessage
		}
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Case+"/"+tc.Stage, func(t *testing.T) {
			f := &fake{raw: string(tc.Response), user: "researcher"}
			target := Target{ID: tc.Parent, Array: strings.HasSuffix(tc.Case, "array")}
			p := prepared(t, f, target)
			if target.Array {
				task := "0"
				want := int64(2)
				if tc.Stage == "sibling_remained" {
					task = "1"
					want = 1
				}
				if p.View().Tasks != want {
					t.Fatal(p.View())
				}
				tp := prepared(t, f, Target{ID: tc.Parent + "_" + task})
				if tp.View().Scope != "task" || tp.View().Tasks != 1 {
					t.Fatal(tp.View())
				}
			}
			if _, err := Execute(context.Background(), client(f), p); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReviewMatchesOnlySelectedAttempt(t *testing.T) {
	f := queue(t, nil)
	p := prepared(t, f, Target{ID: "9100_39"})
	jobs, err := hummingbird.ParseQueue(f.raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	j := jobs[0]
	if !p.MatchesSelection(j) {
		t.Fatal("current selection rejected")
	}
	for _, edit := range []func(*domain.Job){
		func(j *domain.Job) { j.Restarts++ }, func(j *domain.Job) { j.RawID++ }, func(j *domain.Job) { j.User = "other" }, func(j *domain.Job) { j.Cluster = "other" }, func(j *domain.Job) { j.Submit = j.Submit.Add(time.Second) },
	} {
		copy := j
		edit(&copy)
		if p.MatchesSelection(copy) {
			t.Fatal("changed selection accepted")
		}
	}
}
