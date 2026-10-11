package query

import (
	"context"
	"testing"
)

func TestPriorityDemoAndValidation(t *testing.T) {
	r, err := (Service{Demo: true}).Execute(context.Background(), Request{Command: "priority"})
	if err != nil || r.Source != "demo" || r.HistoryDays != 0 || len(r.Data.([]Priority)) != 2 {
		t.Fatal(r, err)
	}
	for _, q := range []Request{{Command: "priority", ID: "1"}, {Command: "priority", Stderr: true}} {
		if Validate(q) == nil {
			t.Fatal("invalid priority query", q)
		}
	}
	b := &backend{}
	if _, err = (Service{Backend: b}).Execute(context.Background(), Request{Command: "priority"}); err == nil || b.calls != 0 {
		t.Fatal("unsupported report refreshed queue")
	}
}
