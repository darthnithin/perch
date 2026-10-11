package ui

import (
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"strings"
	"testing"
)

func TestJobGPURequestsHaveScopeAndNeverBecomeAllocations(t *testing.T) {
	m := demoModel(140, 40)
	defer m.Close()
	j := domain.Job{ID: "42", Name: "gpu-study", State: "PENDING", GPURequests: []domain.GPURequest{{Scope: "node", Count: 1}, {Scope: "task", Type: "p100", Count: 2}}}
	for _, width := range []int{40, 132} {
		text := ansi.Strip(m.details(j, width))
		if !strings.Contains(text, "1 GPU per node") || !strings.Contains(text, "2 p100 GPUs per task") || strings.Contains(text, "Allocated") {
			t.Fatal(text)
		}
		for _, line := range strings.Split(text, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatal("overflow", line)
			}
		}
	}
	text := ansi.Strip(m.glance(j, 50, 20))
	if !strings.Contains(text, "1 GPU per node requested") || !strings.Contains(text, "2 p100 GPUs per task requested") {
		t.Fatal(text)
	}
	j.GPURequests = nil
	j.GPURequestNote = "GPU request details unavailable: unsupported scheduler format."
	if text = ansi.Strip(m.details(j, 40)); !strings.Contains(text, "GPU request details unavailable") || strings.Contains(text, "0 GPU") {
		t.Fatal(text)
	}
}

func TestGlanceGPURequestsKeepWholeConstraintsAndExplainOverflow(t *testing.T) {
	requests := []domain.GPURequest{{Scope: "node", Type: "nvidia_a100_80gb_pcie", Count: 2}, {Scope: "task", Count: 1}}
	for _, width := range []int{28, 32, 48} {
		lines := glanceGPURequests(requests, width, 8)
		text := ansi.Strip(strings.Join(lines, "\n"))
		flat := strings.Join(strings.Fields(text), " ")
		if !strings.Contains(flat, "2 nvidia_a100_80gb_pcie GPUs per node requested") || !strings.Contains(flat, "1 GPU per task requested") {
			t.Fatal("lost scope or type", text)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > width {
				t.Fatal("overflow", line)
			}
		}
	}
	text := ansi.Strip(strings.Join(glanceGPURequests(requests, 28, 1), "\n"))
	if text != "2 GPU requests · enter" {
		t.Fatal("partial constraint should be deferred to inspector", text)
	}
	short := []domain.GPURequest{{Scope: "node", Count: 1}, {Scope: "task", Count: 2}, {Scope: "job", Count: 3}}
	text = ansi.Strip(strings.Join(glanceGPURequests(short, 32, 2), "\n"))
	if text != "1 GPU per node requested\n+2 GPU requests · enter" {
		t.Fatal("missing explicit overflow count", text)
	}
	if len(glanceGPURequests(requests, 32, 0)) != 0 {
		t.Fatal("must preserve lifecycle rows when no room remains")
	}
}
