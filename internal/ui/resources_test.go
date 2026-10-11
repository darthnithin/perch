package ui

import (
	"perch/internal/hummingbird"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
)

func TestCompactResourcesKeepChartsAndCommandNavigation(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	press(m, 'u')
	rss, cpu := int64(10<<20), 5.0
	m.recordSample(hummingbird.LiveSample{At: m.now, Steps: []hummingbird.LiveStep{
		{ID: "9112.0", PeakRSS: &rss, CPUSeconds: &cpu},
		{ID: "9112.batch", PeakRSS: &rss},
	}})
	m.syncPanel()
	if m.panel.Height() != 14 || !m.compactTextReading() {
		t.Fatalf("resources have %d rows", m.panel.Height())
	}
	if target := m.mouseTarget(tea.Mouse{X: 6, Y: m.height - 5}); target.kind == "input" {
		t.Fatal("hidden command box intercepts the resource view")
	}
	selected, step := m.selectedID, m.resourceStep
	press(m, '/')
	if !m.input.Focused() || m.panel.Height() != 10 {
		t.Fatal("command entry did not restore its visible box")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.panel.Height() != 14 || m.selectedID != selected || m.resourceStep != step {
		t.Fatal("leaving command entry changed the resource selection or space")
	}
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 18})
	if m.panel.Height() != 8 {
		t.Fatal("smallest resource view did not reclaim the command rows")
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	if m.compactTextReading() || m.panel.Height() != 18 {
		t.Fatal("roomy resource view did not restore the ordinary layout")
	}
}

func TestRecordedStepsPreserveUnitsUnknownsAndNarrowLayout(t *testing.T) {
	cpu, rss := 0.0, int64(2<<20)
	steps := []domain.Step{
		{ID: "12.0", State: "COMPLETED", Tasks: 2, PeakRSS: &rss, TotalCPU: &cpu, RSSScope: "largest recorded task peak"},
		{ID: "12.extern", State: "COMPLETED"},
	}
	for _, width := range []int{40, 72, 120} {
		text := ansi.Strip(recordedSteps(steps, width))
		for _, want := range []string{"Recorded steps", "12.0", "12.extern", "0.000 s", "2.0 MiB", "unavailable", "largest recorded task peak"} {
			if !strings.Contains(text, want) {
				t.Fatal("missing", want, text)
			}
		}
		for _, line := range strings.Split(text, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatal("overflow", width, line)
			}
		}
	}
	if !strings.Contains(recordedSteps(nil, 40), "No step accounting records") {
		t.Fatal("invented steps")
	}
}

func TestStepChartsKeepIndependentHistoryAndGaps(t *testing.T) {
	m := demoModel(120, 40)
	m.view = "resources"
	at := m.now
	rss, cpu, batch := int64(10<<20), 5.0, int64(1<<20)
	m.recordSample(hummingbird.LiveSample{At: at, Steps: []hummingbird.LiveStep{
		{ID: "12.batch", PeakRSS: &batch}, {ID: "12.10", PeakRSS: &rss}, {ID: "12.2", PeakRSS: &rss, CPUSeconds: &cpu},
	}})
	if m.resourceStep != "12.2" {
		t.Fatal("prefer first numbered step", m.resourceStep)
	}
	m.recordSample(hummingbird.LiveSample{At: at.Add(time.Second), Steps: []hummingbird.LiveStep{{ID: "12.batch", PeakRSS: &batch}}})
	samples := m.resourceSamples()
	if *samples[0].PeakRSS != rss || samples[1].PeakRSS != nil || samples[1].CPUSeconds != nil {
		t.Fatal("missing step replaced with batch values", samples)
	}
	m.cycleResourceStep(true)
	if m.resourceStep != "12.10" {
		t.Fatal(m.resourceStep)
	}
	m.cycleResourceStep(true)
	if m.resourceStep != "12.batch" || *m.resourceSamples()[1].PeakRSS != batch {
		t.Fatal(m.resourceStep)
	}
	m.cycleResourceStep(true)
	if m.resourceStep != "12.2" {
		t.Fatal("cycle", m.resourceStep)
	}
	m.cycleResourceStep(false)
	if m.resourceStep != "12.batch" {
		t.Fatal("reverse cycle", m.resourceStep)
	}
	for _, width := range []int{40, 72, 120} {
		text := ansi.Strip(m.resources(domain.Job{State: "RUNNING"}, width))
		for _, line := range strings.Split(text, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatal(width, line)
			}
		}
		if !strings.Contains(text, "Step 12.batch") || !strings.Contains(text, "1.0 MiB") {
			t.Fatal(text)
		}
	}
	m.invalidateDetail()
	if m.resourceStep != "" || len(m.live) != 0 {
		t.Fatal("history leaked to another view/attempt")
	}
}
