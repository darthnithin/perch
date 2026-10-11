package ui

import (
	"errors"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/hummingbird"
	"strings"
	"testing"
	"time"
)

func TestChartPreservesMissingIntervals(t *testing.T) {
	now := time.Now()
	value := float64(50)
	points := []chartPoint{{now, &value}, {now.Add(8 * time.Second), nil}, {now.Add(16 * time.Second), &value}}
	view := ansi.Strip(chart(points, 40, 5))
	lines := strings.Split(view, "\n")
	if len(lines) != 5 {
		t.Fatal("chart height", len(lines))
	}
	for _, line := range lines {
		r := []rune(line)
		if len(r) != 40 {
			t.Fatal("chart width", len(r))
		}
		for _, c := range r[15:25] {
			if c != ' ' && c != '\u2800' {
				t.Fatal("missing sample was bridged", view)
			}
		}
	}
}

func TestRepeatedResourceFailuresKeepBoundedGaps(t *testing.T) {
	m := demoModel(80, 24)
	m.view = "resources"
	for i := range 120 {
		m.now = m.now.Add(8 * time.Second)
		m.Update(detailMsg{sequence: m.detailSequence, view: "resources", err: errors.New("unavailable")})
		if len(m.live) != min(i+1, 90) {
			t.Fatal("unbounded missing history", len(m.live))
		}
	}
	for _, s := range m.live {
		if s.PeakRSS != nil || s.CPUSeconds != nil {
			t.Fatal("failure invented zero")
		}
	}
	rss := int64(1 << 20)
	m.Update(detailMsg{sequence: m.detailSequence, view: "resources", sample: hummingbird.LiveSample{At: m.now.Add(8 * time.Second), PeakRSS: &rss}})
	if len(m.live) != 90 || m.live[89].PeakRSS == nil || m.detailError != "" {
		t.Fatal("recovery failed")
	}
}
func TestChartDoesNotInventInitialHistory(t *testing.T) {
	value := float64(1)
	if !strings.Contains(ansi.Strip(chart([]chartPoint{{time.Now(), &value}}, 30, 5)), "second sample") {
		t.Fatal("single sample became a history")
	}
}
