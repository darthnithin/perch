package nautilusui

import (
	tea "charm.land/bubbletea/v2"
	"errors"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/nautilus"
	"strings"
	"testing"
	"time"
)

func TestMetricsNavigationResizeAndRetainedFailures(t *testing.T) {
	for _, width := range []int{48, 80, 140} {
		m := ready(t, width, 18)
		key(m, "2")
		filter(m, "retrying-replacement")
		cmd := press(m, 'u')
		applyCommand(t, m, cmd)
		l := m.logs
		if l == nil || l.metricsSnapshot == nil {
			t.Fatal("missing resources")
		}
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "0.215 cores") || !strings.Contains(view, "146Mi working set") {
			t.Fatal(view)
		}
		for _, w := range []int{48, 80, 140} {
			m.Update(tea.WindowSizeMsg{Width: w, Height: 18})
			for _, line := range strings.Split(ansi.Strip(m.View().Content), "\n") {
				if ansi.StringWidth(line) > w {
					t.Fatal("overflow")
				}
			}
		}
		old := l.metricsSnapshot
		m.now = m.now.Add(3 * time.Second)
		m.readMetrics(false)
		m.Update(metricsMsg{l, l.sequence, nautilus.MetricsSnapshot{}, errors.New("denied\x1b[2J")})
		if l.metricsSnapshot != old || strings.Contains(l.err, "\x1b") || !strings.Contains(m.metricsContent(), "Retained sample") {
			t.Fatal("lost retained sample or error")
		}
		if !strings.Contains(ansi.Strip(m.View().Content), "Read failed · sample") {
			t.Fatal("retained sample failure must remain visible while scrolling")
		}
		m.now = m.now.Add(3 * time.Minute)
		if !strings.Contains(m.metricsContent(), "Stale sample") {
			t.Fatal("age is not independent of namespace refresh")
		}
		m.now = m.now.Add(3 * time.Second)
		late := m.readMetrics(false)()
		press(m, tea.KeyEscape)
		m.Update(late)
		if m.logs != nil || m.tab != 1 || m.filters[1] != "retrying-replacement" {
			t.Fatal("late read lost return context")
		}
	}
}

func TestJobMetricsChoosesOwnedPodAndReturns(t *testing.T) {
	m := ready(t, 80, 24)
	filter(m, "retrying")
	press(m, 'u')
	if m.logs == nil || m.logs.stage != "pods" {
		t.Fatal("no Pod picker")
	}
	key(m, "j")
	applyCommand(t, m, press(m, tea.KeyEnter))
	if m.logs.metricsSnapshot.Pod.Name != "retrying-replacement" {
		t.Fatal("wrong Pod")
	}
	press(m, tea.KeyEscape)
	if m.logs.stage != "pods" || m.logs.index != 1 {
		t.Fatal("lost Pod picker")
	}
	press(m, tea.KeyEscape)
	if m.logs != nil || m.filters[0] != "retrying" {
		t.Fatal("lost Job context")
	}
}
