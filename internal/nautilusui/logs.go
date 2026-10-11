package nautilusui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/nautilus"
)

type logSource interface {
	ReadLogs(context.Context, nautilus.LogRequest) (nautilus.LogSnapshot, error)
}

type logView struct {
	metrics         bool
	metricsSnapshot *nautilus.MetricsSnapshot
	stage           string // pods, containers, output
	pods            []nautilus.Pod
	pod             nautilus.Pod
	containers      []nautilus.Container
	index           int
	podIndex        int
	fromJob         bool
	previous, busy  bool
	sequence        uint64
	cancel          context.CancelFunc
	lastRequest     time.Time
	snapshot        *nautilus.LogSnapshot
	err             string
	panel           viewport.Model
	renderedText    string
	renderedWidth   int
}
type logMsg struct {
	view     *logView
	sequence uint64
	snapshot nautilus.LogSnapshot
	err      error
}

func (m *Model) closeLogs() {
	if m.logs != nil && m.logs.cancel != nil {
		m.logs.cancel()
	}
	m.logs = nil
}
func (m *Model) cancelLogRead() {
	if l := m.logs; l != nil {
		if l.cancel != nil {
			l.cancel()
		}
		l.sequence++
		if l.busy {
			l.err = "Read interrupted; r retries this snapshot."
		}
		l.busy = false
	}
}

func (m *Model) openLogs() tea.Cmd {
	return m.openPodView(false)
}
func (m *Model) openMetrics() tea.Cmd { return m.openPodView(true) }
func (m *Model) openPodView(metrics bool) tea.Cmd {
	label := "logs"
	if metrics {
		label = "resource metrics"
	}
	if m.tab > 1 || m.pods == nil {
		m.notice = "Read the namespace Pods before choosing " + label + "."
		return nil
	}
	_, ok := m.client.(logSource)
	if metrics {
		_, ok = m.client.(metricsSource)
	}
	if !ok {
		m.notice = "Pod " + label + " are unavailable on this connection."
		return nil
	}
	rows := m.rows(m.tab)
	if len(rows) == 0 {
		m.notice = "Select a Job or Pod first."
		return nil
	}
	m.closeLogs()
	m.help, m.notice = false, ""
	l := &logView{stage: "pods", fromJob: m.tab == 0, panel: viewport.New()}
	l.metrics = metrics
	l.panel.SoftWrap = true
	if m.tab == 0 && m.jobs != nil {
		for _, j := range m.jobs.Items {
			if j.UID == m.selectedUID[0] {
				l.pods = nautilus.PodsForJob(j, m.pods.Items)
				break
			}
		}
		if len(l.pods) == 0 {
			m.notice = "No controller-owned Pods observed for this Job."
			return nil
		}
		sort.Slice(l.pods, func(i, j int) bool { return l.pods[i].Name < l.pods[j].Name })
	} else {
		for _, p := range m.pods.Items {
			if p.UID == m.selectedUID[1] {
				l.pod = p
				break
			}
		}
		if l.pod.UID == "" {
			m.notice = "Selected Pod is no longer in this snapshot."
			return nil
		}
		l.chooseContainers()
	}
	m.logs = l
	if metrics && !l.fromJob {
		l.stage = "output"
		return m.readMetrics(true)
	}
	return nil
}

func (l *logView) chooseContainers() {
	l.stage, l.index = "containers", 0
	l.containers = append([]nautilus.Container(nil), l.pod.Containers...)
	rank := map[string]int{"app": 0, "sidecar": 1, "init": 2, "ephemeral": 3}
	sort.SliceStable(l.containers, func(i, j int) bool {
		a, b := l.containers[i], l.containers[j]
		if rank[a.Role] != rank[b.Role] {
			return rank[a.Role] < rank[b.Role]
		}
		return a.Name < b.Name
	})
}
func (m *Model) activateLogChoice() tea.Cmd {
	l := m.logs
	if l.stage == "pods" && len(l.pods) > 0 {
		l.podIndex = l.index
		l.pod = l.pods[l.index]
		if l.metrics {
			l.stage = "output"
			l.panel.GotoTop()
			return m.readMetrics(true)
		}
		l.chooseContainers()
	} else if l.stage == "containers" && len(l.containers) > 0 {
		l.stage, l.previous, l.snapshot, l.err = "output", false, nil, ""
		l.panel.GotoTop()
		return m.readLogs(true)
	}
	return nil
}
func (m *Model) readLogs(force bool) tea.Cmd {
	l := m.logs
	if l != nil && l.metrics {
		return m.readMetrics(force)
	}
	if l == nil || l.stage != "output" || l.busy || m.connecting || m.ctx.Err() != nil {
		return nil
	}
	if m.authRequired {
		m.notice = "Sign in with c before reading logs again."
		l.err = m.notice
		return nil
	}
	if !force && m.now.Sub(l.lastRequest) < 2*time.Second {
		return nil
	}
	reader, ok := m.client.(logSource)
	if !ok {
		l.err = "This connection cannot read container logs."
		return nil
	}
	l.busy = true
	l.sequence++
	l.lastRequest = m.now
	sequence := l.sequence
	ctx, cancel := context.WithCancel(m.ctx)
	l.cancel = cancel
	req := nautilus.LogRequest{Pod: l.pod.Ref, Container: l.containers[l.index].Name, Previous: l.previous}
	return func() tea.Msg {
		defer cancel()
		data, err := reader.ReadLogs(ctx, req)
		return logMsg{l, sequence, data, err}
	}
}
func (m *Model) receiveLogs(msg logMsg) {
	l := m.logs
	if l == nil || l != msg.view || l.sequence != msg.sequence || !l.busy {
		return
	}
	l.busy = false
	if msg.err != nil {
		l.err = domain.Label(msg.err.Error())
		if l.err == "" {
			l.err = "Unable to read container logs."
		}
		var e *nautilus.QueryError
		if errors.As(msg.err, &e) && e.Kind == nautilus.Unauthenticated {
			m.logAuthRequired, m.authRequired = true, true
		}
		return
	}
	if msg.snapshot.Pod != l.pod.Ref || msg.snapshot.Container != l.containers[l.index].Name || msg.snapshot.Previous != l.previous {
		l.err = "Log result did not match the selected Pod and container."
		return
	}
	msg.snapshot.Text = clean(msg.snapshot.Text)
	l.snapshot, l.err = &msg.snapshot, ""
}
func (m *Model) logKey(key string, msg tea.KeyPressMsg) (bool, tea.Cmd) {
	l := m.logs
	switch key {
	case "esc":
		m.cancelLogRead()
		if l.metrics && l.stage == "output" {
			if l.fromJob {
				l.stage = "pods"
				l.index = l.podIndex
				l.metricsSnapshot = nil
				l.err = ""
			} else {
				m.closeLogs()
			}
			m.notice = ""
			return true, nil
		}
		if l.stage == "output" {
			l.stage = "containers"
			l.snapshot = nil
			l.err = ""
		} else if l.stage == "containers" && l.fromJob {
			l.stage = "pods"
			l.index = l.podIndex
		} else {
			m.closeLogs()
		}
		m.notice = ""
	case "enter":
		return true, m.activateLogChoice()
	case "r":
		return true, m.readLogs(false)
	case "p":
		if l.stage == "output" && !l.metrics {
			m.cancelLogRead()
			l.previous = !l.previous
			l.snapshot, l.err = nil, ""
			l.panel.GotoTop()
			return true, m.readLogs(true)
		}
	case "up", "down", "j", "k", "pgup", "pgdown", "home", "end":
		if l.stage == "output" {
			if key == "home" {
				l.panel.GotoTop()
			} else if key == "end" {
				l.panel.GotoBottom()
			} else {
				var cmd tea.Cmd
				l.panel, cmd = l.panel.Update(msg)
				return true, cmd
			}
		} else {
			delta := 1
			switch key {
			case "up", "k":
				delta = -1
			case "pgup":
				delta = -m.visibleRows()
			case "pgdown":
				delta = m.visibleRows()
			case "home":
				delta = -l.choiceCount()
			case "end":
				delta = l.choiceCount()
			}
			l.move(delta)
		}
	default:
		return false, nil
	}
	return true, nil
}
func (l *logView) choiceCount() int {
	if l.stage == "pods" {
		return len(l.pods)
	}
	return len(l.containers)
}
func (l *logView) move(delta int) { l.index = max(0, min(l.choiceCount()-1, l.index+delta)) }
func (m *Model) logStart() int    { return max(0, m.logs.index-m.visibleRows()+1) }
func (m *Model) syncLogs() {
	if l := m.logs; l != nil && l.stage == "output" {
		l.panel.SetWidth(max(1, m.width-8))
		l.panel.SetHeight(max(1, m.height-16))
		text := "Reading container logs…"
		if l.snapshot != nil {
			text = l.snapshot.Text
			if text == "" {
				text = "No output returned for this container."
			}
		} else if l.err != "" {
			text = "Unable to read container logs.\n\n" + l.err
		}
		if l.metrics {
			text = m.metricsContent()
			l.panel.SetHeight(max(1, m.height-14))
		}
		width := max(1, m.width-8)
		if text != l.renderedText || width != l.renderedWidth {
			l.panel.SetContent(ansi.Wrap(text, width, " "))
			l.renderedText, l.renderedWidth = text, width
		}
		l.panel.SetYOffset(l.panel.YOffset())
	}
}
func (m *Model) logBody(w, h int) (string, string) {
	l := m.logs
	if l.metrics && l.stage == "output" {
		return "Resources · " + l.pod.Name, l.panel.View()
	}
	if l.stage == "output" {
		mode := "current"
		if l.previous {
			mode = "previous"
		}
		state := "snapshot"
		style := muted
		if l.err != "" {
			style = warning
			state = "read failed"
			if l.snapshot != nil {
				state = "stale"
			}
		}
		if l.busy {
			state = "reading…"
			if l.snapshot != nil {
				state = "retained · reading…"
			}
		}
		header := accent.Render(l.containers[l.index].Name) + muted.Render(" · "+mode+" · ") + style.Render(state)
		if l.snapshot != nil && !l.snapshot.ObservedAt.IsZero() {
			header += muted.Render(" · read " + l.snapshot.ObservedAt.Local().Format("15:04:05"))
		}
		return l.pod.Name, fit(header, w-4) + "\n\n" + l.panel.View()
	}
	title := "Choose a Pod · logs"
	if l.metrics {
		title = "Choose a Pod · resources"
	}
	if l.stage == "containers" {
		title = "Containers · " + l.pod.Name
	}
	lines := []string{muted.Render("NAME · " + map[bool]string{true: "STATE", false: "ROLE / STATE"}[l.stage == "pods"]), ""}
	start := m.logStart()
	for i := start; i < min(l.choiceCount(), start+m.visibleRows()); i++ {
		label := ""
		if l.stage == "pods" {
			p := l.pods[i]
			label = p.Name + " · " + p.Phase
		} else {
			c := l.containers[i]
			label = c.Name + " · " + c.Role + " · " + c.State.Phase
		}
		pointer := "  "
		if i == l.index {
			pointer = "› "
		}
		line := fit(pointer+label, w-4)
		if i == l.index {
			line = selected.Render(line)
		}
		lines = append(lines, line)
	}
	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	lines = append(lines, muted.Render(fmt.Sprintf("%d–%d of %d · Enter to open", min(start+1, l.choiceCount()), min(l.choiceCount(), start+m.visibleRows()), l.choiceCount())))
	return title, strings.Join(lines, "\n")
}
