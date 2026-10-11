package hummingbird

import (
	"encoding/json"
	"testing"
	"time"
)

func TestQueueEstimateIsNotAnActualStart(t *testing.T) {
	now := time.Unix(1800000000, 0)
	for _, state := range []string{"PENDING", "RUNNING"} {
		for _, available := range []bool{true, false} {
			var input map[string]any
			if err := json.Unmarshal([]byte(fixture(t, "queue")), &input); err != nil {
				t.Fatal(err)
			}
			j := input["jobs"].([]any)[0].(map[string]any)
			start := now.Add(-time.Minute)
			if state == "PENDING" {
				start = now.Add(time.Hour)
			}
			j["job_state"] = []string{state}
			j["start_time"] = map[string]any{"set": available, "infinite": false, "number": start.Unix()}
			raw, _ := json.Marshal(input)
			jobs, err := ParseQueue(string(raw), now)
			if err != nil {
				t.Fatal(err)
			}
			got := jobs[0]
			if state == "PENDING" {
				if !got.Start.IsZero() || got.Elapsed != 0 || got.EstimatedStart.IsZero() == available {
					t.Fatal("prediction became execution", got)
				}
				if available && !got.EstimatedStart.Equal(start) {
					t.Fatal("estimate changed")
				}
			} else if !got.EstimatedStart.IsZero() || got.Start.IsZero() == available {
				t.Fatal("running timestamp became an estimate", got)
			}
		}
	}
}
