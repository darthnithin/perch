// Chip's developer review: approved original-derived octant artwork in a terminal.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"perch/assets/bird"
)

var states = []string{"idle", "running", "queued", "complete", "attention", "reconnecting"}

type tick time.Time
type model struct {
	frame, width       int
	runningFrame       int
	slowElapsed        time.Duration
	paused             bool
	sampleState, label string
}

func (m model) next() tea.Cmd {
	state := m.sampleState
	if state == "" {
		state = "running" // Review all states on the shortest playback interval.
	}
	return tea.Tick(bird.FrameInterval(state), func(t time.Time) tea.Msg { return tick(t) })
}
func (m model) Init() tea.Cmd { return m.next() }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
	case tea.KeyPressMsg:
		switch v.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "space":
			m.paused = !m.paused
		case "right":
			m.paused = true
			m.frame = (m.frame + 1) % 64
			m.runningFrame, m.slowElapsed = m.frame, 0
		case "left":
			m.paused = true
			m.frame = (m.frame + 63) % 64
			m.runningFrame, m.slowElapsed = m.frame, 0
		}
	case tick:
		if !m.paused {
			if m.sampleState != "" {
				m.frame = (m.frame + 1) % 64
			} else {
				m.runningFrame = (m.runningFrame + 1) % 64
				m.slowElapsed += bird.FrameInterval("running")
				if m.slowElapsed >= bird.FrameInterval("idle") {
					m.slowElapsed -= bird.FrameInterval("idle")
					m.frame = (m.frame + 1) % 64
				}
			}
		}
		return m, m.next()
	}
	return m, nil
}
func card(state string, frame int) string {
	rows := bird.OctantFrame(state, frame, false)
	lines := []string{fmt.Sprintf("\x1b[38;2;230;173;143m%s #%02d · %d ms\x1b[m", state, frame, bird.FrameInterval(state).Milliseconds()), ""}
	for _, row := range rows[:len(rows)-1] {
		lines = append(lines, "   "+row)
	}
	top := "╭" + strings.Repeat("─", 34) + "╮"
	for x := 0; x < ansi.StringWidth(rows[len(rows)-1]); x++ {
		cell := ansi.Cut(rows[len(rows)-1], x, x+1)
		if strings.TrimSpace(ansi.Strip(cell)) != "" {
			top = ansi.Cut(top, 0, 3+x) + cell + ansi.Cut(top, 4+x, 36)
		}
	}
	lines = append(lines, top, fmt.Sprintf("│ %-32s │", "My jobs"), fmt.Sprintf("│ %-32s │", ""), fmt.Sprintf("│ %-32s │", "9100   parameter-sweep"), "╰"+strings.Repeat("─", 34)+"╯")
	for i := range lines {
		lines[i] += strings.Repeat(" ", max(0, 36-ansi.StringWidth(lines[i])))
	}
	return strings.Join(lines, "\n")
}
func (m model) View() tea.View {
	if m.sampleState != "" {
		motion := "fixed pose"
		if !m.paused {
			motion = "playing"
		}
		heading := "Chip · " + m.label
		if m.label == "" {
			heading = "Chip · single-pose rendering test"
		}
		content := heading + "\n\n" + card(m.sampleState, m.frame) + fmt.Sprintf("\n\n%s #%02d · %s\nspace animate · arrows step · q quit", m.sampleState, m.frame, motion)
		v := tea.NewView(content)
		v.AltScreen = true
		v.BackgroundColor = lipgloss.Color("#1c1b1a")
		return v
	}
	columns := min(3, max(1, m.width/40))
	var chunks []string
	for i := 0; i < len(states); i += columns {
		var group []string
		for j := i; j < min(i+columns, len(states)); j++ {
			frame := m.frame
			if states[j] == "running" {
				frame = m.runningFrame
			}
			group = append(group, card(states[j], frame))
		}
		chunks = append(chunks, lipgloss.JoinHorizontal(lipgloss.Top, interleave(group, "   ")...))
	}
	state := "playing"
	toggle := "pause"
	if m.paused {
		state = "paused"
		toggle = "play"
	}
	v := tea.NewView("Chip · terminal drawing review\n\n" + strings.Join(chunks, "\n\n") + fmt.Sprintf("\n\n%s · space %s · ←/→ align and step · q quit", state, toggle))
	v.AltScreen = true
	v.BackgroundColor = lipgloss.Color("#1c1b1a")
	return v
}
func interleave(in []string, sep string) []string {
	var out []string
	for i, s := range in {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, s)
	}
	return out
}
func main() {
	asJSON := flag.Bool("json", false, "export the same review frames as ANSI JSON")
	static := flag.Bool("static", false, "start paused")
	sheet := flag.Bool("sheet", false, "print every animation frame into terminal scrollback")
	state := flag.String("state", "all", "sheet sprite: all, idle, running, queued, complete, attention, reconnecting")
	columns := flag.Int("columns", 0, "sheet columns (1–12); default fits the current terminal")
	sample := flag.Bool("sample", false, "review one sprite in a small fixed-pose TUI")
	plain := flag.Bool("print", false, "print the sample into scrollback instead of a TUI")
	label := flag.String("label", "", "sample profile label")
	flag.Parse()
	fail := func(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
	if flag.NArg() != 0 {
		fail("unexpected positional argument")
	}
	if *sample {
		if *sheet || *asJSON || *columns != 0 {
			fail("--sample cannot be combined with --sheet, --json or --columns")
		}
		selected := *state
		if selected == "all" {
			selected = "idle"
		}
		valid := false
		for _, s := range states {
			if s == selected {
				valid = true
			}
		}
		if !valid {
			fail("invalid sample state")
		}
		m := model{width: 48, paused: true, sampleState: selected, label: *label}
		if *plain {
			fmt.Println(m.View().Content)
			return
		}
		if _, err := tea.NewProgram(m).Run(); err != nil {
			fail(err.Error())
		}
		return
	}
	if *plain || *label != "" {
		fail("--print and --label require --sample")
	}
	if *sheet {
		if *asJSON {
			fail("choose --sheet or --json")
		}
		selected := states
		if *state != "all" {
			found := false
			for _, s := range states {
				if s == *state {
					found = true
				}
			}
			if !found {
				fail("unknown state; choose idle, running, queued, complete, attention, or reconnecting")
			}
			selected = []string{*state}
		}
		if *columns < 0 || *columns > 12 {
			fail("columns must be between 1 and 12 (or omitted)")
		}
		if *columns == 0 {
			width, _, err := term.GetSize(os.Stdout.Fd())
			if err != nil {
				width = 100
			}
			*columns = min(12, max(1, (width+2)/(sheetCardWidth+2)))
		}
		if err := printSheet(os.Stdout, selected, *columns); err != nil {
			fail(err.Error())
		}
		return
	}
	if *state != "all" || *columns != 0 {
		fail("--state and --columns require --sheet")
	}

	if *asJSON {
		frames := make([]string, 64)
		for i := range frames {
			frames[i] = (model{frame: i, runningFrame: i, width: 120, paused: true}).View().Content
		}
		if err := json.NewEncoder(os.Stdout).Encode(frames); err != nil {
			panic(err)
		}
		return
	}
	if _, err := tea.NewProgram(model{width: 120, paused: *static}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
