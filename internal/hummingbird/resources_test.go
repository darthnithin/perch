package hummingbird

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"perch/internal/domain"
	"perch/internal/transport"
)

func accountingVariant(t *testing.T, edit func(map[string]any)) domain.Job {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(fixture(t, "accounting")), &data); err != nil {
		t.Fatal(err)
	}
	j := data["jobs"].([]any)[0].(map[string]any)
	edit(j)
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := ParseAccounting(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return jobs[0]
}

func TestRequestedAndAllocatedMemoryRemainDistinct(t *testing.T) {
	j := accountingVariant(t, func(j map[string]any) {
		tres := j["tres"].(map[string]any)
		tres["allocated"] = []any{map[string]any{"type": "mem", "count": 8192}}
		tres["requested"] = []any{map[string]any{"type": "mem", "count": 4096}}
	})
	if j.RequestedMemory == nil || *j.RequestedMemory != 4<<30 || j.AllocatedMemory == nil || *j.AllocatedMemory != 8<<30 {
		t.Fatal(j)
	}
	j = accountingVariant(t, func(j map[string]any) {
		j["allocation_nodes"] = 0
		tres := j["tres"].(map[string]any)
		tres["allocated"] = []any{}
		tres["requested"] = []any{map[string]any{"type": "mem", "count": 128}}
	})
	if j.RequestedMemory == nil || *j.RequestedMemory != 128<<20 || j.AllocatedMemory != nil {
		t.Fatal("unallocated request lost or invented allocation", j)
	}
}

func TestMalformedFractionalCPUAccountingIsUnavailable(t *testing.T) {
	seconds := int64(1)
	for _, micros := range []int64{-1, -1000000} {
		if (cpuTime{Seconds: &seconds, Microseconds: micros}).value() != nil {
			t.Fatal("invalid microseconds accepted", micros)
		}
	}
	if (cpuTime{Microseconds: 123}).value() != nil {
		t.Fatal("missing seconds invented measurement")
	}
	if got := (cpuTime{Seconds: &seconds, Microseconds: 250000}).value(); got == nil || *got != 1.25 {
		t.Fatal(got)
	}
	for _, micros := range []int64{1000000, 1296842, 12500000} {
		got := (cpuTime{Seconds: &seconds, Microseconds: micros}).value()
		if got == nil || *got != 1+float64(micros)/1e6 {
			t.Fatal("aggregated microseconds lost", micros, got)
		}
	}
}

func TestAccountingMemoryPeakScope(t *testing.T) {
	for _, tc := range []struct {
		task  any
		scope string
	}{{0, "task peak"}, {-1, "node peak"}, {nil, "task scope unavailable"}} {
		j := accountingVariant(t, func(j map[string]any) {
			step := j["steps"].([]any)[0].(map[string]any)
			values := step["tres"].(map[string]any)["requested"].(map[string]any)["max"].([]any)
			for _, v := range values {
				metric := v.(map[string]any)
				if metric["type"] == "mem" {
					metric["task"] = tc.task
				}
			}
		})
		if j.PeakRSS == nil || *j.PeakRSS != 49397760 || !strings.Contains(j.RSSScope, tc.scope) {
			t.Fatal(j.RSSScope, j.PeakRSS)
		}
	}
}

func TestLiveMeasurementsMatchOnlyRequestedBatchStep(t *testing.T) {
	for _, tc := range []struct {
		out   string
		known bool
	}{{"12.0|10G|10:00\n12.batch|1024K|00:02.500\n", true}, {"12.0|10G|10:00\n", false}, {"12.batch|||\n", false}} {
		f := &fakeRunner{results: []transport.Result{{Stdout: tc.out}}}
		s, err := (&Client{Runner: f}).LiveResources(context.Background(), domain.Job{RawID: 12, State: "RUNNING"})
		if err != nil {
			t.Fatal(err)
		}
		if tc.known {
			if s.PeakRSS == nil || *s.PeakRSS != 1<<20 || s.CPUSeconds == nil || *s.CPUSeconds != 2.5 {
				t.Fatal(s)
			}
		} else if s.PeakRSS != nil || s.CPUSeconds != nil {
			t.Fatal("missing batch became another step or zero", s)
		}
	}
	f := &fakeRunner{results: []transport.Result{{ExitCode: 1, Stderr: "accounting disabled"}}}
	if _, err := (&Client{Runner: f}).LiveResources(context.Background(), domain.Job{RawID: 12, State: "RUNNING"}); err == nil {
		t.Fatal("failed query became successful missing sample")
	}
}

func TestRealMultiStepCPUAndMemoryAccounting(t *testing.T) {
	jobs, err := ParseAccounting(fixture(t, "accounting-multistep"))
	if err != nil || len(jobs) != 1 {
		t.Fatal(jobs, err)
	}
	j := jobs[0]
	if j.ID != "9400" || j.TotalCPU == nil || *j.TotalCPU != 1.296842 {
		t.Fatal("Slurm's accumulated microseconds were lost", j.TotalCPU)
	}
	if got, ok := j.Efficiency(); !ok || got != 1.296842/16*100 {
		t.Fatal("two CPU allocation ratio", got, ok)
	}
	if len(j.Steps) != 3 || j.PeakRSS == nil || *j.PeakRSS != 31154176 {
		t.Fatal("steps or largest peak lost", j.Steps, j.PeakRSS)
	}
	for i, want := range []struct {
		id  string
		cpu float64
		rss int64
	}{{"9400.batch", .837101, 31154176}, {"9400.extern", 0, -1}, {"9400.0", .459741, 27254784}} {
		s := j.Steps[i]
		if s.ID != want.id || s.TotalCPU == nil || *s.TotalCPU != want.cpu {
			t.Fatal("step CPU time", s)
		}
		if want.rss < 0 && s.PeakRSS != nil || want.rss >= 0 && (s.PeakRSS == nil || *s.PeakRSS != want.rss) {
			t.Fatal("missing and measured RSS conflated", s)
		}
	}
}

func TestLiveStepsAreIndependentAndBounded(t *testing.T) {
	raw := "12.0|10M|00:07.250\n12.batch|1024K|00:00.000\n12.extern|||\n"
	f := &fakeRunner{results: []transport.Result{{Stdout: raw}}}
	s, err := (&Client{Runner: f}).LiveResources(context.Background(), domain.Job{RawID: 12, State: "RUNNING"})
	if err != nil || len(s.Steps) != 3 {
		t.Fatal(s, err)
	}
	if *s.Steps[0].PeakRSS != 10<<20 || *s.Steps[0].CPUSeconds != 7.25 || s.Steps[2].CPUSeconds != nil || s.Steps[2].PeakRSS != nil {
		t.Fatal(s.Steps)
	}
	if *s.PeakRSS != 1<<20 || *s.CPUSeconds != 0 {
		t.Fatal("legacy fields must remain batch scoped", s)
	}
	command := strings.Join(f.commands[0], " ")
	if !strings.Contains(command, "--allsteps --jobs=12 ") || strings.Contains(command, ".batch") {
		t.Fatal(command)
	}
	for _, invalid := range []string{"13.0|1M|00:00", "12.0|1M|00:00\n12.0|2M|00:01", "12.0|1M", "12.bad|1M|00:00", "12.0\x1b[2J|1M|00:00", "12.0|1M|" + strings.Repeat("0", 257)} {
		if _, err := parseLiveSteps(invalid, 12); err == nil {
			t.Fatal("accepted", invalid)
		}
	}
	var many strings.Builder
	for i := 0; i < 1025; i++ {
		fmt.Fprintf(&many, "12.%d|1M|00:00\n", i)
	}
	if _, err := parseLiveSteps(many.String(), 12); err == nil {
		t.Fatal("unbounded steps")
	}
}

func TestRealLiveStepsTreatExternSentinelAsUnavailable(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/hummingbird/live-multistep.txt")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := parseLiveSteps(string(raw), 9400)
	if err != nil || len(steps) != 3 {
		t.Fatal(steps, err)
	}
	for _, step := range steps {
		switch step.ID {
		case "9400.extern":
			if step.CPUSeconds != nil || step.PeakRSS != nil {
				t.Fatal("extern sentinel became measurement", step)
			}
		case "9400.batch":
			if step.CPUSeconds == nil || *step.CPUSeconds != 0 || step.PeakRSS == nil || *step.PeakRSS != 4776*1024 {
				t.Fatal(step)
			}
		case "9400.0":
			if step.CPUSeconds == nil || *step.CPUSeconds != 14 || step.PeakRSS == nil || *step.PeakRSS != 36428*1024 {
				t.Fatal(step)
			}
		}
	}
}
