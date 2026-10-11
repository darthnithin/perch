package query

import (
	"context"
	"testing"
)

func TestNodeReportQuery(t *testing.T) {
	r, e := (Service{Demo: true}).Execute(context.Background(), Request{Command: "nodes"})
	if e != nil || r.Source != "demo" || r.HistoryDays != 0 || len(r.Data.([]Node)) != 20 {
		t.Fatal(r, e)
	}
	for _, r := range []Request{{Command: "nodes", ID: "node-06"}, {Command: "nodes", Stderr: true}} {
		if Validate(r) == nil {
			t.Fatal(r)
		}
	}
	b := &backend{}
	if _, e := (Service{Backend: b}).Execute(context.Background(), Request{Command: "nodes"}); e == nil || b.calls != 0 {
		t.Fatal("node report caused unrelated refresh")
	}
}
