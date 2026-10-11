package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/allocshell"
	"perch/internal/domain"
)

type shellPreview struct {
	job            domain.Job
	plan           allocshell.Plan
	previousView   string
	previousPanel  int
	stage, problem string
	sequence       int
	cancel         context.CancelFunc
}
type shellPreparedMsg struct {
	sequence int
	open     bool
	plan     allocshell.Plan
	command  *exec.Cmd
	err      error
}
type shellReturnedMsg struct {
	sequence int
	err      error
}

func (m *Model) shellAvailable() bool {
	j, ok := m.selected()
	return ok && m.tab < 2 && j.State == "RUNNING" && j.TaskRange == "" && j.AwaitingTasks == 0 && (m.options.Demo || m.client != nil && m.connected && m.cachedDetailAllowed(j))
}

func (m *Model) openShellPreview() tea.Cmd {
	if !m.shellAvailable() {
		m.setNotice("Choose one running job or array task from a connected, refreshed queue.")
		return nil
	}
	j, _ := m.selected()
	m.cancelRequests()
	m.generation++
	m.detailSequence++
	m.busy, m.detailBusy = false, false
	m.input.Blur()
	m.help, m.palette, m.searching = false, false, false
	m.shellForm = &shellPreview{job: j, previousView: m.view, previousPanel: m.panel.YOffset()}
	m.view = "shell"
	m.panel.GotoTop()
	return m.prepareShell(false, "")
}

func (m *Model) prepareShell(open bool, node string) tea.Cmd {
	f := m.shellForm
	if f == nil {
		return nil
	}
	if f.cancel != nil {
		f.cancel()
	}
	m.shellSequence++
	f.sequence = m.shellSequence
	f.stage, f.problem = "checking", ""
	if open {
		f.stage = "opening"
	}
	sequence, job, plan := f.sequence, f.job, f.plan
	options := allocshell.Options{Node: node, CPUs: 1, MemoryMiB: 64, Minutes: 10}
	if m.options.Demo {
		return func() tea.Msg {
			return shellPreparedMsg{sequence: sequence, open: open, plan: allocshell.Plan{JobID: job.ID, AllocationID: job.RawID, Attempt: job.AttemptKey(), Started: job.Start, Name: job.Name, Node: job.Nodes, Nodes: []string{job.Nodes}, WorkDir: job.WorkDir, Options: options}}
		}
	}
	client := *m.client // Identify may update User; do not race the refresh client.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	f.cancel = cancel
	return func() tea.Msg {
		defer cancel()
		msg := shellPreparedMsg{sequence: sequence, open: open}
		if open {
			msg.plan, msg.err = allocshell.Revalidate(ctx, &client, plan)
		} else {
			msg.plan, msg.err = allocshell.Prepare(ctx, &client, job.ID, options)
		}
		if msg.err == nil && (msg.plan.Attempt != job.AttemptKey() || !msg.plan.Started.Equal(job.Start)) {
			msg.err = fmt.Errorf("This job has changed since you selected it. Return to Jobs and select its current attempt.")
		}
		if open && msg.err == nil {
			runner, ok := client.Runner.(interface {
				InteractiveCommand([]string) (*exec.Cmd, error)
			})
			if !ok {
				msg.err = fmt.Errorf("This connection cannot open a foreground shell.")
			} else {
				msg.command, msg.err = runner.InteractiveCommand(msg.plan.Command())
			}
		}
		return msg
	}
}

func (m *Model) receiveShell(msg shellPreparedMsg) tea.Cmd {
	f := m.shellForm
	if m.view != "shell" || f == nil || msg.sequence != f.sequence {
		return nil
	}
	f.cancel = nil
	if msg.err != nil {
		f.stage, f.problem = "error", domain.Label(msg.err.Error())
		m.panel.GotoTop()
		return nil
	}
	f.plan, f.stage = msg.plan, "ready"
	if !msg.open {
		return nil
	}
	if m.options.Demo {
		m.setNotice("Demo preview only · no shell was opened.")
		return m.closeShellPreview()
	}
	if msg.command == nil {
		f.stage, f.problem = "error", "No shell command was prepared."
		return nil
	}
	m.hoverKnown = false
	m.pointerSuspended = true
	f.stage = "active"
	sequence := f.sequence
	return tea.Exec(&pointerExec{command: msg.command, reset: m.PointerReset(), limit: time.Duration(f.plan.Options.Minutes)*time.Minute + 15*time.Second}, func(err error) tea.Msg { return shellReturnedMsg{sequence: sequence, err: err} })
}

func (m *Model) closeShellPreview() tea.Cmd {
	f := m.shellForm
	if f == nil {
		return nil
	}
	if f.cancel != nil {
		f.cancel()
	}
	m.shellSequence++
	m.shellForm = nil
	m.view = f.previousView
	m.syncPanel()
	m.panel.SetYOffset(f.previousPanel)
	m.hoverKnown, m.pointerSuspended = false, false
	m.lastPointer = ""
	return tea.Batch(m.refresh(), m.fetchDetail())
}

func (m *Model) shellKey(key tea.KeyPressMsg) tea.Cmd {
	f := m.shellForm
	switch key.String() {
	case "esc":
		return m.closeShellPreview()
	case "enter":
		if f.stage == "ready" && m.width >= 48 && m.height >= 18 {
			return m.prepareShell(true, f.plan.Node)
		}
	case "r":
		if f.stage == "error" || f.stage == "ready" {
			return m.prepareShell(false, f.plan.Node)
		}
	case "n":
		if f.stage == "ready" && len(f.plan.Nodes) > 1 {
			for i, node := range f.plan.Nodes {
				if node == f.plan.Node {
					return m.prepareShell(false, f.plan.Nodes[(i+1)%len(f.plan.Nodes)])
				}
			}
		}
	case "c":
		m.closeShellPreview()
		return m.action("c")
	case "q":
		return m.action("q")
	case "1", "2", "3", "4", "tab", "shift+tab":
		cmd := m.closeShellPreview()
		return tea.Batch(cmd, m.action(key.String()))
	case "home":
		m.panel.GotoTop()
	case "end":
		m.panel.GotoBottom()
	default:
		var cmd tea.Cmd
		m.panel, cmd = m.panel.Update(key)
		return cmd
	}
	return nil
}

func (m *Model) shellContent() string {
	f := m.shellForm
	if f == nil {
		return ""
	}
	if f.stage == "checking" {
		return accent.Render("Checking your running allocation…") + "\n\n" + muted.Render("Nothing has been opened.")
	}
	if f.stage == "opening" {
		return accent.Render("Checking again before opening…") + "\n\n" + muted.Render("Esc cancels until the terminal handoff.")
	}
	if f.stage == "error" {
		return red.Render("Shell not opened") + "\n\n" + f.problem + "\n\n" + muted.Render("Retry the check or return to your job.")
	}
	p := f.plan
	if m.layout().bodyHeight < 14 {
		return strings.Join([]string{
			fit(ink.Bold(true).Render("#"+p.JobID+" · "+p.Name), m.width-8),
			"Node  " + accent.Render(p.Node),
			"1 CPU · 64 MiB · no GPUs requested",
			"10m limit · parent may end sooner", "",
			"Shares resources with this running job.", "",
			"Directory", p.WorkDir, "",
			"Uses your login environment. Batch-script variables and modules are not copied.", "",
			"Exit or Ctrl-D returns to Perch. The parent job is not cancelled.",
		}, "\n")
	}
	lines := []string{ink.Bold(true).Render(p.Name), muted.Render("#"+p.JobID) + "  " + green.Render("running"), "",
		"Node       " + accent.Render(p.Node), "CPUs       1 shared", "Memory     64 MiB shared", "GPUs       None requested", "Time limit 10m · parent may end sooner", "",
		"Directory", ink.Render(p.WorkDir), "",
		accent.Render("A new shell alongside your workload"),
		ansi.Wrap("Shares this allocation’s resources. Uses your login environment; variables and modules from the batch script are not copied.", min(68, m.width-8), " "), "",
		ansi.Wrap("Exit or Ctrl-D returns to Perch. The parent job is not cancelled.", min(68, m.width-8), " ")}
	if len(p.Nodes) > 1 {
		lines = append(lines, "", muted.Render(fmt.Sprintf("%d allocated nodes · n chooses the next", len(p.Nodes))))
	}
	if m.options.Demo {
		lines = append([]string{yellow.Render("Demo · preview only"), ""}, lines...)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) shellFooter() string {
	f := m.shellForm
	if f == nil {
		return "esc back"
	}
	if f.stage == "ready" {
		if len(f.plan.Nodes) > 1 {
			return "enter open shell  n node  esc back"
		}
		return "enter open shell  ↑↓ scroll  esc back"
	}
	if f.stage == "error" {
		return "r retry  c reconnect  ↑↓ scroll  esc back"
	}
	return "esc cancel"
}
