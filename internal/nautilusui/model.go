// Package nautilusui presents namespace workloads without translating them into
// Slurm jobs. All reads go through the bounded Nautilus adapter.
package nautilusui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/nautilus"
)

type Source interface {
	Scope() nautilus.Scope
	Refresh(context.Context) nautilus.Refresh
}

type Options struct {
	Context       string
	PointerStyle  string
	Demo, NoMouse bool
	// Connect is used only by an explicit c or /connect action, with the TUI
	// suspended. It must handle foreground login and restore terminal modes.
	Connect func(context.Context) (*nautilus.Connection, error)
	Close   func()
}

type Model struct {
	client                                        Source
	options                                       Options
	ctx                                           context.Context
	cancel                                        context.CancelFunc
	requestCancel                                 context.CancelFunc
	scope                                         nautilus.Scope
	jobs                                          *nautilus.Collection[nautilus.Job]
	pods                                          *nautilus.Collection[nautilus.Pod]
	quotas                                        *nautilus.Collection[nautilus.Quota]
	quotasErr                                     *nautilus.QueryError
	jobsErr, podsErr                              *nautilus.QueryError
	width, height, tab                            int
	lastResourceTab                               int
	selection                                     [2]int
	selectedUID                                   [2]string
	filters                                       [2]string
	inspect, help, busy, connecting, authRequired bool
	sequence                                      uint64
	failures                                      int
	now, nextRefresh, lastRequest                 time.Time
	notice                                        string
	input                                         textinput.Model
	panel                                         viewport.Model
	logs                                          *logView
	logAuthRequired                               bool
	hoverMouse                                    tea.Mouse
	hoverKnown                                    bool
	lastPointer                                   string
}

type refreshMsg struct {
	sequence uint64
	result   nautilus.Refresh
}
type clockMsg time.Time
type loginMsg struct {
	conn *nautilus.Connection
	err  error
}

func New(parent context.Context, client Source, options Options) *Model {
	ctx, cancel := context.WithCancel(parent)
	m := &Model{client: client, options: options, scope: client.Scope(), ctx: ctx, cancel: cancel, now: time.Now()}
	m.input = textinput.New()
	m.input.Prompt = "> "
	m.input.Placeholder = "Filter workloads or enter /help"
	m.input.CharLimit = 200
	m.panel = viewport.New()
	m.panel.SoftWrap = true
	return m
}

func (m *Model) Close() {
	m.closeLogs()
	m.cancel()
	if m.requestCancel != nil {
		m.requestCancel()
	}
	if m.options.Close != nil {
		m.options.Close()
		m.options.Close = nil
	}
}

func (m *Model) Init() tea.Cmd { return tea.Batch(m.refresh(), tick()) }
func tick() tea.Cmd            { return tea.Tick(time.Second, func(t time.Time) tea.Msg { return clockMsg(t) }) }

func (m *Model) refresh() tea.Cmd {
	if m.busy || m.connecting || m.ctx.Err() != nil {
		return nil
	}
	m.busy = true
	m.sequence++
	m.lastRequest = m.now
	sequence, client := m.sequence, m.client
	ctx, cancel := context.WithCancel(m.ctx)
	m.requestCancel = cancel
	return func() tea.Msg { defer cancel(); return refreshMsg{sequence, client.Refresh(ctx)} }
}

func (m *Model) receive(msg refreshMsg) {
	if !m.busy || msg.sequence != m.sequence || msg.result.Scope != m.scope {
		return
	}
	m.busy = false
	m.jobsErr, m.podsErr, m.quotasErr = msg.result.Jobs.Error, msg.result.Pods.Error, msg.result.Quotas.Error
	if msg.result.Quotas.OK() {
		m.quotas = msg.result.Quotas.Collection
	}
	if msg.result.Jobs.OK() {
		m.jobs = msg.result.Jobs.Collection
	}
	if msg.result.Pods.OK() {
		m.pods = msg.result.Pods.Collection
	}
	m.authRequired = (m.quotasErr != nil && m.quotasErr.Kind == nautilus.Unauthenticated) || m.logAuthRequired || (m.jobsErr != nil && m.jobsErr.Kind == nautilus.Unauthenticated) || (m.podsErr != nil && m.podsErr.Kind == nautilus.Unauthenticated)
	if m.jobsErr != nil || m.podsErr != nil || m.quotasErr != nil {
		m.failures = min(3, m.failures+1)
	} else {
		m.failures = 0
	}
	m.nextRefresh = m.now.Add(15 * time.Second * time.Duration(1<<m.failures))
	for tab := range 2 {
		m.normalize(tab)
	}
}

type row struct {
	ref   nautilus.Ref
	state string
}

func (m *Model) rows(tab int) []row {
	var rows []row
	if tab == 0 && m.jobs != nil {
		for _, j := range m.jobs.Items {
			rows = append(rows, row{j.Ref, j.State()})
		}
	} else if tab == 1 && m.pods != nil {
		for _, p := range m.pods.Items {
			rows = append(rows, row{p.Ref, p.Phase})
		}
	}
	filter := strings.ToLower(m.filters[tab])
	filtered := rows[:0]
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.ref.Name+" "+r.state), filter) {
			filtered = append(filtered, r)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].ref.Name < filtered[j].ref.Name })
	return filtered
}

func (m *Model) normalize(tab int) {
	rows := m.rows(tab)
	for i, row := range rows {
		if row.ref.UID == m.selectedUID[tab] {
			m.selection[tab] = i
			return
		}
	}
	if m.selectedUID[tab] != "" && m.tab == tab && m.inspect {
		m.inspect = false
		m.notice = "Selected object left this snapshot; absence is not completion."
	}
	m.selection[tab] = max(0, min(m.selection[tab], len(rows)-1))
	m.selectedUID[tab] = ""
	if len(rows) > 0 {
		m.selectedUID[tab] = rows[m.selection[tab]].ref.UID
	}
}

func (m *Model) move(delta int) {
	rows := m.rows(m.tab)
	m.selection[m.tab] = max(0, min(len(rows)-1, m.selection[m.tab]+delta))
	if len(rows) > 0 {
		m.selectedUID[m.tab] = rows[m.selection[m.tab]].ref.UID
	}
}

func (m *Model) switchTab(tab int) {
	m.closeLogs()
	m.tab, m.inspect, m.help = tab, false, false
	if tab < 2 {
		m.lastResourceTab = tab
	}
	m.panel.GotoTop()
	m.input.Blur()
	m.notice = ""
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	m.panel.SetWidth(max(1, m.width-8))
	m.panel.SetHeight(max(1, m.height-14))
	m.panel.SetContent(ansi.Wrap(m.panelText(), max(1, m.width-8), " "))
	m.panel.SetYOffset(m.panel.YOffset())
	m.syncLogs()
	if pointer := m.syncPointer(); pointer != nil {
		cmd = tea.Sequence(pointer, cmd)
	}
	return m, cmd
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.hoverKnown = false
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(max(1, m.width-12))
	case clockMsg:
		m.now = time.Time(msg)
		if !m.authRequired && !m.now.Before(m.nextRefresh) {
			return tea.Batch(tick(), m.refresh())
		}
		return tick()
	case refreshMsg:
		m.receive(msg)
	case logMsg:
		m.receiveLogs(msg)
	case metricsMsg:
		m.receiveMetrics(msg)
	case loginMsg:
		m.hoverKnown = false
		m.lastPointer = ""
		m.connecting = false
		if msg.err != nil || msg.conn == nil || msg.conn.Client == nil {
			if msg.conn != nil {
				msg.conn.Close()
			}
			m.notice = "Sign-in did not complete. Press c to try again."
			return nil
		}
		if m.options.Close != nil {
			m.options.Close()
		}
		if msg.conn.Scope != m.scope {
			m.closeLogs()
			m.jobs, m.pods, m.quotas = nil, nil, nil
			m.jobsErr, m.podsErr, m.quotasErr = nil, nil, nil
			m.selectedUID, m.selection = [2]string{}, [2]int{}
			m.inspect = false
		}
		m.client, m.scope, m.options.Close = msg.conn.Client, msg.conn.Scope, msg.conn.Close
		m.authRequired, m.failures, m.notice = false, 0, ""
		m.logAuthRequired = false
		return tea.Batch(m.refresh(), m.readLogs(true))
	case tea.MouseMotionMsg:
		m.hoverMouse, m.hoverKnown = msg.Mouse(), true
	case tea.BlurMsg:
		m.hoverKnown = false
	case tea.MouseClickMsg:
		m.hoverMouse, m.hoverKnown = msg.Mouse(), true
		return m.mouseClick(msg.Mouse())
	case tea.MouseWheelMsg:
		m.hoverMouse, m.hoverKnown = msg.Mouse(), true
		if m.options.NoMouse || m.width < 48 || m.height < 18 {
			return nil
		}
		p := msg.Mouse()
		if p.X < 3 || p.X >= m.width-3 || p.Y < 5 || p.Y >= m.height-9 || p.Mod != 0 {
			return nil
		}
		if p.Button != tea.MouseWheelDown && p.Button != tea.MouseWheelUp {
			return nil
		}
		delta := 3
		if p.Button == tea.MouseWheelUp {
			delta = -3
		}
		if m.logs != nil && !m.help {
			if m.logs.stage == "output" {
				m.logs.panel.SetYOffset(m.logs.panel.YOffset() + delta)
			} else {
				m.logs.move(delta)
			}
			return nil
		}
		if m.tab < 2 && !m.inspect && !m.help {
			m.move(delta)
		} else {
			m.panel.SetYOffset(m.panel.YOffset() + delta)
		}
	case tea.PasteMsg:
		m.hoverKnown = false
		if m.input.Focused() {
			m.input.SetValue(domain.Label(m.input.Value() + msg.Content))
		}
	case tea.KeyPressMsg:
		m.hoverKnown = false
		key := msg.String()
		if key == "ctrl+c" {
			m.Close()
			return tea.Quit
		}
		if m.input.Focused() {
			switch key {
			case "esc":
				m.input.Blur()
				m.input.SetValue("")
				return nil
			case "enter":
				return m.submit()
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return cmd
		}
		if m.logs != nil && !m.help {
			if handled, cmd := m.logKey(key, msg); handled {
				return cmd
			}
		}
		switch key {
		case "q":
			m.Close()
			return tea.Quit
		case "1", "2", "3", "4":
			m.switchTab(int(key[0] - '1'))
		case "tab":
			m.switchTab((m.tab + 1) % 4)
		case "shift+tab":
			m.switchTab((m.tab + 3) % 4)
		case "?":
			m.help = !m.help
			m.panel.GotoTop()
		case "esc":
			if m.help {
				m.help = false
			} else if m.inspect {
				m.inspect = false
			} else if m.tab < 2 {
				m.filters[m.tab] = ""
				m.normalize(m.tab)
			} else {
				m.switchTab(m.lastResourceTab)
			}
		case "enter":
			if m.tab < 2 && len(m.rows(m.tab)) > 0 {
				m.inspect = true
				m.help = false
				m.panel.GotoTop()
			}
		case "/", "f":
			m.input.SetValue("/")
			if key == "f" {
				m.input.SetValue("/find ")
			}
			m.input.CursorEnd()
			return m.input.Focus()
		case "r":
			if m.now.Sub(m.lastRequest) >= 2*time.Second {
				return m.refresh()
			}
		case "c":
			return m.reconnect()
		case "l":
			return m.openLogs()
		case "u":
			return m.openMetrics()
		case "up", "down", "j", "k", "pgup", "pgdown", "home", "end":
			if m.tab < 2 && !m.inspect && !m.help {
				delta := 1
				if key == "up" || key == "k" {
					delta = -1
				}
				if key == "pgup" {
					delta = -m.visibleRows()
				}
				if key == "pgdown" {
					delta = m.visibleRows()
				}
				if key == "home" {
					delta = -len(m.rows(m.tab))
				}
				if key == "end" {
					delta = len(m.rows(m.tab))
				}
				m.move(delta)
			} else {
				if key == "home" {
					m.panel.GotoTop()
				} else if key == "end" {
					m.panel.GotoBottom()
				} else {
					var cmd tea.Cmd
					m.panel, cmd = m.panel.Update(msg)
					return cmd
				}
			}
		}
	}
	return nil
}

func (m *Model) submit() tea.Cmd {
	value := domain.Label(m.input.Value())
	name, arg, _ := strings.Cut(value, " ")
	arg = strings.TrimSpace(arg)
	m.input.Blur()
	m.input.SetValue("")
	m.notice = ""
	for i, command := range []string{"/jobs", "/pods", "/namespace", "/connection"} {
		if name == command && arg == "" {
			m.switchTab(i)
			return nil
		}
	}
	switch {
	case value == "/quotas":
		m.switchTab(2)
		return nil
	case value == "/refresh":
		if m.logs != nil && !m.help {
			return m.readLogs(false)
		}
		if m.now.Sub(m.lastRequest) >= 2*time.Second {
			return m.refresh()
		}
	case value == "/connect":
		return m.reconnect()
	case value == "/logs":
		return m.openLogs()
	case value == "/metrics":
		return m.openMetrics()
	case value == "/help":
		m.help = true
		m.panel.GotoTop()
	case value == "/quit":
		m.Close()
		return tea.Quit
	case name == "/find" || !strings.HasPrefix(value, "/"):
		m.closeLogs()
		if m.tab > 1 {
			m.switchTab(0)
		}
		if name != "/find" {
			arg = value
		}
		m.filters[m.tab] = arg
		m.inspect = false
		m.help = false
		m.normalize(m.tab)
	default:
		m.notice = "Unknown command. /help lists available commands."
	}
	return nil
}

type loginCommand struct {
	run    func() error
	reset  string
	output io.Writer
}

func (c *loginCommand) Run() error {
	// Exec assigns this writer after suspending the renderer. A queued Raw
	// command can be buffered until after the foreground login returns.
	if c.reset != "" {
		if _, err := io.WriteString(c.output, c.reset); err != nil {
			return err
		}
	}
	return c.run()
}
func (*loginCommand) SetStdin(io.Reader)      {}
func (c *loginCommand) SetStdout(w io.Writer) { c.output = w }
func (*loginCommand) SetStderr(io.Writer)     {}

func (m *Model) reconnect() tea.Cmd {
	if m.connecting {
		return nil
	}
	if m.options.Connect == nil {
		m.notice = "Offline demo · no sign-in needed"
		return nil
	}
	m.cancelLogRead()
	if m.requestCancel != nil {
		m.requestCancel()
	}
	m.sequence++
	m.busy = false
	m.connecting = true
	m.hoverKnown = false
	m.authRequired = true
	var conn *nautilus.Connection
	command := &loginCommand{reset: m.PointerReset(), run: func() error {
		ctx, cancel := context.WithTimeout(m.ctx, 150*time.Second)
		defer cancel()
		var err error
		conn, err = m.options.Connect(ctx)
		return err
	}}
	return tea.Exec(command, func(err error) tea.Msg {
		if m.ctx.Err() != nil {
			if conn != nil {
				conn.Close()
			}
			return loginMsg{err: fmt.Errorf("workspace closed")}
		}
		return loginMsg{conn, err}
	})
}
