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
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"perch/assets/bird"
	"perch/internal/app"
	"perch/internal/cancellation"
	"perch/internal/demo"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/launch"
	"perch/internal/notify"
	"perch/internal/preferences"
	"perch/internal/query"
	"perch/internal/snapshotcache"
	"perch/internal/timewarning"
	"perch/internal/transport"
)

type Options struct {
	TimeWarnings                          timewarning.Policy
	Cache                                 *snapshotcache.Store
	ColorMode                             string
	PointerStyle                          string
	Diagnostics                           func() string
	Demo, ReducedMotion, NoColor, NoMouse bool
	Host                                  string
	Theme                                 string
	StateDir                              string
	Notifier                              notify.Sender
	Journal                               *launch.Journal
	Preferences                           *preferences.Store
}
type Model struct {
	cachedSavedAt                        time.Time
	cacheBlocked                         bool
	liveScope                            snapshotcache.Scope
	cachedStore                          *app.Store
	cachedScope                          snapshotcache.Scope
	cacheVerified                        bool
	cacheFresh                           [3]bool
	cacheStatus                          string
	cacheWriteAt                         time.Time
	detectedColorProfile                 colorprofile.Profile
	colorProfileKnown                    bool
	hoverMouse                           tea.Mouse
	hoverKnown, pointerSuspended         bool
	lastPointer                          string
	settings                             *settingsForm
	settingsSequence                     int
	warningForm                          *warningForm
	warningSequence                      int
	activity                             activityState
	waitsAt                              time.Time
	waitsReport                          query.WaitHistory
	notifications                        *notify.Session
	notificationScope                    notify.Scope
	notificationStatus                   string
	notificationContext                  context.Context
	notificationCancel                   context.CancelFunc
	launchForm                           *launchForm
	shellForm                            *shellPreview
	shellSequence                        int
	cancelForm                           *cancelReview
	cancelSequence                       int
	lastCancellation                     *cancellation.Receipt
	launchSequence                       int
	pendingLaunchFocus                   string
	softwareRequest                      query.Request
	softwareBody                         string
	softwareRenderedText                 string
	softwareRenderedWidth                int
	scriptText                           string
	scriptLoaded                         bool
	scriptRenderedWidth                  int
	moduleInfo                           *query.ModuleInfo
	nodeRows                             []query.Node
	nodesGPUOnly                         bool
	catalog, catalogReturn               *softwareCatalog
	reportAt                             time.Time
	reportOrigin                         reportReturn
	receiptsBody                         string
	input                                textinput.Model
	inputError                           string
	options                              Options
	client                               *hummingbird.Client
	store                                *app.Store
	report                               domain.Report
	width, height                        int
	tab                                  int
	selection                            int
	selectedID                           string
	search                               string
	searching                            bool
	palette                              bool
	paletteIndex                         int
	paletteQuery                         string
	paletteJob                           *domain.Job
	tabSelections                        [2]string
	tabFilters                           [2]string
	help                                 bool
	view                                 string
	viewport                             viewport.Model
	panel                                viewport.Model
	frame                                int
	framePending                         bool
	birdState                            string
	completeUntil                        time.Time
	now                                  time.Time
	connected, busy, detailBusy          bool
	generation, detailSequence           int
	cancel, detailCancel                 context.CancelFunc
	nextRefresh, lastRequest, nextDetail time.Time
	failures                             int
	lastError, notice                    string
	noticeUntil                          time.Time
	clipboardCancel                      context.CancelFunc
	scenario                             int
	logCursor                            hummingbird.LogCursor
	logRaw                               string
	logRenderWidth                       int
	readerSearch                         *readerSearch
	jobSorts                             [2]jobSort
	logHint                              observedLogHint
	logFollowing, logLoaded              bool
	detailError                          string
	logErr                               bool
	live                                 []hummingbird.LiveSample
	resourceStep                         string
	projection                           jobProjection
}
type clockMsg time.Time
type frameMsg time.Time
type probeMsg struct {
	report     domain.Report
	err        error
	generation int
}
type refreshMsg struct {
	result     hummingbird.Refresh
	err        error
	generation int
}
type detailMsg struct {
	module   *query.ModuleInfo
	nodes    []query.Node
	modules  []query.Software
	observed time.Time
	sequence int
	view     string
	text     string
	chunk    hummingbird.LogChunk
	sample   hummingbird.LiveSample
	err      error
}
type loginMsg struct{ err error }

func New(client *hummingbird.Client, options Options) *Model {
	m := &Model{client: client, options: options, store: app.NewStore(), width: 100, height: 32, view: "details", now: time.Now(), birdState: "idle"}
	m.store.SetTimeWarnings(options.TimeWarnings)
	m.notificationContext, m.notificationCancel = context.WithCancel(context.Background())
	m.viewport = viewport.New()
	m.viewport.SoftWrap = true
	m.panel = viewport.New()
	m.panel.SoftWrap = true
	m.input = textinput.New()
	m.input.Prompt = "> "
	m.input.Placeholder = "Find a command or jump to a job"
	m.input.CharLimit = 200
	m.input.SetVirtualCursor(true)
	styles := m.input.Styles()
	styles.Focused.Text = ink
	styles.Blurred.Text = ink
	styles.Focused.Prompt = accent
	styles.Blurred.Prompt = accent
	styles.Focused.Placeholder = muted
	styles.Blurred.Placeholder = muted
	m.input.SetStyles(styles)
	m.input.SetWidth(90)
	m.loadCache()
	return m
}
func clockTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return clockMsg(t) })
}
func frameTick(state string) tea.Cmd {
	return tea.Tick(bird.FrameInterval(state), func(t time.Time) tea.Msg { return frameMsg(t) })
}
func (m *Model) nextFrame() tea.Cmd {
	if m.framePending || m.options.ReducedMotion || m.options.NoColor {
		return nil
	}
	m.framePending = true
	return frameTick(m.chipState())
}
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{clockTick(), m.probe()}
	if !m.options.ReducedMotion && !m.options.NoColor {
		cmds = append(cmds, m.nextFrame())
	}
	return tea.Batch(cmds...)
}
func (m *Model) Close() {
	if m.clipboardCancel != nil {
		m.clipboardCancel()
	}
	m.notificationCancel()
	if m.notifications != nil {
		m.notifications.Close()
	}
	m.cancelRequests()
}

// Reconnecting stops network work without closing the workspace's notification
// context or disk lock. Those belong to final shutdown, not SSH authentication.
func (m *Model) cancelRequests() {
	if m.cancelForm != nil && m.cancelForm.cancel != nil {
		m.cancelForm.cancel()
	}
	if m.shellForm != nil && m.shellForm.cancel != nil {
		m.shellForm.cancel()
	}
	if m.launchForm != nil && m.launchForm.cancel != nil {
		m.launchForm.cancel()
	}
	if m.cancel != nil {
		m.cancel()
	}
	if m.detailCancel != nil {
		m.detailCancel()
	}
}
func (m *Model) probe() tea.Cmd {
	m.generation++
	generation := m.generation
	m.busy = true
	m.birdState = "reconnecting"
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	m.cancel = cancel
	if m.options.Demo {
		return func() tea.Msg {
			cancel()
			return probeMsg{report: domain.Report{Host: "demo", User: "researcher", Cluster: "hummingbird", Parser: hummingbird.Parser, Version: "Slurm 25.11.4"}, generation: generation}
		}
	}
	client := m.client
	return func() tea.Msg {
		defer cancel()
		report, e := client.Probe(ctx)
		return probeMsg{report: report, err: e, generation: generation}
	}
}
func (m *Model) refresh() tea.Cmd {
	if m.busy {
		return nil
	}
	if m.cachedStore != nil && !m.cacheVerified {
		return m.probe()
	}
	m.busy = true
	m.lastRequest = m.now
	generation := m.generation
	if m.options.Demo {
		scenario, now := m.scenario, m.now
		return func() tea.Msg { return refreshMsg{result: demo.Snapshot(now, scenario), generation: generation} }
	}
	history := m.now.Sub(m.store.Snapshot.HistoryAt) >= 60*time.Second
	partitions := m.now.Sub(m.store.Snapshot.PartitionsAt) >= 60*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	m.cancel = cancel
	client := m.client
	return func() tea.Msg {
		defer cancel()
		r, e := client.Refresh(ctx, history, partitions)
		return refreshMsg{result: r, err: e, generation: generation}
	}
}
func (m *Model) displayJobs() []domain.Job {
	m.ensureJobProjection()
	return m.projection.all
}
func (m *Model) jobs() []domain.Job {
	m.ensureJobProjection()
	if m.projection.filtered && m.projection.tab == m.tab && m.projection.search == m.search && m.projection.sort == m.currentJobSort() {
		return m.projection.jobs
	}
	var out []domain.Job
	search := strings.ToLower(m.search)
	for _, j := range m.displayJobs() {
		if m.tab == 0 && j.Terminal() {
			continue
		}
		if m.tab == 1 && !j.Terminal() {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(j.ID+" "+j.Name+" "+j.State+" "+j.Partition), search) {
			continue
		}
		out = append(out, j)
	}
	sortJobRows(out, m.currentJobSort())
	m.projection.jobs, m.projection.tab, m.projection.search, m.projection.filtered = out, m.tab, m.search, true
	m.projection.sort = m.currentJobSort()
	return out
}
func (m *Model) selected() (domain.Job, bool) {
	jobs := m.jobs()
	if m.selection < 0 || m.selection >= len(jobs) {
		return domain.Job{}, false
	}
	return jobs[m.selection], true
}
func (m *Model) normalizeSelection() {
	jobs := m.jobs()
	for i, j := range jobs {
		if j.ID == m.selectedID {
			m.selection = i
			return
		}
	}
	m.selection = min(max(0, m.selection), max(0, len(jobs)-1))
	if len(jobs) > 0 {
		m.selectedID = jobs[m.selection].ID
	} else {
		m.selectedID = ""
	}
}
func (m *Model) invalidateDetail() {
	m.cacheBlocked = false
	if j, ok := m.selected(); !ok || j.AttemptKey() != m.logHint.attempt {
		m.logHint = observedLogHint{}
	}
	m.detailSequence++
	if m.detailCancel != nil {
		m.detailCancel()
	}
	m.detailBusy = false
	m.logCursor = hummingbird.LogCursor{}
	m.logRaw = ""
	m.readerSearch = nil
	m.moduleInfo = nil
	m.scriptText, m.softwareRenderedText = "", ""
	m.scriptLoaded = false
	m.scriptRenderedWidth, m.softwareRenderedWidth = 0, 0
	m.logRenderWidth = 0
	m.logFollowing, m.logLoaded = true, false
	m.detailError = ""
	m.live = nil
	m.resourceStep = ""
	m.viewport.SetContent("")
	m.viewport.GotoTop()
}
func (m *Model) fetchDetail() tea.Cmd {
	j, ok := m.selected()
	if !ok || m.detailBusy || m.tab > 1 {
		return nil
	}
	view := m.view
	if j.AwaitingTasks > 0 {
		return nil
	}
	if view == "resources" && !j.Running() {
		return nil
	}
	if view != "logs" && view != "script" && view != "resources" {
		return nil
	}
	if !m.cachedDetailAllowed(j) {
		m.cacheBlocked = true
		m.detailError = "Saved job · reconnect and refresh before reading output, scripts or live resources."
		m.viewport.SetContent("Saved job\n\nReconnect and refresh before reading output, scripts or live resources.\n\nPress c to reconnect, or Esc to return.")
		return nil
	}
	m.cacheBlocked = false
	m.detailBusy = true
	m.detailSequence++
	seq := m.detailSequence
	m.nextDetail = m.now.Add(8 * time.Second)
	if m.options.Demo {
		return func() tea.Msg {
			r := detailMsg{sequence: seq, view: view}
			switch view {
			case "logs":
				r.chunk = hummingbird.LogChunk{Raw: demo.Log(j), Reset: true}
			case "script":
				r.text = demo.Script
			case "resources":
				rss := int64(1100) << 20
				cpu := j.Elapsed.Seconds() * 0.95
				batchRSS, batchCPU := int64(4<<20), 0.25
				r.sample = hummingbird.LiveSample{At: time.Now(), PeakRSS: &batchRSS, CPUSeconds: &batchCPU, Scope: "synthetic demo batch step", Steps: []hummingbird.LiveStep{
					{ID: fmt.Sprintf("%d.0", j.RawID), PeakRSS: &rss, CPUSeconds: &cpu},
					{ID: fmt.Sprintf("%d.batch", j.RawID), PeakRSS: &batchRSS, CPUSeconds: &batchCPU},
				}}
			}
			return r
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	m.detailCancel = cancel
	client := m.client
	cursor := m.logCursor
	stderr := m.logErr
	return func() tea.Msg {
		defer cancel()
		r := detailMsg{sequence: seq, view: view}
		switch view {
		case "logs":
			r.chunk, r.err = client.ReadLog(ctx, j, stderr, cursor)
		case "script":
			r.text, r.err = client.Script(ctx, j)
		case "resources":
			r.sample, r.err = client.LiveResources(ctx, j)
		}
		return r
	}
}
func (m *Model) setView(view string) tea.Cmd {
	if j, ok := m.selected(); ok && j.AwaitingTasks > 0 && (view == "logs" || view == "script" || view == "resources") {
		m.setNotice("Choose a confirmed task for output, script or resources.")
		view = "inspect"
	}
	m.view = view
	m.invalidateDetail()
	return m.fetchDetail()
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.update(message)
	if j, ok := m.selected(); ok && j.AwaitingTasks > 0 && (m.view == "logs" || m.view == "script" || m.view == "resources") {
		m.setView("inspect")
	}
	previousViewportHeight := m.viewport.Height()
	viewportHeight := m.height - 16
	if m.compactTextReading() {
		viewportHeight = m.layout().bodyHeight - 4
	}
	if m.view == "receipts" || m.view == "software" {
		viewportHeight = m.layout().bodyHeight - 2
	}
	m.viewport.SetHeight(max(1, viewportHeight))
	if m.view == "logs" && m.logLoaded && m.logRenderWidth != m.viewport.Width() {
		m.displayLogText()
	}
	if m.view == "script" && m.scriptLoaded && m.detailError == "" && m.scriptRenderedWidth != m.viewport.Width() {
		m.displayScriptText()
	}
	if m.view == "script" && m.detailError != "" && !m.cacheBlocked && m.scriptRenderedWidth != m.viewport.Width() {
		m.displayScriptError()
	}
	if m.readerSearch != nil {
		m.readerSearch.input.SetWidth(max(10, m.width-16))
		if _, resized := message.(tea.WindowSizeMsg); (resized || previousViewportHeight != m.viewport.Height()) && m.textReader() {
			m.revealReaderMatch()
		}
	}
	m.syncPanel()
	m.syncSoftware()
	m.syncReceipts()
	if m.warningForm != nil {
		m.warningForm.input.SetWidth(max(1, m.warningsGeometry().width-8))
	}
	if m.view == "activity" {
		m.selectActivity(m.activityIndex())
	}
	// A larger window may fit more rows without changing the underlying text.
	// Viewport setters do not clamp the old scroll offset after resizing.
	m.panel.SetYOffset(m.panel.YOffset())
	m.viewport.SetYOffset(m.viewport.YOffset())
	if m.view == "logs" && m.logFollowing && m.logLoaded && m.detailError == "" {
		m.viewport.GotoBottom()
	}
	if pointer := m.syncPointer(); pointer != nil {
		cmd = tea.Sequence(pointer, cmd)
	}
	return model, cmd
}
func (m *Model) syncPanel() {
	m.panel.SetWidth(max(10, m.width-8))
	m.panel.SetHeight(max(1, m.height-14))
	if m.view == "inspect" && m.compactTextReading() {
		m.panel.SetHeight(max(1, m.layout().bodyHeight-2))
	}
	content := ""
	switch {
	case m.view == "cancel":
		m.panel.SetHeight(max(1, m.layout().bodyHeight-2))
		content = m.cancellationContent()
	case m.view == "shell":
		m.panel.SetHeight(max(1, m.layout().bodyHeight-2))
		content = m.shellContent()
	case m.help:
		content = m.helpText()
	case m.tab == 2:
		content = m.partitions(max(10, m.width-8))
	case m.tab == 3:
		content = m.connection()
	case m.view == "waits":
		m.panel.SetHeight(max(1, m.layout().bodyHeight-2))
		content = m.waitsContent(max(10, m.width-8))
	case m.view == "inspect":
		if j, ok := m.selected(); ok {
			content = m.details(j, max(10, m.width-8))
		}
	case m.view == "resources":
		if j, ok := m.selected(); ok {
			content = m.resources(j, max(10, m.width-8))
		}
	}
	// Wrap prose at words, with hard wrapping for long IDs/paths. The viewport
	// remains responsible for scrolling, not cutting words at cell boundaries.
	content = ansi.Wrap(content, max(10, m.width-8), " ")
	if content != m.panel.GetContent() {
		m.panel.SetContent(content)
	}
}
func (m *Model) update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case cancelPreparedMsg:
		return m, m.receiveCancellationPreview(msg)
	case cancelSentMsg:
		return m, m.receiveCancellationReceipt(msg)
	case shellPreparedMsg:
		return m, m.receiveShell(msg)
	case shellReturnedMsg:
		if m.shellForm == nil || msg.sequence != m.shellForm.sequence {
			return m, nil
		}
		if errors.Is(msg.err, transport.ErrForegroundInterrupted) {
			m.pointerSuspended = false
			m.Close()
			return m, tea.Quit
		}
		if msg.err != nil {
			m.setNotice("Shell ended · " + domain.Label(msg.err.Error()))
		} else {
			m.setNotice("Shell closed · back in Perch")
		}
		return m, m.closeShellPreview()
	case tea.ColorProfileMsg:
		if !m.colorProfileKnown {
			m.detectedColorProfile, m.colorProfileKnown = msg.Profile, true
		}
		if msg.Profile != m.colorProfile() {
			return m, m.applyColorMode()
		}
		return m, nil
	case warningsSavedMsg:
		return m, m.warningsSaved(msg)
	case settingsSavedMsg:
		return m, m.settingsSaved(msg)
	case launchMsg:
		return m, m.receiveLaunch(msg)
	case copiedMsg:
		if m.clipboardCancel != nil {
			m.clipboardCancel()
			m.clipboardCancel = nil
		}
		if msg.err != nil {
			m.setNotice("Clipboard unavailable · " + msg.spec)
		} else if msg.subject == "module command" {
			m.setNotice("Copied: " + msg.spec)
		} else {
			subject := msg.subject
			if subject == "" {
				subject = "failed tasks"
			}
			m.setNotice("Copied " + subject + ": " + msg.spec)
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.hoverKnown = false
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.SetWidth(max(10, m.width-8))
		m.viewport.SetHeight(max(1, m.height-16))
		m.input.SetWidth(max(10, m.width-12))
		// The renderer can paint the previous frame at the new terminal size
		// while this update reflows long text. Repaint after the new geometry
		// is applied so clipped old rows cannot survive incremental redraws.
		return m, tea.ClearScreen
	case frameMsg:
		m.framePending = false
		if m.options.ReducedMotion || m.options.NoColor {
			return m, nil
		}
		state := m.chipState()
		if state != m.birdState {
			m.birdState = state
			m.frame = 0
		} else {
			if state == "complete" {
				m.frame = min(63, m.frame+1)
			} else {
				m.frame = (m.frame + 1) % 64
			}
		}
		return m, m.nextFrame()
	case clockMsg:
		m.now = time.Time(msg)
		if !m.noticeUntil.IsZero() && !m.now.Before(m.noticeUntil) {
			m.notice = ""
			m.noticeUntil = time.Time{}
		}
		cmds := []tea.Cmd{clockTick()}
		if m.view != "shell" && m.view != "cancel" && !m.busy && !m.nextRefresh.IsZero() && !m.now.Before(m.nextRefresh) {
			cmds = append(cmds, m.refresh())
		}
		if m.connected && !m.detailBusy && !m.now.Before(m.nextDetail) && (m.view == "logs" || m.view == "resources") {
			cmds = append(cmds, m.fetchDetail())
		}
		return m, tea.Batch(cmds...)
	case probeMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		m.busy = false
		m.rejectChangedIdentity(msg.report)
		if msg.report.User != "" {
			m.report = msg.report
		}
		if msg.err != nil {
			m.connected = false
			m.lastError = msg.err.Error()
			m.birdState = "reconnecting"
			return m, nil
		}
		scope := snapshotcache.Scope{Host: m.options.Host, User: msg.report.User, Cluster: msg.report.Cluster}
		if m.liveScope.User != "" && scope != m.liveScope {
			m.store = app.NewStore()
			m.store.SetTimeWarnings(m.options.TimeWarnings)
			m.activity = activityState{}
			m.invalidateDetail()
			m.selectedID = ""
			m.waitsAt = time.Time{}
		}
		m.liveScope = scope
		m.report = msg.report
		if m.client != nil {
			m.client = &hummingbird.Client{Runner: m.client.Runner, Host: m.client.Host, User: msg.report.User}
		}
		m.connected = true
		m.lastError = ""
		m.verifyCache(msg.report)
		m.startNotifications()
		return m, m.refresh()
	case refreshMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		m.busy = false
		if msg.err != nil {
			m.connected = false
			m.lastError = msg.err.Error()
			m.failures++
			m.nextRefresh = m.now.Add(time.Duration(min(120, 5*(1<<min(m.failures, 5)))) * time.Second)
			m.birdState = "reconnecting"
			return m, nil
		}
		m.connected = true
		m.failures = 0
		m.nextRefresh = m.now.Add(30 * time.Second)
		m.lastError = ""
		for _, name := range []string{"jobs", "history", "partitions"} {
			if e := msg.result.Errors[name]; e != nil {
				if m.lastError != "" {
					m.lastError += " · "
				}
				m.lastError += name + ": " + e.Error()
			}
		}
		priorSelection, hadPriorSelection := m.selected()
		limitedBefore := m.store.ArrayTrackingLimited()
		displayLimitedBefore := m.store.ArrayDisplayLimited()
		events := m.store.Apply(msg.result)
		m.updateCacheView(msg.result)
		notifications := tea.Batch(m.saveNotifications(events), m.saveCache())
		if !limitedBefore && m.store.ArrayTrackingLimited() && m.options.Notifier != nil {
			m.setNotice("Array alert coverage limited · see Connection")
		}
		if !displayLimitedBefore && m.store.ArrayDisplayLimited() {
			m.setNotice("Array history limit reached · see Connection")
		}
		for _, e := range events {
			if e.Kind == "finished" {
				m.completeUntil = m.now.Add(5120 * time.Millisecond)
				m.frame = 0
				m.setNotice("Finished · " + e.Name)
			}
		}
		if m.pendingLaunchFocus != "" {
			for _, j := range m.store.Snapshot.Jobs {
				if j.ID == m.pendingLaunchFocus || j.ArrayID > 0 && strconv.FormatInt(j.ArrayID, 10) == m.pendingLaunchFocus {
					target := 0
					if j.Terminal() {
						target = 1
					}
					m.switchTab(target)
					m.search = ""
					m.selectedID = j.ID
					m.pendingLaunchFocus = ""
					m.view = "inspect"
					break
				}
			}
		}
		oldID := m.selectedID
		m.normalizeSelection()
		currentSelection, hasCurrentSelection := m.selected()
		attemptChanged := hadPriorSelection && hasCurrentSelection && priorSelection.AttemptKey() != currentSelection.AttemptKey()
		if (oldID != m.selectedID || attemptChanged) && m.view != "cancel" && m.view != "shell" && m.view != "software" && m.view != "launch" && m.view != "receipts" && m.view != "activity" && m.view != "warnings" {
			m.invalidateDetail()
			return m, tea.Batch(notifications, m.fetchDetail())
		}
		return m, notifications
	case cacheMsg:
		if !msg.at.Before(m.cacheWriteAt) {
			m.cacheWriteAt = msg.at
			if msg.err != nil {
				m.cacheStatus = "Cache not updated: " + domain.Label(msg.err.Error())
			} else {
				m.cacheStatus = "Cache saved · " + m.options.Cache.Path()
			}
		}
		return m, nil
	case notificationMsg:
		if msg.session == m.notifications && msg.err != nil {
			m.notificationStatus = "Notification tracking/delivery error: " + domain.Label(msg.err.Error())
			m.setNotice("Notifications need attention · see Connection")
		}
		return m, nil
	case detailMsg:
		if msg.sequence != m.detailSequence {
			return m, nil
		}
		m.detailBusy = false
		if msg.err != nil {
			m.detailError = domain.Label(msg.err.Error())
			if msg.view == "logs" && m.logLoaded {
				// Retain the last readable tail, cursor and scroll position. The
				// heading labels it stale; a later read resumes from this cursor.
				return m, nil
			}
			if msg.view == "resources" {
				m.recordSample(hummingbird.LiveSample{At: m.now})
			}
			m.viewport.SetContent("Unable to read this view.\n\n" + domain.Clean(msg.err.Error()) + "\n\nPress r to retry, or Esc to return.")
			if msg.view == "script" {
				m.scriptText, m.scriptLoaded = "", false
				m.readerSearch = nil
				m.viewport.GotoTop()
				m.displayScriptError()
			}
			if msg.view == "software" {
				m.softwareBody = m.viewport.GetContent()
			}
			if msg.view == "receipts" {
				m.receiptsBody = m.viewport.GetContent()
			}
			return m, nil
		}
		m.detailError = ""
		switch msg.view {
		case "receipts":
			m.receiptsBody = msg.text
		case "software":
			m.softwareBody = msg.text
			m.moduleInfo = msg.module
			if m.softwareRequest.Command == "nodes" {
				m.nodeRows = msg.nodes
				m.reportAt = msg.observed
				m.refreshNodeBody()
			}
			if m.catalog != nil && m.softwareRequest.Command == "software" {
				m.catalog.items = msg.modules
				m.catalog.at = msg.observed
				index := m.catalog.index
				for i, item := range msg.modules {
					if item.Name == m.catalog.selected {
						index = i
						break
					}
				}
				m.selectModule(index)
			}
			m.reportAt = msg.observed
		case "script":
			m.scriptText, m.scriptLoaded = msg.text, true
			if m.readerSearch != nil {
				m.indexReaderSearch(false)
			} else {
				m.displayScriptText()
			}
		case "logs":
			m.logLoaded = true
			if msg.chunk.Reset {
				m.logRaw = ""
			}
			m.logRaw += msg.chunk.Raw
			if len(m.logRaw) > 256<<10 {
				m.logRaw = m.logRaw[len(m.logRaw)-(256<<10):]
			}
			m.logCursor = msg.chunk.Cursor
			m.captureLogHint()
			if m.readerSearch != nil {
				m.indexReaderSearch(false)
			} else {
				m.displayLogText()
			}
			if m.logFollowing {
				m.viewport.GotoBottom()
			}
		case "resources":
			m.recordSample(msg.sample)
		}
	case loginMsg:
		m.pointerSuspended = false
		m.lastPointer = ""
		if msg.err != nil {
			m.lastError = "Sign-in did not complete. Press c to try again."
			m.connected = false
			return m, nil
		}
		return m, m.probe()
	case tea.MouseMotionMsg:
		m.hoverMouse, m.hoverKnown = msg.Mouse(), true
		return m, nil
	case tea.BlurMsg:
		m.hoverKnown = false
		return m, nil
	case tea.MouseClickMsg:
		m.hoverMouse, m.hoverKnown = msg.Mouse(), true
		return m, m.mouseClick(msg.Mouse())
	case tea.MouseWheelMsg:
		m.hoverMouse, m.hoverKnown = msg.Mouse(), true
		return m, m.mouseWheel(msg.Mouse())
	case tea.PasteMsg:
		if m.readerSearchEditing() {
			return m, m.readerSearchUpdate(msg)
		}
		if m.palette {
			m.appendPaletteText(msg.Content)
			return m, nil
		}
		if m.searching {
			m.search = appendSearchText(m.search, msg.Content)
			m.selection = 0
			m.selectedID = ""
			m.normalizeSelection()
			return m, nil
		}
	case tea.KeyPressMsg:
		m.hoverKnown = false
		key := msg.String()
		if key == "ctrl+c" {
			m.Close()
			return m, tea.Quit
		}
		if m.view == "shell" {
			return m, m.shellKey(msg)
		}
		if m.view == "cancel" {
			return m, m.cancellationKey(msg)
		}
		if m.view == "warnings" {
			return m, m.warningsKey(msg)
		}
		if m.view == "settings" {
			return m, m.settingsKey(msg)
		}
		if m.view == "launch" {
			return m, m.launchKey(msg)
		}
		if m.readerSearchEditing() {
			return m, m.readerSearchUpdate(msg)
		}
		if m.input.Focused() {
			switch key {
			case "esc":
				m.input.Blur()
				m.input.SetValue("")
				m.inputError = ""
				return m, nil
			case "enter":
				return m, m.submitInput()
			case "tab":
				value := m.input.Value()
				for _, c := range slashCommands {
					if strings.HasPrefix(c.name, value) {
						m.input.SetValue(c.name + " ")
						m.input.CursorEnd()
						break
					}
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(message)
			m.inputError = ""
			return m, cmd
		}
		if m.searching {
			switch key {
			case "esc":
				m.searching = false
				m.search = ""
			case "enter":
				m.searching = false
			case "backspace":
				r := []rune(m.search)
				if len(r) > 0 {
					m.search = string(r[:len(r)-1])
				}
			default:
				m.search = appendSearchText(m.search, msg.Text)
			}
			m.selection = 0
			m.selectedID = ""
			m.normalizeSelection()
			return m, nil
		}
		if m.palette {
			return m, m.paletteKey(msg)
		}
		if m.help && (key == "esc" || key == "?") {
			m.help = false
			return m, nil
		}
		if m.view == "activity" && !m.help {
			if cmd, handled := m.activityKey(key); handled {
				return m, cmd
			}
		}
		if (m.help || m.tab > 1 || m.view == "inspect" || m.view == "resources" || m.view == "waits") && (key == "up" || key == "down" || key == "j" || key == "k" || key == "pgup" || key == "pgdown" || key == "home" || key == "end") {
			if key == "home" {
				m.panel.GotoTop()
				return m, nil
			}
			if key == "end" {
				m.panel.GotoBottom()
				return m, nil
			}
			var cmd tea.Cmd
			m.panel, cmd = m.panel.Update(message)
			return m, cmd
		}
		if cmd, handled := m.catalogKey(key); handled {
			return m, cmd
		}
		if (m.view == "logs" || m.view == "script" || m.view == "software" || m.view == "receipts") && (key == "up" || key == "down" || key == "j" || key == "k" || key == "pgup" || key == "pgdown" || key == "home" || key == "end") {
			if key == "home" {
				m.viewport.GotoTop()
				if m.view == "logs" {
					m.logFollowing = false
				}
				return m, nil
			}
			if key == "end" {
				m.viewport.GotoBottom()
				if m.view == "logs" {
					if m.readerSearch != nil {
						m.clearReaderSearch()
					}
					m.logFollowing = true
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(message)
			if m.view == "logs" {
				m.logFollowing = m.readerSearch == nil && m.viewport.AtBottom()
			}
			return m, cmd
		}
		return m, m.action(key)
	}
	if m.readerSearchEditing() {
		return m, m.readerSearchUpdate(message)
	}
	if m.view == "warnings" && !m.warningForm.saving {
		var cmd tea.Cmd
		m.warningForm.input, cmd = m.warningForm.input.Update(message)
		return m, cmd
	}
	if m.view == "launch" && m.launchForm.stage == "edit" {
		f := m.launchForm
		var cmd tea.Cmd
		f.fields[f.focus].input, cmd = f.fields[f.focus].input.Update(message)
		return m, cmd
	}

	if m.input.Focused() {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(message)
		return m, cmd
	}
	return m, nil
}
func (m *Model) action(key string) tea.Cmd {
	if strings.HasPrefix(key, "sort:") {
		order := defaultJobSort(strings.TrimPrefix(key, "sort:"))
		return m.applyJobSort(order)
	}
	if key == "y" && m.view == "software" && m.softwareRequest.Command == "module" && !m.help && !m.palette {
		return m.copyModule()
	}
	if key == "g" && m.view == "software" && m.softwareRequest.Command == "nodes" && !m.help && !m.palette {
		if !m.detailBusy && m.detailError == "" {
			m.nodesGPUOnly = !m.nodesGPUOnly
			m.refreshNodeBody()
			m.syncSoftware()
			m.viewport.GotoTop()
		}
		return nil
	}
	if key == "f" && m.view == "software" && m.softwareRequest.Command == "software" && !m.help && !m.palette {
		m.input.SetValue("/software " + m.softwareRequest.ID)
		m.input.CursorEnd()
		m.inputError = ""
		return m.input.Focus()
	}
	if key == "enter" && m.view == "software" && !m.help && !m.palette {
		cmd, _ := m.catalogKey(key)
		return cmd
	}
	if key == "esc" && m.textReader() && m.readerSearch != nil && !m.help {
		m.clearReaderSearch()
		return nil
	}
	if key == "esc" && m.activity.returning {
		return m.returnToActivity()
	}
	switch key {
	case "1", "2", "3", "4", "tab", "shift+tab", "f", "filter-jobs", "waits", "receipts", "software", "priority", "nodes", "launch", "settings":
		if key != "f" || !m.textReader() || m.help {
			m.activity.returning = false
		}
	}

	if problem := m.jobActionProblem(key); problem != "" {
		m.setNotice(problem)
		return nil
	}
	switch key {
	case "sort":
		m.help, m.palette = false, true
		m.paletteJob = nil
		m.paletteIndex, m.paletteQuery = 0, "Sort jobs"
	case "a", "actions":
		j, _ := m.selected()
		m.help, m.palette = false, true
		m.paletteJob = &j
		m.paletteIndex, m.paletteQuery = 0, ""
	case "cancel-job":
		return m.openCancellation(false)
	case "cancel-array":
		return m.openCancellation(true)
	case "cancel-receipt":
		return m.showCancellationReceipt()
	case "o", "shell":
		return m.openShellPreview()
	case "n":
		if m.tab == 2 && m.view == "details" && !m.help {
			return m.showSoftware("nodes", "")
		}
		if m.tab < 2 && m.view == "details" && !m.help {
			return m.openLaunch()
		}
	case "activity":
		return m.showActivity()
	case "nodes":
		return m.showSoftware("nodes", "")
	case "priority":
		return m.showSoftware("priority", "")
	case "waits":
		return m.showWaits()
	case "warnings":
		return m.openWarnings()
	case "settings":
		return m.openSettings()
	case "receipts":
		return m.showReceipts()
	case "launch":
		return m.openLaunch()
	case "software":
		return m.showSoftware("software", "")
	case "q":
		m.Close()
		return tea.Quit
	case "y", "copy-failed":
		return m.copyFailed()
	case "?":
		m.help = true
		m.panel.GotoTop()
	case "ctrl+k":
		m.help = false
		m.palette = true
		m.paletteJob = nil
		m.paletteIndex = 0
		m.paletteQuery = ""
	case "/":
		m.input.SetValue("/")
		m.input.CursorEnd()
		m.inputError = ""
		return m.input.Focus()
	case ":":
		m.input.SetValue("")
		m.inputError = ""
		return m.input.Focus()
	case "f", "filter-jobs":
		if key == "f" && m.textReader() && !m.help {
			return m.startReaderSearch()
		}
		m.help = false
		if m.tab > 1 {
			m.switchTab(0)
		}
		m.searching = true
		m.search = ""
		m.invalidateDetail()
		m.view = "details"
	case "esc":
		if m.view == "software" {
			return m.returnFromReport()
		}
		if m.view == "details" {
			m.search = ""
		}
		m.help = false
		m.normalizeSelection()
		return m.setView("details")
	case "tab", "shift+tab", "1", "2", "3", "4":
		m.help = false
		m.pendingLaunchFocus = ""
		m.panel.GotoTop()
		target := int(key[0] - '1')
		if key == "tab" {
			target = (m.tab + 1) % 4
		} else if key == "shift+tab" {
			target = (m.tab + 3) % 4
		}
		m.switchTab(target)
		return m.setView("details")
	case "down", "j", "up", "k":
		m.pendingLaunchFocus = ""
		if key == "down" || key == "j" {
			m.selection++
		} else {
			m.selection--
		}
		m.selectedID = ""
		m.normalizeSelection()
		m.invalidateDetail()
		return m.fetchDetail()
	case "enter":
		m.help = false
		return m.setView("inspect")
	case "l":
		m.help = false
		return m.setView("logs")
	case "s":
		m.help = false
		return m.setView("script")
	case "u":
		m.help = false
		return m.setView("resources")
	case "[", "]":
		if m.textReader() && m.readerSearch != nil && !m.help {
			m.moveReaderMatch(key == "]")
			return nil
		}
		if m.view == "resources" && !m.help {
			m.cycleResourceStep(key == "]")
		}
	case "e":
		if m.view == "logs" {
			m.logErr = !m.logErr
			return m.setView("logs")
		}
	case "r":
		if m.view == "receipts" {
			return m.showReceipts()
		}
		if m.view == "software" {
			if m.detailBusy {
				return nil
			}
			return m.showSoftware(m.softwareRequest.Command, m.softwareRequest.ID)
		}
		if m.now.Sub(m.lastRequest) < 2*time.Second {
			return nil
		}
		if m.view == "logs" || m.view == "script" || m.view == "resources" {
			m.lastRequest = m.now
			return m.fetchDetail()
		}
		return m.refresh()
	case "c":
		if m.options.Demo {
			return m.probe()
		}
		if m.client == nil {
			return nil
		}
		cmd := m.client.LoginCommand()
		if cmd == nil {
			return m.probe()
		}
		m.cancelRequests()
		// Queries cancelled before the interactive login may return afterward.
		// Keep the visible data, but invalidate those replies and allow a new
		// detail query once the connection probe succeeds.
		m.detailSequence++
		m.detailBusy = false
		m.generation++
		m.busy = false
		m.connected = false
		m.hoverKnown = false
		m.pointerSuspended = true
		return tea.Exec(&pointerExec{command: cmd, reset: m.PointerReset()}, func(e error) tea.Msg { return loginMsg{err: e} })
	case "d":
		if m.options.Demo {
			m.scenario++
			return m.refresh()
		}
	}
	return nil
}

type paletteCommand struct{ label, key string }

var commands = []paletteCommand{
	{"Prepare a new job", "launch"},
	{"Shell in selected allocation", "shell"},
	{"Inspect selected job", "enter"},
	{"Filter jobs", "f"},
	{"Appearance and input settings", "settings"},
	{"Saved submission receipts", "receipts"},
	{"Browse cluster software", "software"},
	{"Queue wait history by partition", "waits"},
	{"Queue priority factors", "priority"},
	{"Node CPU, memory and GPU allocations", "nodes"},
	{"Refresh status", "r"}, {"Copy failed array tasks", "copy-failed"}, {"Read job output", "l"}, {"View submitted script", "s"}, {"Live resource samples", "u"}, {"Reconnect to Hummingbird", "c"}, {"Show active jobs", "1"}, {"Show history", "2"}, {"Show partitions", "3"}, {"Show connection details", "4"}, {"Keyboard help", "?"}, {"Quit Perch", "q"},
	{"Review cancellation of selected job/task", "cancel-job"},
	{"Review cancellation of whole array", "cancel-array"},
	{"Last cancellation receipt", "cancel-receipt"},
	{"Actions for selected job", "actions"},
	{"Activity this session", "activity"},
	{"Time warning preferences", "warnings"},
	{"Sort jobs · default", "sort:default"},
	{"Sort jobs · name A–Z", "sort:name"},
	{"Sort jobs · state A–Z", "sort:state"},
	{"Sort jobs · elapsed ↓", "sort:elapsed"},
	{"Sort jobs · CPUs ↓", "sort:cpus"},
	{"Sort jobs · memory ↓", "sort:memory"},
	{"Sort jobs · latest", "sort:submitted"},
}

func (m *Model) chipState() string {
	if m.cachedStore != nil {
		return "reconnecting"
	}
	if !m.connected {
		return "reconnecting"
	}
	if m.now.Before(m.completeUntil) {
		return "complete"
	}
	running, pending := false, false
	for _, j := range m.store.Snapshot.Jobs {
		running = running || j.Running()
		pending = pending || j.State == "PENDING"
	}
	for i := len(m.store.Events) - 1; i >= 0; i-- {
		e := m.store.Events[i]
		if m.now.Sub(e.At) < 30*time.Second && e.Kind == "failed" {
			return "attention"
		}
	}
	if running {
		return "running"
	}
	if pending {
		return "queued"
	}
	return "idle"
}
func (m *Model) statusText() string {
	if m.cachedStore != nil {
		if !m.connected {
			return "cached · disconnected"
		}
		if m.lastError != "" {
			return "cached · partial refresh"
		}
		return "cached · refreshing"
	}
	if m.busy && m.store.Snapshot.At.IsZero() {
		return "connecting"
	}
	if !m.connected {
		return "disconnected"
	}
	if m.lastError != "" {
		return "partial refresh"
	}
	if m.options.Demo {
		return "demo · offline"
	}
	return "connected"
}
func (m *Model) age() string {
	if m.view == "software" {
		if m.detailBusy {
			return "reading report…"
		}
		if m.detailError != "" {
			return "report read failed"
		}
		if m.reportAt.IsZero() {
			return "report not read"
		}
		seconds := max(0, int(m.now.Sub(m.reportAt).Seconds()))
		if seconds < 60 {
			return fmt.Sprintf("report %ds ago", seconds)
		}
		return fmt.Sprintf("report %dm ago", seconds/60)
	}
	at := m.visibleStore().Snapshot.At
	if m.cachedStore != nil {
		if m.tab == 1 || m.view == "waits" {
			at = m.cachedStore.Snapshot.HistoryAt
		}
		if m.tab == 2 {
			at = m.cachedStore.Snapshot.PartitionsAt
		}
	}
	if at.IsZero() {
		return "No snapshot yet"
	}
	seconds := max(0, int(m.now.Sub(at).Seconds()))
	if seconds < 60 {
		return fmt.Sprintf("updated %ds ago", seconds)
	}
	return fmt.Sprintf("updated %dm ago", seconds/60)
}
