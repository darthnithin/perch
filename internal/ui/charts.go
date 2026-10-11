package ui

import (
	"fmt"
	"math"
	"time"

	"github.com/NimbleMarkets/ntcharts/canvas"
	"github.com/NimbleMarkets/ntcharts/linechart"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/hummingbird"
)

type chartPoint struct {
	At    time.Time
	Value *float64
}

func (m *Model) recordSample(sample hummingbird.LiveSample) {
	m.live = append(m.live, sample)
	if m.resourceStep == "" {
		if ids := m.resourceStepIDs(); len(ids) > 0 {
			m.resourceStep = ids[0]
		}
	}
	if len(m.live) > 90 {
		m.live = m.live[len(m.live)-90:]
	}
}

// ntcharts is used only as a canvas renderer. Its Bubble Tea v1 Update/Init
// methods are never invoked; Perch's v2 model owns all input and scheduling.
// Each measured segment is drawn separately, preserving missing-data gaps.
func chart(points []chartPoint, width, height int) string {
	if len(points) < 2 {
		return muted.Render("Collecting history · a second sample will start the chart")
	}
	start, end := points[0].At, points[len(points)-1].At
	span := end.Sub(start).Seconds()
	if span <= 0 {
		return muted.Render("Waiting for a later measurement")
	}
	maximum := float64(1)
	known := 0
	for _, p := range points {
		if p.Value != nil && !math.IsNaN(*p.Value) && !math.IsInf(*p.Value, 0) && *p.Value >= 0 {
			maximum = max(maximum, *p.Value)
			known++
		}
	}
	if known == 0 {
		return muted.Render("No measurements in this interval")
	}
	m := linechart.New(width, height, 0, span, 0, maximum*1.1)
	m.SetXStep(0)
	m.SetYStep(0)
	valid := func(p chartPoint) bool {
		return p.Value != nil && !math.IsNaN(*p.Value) && !math.IsInf(*p.Value, 0) && *p.Value >= 0
	}
	for i, p := range points {
		if !valid(p) {
			continue
		}
		point := canvas.Float64Point{X: p.At.Sub(start).Seconds(), Y: *p.Value}
		m.DrawBrailleLine(point, point)
		if i > 0 && valid(points[i-1]) && p.At.Sub(points[i-1].At) <= 20*time.Second {
			previous := points[i-1]
			m.DrawBrailleLine(canvas.Float64Point{X: previous.At.Sub(start).Seconds(), Y: *previous.Value}, point)
		}
	}
	return green.Render(ansi.Strip(m.View()))
}
func memoryPoints(samples []hummingbird.LiveSample) []chartPoint {
	points := make([]chartPoint, len(samples))
	for i, s := range samples {
		points[i].At = s.At
		if s.PeakRSS != nil {
			value := float64(*s.PeakRSS) / (1 << 20)
			points[i].Value = &value
		}
	}
	return points
}
func cpuPoints(samples []hummingbird.LiveSample) []chartPoint {
	points := make([]chartPoint, len(samples))
	for i, s := range samples {
		points[i] = chartPoint{At: s.At, Value: s.CPUSeconds}
	}
	return points
}
func chartRange(samples []hummingbird.LiveSample, width int) string {
	if len(samples) == 0 {
		return ""
	}
	return muted.Render(joinLR(samples[0].At.Local().Format("15:04:05"), samples[len(samples)-1].At.Local().Format("15:04:05"), width))
}
func sampleCount(samples []hummingbird.LiveSample) string {
	known := 0
	for _, s := range samples {
		if s.PeakRSS != nil {
			known++
		}
	}
	return fmt.Sprintf("%d samples · %d with memory measurements", len(samples), known)
}
