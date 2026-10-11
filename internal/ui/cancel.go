package ui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/cancellation"
	"perch/internal/domain"
)

type cancelReview struct {
	job                                       domain.Job
	target                                    cancellation.Target
	plan                                      *cancellation.Plan
	preview                                   cancellation.Preview
	receipt                                   *cancellation.Receipt
	stage, problem, previousView              string
	returnAttempt                             string
	previousPanel, previousViewport, sequence int
	cancel                                    context.CancelFunc
}
type cancelPreparedMsg struct {
	sequence int
	plan     *cancellation.Plan
	preview  cancellation.Preview
	err      error
}
type cancelSentMsg struct {
	sequence int
	receipt  *cancellation.Receipt
	err      error
}

func (m *Model) cancellationProblem(array bool) string {
	j, ok := m.selected()
	if m.tab != 0 || !ok {
		return "Choose a current job in Jobs."
	}
	if j.AwaitingTasks > 0 || !slices.Contains([]string{"PENDING", "RUNNING", "SUSPENDED"}, j.State) {
		return "Choose a pending, running or suspended job."
	}
	if array && j.ArrayID == 0 {
		return "Choose a task or range from an array."
	}
	if !array && j.ArrayID > 0 && (j.TaskID == nil || j.TaskRange != "") {
		return "Choose one array task, or use /cancel-array."
	}
	if !m.options.Demo && (m.client == nil || !m.connected || m.cachedStore != nil) {
		return "Reconnect and refresh the queue first."
	}
	return ""
}

func (m *Model) openCancellation(array bool) tea.Cmd {
	if problem := m.cancellationProblem(array); problem != "" {
		m.setNotice(problem)
		return nil
	}
	j, _ := m.selected()
	target := cancellation.Target{ID: j.ID, Array: array}
	if array {
		target.ID = strconv.FormatInt(j.ArrayID, 10)
	}
	if err := cancellation.Validate(target); err != nil {
		m.setNotice(err.Error())
		return nil
	}
	m.beginCancellationView()
	m.cancelForm.job, m.cancelForm.target = j, target
	return m.prepareCancellation()
}

func (m *Model) beginCancellationView() {
	m.cancelRequests()
	m.generation++
	m.detailSequence++
	m.cancelSequence++
	m.busy, m.detailBusy = false, false
	m.input.Blur()
	m.help, m.palette, m.searching = false, false, false
	m.pendingLaunchFocus = ""
	m.cancelForm = &cancelReview{previousView: m.view, previousPanel: m.panel.YOffset(), previousViewport: m.viewport.YOffset(), sequence: m.cancelSequence}
	if j, ok := m.selected(); ok {
		m.cancelForm.returnAttempt = j.AttemptKey()
	}
	m.view = "cancel"
	m.panel.GotoTop()
}

func (m *Model) prepareCancellation() tea.Cmd {
	f := m.cancelForm
	if f.cancel != nil {
		f.cancel()
	}
	m.cancelSequence++
	f.sequence = m.cancelSequence
	f.stage, f.problem = "checking", ""
	m.panel.GotoTop()
	sequence, target, job := f.sequence, f.target, f.job
	if m.options.Demo {
		v := cancellation.Preview{Target: target, Host: "demo", User: job.User, Cluster: job.Cluster, Name: job.Name, Tasks: 1, Scope: "job", At: m.now, Expires: m.now.Add(cancellation.PreviewLifetime)}
		v.States = []string{"1 " + job.State}
		if job.ArrayID > 0 {
			v.Scope = "task"
		}
		if target.Array {
			v.Scope = "array"
			v.Tasks = 0
			counts := map[string]int64{}
			for _, j := range m.store.Snapshot.Jobs {
				if j.ArrayID == job.ArrayID && slices.Contains([]string{"RUNNING", "PENDING", "SUSPENDED"}, j.State) {
					v.Tasks += j.Count()
					counts[j.State] += j.Count()
				}
			}
			v.States = nil
			for _, s := range []string{"RUNNING", "PENDING", "SUSPENDED"} {
				if counts[s] > 0 {
					v.States = append(v.States, fmt.Sprintf("%d %s", counts[s], s))
				}
			}
		}
		return func() tea.Msg { return cancelPreparedMsg{sequence: sequence, preview: v} }
	}
	client := *m.client
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	f.cancel = cancel
	return func() tea.Msg {
		defer cancel()
		p, err := cancellation.Prepare(ctx, &client, target)
		msg := cancelPreparedMsg{sequence: sequence, plan: p, err: err}
		if err == nil {
			msg.preview = p.View()
			if !p.MatchesSelection(job) {
				msg.err = fmt.Errorf("This selection belongs to an older job attempt. Return to Jobs and refresh.")
			}
		}
		return msg
	}
}

func (m *Model) receiveCancellationPreview(msg cancelPreparedMsg) tea.Cmd {
	f := m.cancelForm
	if m.view != "cancel" || f == nil || msg.sequence != f.sequence || f.stage != "checking" {
		return nil
	}
	f.cancel = nil
	if msg.err != nil {
		f.stage, f.problem = "error", domain.Label(msg.err.Error())
		return nil
	}
	f.plan, f.preview, f.stage = msg.plan, msg.preview, "ready"
	return nil
}

func (m *Model) confirmCancellation() tea.Cmd {
	f := m.cancelForm
	if f == nil || f.stage != "ready" || m.width < 48 || m.height < 18 {
		return nil
	}
	if !m.panel.AtBottom() {
		m.setNotice("Review the remaining details before confirming.")
		return nil
	}
	f.stage = "sending"
	m.panel.GotoTop()
	sequence, plan, v := f.sequence, f.plan, f.preview
	// Retain an honest message for forced shutdown while the request is in flight.
	f.receipt = &cancellation.Receipt{Status: "unknown", Target: v.Target, Host: v.Host, User: v.User, Cluster: v.Cluster, At: time.Now(), Detail: "Outcome unknown. Inspect queue and history before considering another request."}
	if m.options.Demo {
		return func() tea.Msg {
			return cancelSentMsg{sequence: sequence, receipt: &cancellation.Receipt{Status: "demo", Target: v.Target, Host: "demo", Detail: "Demo only. No cancellation was sent and no job state was changed."}}
		}
	}
	client := *m.client
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	f.cancel = cancel
	return func() tea.Msg {
		defer cancel()
		r, err := cancellation.Execute(ctx, &client, plan)
		return cancelSentMsg{sequence: sequence, receipt: r, err: err}
	}
}

func (m *Model) receiveCancellationReceipt(msg cancelSentMsg) tea.Cmd {
	f := m.cancelForm
	if m.view != "cancel" || f == nil || msg.sequence != f.sequence || f.stage != "sending" {
		return nil
	}
	f.cancel = nil
	f.receipt = msg.receipt
	if msg.receipt == nil {
		f.stage = "error"
		f.problem = "No cancellation request was sent."
		if msg.err != nil {
			f.problem += " " + domain.Label(msg.err.Error())
		}
		return nil
	}
	f.stage = "receipt"
	m.lastCancellation = msg.receipt
	m.panel.GotoTop()
	return m.refresh()
}

func (m *Model) showCancellationReceipt() tea.Cmd {
	if m.lastCancellation == nil {
		m.setNotice("No cancellation request in this session.")
		return nil
	}
	m.beginCancellationView()
	m.cancelForm.stage, m.cancelForm.receipt = "receipt", m.lastCancellation
	return nil
}

// CancellationSummary survives alternate-screen teardown, including an
// interrupted request whose actual remote outcome could not be established.
func (m *Model) CancellationSummary() string {
	r := m.lastCancellation
	if m.cancelForm != nil && m.cancelForm.stage == "sending" {
		r = m.cancelForm.receipt
	}
	if r == nil || r.Status == "demo" {
		return ""
	}
	return fmt.Sprintf("Perch cancellation · %s · %s\nHost: %s · cluster: %s\n%s\n", domain.Label(r.Target.ID), domain.Label(r.Status), domain.Label(r.Host), domain.Label(r.Cluster), domain.Label(r.Detail))
}

func (m *Model) closeCancellation() tea.Cmd {
	f := m.cancelForm
	if f == nil || f.stage == "sending" {
		return nil
	}
	if f.cancel != nil {
		f.cancel()
	}
	m.cancelSequence++
	m.cancelForm = nil
	m.view = f.previousView
	attempt := ""
	if j, ok := m.selected(); ok {
		attempt = j.AttemptKey()
	}
	if attempt != f.returnAttempt {
		m.invalidateDetail()
		m.view = "details"
		m.setNotice("Selection changed · returned to the job list")
	}
	m.syncPanel()
	if attempt == f.returnAttempt {
		m.panel.SetYOffset(f.previousPanel)
		m.viewport.SetYOffset(f.previousViewport)
	}
	m.hoverKnown = false
	return tea.Batch(m.refresh(), m.fetchDetail())
}

func (m *Model) cancellationKey(key tea.KeyPressMsg) tea.Cmd {
	f := m.cancelForm
	if f.stage == "sending" {
		return nil
	}
	switch key.String() {
	case "ctrl+x":
		return m.confirmCancellation()
	case "esc":
		return m.closeCancellation()
	case "enter":
		if f.stage == "receipt" {
			return m.closeCancellation()
		}
	case "r":
		if f.stage == "error" {
			return m.prepareCancellation()
		}
		if f.stage == "receipt" {
			return m.refresh()
		}
	case "q":
		return m.action("q")
	case "1", "2", "3", "4", "tab", "shift+tab":
		cmd := m.closeCancellation()
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

func (m *Model) cancellationContent() string {
	f := m.cancelForm
	switch f.stage {
	case "checking":
		return accent.Render("Checking the current job and scope…") + "\n\n" + muted.Render("Nothing has been cancelled. Esc returns.")
	case "error":
		return red.Render("Cancellation not sent") + "\n\n" + f.problem + "\n\n" + muted.Render("Return to Jobs to inspect the current state.")
	case "sending":
		return accent.Render("Checking again and requesting cancellation…") + "\n\n#" + f.preview.Target.ID + "\n\n" + muted.Render("Waiting for a receipt. No automatic retry.\nIf you quit, the outcome may be unknown.")
	case "receipt":
		r := f.receipt
		title := yellow.Render("Outcome unknown")
		if r.Status == "requested" {
			title = accent.Render("Cancellation requested")
		}
		if r.Status == "unconfirmed" {
			title = yellow.Render("Cancellation unconfirmed")
		}
		if r.Status == "demo" {
			title = accent.Render("Demo · no cancellation sent")
		}
		scope := "Job · #" + r.Target.ID
		if strings.Contains(r.Target.ID, "_") {
			scope = "Array task · #" + r.Target.ID
		}
		if r.Target.Array {
			scope = "Whole array · #" + r.Target.ID
		}
		detail := strings.TrimPrefix(r.Detail, "Outcome unknown. ")
		detail = strings.TrimPrefix(detail, "Cancellation requested. ")
		return title + "\n" + scope + "\n\n" + detail + "\n\n" + muted.Render("/last-cancel reopens this session’s receipt.")
	}
	v := f.preview
	scope, effect, files := "One job", "Stops this job and all its steps.", "Output files are kept."
	if v.Scope == "task" {
		scope, effect, files = "One task", "Stops this task and all its steps.", "Sibling tasks continue. Files are kept."
	}
	if v.Scope == "array" {
		scope, effect = fmt.Sprintf("Whole array · %d tasks", v.Tasks), "Stops all remaining tasks and steps."
	}
	owner := v.User + " @ " + v.Cluster
	if m.options.Demo {
		owner = "Demo only · no real cancellation"
	}
	w := m.width - 8
	if m.layout().bodyHeight < 17 {
		return strings.Join([]string{fit(ink.Bold(true).Render(v.Name), w), accent.Render("#" + v.Target.ID + " · " + scope), muted.Render(strings.Join(v.States, " · ")), owner, red.Render(effect), files, "Success dependencies may stay blocked.", muted.Render("Host · " + domain.Label(v.Host))}, "\n")
	}
	return strings.Join([]string{ink.Bold(true).Render(v.Name), accent.Render("#" + v.Target.ID), "", red.Render(scope), strings.Join(v.States, " · "), "", "Owner      " + owner, "Host       " + domain.Label(v.Host), "", red.Render(effect), files, "Success dependencies may stay blocked.", "", muted.Render("Nothing is sent until you confirm.\nIdentity and scope are checked again first.")}, "\n")
}

func (m *Model) cancellationButton() string {
	f := m.cancelForm
	if f.stage == "ready" {
		if !m.panel.AtBottom() {
			return "Review remaining details ↓"
		}
		return "Cancel " + f.preview.Scope + " · Ctrl+X"
	}
	if f.stage == "receipt" {
		return "Back to workspace ↵"
	}
	return ""
}

func (m *Model) cancellationFooter() string {
	f := m.cancelForm
	switch f.stage {
	case "ready":
		if !m.panel.AtBottom() {
			return "end review  ↑↓ scroll  esc back"
		}
		return "ctrl+x cancel " + f.preview.Scope + "  ↑↓ scroll  esc back"
	case "receipt":
		return "enter back  r refresh  ↑↓ scroll"
	case "error":
		return "r check again  ↑↓ scroll  esc back"
	case "sending":
		return "Waiting for Hummingbird…"
	default:
		return "esc back"
	}
}

func (m *Model) cancellationButtonView() string {
	label := m.cancellationButton()
	if m.cancelForm.stage == "ready" {
		return red.Bold(true).Render(label)
	}
	if label != "" {
		return accent.Bold(true).Render(label)
	}
	return muted.Render("Nothing to confirm")
}

// Button geometry matches the compact review's dedicated action row.
func (m *Model) cancellationButtonWidth() int { return ansi.StringWidth(m.cancellationButton()) }
