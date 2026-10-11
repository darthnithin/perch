package hummingbird

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// GPUResource counts scheduler GRES units, which may be MIG instances rather
// than physical cards. Allocation is neither utilization nor schedulability.
type GPUResource struct {
	Type        string `json:"type"`
	Configured  int64  `json:"configured"`
	Allocated   *int64 `json:"allocated"`
	Unallocated *int64 `json:"unallocated"`
}

var gpuCount = regexp.MustCompile(`^gpu(?::([A-Za-z0-9_.-]+))?:([0-9]+)(?:\([^()]*\))*$`)

// Decode only explicit GPU counts. Missing, ambiguous, duplicate or unsupported
// descriptions retain the raw GRES and never turn into invented free capacity.
func nodeGPUs(configured, allocated string) ([]GPUResource, string) {
	totals, ok := gpuCounts(configured)
	if !ok {
		return nil, "GPU counts unavailable; inspect raw GRES."
	}
	if len(totals) == 0 {
		return nil, ""
	}
	used, valid := gpuCounts(allocated)
	if strings.TrimSpace(allocated) == "" || strings.TrimSpace(allocated) == "(null)" {
		valid = false
	}
	for kind := range used {
		if _, exists := totals[kind]; !exists {
			valid = false
		}
	}
	rows := make([]GPUResource, 0, len(totals))
	note := ""
	for kind, total := range totals {
		row := GPUResource{Type: kind, Configured: total}
		count, present := used[kind]
		if valid && present && count <= total {
			remaining := total - count
			row.Allocated = &count
			row.Unallocated = &remaining
		} else {
			note = "GPU allocation counts unavailable or inconsistent; inspect raw GRES."
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Type < rows[j].Type })
	return rows, note
}

func gpuCounts(raw string) (map[string]int64, bool) {
	counts := map[string]int64{}
	if raw == "" || raw == "(null)" {
		return counts, true
	}
	// Parenthesized socket/index lists contain commas; don't split inside them.
	var entries []string
	start := 0
	depth := 0
	for i, r := range raw {
		switch r {
		case '(':
			if depth != 0 {
				return nil, false
			}
			depth = 1
		case ')':
			if depth != 1 {
				return nil, false
			}
			depth = 0
		case ',':
			if depth == 0 {
				entries = append(entries, raw[start:i])
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, false
	}
	entries = append(entries, raw[start:])
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry != "gpu" && !strings.HasPrefix(entry, "gpu:") {
			continue
		}
		parts := gpuCount.FindStringSubmatch(entry)
		if parts == nil {
			return nil, false
		}
		count, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || count > 1000000 {
			return nil, false
		}
		if _, duplicate := counts[parts[1]]; duplicate {
			return nil, false
		}
		counts[parts[1]] = count
	}
	if _, generic := counts[""]; generic && len(counts) > 1 {
		return nil, false
	}
	return counts, true
}
