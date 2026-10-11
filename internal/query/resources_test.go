package query

import (
	"context"
	"encoding/json"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"strings"
	"testing"
	"time"
)

func TestResourceOutputPreservesAllocationAndCaveat(t *testing.T) {
	requested, allocated, cpu := int64(1<<30), int64(2<<30), 10.5
	out := job(domain.Job{State: "TIMEOUT", CPUs: 2, Elapsed: 10 * time.Second, RequestedMemory: &requested, AllocatedMemory: &allocated, TotalCPU: &cpu})
	if out.RequestedMemoryBytes == nil || *out.RequestedMemoryBytes != requested || out.AllocatedMemoryBytes == nil || *out.AllocatedMemoryBytes != allocated {
		t.Fatal(out)
	}
	if out.CPUEfficiencyPercent == nil || *out.CPUEfficiencyPercent != 52.5 || out.CPUEfficiencyNote == "" {
		t.Fatal(out)
	}
	out = job(domain.Job{CPUs: 2, Elapsed: 10 * time.Second, TotalCPU: &cpu, Restarts: 1})
	if out.CPUEfficiencyPercent != nil || out.CPUEfficiencyNote == "" {
		t.Fatal("ambiguous attempt returned unqualified ratio")
	}
}

func TestGPURequestsReachJSONWithoutSummingOrAliasing(t *testing.T) {
	source := domain.Job{GPURequests: []domain.GPURequest{{Scope: "node", Type: "p100", Count: 2}, {Scope: "task", Count: 1}}}
	out := job(source)
	if len(out.GPURequests) != 2 || out.GPURequests[0].Scope != "node" || out.GPURequests[1].Scope != "task" {
		t.Fatal(out)
	}
	source.GPURequests[0].Count = 99
	if out.GPURequests[0].Count != 2 {
		t.Fatal("query result aliases mutable input")
	}
	raw, err := json.Marshal(job(domain.Job{}))
	if err != nil || !strings.Contains(string(raw), `"gpu_requests":null`) {
		t.Fatal(string(raw), err)
	}
	out = job(domain.Job{GPURequestNote: "Unavailable\x1b[2J"})
	if out.GPURequests != nil || out.GPURequestNote != "Unavailable" {
		t.Fatal(out)
	}
}

func TestResourceStepsReachSharedServiceWithoutInventedMeasurements(t *testing.T) {
	cpu, rss := 0.0, int64(4096)
	b := &backend{refresh: hummingbird.Refresh{QueueOK: true, HistoryOK: true, History: []domain.Job{{
		ID: "12", State: "COMPLETED", Steps: []domain.Step{
			{ID: "12.0\x1b[2J", State: "COMPLETED", Tasks: 2, PeakRSS: &rss, TotalCPU: &cpu, RSSScope: "largest recorded task peak"},
			{ID: "12.extern", State: "COMPLETED"},
		},
	}}}}
	r, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "resources", ID: "12"})
	if err != nil {
		t.Fatal(err)
	}
	got := r.Data.(Resources)
	if len(got.Steps) != 2 || got.Steps[0].ID != "12.0" || *got.Steps[0].Tasks != 2 || got.Steps[0].TotalCPUSeconds == nil || *got.Steps[0].TotalCPUSeconds != 0 {
		t.Fatal(got.Steps)
	}
	if got.Steps[1].Tasks != nil || got.Steps[1].PeakRSSBytes != nil || got.Steps[1].TotalCPUSeconds != nil {
		t.Fatal("unknown became zero", got.Steps[1])
	}
	raw, err := json.Marshal(got)
	if err != nil || !strings.Contains(string(raw), `"total_cpu_seconds":null`) || strings.Contains(string(raw), `\u001b`) {
		t.Fatal(string(raw), err)
	}
}

func TestPendingJSONSeparatesEstimatedAndActualStart(t *testing.T) {
	at := time.Unix(1800000000, 0)
	out := job(domain.Job{State: "PENDING", EstimatedStart: at})
	raw, err := json.Marshal(out)
	if err != nil || out.Start != nil || out.EstimatedStart == nil || !out.EstimatedStart.Equal(at) || !strings.Contains(string(raw), `"started_at":null`) {
		t.Fatal(string(raw), err)
	}
	out = job(domain.Job{State: "RUNNING", Start: at})
	if out.Start == nil || out.EstimatedStart != nil {
		t.Fatal(out)
	}
}

type liveStepBackend struct {
	backend
	sample hummingbird.LiveSample
}

func (b *liveStepBackend) LiveResources(context.Context, domain.Job) (hummingbird.LiveSample, error) {
	return b.sample, nil
}

func TestLiveStepsReachResourceJSONWithBatchCompatibility(t *testing.T) {
	rss, cpu := int64(32<<20), 12.5
	batch := int64(1 << 20)
	b := &liveStepBackend{backend: backend{refresh: hummingbird.Refresh{QueueOK: true, HistoryOK: true, Queue: []domain.Job{{ID: "12", RawID: 12, State: "RUNNING"}}}}, sample: hummingbird.LiveSample{At: time.Now(), PeakRSS: &batch, Scope: "batch step", Steps: []hummingbird.LiveStep{{ID: "12.0", PeakRSS: &rss, CPUSeconds: &cpu}, {ID: "12.extern"}}}}
	r, err := (Service{Backend: b}).Execute(context.Background(), Request{Command: "resources", ID: "12"})
	if err != nil {
		t.Fatal(err)
	}
	got := r.Data.(Resources)
	if len(got.LiveSteps) != 2 || *got.LivePeakRSSBytes != batch || *got.LiveSteps[0].PeakRSSBytes != rss || got.LiveSteps[1].AverageTaskCPUSeconds != nil {
		t.Fatal(got)
	}
	raw, err := json.Marshal(got)
	if err != nil || !strings.Contains(string(raw), `"average_task_cpu_seconds":null`) || !strings.Contains(string(raw), `"live_steps"`) {
		t.Fatal(string(raw), err)
	}
}
