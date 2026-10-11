package ui

import (
	"fmt"
	"perch/internal/hummingbird"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
)

func recordedSteps(steps []domain.Step, width int) string {
	lines := []string{accent.Bold(true).Render("Recorded steps"), muted.Render(ansi.Wordwrap("Each peak belongs to its step; peaks are not summed into whole-job memory usage.", width, " "))}
	if len(steps) == 0 {
		return strings.Join(append(lines, "", muted.Render("No step accounting records observed.")), "\n")
	}
	for _, s := range steps {
		lines = append(lines, "", joinLR(ink.Bold(true).Render(domain.Label(s.ID)), stateLabel(domain.Job{State: s.State}), width))
		tasks, cpu, rss := "unavailable", "unavailable", "unavailable"
		if s.Tasks > 0 {
			tasks = fmt.Sprint(s.Tasks)
		}
		if s.TotalCPU != nil {
			cpu = fmt.Sprintf("%.3f s", *s.TotalCPU)
		}
		if s.PeakRSS != nil {
			rss = domain.Bytes(*s.PeakRSS)
		}
		lines = append(lines, joinLR(muted.Render("Tasks"), ink.Render(tasks), width),
			joinLR(muted.Render("Recorded CPU time"), ink.Render(cpu), width),
			joinLR(muted.Render("Peak RSS"), ink.Render(rss), width))
		if s.RSSScope != "" {
			lines = append(lines, muted.Render(ansi.Wordwrap(domain.Label(s.RSSScope), width, " ")))
		}
	}
	return strings.Join(lines, "\n")
}

// Step choice is local: switching charts never triggers another scheduler query.
func (m *Model) resourceStepIDs() []string {
	seen := map[string]bool{}
	ids := []string{}
	for _, sample := range m.live {
		for _, step := range sample.Steps {
			if !seen[step.ID] {
				seen[step.ID] = true
				ids = append(ids, step.ID)
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a := ids[i][strings.LastIndex(ids[i], ".")+1:]
		b := ids[j][strings.LastIndex(ids[j], ".")+1:]
		ai, ae := strconv.ParseUint(a, 10, 64)
		bi, be := strconv.ParseUint(b, 10, 64)
		if ae == nil && be == nil {
			return ai < bi
		}
		if ae == nil {
			return true
		}
		if be == nil {
			return false
		}
		return a < b
	})
	return ids
}

func (m *Model) cycleResourceStep(forward bool) {
	ids := m.resourceStepIDs()
	if len(ids) == 0 {
		return
	}
	index := 0
	for i, id := range ids {
		if id == m.resourceStep {
			index = i
			break
		}
	}
	if forward {
		index = (index + 1) % len(ids)
	} else {
		index = (index + len(ids) - 1) % len(ids)
	}
	m.resourceStep = ids[index]
	m.panel.GotoTop()
}

func (m *Model) resourceSamples() []hummingbird.LiveSample {
	out := make([]hummingbird.LiveSample, len(m.live))
	for i, sample := range m.live {
		out[i].At = sample.At
		for _, step := range sample.Steps {
			if step.ID == m.resourceStep {
				out[i].PeakRSS, out[i].CPUSeconds = step.PeakRSS, step.CPUSeconds
				break
			}
		}
	}
	return out
}
