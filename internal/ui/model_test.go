package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/demo"
	"perch/internal/hummingbird"
	"perch/internal/transport"
)

func demoModel(width, height int) *Model {
	m := New(nil, Options{Demo: true, ReducedMotion: true})
	m.now = time.Date(2026, 10, 9, 17, 0, 0, 0, time.UTC)
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m.Update(refreshMsg{result: demo.Snapshot(m.now, 0), generation: m.generation})
	return m
}
func press(m *Model, key rune) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
	return cmd
}
func TestResponsiveViewsStayWithinTerminal(t *testing.T) {
	for _, size := range [][2]int{{160, 44}, {144, 40}, {110, 32}, {80, 24}, {60, 18}, {48, 14}} {
		for _, view := range []string{"details", "inspect", "logs", "script", "resources", "partitions", "connection", "help", "commands"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], view), func(t *testing.T) {
				m := demoModel(size[0], size[1])
				m.view = view
				switch view {
				case "partitions":
					m.tab = 2
				case "connection":
					m.tab = 3
				case "help":
					m.help = true
				case "commands":
					m.palette = true
				}
				m.syncPanel()
				content := m.View().Content
				lines := strings.Split(content, "\n")
				if len(lines) > size[1] {
					t.Fatalf("height %d > %d", len(lines), size[1])
				}
				for i, line := range lines {
					if width := ansi.StringWidth(line); width > size[0] {
						t.Errorf("line %d width %d > %d", i, width, size[0])
					}
				}
				if !strings.Contains(strings.ToLower(strings.ReplaceAll(ansi.Strip(content), " ", "")), "perch") {
					t.Fatal("missing brand")
				}
			})
		}
	}
}
func TestStaleResponsesAndScopedLogs(t *testing.T) {
	m := demoModel(100, 32)
	oldJobs := len(m.store.Snapshot.Jobs)
	m.Update(refreshMsg{generation: m.generation - 1, result: hummingbird.Refresh{QueueOK: true}})
	if len(m.store.Snapshot.Jobs) != oldJobs {
		t.Fatal("old connection response applied")
	}
	cmd := press(m, 'l')
	if cmd == nil {
		t.Fatal("no log query")
	}
	reply := cmd()
	m.Update(reply)
	if !strings.Contains(m.logRaw, "parameter-sweep") {
		t.Fatal(m.logRaw)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	press(m, 'j')
	m.Update(reply)
	if m.logRaw != "" {
		t.Fatal("old job log applied to new selection")
	}
}

func TestReconnectInvalidatesPendingDetail(t *testing.T) {
	ssh, err := transport.NewSSH("example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	// No connection is opened: only construct the interactive command.
	defer ssh.Close()
	m := New(&hummingbird.Client{Runner: ssh}, Options{})
	m.detailBusy = true
	m.detailSequence = 7
	m.logRaw = "retained output"
	if press(m, 'c') == nil {
		t.Fatal("missing interactive login command")
	}
	m.Update(detailMsg{sequence: 7, view: "logs", chunk: hummingbird.LogChunk{Raw: "stale", Reset: true}})
	if m.detailBusy || m.logRaw != "retained output" || m.detailSequence <= 7 {
		t.Fatal("reconnect retained a pending detail request or accepted its stale result")
	}
}
func TestFilterAndNarrowInspection(t *testing.T) {
	m := demoModel(60, 18)
	press(m, 'f')
	for _, r := range "embedding" {
		press(m, r)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.jobs()) != 1 || m.searching {
		t.Fatal(m.jobs(), m.searching)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.view != "inspect" {
		t.Fatal(m.view)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if !m.panel.AtBottom() {
		t.Fatal("inspector did not scroll")
	}
	id := m.selectedID
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	selected, ok := m.selected()
	if !ok || selected.ID != id || m.search != "embedding" || m.view != "details" {
		t.Fatal("returning from inspection lost the filtered context", selected.ID, id)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	selected, ok = m.selected()
	if !ok || selected.ID != id || m.search != "" {
		t.Fatal("clearing filter changed the selected job", selected.ID, id)
	}
}
func TestNoColorAndStaticChip(t *testing.T) {
	m := demoModel(160, 44)
	m.options.NoColor = true
	if strings.Contains(m.View().Content, "\x1b") {
		t.Fatal("NO_COLOR contains terminal escapes")
	}
	m.options.NoColor = false
	first := m.View().Content
	m.frame += 5
	if first != m.View().Content {
		t.Fatal("reduced motion still animates")
	}
}
func TestRenderArtifacts(t *testing.T) {
	dir := os.Getenv("PERCH_RENDER_DIR")
	if dir == "" {
		t.Skip("set PERCH_RENDER_DIR to capture ANSI frames")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{160, 44}, {110, 32}, {80, 24}} {
		m := demoModel(size[0], size[1])
		name := fmt.Sprintf("demo-%dx%d.ansi", size[0], size[1])
		if err := os.WriteFile(filepath.Join(dir, name), []byte(m.View().Content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestChipAnimationDoesNotRefreshJobsAndRespectsReducedMotion(t *testing.T) {
	m := demoModel(160, 44)
	before := m.generation
	frame := m.frame
	_, cmd := m.Update(frameMsg(m.now))
	if cmd != nil || m.frame != frame {
		t.Fatal("reduced motion scheduled animation")
	}
	m.options.ReducedMotion = false
	_, cmd = m.Update(frameMsg(m.now))
	if cmd == nil || m.birdState != m.chipState() {
		t.Fatal("animation not scheduled or state not synchronized")
	}
	m.Update(frameMsg(m.now))
	if m.frame != 1 || m.generation != before {
		t.Fatal("frame did not advance independently of SSH queries")
	}
	m.options.NoColor = true
	_, cmd = m.Update(frameMsg(m.now))
	if cmd != nil {
		t.Fatal("no-color mode scheduled animation")
	}
}

func TestCompletionAnimationSettlesInsteadOfLooping(t *testing.T) {
	m := demoModel(160, 44)
	m.options.ReducedMotion = false
	m.completeUntil = m.now.Add(5120 * time.Millisecond)
	m.birdState = "complete"
	for i := 0; i < 100; i++ {
		m.Update(frameMsg(m.now))
	}
	if m.frame != 63 {
		t.Fatalf("completion wrapped to %d", m.frame)
	}
	m.now = m.completeUntil.Add(time.Second)
	m.Update(frameMsg(m.now))
	if m.birdState == "complete" || m.frame != 0 {
		t.Fatal("completion did not return to current workload state")
	}
}
