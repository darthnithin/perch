package hummingbird

import "testing"

func TestNodeGPUAssignments(t *testing.T) {
	for _, tc := range []struct {
		config, used   string
		types          []string
		totals, counts []int64
	}{
		{"gpu:p100:4", "gpu:p100:4(IDX:0-3)", []string{"p100"}, []int64{4}, []int64{4}},
		{"gpu:4(S:0-1)", "gpu:2(IDX:0,2)", []string{""}, []int64{4}, []int64{2}},
		{"gpu:a100:2(S:0),gpu:h100:8(S:1),mps:100", "gpu:a100:0(IDX:N/A),gpu:h100:3(IDX:0,2,4),mps:0", []string{"a100", "h100"}, []int64{2, 8}, []int64{0, 3}},
		{"gpu:1g.10gb:7", "gpu:1g.10gb:2(IDX:0-1)", []string{"1g.10gb"}, []int64{7}, []int64{2}},
	} {
		rows, note := nodeGPUs(tc.config, tc.used)
		if note != "" || len(rows) != len(tc.types) {
			t.Fatalf("%s: %+v %s", tc.config, rows, note)
		}
		for i, r := range rows {
			if r.Type != tc.types[i] || r.Configured != tc.totals[i] || r.Allocated == nil || *r.Allocated != tc.counts[i] || r.Unallocated == nil || *r.Unallocated != tc.totals[i]-tc.counts[i] {
				t.Fatalf("%+v", r)
			}
		}
	}
}
func TestNodeGPUUnknownIsNotZero(t *testing.T) {
	for _, used := range []string{"", "(null)", "gpu:p100:5(IDX:0-4)", "gpu:4", "gpu:h100:2", "gpu:p100:?", "gpu:p100:1,gpu:p100:2", "gpu:p100:2(IDX:0,1", "mps:100"} {
		rows, note := nodeGPUs("gpu:p100:4", used)
		if len(rows) != 1 || rows[0].Configured != 4 || rows[0].Allocated != nil || rows[0].Unallocated != nil || note == "" {
			t.Fatalf("%q: %+v %q", used, rows, note)
		}
	}
	for _, config := range []string{"gpu:p100:?", "gpu:p100:1G", "gpu:4,gpu:p100:4", "gpu:p100:2,gpu:p100:2", "gpu:999999999999999999999999", "gpu:2(IDX:0,1", "gpu:p(x)100:4"} {
		rows, note := nodeGPUs(config, "gpu:p100:0")
		if len(rows) != 0 || note == "" {
			t.Fatalf("ambiguous %q: %+v %q", config, rows, note)
		}
	}
	for _, config := range []string{"", "(null)", "mps:100,shard:32,bandwidth:lustre:4G"} {
		rows, note := nodeGPUs(config, config)
		if len(rows) != 0 || note != "" {
			t.Fatalf("non-GPU %q: %+v %q", config, rows, note)
		}
	}
}
