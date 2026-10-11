package hummingbird

import (
	"context"
	"encoding/json"
	"perch/internal/transport"
	fixtures "perch/testdata/hummingbird"
	"strings"
	"testing"
)

func TestNodeAllocationsLiveFixture(t *testing.T) {
	rows, err := ParseNodes(fixtures.Nodes)
	if err != nil || len(rows) != 20 {
		t.Fatal(rows, err)
	}
	n := rows[0]
	if n.Name != "node-06" || *n.CPUs != 24 || *n.AllocatedCPUs != 9 || *n.IdleCPUs != 15 || *n.AllocatedMemory != 120*(1<<30) || *n.UnallocatedMemory != 0 || strings.Join(n.States, ",") != "MIXED,PLANNED" {
		t.Fatal(n)
	}
	for _, n := range rows {
		if n.Name == "node-24" && (!strings.Contains(n.GRES, "gpu:p100:4") || !strings.Contains(n.GRESUsed, "gpu:p100:4")) {
			t.Fatal(n)
		}
	}
}
func nodeEnvelope(row map[string]any) string {
	r := map[string]any{"meta": map[string]any{"plugin": map[string]any{"data_parser": "data_parser/" + Parser}}, "nodes": []any{row}, "errors": []any{}}
	b, _ := json.Marshal(r)
	return string(b)
}
func TestNodeMemoryUnknownReservedAndOverallocated(t *testing.T) {
	for _, tc := range []struct {
		allocated, reserved any
		want                *int64
	}{{nil, 0, nil}, {0, nil, nil}, {80, 10, ptrNode(10 << 20)}, {110, 10, ptrNode(-20 << 20)}} {
		raw := nodeEnvelope(map[string]any{"name": "test-node", "real_memory": 100, "alloc_memory": tc.allocated, "specialized_memory": tc.reserved, "free_mem": 99999})
		rows, e := ParseNodes(raw)
		if e != nil {
			t.Fatal(e)
		}
		n := rows[0]
		if (n.UnallocatedMemory == nil) != (tc.want == nil) || tc.want != nil && *n.UnallocatedMemory != *tc.want {
			t.Fatal(n, tc.want)
		}
		if n.CPUs != nil || n.IdleCPUs != nil || n.Partitions == nil {
			t.Fatal("missing became zero or null list", n)
		}
	}
}
func ptrNode(n int64) *int64 { return &n }
func TestNodeBoundsAndErrors(t *testing.T) {
	for _, raw := range []string{`{}`, strings.Replace(fixtures.Nodes, Parser, "v0.0.43", 1), nodeEnvelope(map[string]any{"name": "bad\x1b[2J"}), nodeEnvelope(map[string]any{"name": "n", "alloc_memory": -1}), nodeEnvelope(map[string]any{"name": "n", "real_memory": 9223372036854775807}), nodeEnvelope(map[string]any{"name": "n", "cpus": "24"}), strings.Repeat(" ", (8<<20)+1)} {
		if _, err := ParseNodes(raw); err == nil {
			t.Fatal("malformed data accepted")
		}
	}
	raw := nodeEnvelope(map[string]any{"name": "n"})
	var r map[string]any
	json.Unmarshal([]byte(raw), &r)
	r["nodes"] = []any{map[string]any{"name": "n"}, map[string]any{"name": "n"}}
	b, _ := json.Marshal(r)
	if _, err := ParseNodes(string(b)); err == nil {
		t.Fatal("duplicate nodes accepted")
	}
	r["nodes"] = []any{}
	b, _ = json.Marshal(r)
	rows, e := ParseNodes(string(b))
	if e != nil || rows == nil || len(rows) != 0 {
		t.Fatal(rows, e)
	}
}
func TestNodesCommandScope(t *testing.T) {
	r := &fakeRunner{results: []transport.Result{{Stdout: fixtures.Nodes}}}
	rows, e := (&Client{Runner: r}).Nodes(context.Background())
	if e != nil || len(rows) != 20 {
		t.Fatal(rows, e)
	}
	got := strings.Join(r.commands[0], " ")
	for _, want := range []string{"-u SLURM_CLUSTERS", "--local", "--json=" + Parser, "show nodes"} {
		if !strings.Contains(got, want) {
			t.Fatal(got)
		}
	}
	r = &fakeRunner{results: []transport.Result{{ExitCode: 1}}}
	if _, e := (&Client{Runner: r}).Nodes(context.Background()); e == nil {
		t.Fatal("command error became empty")
	}
}
