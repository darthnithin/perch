package hummingbird

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"perch/internal/domain"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, e := os.ReadFile("../../testdata/hummingbird/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestRealAccountingFixture(t *testing.T) {
	jobs, err := ParseAccounting(fixture(t, "accounting"))
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatal(len(jobs))
	}
	j := jobs[0]
	if j.ID != "9000_0" || j.RawID != 9001 || j.TaskID == nil || *j.TaskID != 0 || j.State != "COMPLETED" {
		t.Fatalf("%+v", j)
	}
	if j.RequestedMemory == nil || *j.RequestedMemory != 2<<30 {
		t.Fatal(j.RequestedMemory)
	}
	if j.PeakRSS == nil || *j.PeakRSS != 49397760 {
		t.Fatal(j.PeakRSS)
	}
	if j.TotalCPU == nil || math.Abs(*j.TotalCPU-490.282971) > 1e-6 {
		t.Fatal(j.TotalCPU)
	}
	if len(j.Steps) != 2 || j.Limit != 2*time.Hour || j.Elapsed != 494*time.Second {
		t.Fatal(j)
	}
	if j.Stdout != "/home/researcher/experiments/logs/sim-9000_0.out" || j.Stderr != j.Stdout {
		t.Fatal(j.Stdout, j.Stderr)
	}
	if j.ExitCode == nil || *j.ExitCode != 0 || j.Signal != nil {
		t.Fatal(j.ExitCode, j.Signal)
	}
}
func TestRealQueueAndPartitions(t *testing.T) {
	jobs, e := ParseQueue(fixture(t, "queue"), time.Unix(1791600000, 0))
	if e != nil {
		t.Fatal(e)
	}
	if len(jobs) != 3 || jobs[0].ArrayID != 9100 || jobs[0].ID != "9100_39" {
		t.Fatal(jobs)
	}
	if jobs[0].ExitCode != nil || jobs[0].RequestedMemory == nil || *jobs[0].RequestedMemory != 4<<30 {
		t.Fatal(jobs[0])
	}
	ps, e := ParsePartitions(fixture(t, "partitions"))
	if e != nil {
		t.Fatal(e)
	}
	if len(ps) != 3 {
		t.Fatal(ps)
	}
	for _, p := range ps {
		if p.Name == "128x24" && (p.NodeCount != 18 || p.CPUs != 432) {
			t.Fatalf("double-counted partition: %+v", p)
		}
	}
}
func TestUnsetInfiniteAndCollapsedArray(t *testing.T) {
	var raw map[string]any
	if e := json.Unmarshal([]byte(fixture(t, "queue")), &raw); e != nil {
		t.Fatal(e)
	}
	j := raw["jobs"].([]any)[0].(map[string]any)
	j["array_task_string"] = "0-999999%32"
	j["job_state"] = []string{"PENDING"}
	j["time_limit"] = map[string]any{"set": true, "infinite": true, "number": 999}
	j["memory_per_node"] = map[string]any{"set": false, "number": 123}
	raw["jobs"] = []any{j}
	b, _ := json.Marshal(raw)
	jobs, e := ParseQueue(string(b), time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if jobs[0].ID != "9100_[0-999999]" || jobs[0].TaskRange != "0-999999" || jobs[0].Count() != 1000000 || jobs[0].TaskID != nil || jobs[0].Limit != 0 || jobs[0].RequestedMemory != nil || jobs[0].Elapsed != 0 {
		t.Fatal(jobs[0])
	}
}
func TestSchemaFailureIsNotEmptyQueue(t *testing.T) {
	for _, raw := range []string{"{}", `{"jobs":null}`, strings.ReplaceAll(fixture(t, "queue"), "v0.0.44", "v0.0.99"), strings.Replace(fixture(t, "queue"), `"errors": []`, `"errors": [{"error":"denied"}]`, 1)} {
		if _, e := ParseQueue(raw, time.Now()); e == nil {
			t.Fatal("accepted invalid schema")
		}
	}
}
func TestLogFilenameTokens(t *testing.T) {
	task := int64(3)
	j := domain.Job{RawID: 55, ArrayID: 42, TaskID: &task, WorkDir: "/work", User: "u", Name: "experiment"}
	got, e := ResolvePath("logs/%A_%04a_%%_%j.out", j)
	if e != nil || got != "/work/logs/42_0003_%_55.out" {
		t.Fatal(got, e)
	}
	j.TaskID = nil
	if _, e := ResolvePath("%a", j); e == nil {
		t.Fatal("range accepted")
	}
	if _, e := ResolvePath("%N.out", j); e == nil {
		t.Fatal("node pattern accepted without node selection")
	}
}

func TestQueueCollapsedRangesCountUnionAndRejectReservedIndices(t *testing.T) {
	for _, expression := range []string{"0-9,5-14,0-12:2", "[0-9,5-14%4]", "[0-9,5-14]%4", "4294967294", "0-9%bad"} {
		var raw map[string]any
		if err := json.Unmarshal([]byte(fixture(t, "queue")), &raw); err != nil {
			t.Fatal(err)
		}
		j := raw["jobs"].([]any)[0].(map[string]any)
		raw["jobs"] = []any{j}
		j["array_task_id"] = map[string]any{"set": false, "number": 0}
		j["array_task_string"] = expression
		data, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		jobs, err := ParseQueue(string(data), time.Now())
		if expression == "4294967294" || expression == "0-9%bad" {
			if err == nil {
				t.Fatal("Slurm sentinel or malformed throttle accepted")
			}
		} else if err != nil || len(jobs) != 1 || jobs[0].Count() != 15 {
			t.Fatalf("wrong parsed range count: %+v %v", jobs, err)
		}
	}
}
