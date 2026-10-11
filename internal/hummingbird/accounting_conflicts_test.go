package hummingbird

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"perch/internal/transport"
)

// Use the captured accounting envelope so this exercises the production parser,
// including its schema, identity and terminal-state normalization.
func accountingCopies(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	var first, second map[string]any
	if err := json.Unmarshal([]byte(fixture(t, "accounting")), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(fixture(t, "accounting")), &second); err != nil {
		t.Fatal(err)
	}
	j := second["jobs"].([]any)[0].(map[string]any)
	mutate(j)
	first["jobs"] = append(first["jobs"].([]any), j)
	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestAccountingRejectsContradictoryOutcomeForSameAttempt(t *testing.T) {
	raw := accountingCopies(t, func(j map[string]any) {
		j["state"].(map[string]any)["current"] = []string{"FAILED"}
	})
	for _, reverse := range []bool{false, true} {
		if reverse {
			var response map[string]any
			_ = json.Unmarshal([]byte(raw), &response)
			jobs := response["jobs"].([]any)
			jobs[0], jobs[1] = jobs[1], jobs[0]
			data, _ := json.Marshal(response)
			raw = string(data)
		}
		jobs, err := ParseAccounting(raw)
		if err == nil || len(jobs) != 0 || !strings.Contains(err.Error(), "conflicting accounting outcomes") {
			t.Fatalf("conflicting outcome was accepted (reverse=%t): %d jobs, %v", reverse, len(jobs), err)
		}
	}
	client := Client{User: "researcher", Runner: &fakeRunner{results: []transport.Result{
		{Stdout: fixture(t, "queue")}, {Stdout: raw}, {Stdout: fixture(t, "partitions")},
	}}}
	r, err := client.Refresh(context.Background(), true, true)
	if err != nil || !r.QueueOK || r.HistoryOK || !r.PartitionsOK || r.Errors["history"] == nil || len(r.History) != 0 {
		t.Fatalf("conflict must fail only accounting: %+v, %v", r, err)
	}
}

func TestAccountingDuplicateOutcomesKeepDistinctAttempts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"exact duplicate", func(j map[string]any) {}},
		{"retry", func(j map[string]any) {
			j["restart_cnt"] = 1
			j["state"].(map[string]any)["current"] = []string{"FAILED"}
		}},
		{"reused ID", func(j map[string]any) {
			tm := j["time"].(map[string]any)
			tm["submission"] = tm["submission"].(float64) + 3600
			j["state"].(map[string]any)["current"] = []string{"FAILED"}
		}},
		{"different cluster", func(j map[string]any) {
			j["cluster"] = "another-cluster"
			j["state"].(map[string]any)["current"] = []string{"FAILED"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobs, err := ParseAccounting(accountingCopies(t, tc.mutate))
			if err != nil || len(jobs) != 2 {
				t.Fatalf("valid duplicate/attempt history rejected: %d jobs, %v", len(jobs), err)
			}
		})
	}
}

func TestAccountingRejectsAmbiguousAttemptIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"allocation ID", func(j map[string]any) { j["job_id"] = j["job_id"].(float64) + 1 }},
		{"missing submission", func(j map[string]any) { j["time"].(map[string]any)["submission"] = 0 }},
		{"unset submission sentinel", func(j map[string]any) { j["time"].(map[string]any)["submission"] = 4294967294 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := accountingCopies(t, tc.mutate)
			for _, reverse := range []bool{false, true} {
				if reverse {
					var response map[string]any
					_ = json.Unmarshal([]byte(raw), &response)
					jobs := response["jobs"].([]any)
					jobs[0], jobs[1] = jobs[1], jobs[0]
					data, _ := json.Marshal(response)
					raw = string(data)
				}
				jobs, err := ParseAccounting(raw)
				if err == nil || len(jobs) != 0 || !strings.Contains(err.Error(), "ambiguous accounting identity") {
					t.Fatalf("ambiguous identity accepted (reverse=%t): %d jobs, %v", reverse, len(jobs), err)
				}
			}
		})
	}
}

func TestAccountingMissingTimeAloneIsNotAnIdentityConflict(t *testing.T) {
	raw := accountingCopies(t, func(j map[string]any) {})
	var response map[string]any
	_ = json.Unmarshal([]byte(raw), &response)
	for _, record := range response["jobs"].([]any) {
		record.(map[string]any)["time"].(map[string]any)["submission"] = 0
	}
	for _, duplicate := range []bool{false, true} {
		copy := response
		if !duplicate {
			copy = map[string]any{}
			for k, v := range response {
				copy[k] = v
			}
			copy["jobs"] = response["jobs"].([]any)[:1]
		}
		data, _ := json.Marshal(copy)
		jobs, err := ParseAccounting(string(data))
		if err != nil || len(jobs) == 0 || !jobs[0].Submit.IsZero() {
			t.Fatalf("missing time must remain missing without inventing a conflict: %+v, %v", jobs, err)
		}
	}
}

func TestAccountingRangeIDsCannotHideMissingSubmissionConflict(t *testing.T) {
	raw := accountingCopies(t, func(j map[string]any) {
		j["time"].(map[string]any)["submission"] = 0
	})
	var response map[string]any
	_ = json.Unmarshal([]byte(raw), &response)
	rows := response["jobs"].([]any)
	for i, row := range rows {
		j := row.(map[string]any)
		j["job_id"] = 9000
		a := j["array"].(map[string]any)
		a["task_id"].(map[string]any)["set"] = false
		a["task"] = []string{"0-10", "5-15"}[i]
	}
	for _, reverse := range []bool{false, true} {
		if reverse {
			rows[0], rows[1] = rows[1], rows[0]
		}
		data, _ := json.Marshal(response)
		jobs, err := ParseAccounting(string(data))
		if err == nil || len(jobs) != 0 || !strings.Contains(err.Error(), "overlapping members mix known and missing submission times") {
			t.Fatalf("different range IDs bypassed identity validation: %v, %v", jobs, err)
		}
	}
}
