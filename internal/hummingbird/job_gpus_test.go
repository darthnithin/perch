package hummingbird

import (
	"encoding/json"
	"perch/internal/domain"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGPURequestsPreserveOverlappingScopesAndTypes(t *testing.T) {
	got, note := jobGPURequests("gres/gpu=4", "cpu=1,gres/gpu:p100:2", "gres/gpu:1", "gres/gpu:a100=1")
	want := []domain.GPURequest{{Scope: "job", Count: 4}, {Scope: "node", Type: "p100", Count: 2}, {Scope: "socket", Count: 1}, {Scope: "task", Type: "a100", Count: 1}}
	if note != "" || !reflect.DeepEqual(got, want) {
		t.Fatal(got, note)
	}
	// Generic and typed constraints are separate statements, not an additive total.
	got, note = jobGPURequests("", "gres/gpu:2,gres/gpu:p100:2", "", "")
	if note != "" || len(got) != 2 || got[0].Count != 2 || got[1].Count != 2 {
		t.Fatal(got, note)
	}
}

func TestGPURequestMissingZeroAndMalformedAreDistinct(t *testing.T) {
	for _, raw := range []string{"", "cpu=1", "gres/gpuutil:10", "gres/gpumem:100"} {
		got, note := jobGPURequests("", raw, "", "")
		if got != nil || note != "" {
			t.Fatal(raw, got, note)
		}
	}
	got, note := jobGPURequests("", "gres/gpu:0", "", "")
	if len(got) != 1 || got[0].Count != 0 || note != "" {
		t.Fatal(got, note)
	}
	for _, raw := range []string{"gres/gpu", "gres/gpu:-1", "gres/gpu:1K", "gres/gpu:18446744073709551615", "gres/gpu:2,gres/gpu:3", "gres/gpu:p100:1,gres/gpu:p100:2", "gres/gpu:1\x1b[2J", strings.Repeat("x", 4097)} {
		got, note := jobGPURequests("", raw, "", "")
		if got != nil || note == "" {
			t.Fatal(raw, got, note)
		}
	}
}

func TestQueueGPURequestDoesNotDependOnAggregateTRES(t *testing.T) {
	var response map[string]any
	if err := json.Unmarshal([]byte(fixture(t, "queue")), &response); err != nil {
		t.Fatal(err)
	}
	job := response["jobs"].([]any)[0].(map[string]any)
	// Observed in owned Hummingbird probe 803377: aggregate requests omit GPU.
	job["tres_per_node"] = "gres/gpu:1"
	job["tres_req_str"] = "cpu=1,mem=512M,node=1,billing=1"
	job["tres_alloc_str"] = ""
	job["job_state"] = []string{"PENDING"}
	raw, _ := json.Marshal(response)
	got, err := ParseQueue(string(raw), time.Now())
	if err != nil || len(got[0].GPURequests) != 1 || got[0].GPURequests[0].Scope != "node" || got[0].GPURequests[0].Count != 1 || got[0].State != "PENDING" {
		t.Fatal(got, err)
	}
	job["tres_per_node"] = "gres/gpu:broken"
	raw, _ = json.Marshal(response)
	got, err = ParseQueue(string(raw), time.Now())
	if err != nil || got[0].GPURequests != nil || got[0].GPURequestNote == "" {
		t.Fatal("optional malformed GPU metadata should not erase the job", got, err)
	}
}
