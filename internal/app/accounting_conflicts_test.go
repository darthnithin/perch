package app

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestConflictingAccountingCannotEmitOutcome(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/hummingbird/accounting.json")
	if err != nil {
		t.Fatal(err)
	}
	history, err := hummingbird.ParseAccounting(string(raw))
	if err != nil || len(history) != 1 {
		t.Fatalf("fixture: %v", err)
	}
	active := history[0]
	active.State = "RUNNING"
	store := NewStore()
	at := time.Now()
	store.Apply(hummingbird.Refresh{At: at, QueueOK: true, HistoryOK: true, Queue: []domain.Job{active}})
	var response map[string]any
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	var copyResponse map[string]any
	_ = json.Unmarshal(raw, &copyResponse)
	conflict := copyResponse["jobs"].([]any)[0].(map[string]any)
	conflict["state"].(map[string]any)["current"] = []string{"FAILED"}
	response["jobs"] = append(response["jobs"].([]any), conflict)
	conflictingRaw, _ := json.Marshal(response)
	rows, parseErr := hummingbird.ParseAccounting(string(conflictingRaw))
	if parseErr == nil {
		t.Fatal("contradiction accepted")
	}
	for i := 1; i <= 2; i++ {
		events := store.Apply(hummingbird.Refresh{At: at.Add(time.Duration(i) * time.Minute), QueueOK: true,
			HistoryOK: false, History: rows, Errors: map[string]error{"history": parseErr}})
		if len(events) != 0 || len(store.Snapshot.Jobs) != 1 || store.Snapshot.Jobs[0].State != "UNKNOWN" {
			t.Fatalf("conflict became an outcome: %+v, %+v", store.Snapshot.Jobs, events)
		}
	}
	events := store.Apply(hummingbird.Refresh{At: at.Add(3 * time.Minute), QueueOK: true, HistoryOK: true, History: history})
	if len(events) != 1 || events[0].State != "COMPLETED" || events[0].Attempt != active.AttemptKey() {
		t.Fatalf("confirmed recovery lost exact-attempt completion: %+v", events)
	}
}
