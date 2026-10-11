package main

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"perch/assets/bird"
)

func TestPlaybackPreservesEveryRunningFrameAndSlowerOtherStates(t *testing.T) {
	m := model{width: 120}
	for i := 1; i <= 128; i++ {
		next, cmd := m.Update(tick(time.Time{}))
		m = next.(model)
		if cmd == nil || m.runningFrame != i%64 || m.frame != (i/2)%64 {
			t.Fatalf("tick %d skipped or mistimed a source frame: %+v", i, m)
		}
	}
	if bird.FrameInterval("running") != 40*time.Millisecond || bird.FrameInterval("idle") != 80*time.Millisecond {
		t.Fatal("terminal playback cadence changed")
	}
}

func TestPauseAndManualStepAlignTheComparison(t *testing.T) {
	m := model{width: 120, frame: 7, runningFrame: 15, halfTick: true, paused: true}
	next, _ := m.Update(tick(time.Time{}))
	if next.(model) != m {
		t.Fatal("paused review advanced")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = next.(model)
	if !m.paused || m.frame != 8 || m.runningFrame != 8 || m.halfTick {
		t.Fatal("stepping did not align clocks", m)
	}
	if strings.Count(m.View().Content, "#08") != 6 {
		t.Fatal("comparison omitted a source frame")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if next.(model).frame != 7 || next.(model).runningFrame != 7 {
		t.Fatal("reverse stepping did not align clocks")
	}
}

func TestSingleStatePlaybackAdvancesOncePerOwnInterval(t *testing.T) {
	for _, state := range states {
		m := model{sampleState: state, frame: 63}
		next, cmd := m.Update(tick(time.Time{}))
		if cmd == nil || next.(model).frame != 0 {
			t.Fatal("single-state review skipped its loop boundary", state)
		}
		if !strings.Contains(next.(model).View().Content, state+" #00") {
			t.Fatal("single-state view used the wrong frame", state)
		}
	}
}
