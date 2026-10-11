package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/transport"
)

type launchRunner struct {
	checks, submissions int
	fail                bool
}

func (r *launchRunner) Run(context.Context, []string) (transport.Result, error) {
	return transport.Result{}, nil
}
func (r *launchRunner) Batch(context.Context, [][]string) ([]transport.Result, error) {
	return nil, errors.New("not used")
}
func (r *launchRunner) RunInput(_ context.Context, args []string, script string) (transport.Result, error) {
	if strings.Contains(strings.Join(args, " "), "--test-only") {
		r.checks++
		return transport.Result{Stderr: "scheduler estimate"}, nil
	}
	r.submissions++
	if r.fail {
		return transport.Result{}, errors.New("connection lost")
	}
	return transport.Result{Stdout: "812999\n"}, nil
}
func readyLaunch(t *testing.T) (*Model, *launchRunner) {
	t.Helper()
	m := demoModel(100, 32)
	m.options.Demo = false
	r := &launchRunner{}
	m.client = &hummingbird.Client{Runner: r}
	command(m, "/run")
	f := m.launchForm
	f.fields[0].input.SetValue("/scratch/example")
	f.fields[1].input.SetValue("printf 'hello world'")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if f.stage != "review" || r.checks != 0 || r.submissions != 0 {
		t.Fatal("preview contacted cluster")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	if cmd == nil {
		t.Fatal("validation did not start")
	}
	m.Update(cmd())
	if f.stage != "ready" || r.checks != 1 || r.submissions != 0 {
		t.Fatal("validation submitted or failed")
	}
	return m, r
}
func TestLaunchRequiresReviewValidationAndSingleExplicitSubmission(t *testing.T) {
	m := demoModel(100, 32)
	command(m, "/run")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if cmd != nil || m.launchForm.stage != "edit" {
		t.Fatal("submitted from edit")
	}
	m, r := readyLaunch(t)
	f := m.launchForm
	_, cmd = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if cmd == nil || f.stage != "submitting" {
		t.Fatal("explicit submission missing")
	}
	_, again := m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if again != nil {
		t.Fatal("double submit")
	}
	m.Update(cmd())
	if f.stage != "submitted" || f.receipt.JobID != "812999" || r.submissions != 1 {
		t.Fatal(f.stage, r.submissions)
	}
	_, again = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if again != nil {
		t.Fatal("resubmitted completed request")
	}
}
func TestLaunchAmbiguousFailureLocksRetry(t *testing.T) {
	m, r := readyLaunch(t)
	r.fail = true
	_, cmd := m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m.Update(cmd())
	if m.launchForm.stage != "uncertain" || m.launchForm.receipt.Token == "" {
		t.Fatal("uncertain outcome lost")
	}
	for _, key := range []tea.KeyPressMsg{{Code: 'v', Text: "v"}, {Code: 's', Mod: tea.ModCtrl}, {Code: tea.KeyEnter}} {
		_, cmd = m.Update(key)
		if cmd != nil {
			t.Fatal("automatically retried")
		}
	}
	if r.submissions != 1 {
		t.Fatal(r.submissions)
	}
}
func TestLaunchEditingInvalidatesValidationAndLateReply(t *testing.T) {
	m, r := readyLaunch(t)
	press(m, 'e')
	if m.launchForm.stage != "edit" || !m.launchForm.check.At.IsZero() {
		t.Fatal("edit retained validation")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(cmd())
	if m.view != "details" || m.launchForm.stage == "ready" {
		t.Fatal("cancelled check accepted")
	}
	command(m, "/run")
	m.launchForm.stage = "ready"
	m.launchForm.check.At = time.Now().Add(-6 * time.Minute)
	if cmd = m.launchOperation(true); cmd != nil || r.submissions != 0 {
		t.Fatal("expired validation submitted")
	}
}
func TestLaunchRendersAndFieldsRemainUsableAtCompactSizes(t *testing.T) {
	for _, size := range [][2]int{{160, 44}, {100, 32}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		command(m, "/run")
		f := m.launchForm
		for i := range f.fields {
			f.focus = i
			view := ansi.Strip(m.View().Content)
			if !strings.Contains(view, f.fields[i].label) {
				t.Fatalf("field %s missing at %v", f.fields[i].label, size)
			}
			if !strings.Contains(view, f.fields[i].hint) {
				t.Fatalf("focused hint for %s missing at %v", f.fields[i].label, size)
			}
			lines := strings.Split(view, "\n")
			if len(lines) > size[1] {
				t.Fatal("height overflow")
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("width overflow")
				}
			}
		}
	}
}

func TestLaunchMouseNavigationPreservesDraft(t *testing.T) {
	m := demoModel(120, 40)
	command(m, "/run")
	m.launchForm.fields[0].input.SetValue("/scratch/keep-me")
	x, y := renderedTarget(t, m, "Shell command")
	click(m, x, y)
	if m.launchForm.focus != 1 {
		t.Fatal("field click did not focus")
	}
	wheel(m, x, y, tea.MouseWheelDown)
	if m.launchForm.focus != 2 {
		t.Fatal("wheel did not move to next field")
	}
	x, y = renderedTarget(t, m, "2  History")
	click(m, x, y)
	if m.tab != 1 || m.view == "launch" {
		t.Fatal("tab click did not close form")
	}
	command(m, "/run")
	if m.launchForm.fields[0].input.Value() != "/scratch/keep-me" {
		t.Fatal("draft lost")
	}
	m.launchForm.stage = "submitting"
	x, y = renderedTarget(t, m, "2  History")
	click(m, x, y)
	if m.view != "launch" {
		t.Fatal("in-flight receipt hidden")
	}
}

func TestArrayPlanAndParentReceiptFocus(t *testing.T) {
	m := demoModel(120, 40)
	command(m, "/run")
	f := m.launchForm
	f.fields[0].input.SetValue("/scratch/example")
	f.fields[1].input.SetValue("python3 train.py --seed $SLURM_ARRAY_TASK_ID")
	f.fields[9].input.SetValue("0-9%2")
	f.fields[10].input.SetValue("123")
	m.launchReview()
	if f.stage != "review" || f.plan.Array.Tasks != 10 || f.plan.Spec.AfterOK != "123" {
		t.Fatal(f.stage, f.problem, f.plan)
	}
	if !strings.Contains(m.launchContent(110, 25), "10 array tasks") {
		t.Fatal("array summary absent")
	}
	m.view = "details"
	m.pendingLaunchFocus = "999"
	m.store.Snapshot.Jobs = nil
	task := int64(2)
	m.Update(refreshMsg{result: hummingbird.Refresh{QueueOK: true, At: time.Now(), Queue: []domain.Job{{ID: "999_2", ArrayID: 999, TaskID: &task, State: "RUNNING"}}}})
	if m.selectedID != "999_2" || m.pendingLaunchFocus != "" || m.view != "inspect" {
		t.Fatal(m.selectedID, m.pendingLaunchFocus, m.view)
	}
	m.pendingLaunchFocus = "1000"
	m.action("3")
	if m.pendingLaunchFocus != "" {
		t.Fatal("stale launch focus overrides navigation")
	}
}

func TestTopologyFormValidationAndReview(t *testing.T) {
	m := demoModel(120, 40)
	command(m, "/run")
	f := m.launchForm
	f.fields[0].input.SetValue("/scratch/example")
	f.fields[1].input.SetValue("./solver")
	for _, index := range []int{11, 12} {
		for _, invalid := range []string{"0", "-1", "two", ""} {
			f.fields[index].input.SetValue(invalid)
			m.launchReview()
			if f.stage != "edit" || f.problem == "" {
				t.Fatal("invalid topology reviewed", index, invalid)
			}
		}
		f.fields[index].input.SetValue("1")
	}
	f.fields[11].input.SetValue("2")
	f.fields[12].input.SetValue("3")
	f.fields[13].input.SetValue("direct")
	m.launchReview()
	if f.stage != "edit" || !strings.Contains(f.problem, "one node and one task") {
		t.Fatal("direct launch accepted multiple tasks", f.problem)
	}
	f.fields[13].input.SetValue("mpirun")
	f.fields[8].input.SetValue("gnu15/15.2.0 openmpi5/5.0.10")
	m.launchReview()
	if f.stage != "review" || f.plan.Spec.Nodes != 2 || f.plan.Spec.TasksPerNode != 3 || f.plan.Spec.Launcher != "mpirun" || !strings.Contains(f.plan.Script, "ppr:3:node:PE=1") {
		t.Fatal(f.stage, f.problem, f.plan)
	}
	if !strings.Contains(m.launchContent(110, 30), "6 tasks (3/node)") {
		t.Fatal("topology absent from review")
	}
}

func TestLaunchValidationProblemVisibleAtMinimumSize(t *testing.T) {
	m := demoModel(48, 18)
	command(m, "/run")
	m.launchReview()
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "enter the command to run") {
		t.Fatal("form error clipped outside compact panel", view)
	}
}

func TestLaunchValidationFocusesCorrectionAndPreservesDraft(t *testing.T) {
	cases := []struct{ field, value string }{
		{"workdir", "relative/path"}, {"command", ""}, {"name", "bad name"},
		{"cpus", "many"}, {"cpus", "0"}, {"memory", "4GB"},
		{"time", "30s"}, {"time", "9000h"}, {"partition", "a,b"},
		{"gpus", "lots"}, {"gpus", "65"}, {"modules", "bad;module"},
		{"array", "5-1"}, {"afterok", "not-a-job"},
		{"nodes", "0"}, {"nodes", "1025"}, {"tasks", "zero"},
		{"tasks", "4097"}, {"launcher", "unknown"},
	}
	for _, size := range [][2]int{{48, 18}, {80, 24}, {140, 40}} {
		for _, tc := range cases {
			m := demoModel(size[0], size[1])
			command(m, "/run")
			f := m.launchForm
			f.fields[0].input.SetValue("/scratch/preserve-draft")
			f.fields[1].input.SetValue("python3 train.py")
			index := -1
			for i := range f.fields {
				if f.fields[i].key == tc.field {
					index = i
					f.fields[i].input.SetValue(tc.value)
				}
			}
			before := make([]string, len(f.fields))
			for i := range f.fields {
				before[i] = f.fields[i].input.Value()
			}
			m.launchReview()
			if f.stage != "edit" || f.problem == "" || f.focus != index || !f.fields[index].input.Focused() {
				t.Fatalf("%s=%q: focus=%d problem=%q", tc.field, tc.value, f.focus, f.problem)
			}
			for i := range f.fields {
				if f.fields[i].input.Value() != before[i] || (i != index && f.fields[i].input.Focused()) {
					t.Fatal("validation changed a draft or left two focused fields")
				}
			}
			view := ansi.Strip(m.View().Content)
			if !strings.Contains(view, "› "+f.fields[index].label) || !strings.Contains(view, "Correct this field") || !strings.Contains(view, "esc close") {
				t.Fatal("correction or navigation is clipped", view)
			}
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("error overflow", line)
				}
			}
			// A field click uses the same geometry as the error display.
			p := tea.Mouse{X: 5, Y: m.layout().bodyTop + 4, Button: tea.MouseLeft}
			if target := m.mouseTarget(p); target.kind != "field" || target.index != index {
				t.Fatal("correction field is not clickable", target)
			}
			m.launchKey(tea.KeyPressMsg{Code: tea.KeyTab})
			if f.problem != "" || f.focus != (index+1)%len(f.fields) {
				t.Fatal("next field retained another field's error")
			}
		}
	}
}
