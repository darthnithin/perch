package ui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/launch"
	"perch/internal/transport"
)

type launchField struct {
	key, label, hint string
	input            textinput.Model
}
type launchForm struct {
	fields   []launchField
	focus    int
	stage    string
	problem  string
	plan     launch.Plan
	check    launch.Check
	receipt  launch.Receipt
	preview  viewport.Model
	sequence int
	cancel   context.CancelFunc
}
type launchMsg struct {
	sequence int
	submit   bool
	check    launch.Check
	receipt  launch.Receipt
	err      error
}

func (m *Model) openLaunch() tea.Cmd {
	m.input.Blur()
	m.help, m.palette, m.searching = false, false, false
	m.invalidateDetail()
	m.view = "launch"
	if m.launchForm == nil {
		f := &launchForm{stage: "edit", preview: viewport.New()}
		f.preview.SoftWrap = true
		for _, v := range []struct{ key, label, hint, value string }{
			{"workdir", "Cluster directory", "/scratch/your-id/project", ""},
			{"command", "Shell command", "Interpreted by Bash on the compute node", ""},
			{"name", "Job name", "Short name for your queue", "perch-job"},
			{"cpus", "CPUs per task", "Threads available to each task", "1"},
			{"memory", "Memory per node", "Shared by tasks on each node: 512M, 4G", "1G"},
			{"time", "Time limit", "Whole minutes: 10m, 2h30m", "10m"},
			{"partition", "Partition", "Blank uses the scheduler default", ""},
			{"gpus", "GPUs per node", "GPUs shared per node; needs GPU partition", "0"},
			{"modules", "Modules", "Space-separated names, prerequisite first", ""},
			{"array", "Array tasks", "Optional: 0-99%4 = 100 tasks, 4 at once", ""},
			{"afterok", "After success", "IDs: 123,456 · all must succeed", ""},
			{"nodes", "Nodes", "Nodes per allocation, including array tasks", "1"},
			{"tasks", "Tasks per node", "Runs once per task; CPUs apply to each task", "1"},
			{"launcher", "Launcher", "auto / direct / srun / mpirun (Open MPI)", "auto"},
		} {
			input := textinput.New()
			input.Prompt = "  "
			input.Placeholder = v.hint
			input.SetValue(v.value)
			input.CharLimit = 4096
			input.SetVirtualCursor(true)
			input.SetStyles(m.input.Styles())
			f.fields = append(f.fields, launchField{key: v.key, label: v.label, hint: v.hint, input: input})
		}
		m.launchForm = f
	}
	f := m.launchForm
	if f.stage == "edit" {
		return f.fields[f.focus].input.Focus()
	}
	return nil
}
func (m *Model) launchPlan() (launch.Plan, error) {
	f := m.launchForm
	value := func(i int) string { return strings.TrimSpace(f.fields[i].input.Value()) }
	if value(1) == "" {
		return launch.Plan{}, &launch.FieldError{Field: "command", Err: fmt.Errorf("enter the command to run")}
	}
	cpus, e := strconv.Atoi(value(3))
	if e != nil {
		return launch.Plan{}, &launch.FieldError{Field: "cpus", Err: fmt.Errorf("CPUs must be a whole number")}
	}
	memory, e := launch.MemoryMiB(value(4))
	if e != nil {
		return launch.Plan{}, &launch.FieldError{Field: "memory", Err: e}
	}
	minutes, e := launch.Minutes(value(5))
	if e != nil {
		return launch.Plan{}, &launch.FieldError{Field: "time", Err: e}
	}
	gpus, e := strconv.Atoi(value(7))
	if e != nil {
		return launch.Plan{}, &launch.FieldError{Field: "gpus", Err: fmt.Errorf("GPUs must be a whole number")}
	}
	nodes, e := strconv.Atoi(value(11))
	if e != nil || nodes < 1 {
		return launch.Plan{}, &launch.FieldError{Field: "nodes", Err: fmt.Errorf("nodes must be a positive whole number")}
	}
	tasks, e := strconv.Atoi(value(12))
	if e != nil || tasks < 1 {
		return launch.Plan{}, &launch.FieldError{Field: "tasks", Err: fmt.Errorf("tasks per node must be a positive whole number")}
	}
	return launch.Build(launch.Spec{Name: value(2), WorkDir: value(0), Nodes: nodes, TasksPerNode: tasks, Launcher: value(13), CPUs: cpus, MemoryMiB: memory, Minutes: minutes, Partition: value(6), GPUs: gpus, Modules: strings.Fields(value(8)), Array: value(9), AfterOK: value(10), Command: []string{"/bin/bash", "-c", value(1)}})
}
func (m *Model) launchReview() tea.Cmd {
	p, e := m.launchPlan()
	f := m.launchForm
	if e != nil {
		f.problem = e.Error()
		var invalid *launch.FieldError
		if errors.As(e, &invalid) {
			for i := range f.fields {
				if f.fields[i].key == invalid.Field {
					f.fields[f.focus].input.Blur()
					f.focus = i
					return f.fields[i].input.Focus()
				}
			}
		}
		return nil
	}
	f.fields[f.focus].input.Blur()
	f.plan = p
	f.stage = "review"
	f.problem = ""
	f.check = launch.Check{}
	f.preview.GotoTop()
	return nil
}
func (m *Model) launchOperation(submit bool) tea.Cmd {
	f := m.launchForm
	if f.stage != "review" && f.stage != "ready" {
		return nil
	}
	if submit && (f.stage != "ready" || time.Since(f.check.At) > 5*time.Minute) {
		f.problem = "Validate this request before submitting."
		f.stage = "review"
		f.check = launch.Check{}
		return nil
	}
	m.launchSequence++
	f.sequence = m.launchSequence
	seq := f.sequence
	p := f.plan
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	f.cancel = cancel
	f.problem = ""
	if !submit {
		f.check = launch.Check{}
	}
	f.stage = "checking"
	if submit {
		f.stage = "submitting"
	}
	demo := m.options.Demo
	journal, destination := m.options.Journal, m.options.Host
	var runner launch.Runner
	if m.client != nil {
		runner, _ = m.client.Runner.(launch.Runner)
	}
	return func() tea.Msg {
		defer cancel()
		result := launchMsg{sequence: seq, submit: submit}
		if demo {
			if submit {
				result.receipt = launch.Receipt{JobID: "999001", At: time.Now(), Token: "demo-only"}
			} else {
				result.check = launch.Check{At: time.Now(), Message: "Demo validation. No cluster was contacted."}
			}
			return result
		}
		if runner == nil {
			result.err = fmt.Errorf("no launch-capable connection; return to Connection and reconnect")
			return result
		}
		client := launch.Client{Runner: runner, Journal: journal, Destination: destination}
		if submit {
			result.receipt, result.err = client.Submit(ctx, p)
		} else {
			result.check, result.err = client.Check(ctx, p)
		}
		return result
	}
}
func (m *Model) receiveLaunch(msg launchMsg) tea.Cmd {
	f := m.launchForm
	if f == nil || msg.sequence != f.sequence {
		return nil
	}
	f.cancel = nil
	if msg.err != nil {
		f.problem = domain.Clean(msg.err.Error())
		f.stage = "review"
		if msg.submit && msg.receipt.Token != "" {
			f.stage = "uncertain"
			f.receipt = msg.receipt
			if msg.receipt.JobID != "" {
				f.stage = "submitted"
			}
		}
		return nil
	}
	if msg.submit {
		f.stage = "submitted"
		f.receipt = msg.receipt
		if !m.options.Demo {
			m.nextRefresh = m.now
			return m.refresh()
		}
	} else {
		f.stage = "ready"
		f.check = msg.check
	}
	return nil
}
func (m *Model) launchKey(msg tea.KeyPressMsg) tea.Cmd {
	f := m.launchForm
	key := msg.String()
	if key == "esc" {
		if f.stage == "submitting" {
			f.problem = "Waiting for submission outcome. Leaving now could hide an accepted job."
			return nil
		}
		if f.cancel != nil {
			f.cancel()
			m.launchSequence++
			f.sequence = m.launchSequence
			f.stage = "review"
		}
		m.view = "details"
		return nil
	}
	if f.stage == "edit" {
		switch key {
		case "tab", "shift+tab":
			f.problem = ""
			f.fields[f.focus].input.Blur()
			delta := 1
			if key == "shift+tab" {
				delta = -1
			}
			f.focus = (f.focus + delta + len(f.fields)) % len(f.fields)
			return f.fields[f.focus].input.Focus()
		case "enter":
			return m.launchReview()
		}
		var cmd tea.Cmd
		f.fields[f.focus].input, cmd = f.fields[f.focus].input.Update(msg)
		f.problem = ""
		return cmd
	}
	if key == "up" || key == "down" || key == "pgup" || key == "pgdown" || key == "home" || key == "end" {
		var cmd tea.Cmd
		f.preview, cmd = f.preview.Update(msg)
		return cmd
	}
	if f.stage == "checking" || f.stage == "submitting" {
		return nil
	}
	switch key {
	case "e":
		if f.stage == "review" || f.stage == "ready" {
			f.stage = "edit"
			f.check = launch.Check{}
			f.problem = ""
			return f.fields[f.focus].input.Focus()
		}
	case "v":
		return m.launchOperation(false)
	case "ctrl+s":
		return m.launchOperation(true)
	case "enter":
		if f.stage == "submitted" {
			m.view = "details"
			m.tab = 0
			m.search = ""
			if !m.options.Demo {
				m.pendingLaunchFocus = f.receipt.JobID
				m.nextRefresh = m.now
				return m.refresh()
			}
		}
	case "n":
		if f.stage == "submitted" || f.stage == "uncertain" {
			m.launchForm = nil
			return m.openLaunch()
		}
	}
	return nil
}
func (m *Model) launchEditingProblem() bool {
	return m.view == "launch" && m.launchForm != nil && m.launchForm.stage == "edit" && m.launchForm.problem != ""
}
func (m *Model) launchVisible() (int, int) {
	f := m.launchForm
	if m.launchEditingProblem() {
		return f.focus, 1
	}
	visible := max(1, (m.layout().bodyHeight-6)/3)
	return max(0, f.focus-visible+1), visible
}
func (m *Model) launchContent(width, height int) string {
	f := m.launchForm
	if f.stage == "edit" {
		start, visible := m.launchVisible()
		rows := []string{ink.Bold(true).Render("Prepare a job") + muted.Render(fmt.Sprintf(" · %d/%d", f.focus+1, len(f.fields))), ""}
		for i := start; i < min(len(f.fields), start+visible); i++ {
			field := &f.fields[i]
			field.input.SetWidth(max(10, width-3))
			label := muted.Render(field.label)
			if i == f.focus {
				label = accent.Bold(true).Render("› " + field.label)
			}
			rows = append(rows, label, field.input.View(), "")
		}
		if f.problem != "" {
			rows = append(rows, yellow.Render(ansi.Wrap(domain.Label(f.problem), width, " ")))
		}
		return strings.Join(rows, "\n")
	}
	status := "Review your job"
	switch f.stage {
	case "checking":
		status = "Checking with Slurm…"
	case "ready":
		status = "Validated · ready to submit"
	case "submitting":
		status = "Submitting once…"
	case "submitted":
		status = "Submitted · job " + f.receipt.JobID
		if m.options.Demo {
			status = "Demo complete · no job submitted"
		}
	case "uncertain":
		status = "Submission outcome unconfirmed"
	}
	p := f.plan
	partition := p.Spec.Partition
	if partition == "" {
		partition = "scheduler default"
	}
	identity := domain.Label(m.options.Host)
	if m.options.Demo {
		identity = "Demo workspace · no cluster execution"
	}
	dependency := ""
	if p.Spec.AfterOK != "" {
		dependency = "\n\n" + ink.Bold(true).Render("After success: "+p.Spec.AfterOK) + "\n" + muted.Render("Cancels if a dependency cannot succeed.")
	}
	body := fmt.Sprintf("%s\n%s\n\n%s\n%s\nPartition: %s%s\n\n%s\n%s\n\n%s\n%s", ink.Bold(true).Render(p.Spec.Name), identity, muted.Render(p.Spec.WorkDir), p.ResourceSummary(), partition, dependency, muted.Render("Output: "+p.Stdout), muted.Render("Errors: "+p.Stderr), ink.Bold(true).Render("Exact script"), p.Script)
	body += "\n" + ink.Bold(true).Render("Slurm arguments") + "\n" + transport.QuoteArgs(p.Arguments)
	if !f.check.At.IsZero() {
		body += "\n\n" + muted.Render("Scheduler check · estimate only") + "\n" + f.check.Message
	}
	if f.problem != "" {
		body = red.Render(f.problem) + "\n\n" + body
	}
	if f.stage == "submitted" {
		body = green.Render("Accepted by Slurm. Press Enter to find it in your jobs/history.") + "\n" + muted.Render("Saved submissions are available in /receipts after closing this form.") + "\n\n" + body
		if m.options.Demo {
			body = "Demo only. Nothing was sent to Hummingbird.\n\n" + body
		}
	}
	f.preview.SetWidth(width)
	f.preview.SetHeight(max(1, height-2))
	text := ansi.Wordwrap(body, width, "")
	if text != f.preview.GetContent() {
		f.preview.SetContent(text)
	}
	return accent.Bold(true).Render(status) + "\n\n" + f.preview.View()
}
func (m *Model) launchFooter() string {
	if m.width < 64 {
		switch m.launchForm.stage {
		case "edit":
			return "tab next  enter review  esc close"
		case "review":
			return "v validate  e edit  ↑↓ scroll  esc close"
		case "ready":
			return "ctrl+s submit  e edit  ↑↓ scroll  esc close"
		case "submitted":
			return "enter jobs  n new job  esc close"
		case "checking", "submitting":
			return "Waiting for Hummingbird…"
		default:
			return "n new plan  esc close"
		}
	}
	switch m.launchForm.stage {
	case "edit":
		return "tab next · shift+tab previous · enter review · esc close"
	case "review":
		return "v validate · e edit · ↑↓ scroll · esc close"
	case "ready":
		return "ctrl+s submit job · e edit · ↑↓ scroll · esc close"
	case "checking", "submitting":
		return "Waiting for Hummingbird…"
	case "submitted":
		return "enter view jobs · n new job · esc close"
	default:
		return "Check queue before retrying · n new plan · esc close"
	}
}
func (m *Model) launchMouse(p tea.Mouse, wheel bool) tea.Cmd {
	f := m.launchForm
	l := m.layout()
	if p.Mod != 0 || p.X < 4 || p.X >= m.width-4 || p.Y <= l.bodyTop || p.Y >= l.bodyTop+l.bodyHeight-1 {
		return nil
	}
	if wheel {
		if f.stage == "edit" {
			delta := 0
			if p.Button == tea.MouseWheelUp {
				delta = -1
			}
			if p.Button == tea.MouseWheelDown {
				delta = 1
			}
			if delta == 0 {
				return nil
			}
			f.fields[f.focus].input.Blur()
			f.focus = max(0, min(len(f.fields)-1, f.focus+delta))
			f.problem = ""
			return f.fields[f.focus].input.Focus()
		}
		if f.stage != "edit" {
			if p.Button == tea.MouseWheelUp {
				f.preview.ScrollUp(3)
			}
			if p.Button == tea.MouseWheelDown {
				f.preview.ScrollDown(3)
			}
		}
		return nil
	}
	if p.Button != tea.MouseLeft || f.stage != "edit" {
		return nil
	}
	start, visible := m.launchVisible()
	row := p.Y - l.bodyTop - 3
	if row < 0 || row >= visible*3 || row%3 == 2 {
		return nil
	}
	index := start + row/3
	if index >= len(f.fields) {
		return nil
	}
	f.fields[f.focus].input.Blur()
	f.focus = index
	return f.fields[index].input.Focus()
}
