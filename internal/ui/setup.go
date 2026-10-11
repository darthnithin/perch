package ui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/preferences"
)

// SetupModel chooses a destination before a transport, cache or notification
// session exists. It cannot query the cluster or mix data between identities.
type SetupModel struct {
	style                Model
	input                textinput.Model
	width, height, focus int
	remember, saving     bool
	problem, pointer     string
	hover                int
	cancel               context.CancelFunc
	Host                 string
}

type setupSavedMsg struct {
	host string
	err  error
}

func NewSetup(options Options) *SetupModel {
	input := textinput.New()
	input.Prompt = "› "
	input.Placeholder = "your-cruzid@hb.ucsc.edu"
	input.CharLimit = 512
	input.SetValue(options.Host)
	input.CursorEnd()
	input.SetWidth(60)
	return &SetupModel{style: Model{options: options}, input: input, width: 80, height: 24, hover: -1}
}

func (m *SetupModel) Init() tea.Cmd { return m.input.Focus() }
func (m *SetupModel) Close() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *SetupModel) submit() tea.Cmd {
	if m.saving || m.width < 48 || m.height < 18 {
		return nil
	}
	host := strings.TrimSpace(m.input.Value())
	var value preferences.Values
	if err := value.Set("host", host); err != nil {
		m.problem = "Enter user@host or an SSH config alias."
		m.focus = 0
		return m.input.Focus()
	}
	if !m.remember || m.style.options.Preferences == nil {
		m.Host = host
		return tea.Quit
	}
	m.saving, m.problem = true, ""
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	m.cancel = cancel
	store := *m.style.options.Preferences
	return func() tea.Msg {
		defer cancel()
		return setupSavedMsg{host, store.Edit(ctx, "host", &host, false)}
	}
}

func (m *SetupModel) button() string {
	if m.saving {
		return "Saving destination…"
	}
	if m.remember && m.style.options.Preferences != nil {
		return "Save & open workspace  ↵"
	}
	return "Open workspace  ↵"
}

// One geometry definition for layout and pointer targets at every size.
func (m *SetupModel) geometry() (x, y, width int) {
	width = min(72, max(0, m.width-4))
	return (m.width - width) / 2, max(3, (m.height-14)/2), width
}

func (m *SetupModel) target(p tea.Mouse) int {
	if m.style.options.NoMouse || m.saving || p.Mod != 0 || m.width < 48 || m.height < 18 {
		return -1
	}
	x, y, width := m.geometry()
	if p.X < x+2 || p.X >= x+width-2 {
		return -1
	}
	switch p.Y - y {
	case 5:
		return 0
	case 7:
		if m.style.options.Preferences != nil && p.X < x+2+len("[ ] Remember this destination") {
			return 1
		}
	case 9:
		if p.X < x+2+ansi.StringWidth(m.button()) {
			return 2
		}
	}
	return -1
}

func (m *SetupModel) focusField(focus int) tea.Cmd {
	m.focus = focus
	if focus == 0 {
		return m.input.Focus()
	}
	m.input.Blur()
	return nil
}

func (m *SetupModel) pointerCmd(target int) tea.Cmd {
	m.hover = target
	style := m.style.options.PointerStyle
	if style == "" || m.style.options.NoMouse {
		return nil
	}
	shape := "default"
	if target == 0 {
		shape = "text"
	} else if target > 0 {
		shape = "pointer"
	}
	if style == "x11" {
		if shape == "pointer" {
			shape = "hand2"
		} else {
			shape = "xterm"
		}
	}
	if m.pointer == shape {
		return nil
	}
	m.pointer = shape
	return tea.Raw(ansi.SetPointerShape(shape))
}

func (m *SetupModel) PointerReset() string {
	if m.pointer == "" {
		return ""
	}
	shape := "default"
	if m.style.options.PointerStyle == "x11" {
		shape = "xterm"
	}
	return ansi.SetPointerShape(shape)
}

func (m *SetupModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		_, _, width := m.geometry()
		m.input.SetWidth(max(1, width-8))
		return m, m.pointerCmd(-1)
	case tea.ColorProfileMsg:
		if !m.style.colorProfileKnown {
			m.style.detectedColorProfile, m.style.colorProfileKnown = msg.Profile, true
		}
		if msg.Profile != m.style.colorProfile() {
			return m, m.style.applyColorMode()
		}
		return m, nil
	case setupSavedMsg:
		if !m.saving {
			return m, nil
		}
		m.saving = false
		if msg.err != nil {
			m.problem = "Save failed. Uncheck Remember to proceed."
			return m, nil
		}
		m.Host = msg.host
		return m, tea.Quit
	case tea.BlurMsg:
		return m, m.pointerCmd(-1)
	case tea.MouseMotionMsg:
		return m, m.pointerCmd(m.target(msg.Mouse()))
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		target := m.target(msg.Mouse())
		if target < 0 {
			return m, nil
		}
		focus := m.focusField(target)
		switch target {
		case 1:
			m.remember = !m.remember
		case 2:
			return m, m.submit()
		}
		return m, focus
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			m.Close()
			return m, tea.Quit
		}
		if m.saving {
			return m, nil
		}
		switch msg.String() {
		case "tab", "shift+tab":
			delta := 1
			if msg.String() == "shift+tab" {
				delta = 2
			}
			focus := (m.focus + delta) % 3
			if focus == 1 && m.style.options.Preferences == nil {
				focus = (focus + delta) % 3
			}
			return m, tea.Batch(m.focusField(focus), m.pointerCmd(-1))
		case "enter":
			if m.focus == 1 {
				m.remember = !m.remember
				return m, nil
			}
			return m, m.submit()
		case "space":
			if m.focus == 1 {
				m.remember = !m.remember
				return m, nil
			}
			if m.focus == 2 {
				return m, m.submit()
			}
		}
	case tea.PasteMsg:
		if m.saving {
			return m, nil
		}
		if strings.ContainsAny(msg.Content, "\r\n\t") || domain.Label(msg.Content) != msg.Content {
			m.problem = "Paste one SSH destination on a single line."
			return m, nil
		}
		if m.focus == 0 {
			m.problem = ""
		}
	}
	if m.focus == 0 && !m.saving {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(message)
		if _, ok := message.(tea.KeyPressMsg); ok {
			m.problem = ""
		}
		return m, cmd
	}
	return m, nil
}

func (m *SetupModel) View() tea.View {
	if m.width < 48 || m.height < 18 {
		v := tea.NewView("Perch setup needs 48 columns × 18 rows.\nResize to continue · Esc quits.")
		v.AltScreen = true
		return v
	}
	x, y, width := m.geometry()
	remember := "[ ] Remember this destination"
	if m.remember {
		remember = "[x] Remember this destination"
	}
	if m.style.options.Preferences == nil {
		remember = "This session only · --no-config"
	}
	rememberStyle, buttonStyle := muted, accent
	if m.hover == 1 {
		rememberStyle = rememberStyle.Underline(true)
	}
	if m.hover == 2 {
		buttonStyle = buttonStyle.Underline(true)
	}
	if m.focus == 1 {
		rememberStyle = accent.Bold(true)
	}
	if m.focus == 2 {
		buttonStyle = selected
	}
	content := strings.Join([]string{
		ink.Render("Your jobs, a little closer."),
		muted.Render("Use CruzID@hb.ucsc.edu or an SSH alias."), "",
		ink.Bold(true).Render("SSH destination"), m.input.View(), "",
		rememberStyle.Render(remember), "", buttonStyle.Render(m.button()), "",
		muted.Render("Passwords stay with SSH."),
	}, "\n")
	rows := make([]string, m.height)
	rows[y-2] = strings.Repeat(" ", x) + accent.Bold(true).Render("perch") + border.Render("  │  ") + ink.Render("Welcome")
	for i, line := range strings.Split(box("Connect to Hummingbird", content, width, 13, true), "\n") {
		if y+i < m.height {
			rows[y+i] = strings.Repeat(" ", x) + line
		}
	}
	hint := "Tab moves · Enter opens · Esc quits"
	if m.focus == 1 {
		hint = "Space toggles · Tab moves · Esc quits"
	}
	if m.saving {
		hint = "Saving destination…"
	}
	if m.problem != "" {
		hint = m.problem
	}
	// Keep the guidance visible in the minimum size as well as wide screens.
	rows[min(m.height-1, y+13)] = strings.Repeat(" ", x) + yellow.Render(fit(hint, width))
	for i, row := range rows {
		rows[i] = fit(row, m.width)
	}
	text := m.style.applyTheme(strings.Join(rows, "\n"))
	if m.style.options.NoColor {
		text = ansi.Strip(text)
	}
	v := tea.NewView(text)
	v.AltScreen, v.WindowTitle = true, "Perch · Connect"
	if !m.style.options.NoMouse {
		v.MouseMode, v.ReportFocus = tea.MouseModeAllMotion, true
	}
	if !m.style.options.NoColor {
		bg, fg := m.style.canvasColors()
		v.BackgroundColor, v.ForegroundColor = lipgloss.Color(bg), lipgloss.Color(fg)
	}
	return v
}
