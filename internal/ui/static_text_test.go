package ui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func BenchmarkStaticReaders(b *testing.B)        { benchmarkStaticReaders(b, 8<<10) }
func BenchmarkStaticReadersMaximum(b *testing.B) { benchmarkStaticReaders(b, 256<<10) }
func benchmarkStaticReaders(b *testing.B, size int) {
	for _, view := range []string{"script", "software"} {
		b.Run(view, func(b *testing.B) {
			m := demoModel(80, 24)
			defer m.Close()
			m.view = view
			if view == "script" {
				m.Update(detailMsg{sequence: m.detailSequence, view: "script", text: strings.Repeat("x", size)})
			} else {
				m.softwareBody = strings.Repeat("x", size)
				m.syncSoftware()
			}
			b.ResetTimer()
			for b.Loop() {
				m.Update(frameMsg(m.now))
				_ = m.View()
			}
		})
	}
}

func TestStaticReadersLongLinesAndResize(t *testing.T) {
	for _, view := range []string{"script", "software"} {
		m := demoModel(80, 24)
		m.view = view
		plain := "first\n" + strings.Repeat("x", (256<<10)-32) + "\nlast-marker"
		if view == "script" {
			m.Update(detailMsg{sequence: m.detailSequence, view: "script", text: plain})
		} else {
			m.softwareBody = plain
			m.syncSoftware()
		}
		for _, width := range []int{80, 48, 140} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
			for _, line := range strings.Split(m.viewport.GetContent(), "\n") {
				if ansi.StringWidth(line) > m.viewport.Width() {
					t.Fatalf("%s retains oversized viewport line", view)
				}
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
			if !strings.Contains(ansi.Strip(m.View().Content), "last-marker") {
				t.Fatal(view, "lost tail")
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
			if !strings.Contains(ansi.Strip(m.View().Content), "first") {
				t.Fatal(view, "lost beginning")
			}
		}
		m.Close()
	}
}

func TestScriptFailedReadDoesNotReturnOnResize(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	m.view = "script"
	m.Update(detailMsg{sequence: m.detailSequence, view: "script", text: "old script"})
	m.Update(detailMsg{sequence: m.detailSequence, view: "script", err: fmt.Errorf("read unavailable")})
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
	got := ansi.Strip(m.View().Content)
	if strings.Contains(got, "old script") || !strings.Contains(got, "Unable to read") {
		t.Fatal(got)
	}
	m.invalidateDetail()
	if m.scriptLoaded || m.scriptText != "" || m.softwareRenderedWidth != 0 {
		t.Fatal("render cache outlived detail identity")
	}
}

func TestCompactScriptInputAndMouseGeometry(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	m.view = "script"
	m.Update(detailMsg{sequence: m.detailSequence, view: "script", text: strings.Repeat("line\n", 40)})
	if m.viewport.Height() != 6 || !m.compactTextReading() {
		t.Fatal("small script reader did not gain four lines")
	}
	if target := m.mouseTarget(tea.Mouse{X: 10, Y: m.height - 5}); target.kind == "input" {
		t.Fatal("hidden command input captured clicks")
	}
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.input.Focused() || m.compactTextReading() {
		t.Fatal("slash did not restore command input")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.compactTextReading() || m.viewport.Height() != 6 {
		t.Fatal("command cancellation lost reading space")
	}
}
