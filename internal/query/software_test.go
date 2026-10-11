package query

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSoftwareSearchAndModuleDemo(t *testing.T) {
	s := Service{Demo: true}
	r, e := s.Execute(context.Background(), Request{Command: "software", ID: "CUDA"})
	if e != nil || r.Source != "demo" || r.HistoryDays != 0 {
		t.Fatal(r, e)
	}
	modules := r.Data.([]Software)
	if len(modules) != 1 || modules[0].Name != "cuda/12.8.1" {
		t.Fatal(modules)
	}
	r, e = s.Execute(context.Background(), Request{Command: "software", ID: "missing"})
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(r)
	if !strings.Contains(string(raw), `"data":[]`) {
		t.Fatal(string(raw))
	}
	r, e = s.Execute(context.Background(), Request{Command: "module", ID: "cuda/12.8.1"})
	if e != nil || r.Data.(ModuleInfo).LoadCommand != "module load cuda/12.8.1" {
		t.Fatal(r, e)
	}
	b := &backend{}
	if _, e = (Service{Backend: b}).Execute(context.Background(), Request{Command: "software"}); e == nil || b.calls != 0 {
		t.Fatal("unsupported discovery refreshed jobs")
	}
	for _, req := range []Request{{Command: "module", ID: "x;id"}, {Command: "software", Stderr: true}, {Command: "software", ID: strings.Repeat("x", 161)}} {
		if Validate(req) == nil {
			t.Fatal("invalid request accepted", req)
		}
	}
}
