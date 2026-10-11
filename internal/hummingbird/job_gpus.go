package hummingbird

import (
	"perch/internal/domain"
	"regexp"
	"strconv"
	"strings"
)

// Hummingbird v0.0.44 reports e.g. tres_per_node="gres/gpu:1" even
// when tres_req_str omits GPUs. Colon and equals count separators are both
// documented TRES forms. Do not infer requests from allocation totals.
var gpuRequestToken = regexp.MustCompile(`^gres/gpu(?::([A-Za-z0-9_.-]+))?[:=]([0-9]+)$`)

func jobGPURequests(perJob, perNode, perSocket, perTask string) ([]domain.GPURequest, string) {
	var out []domain.GPURequest
	seen := map[string]bool{}
	unavailable := func() ([]domain.GPURequest, string) {
		return nil, "GPU request details unavailable: unsupported scheduler format."
	}
	for i, raw := range []string{perJob, perNode, perSocket, perTask} {
		scope := []string{"job", "node", "socket", "task"}[i]
		if len(raw) > 4096 {
			return unavailable()
		}
		for _, token := range strings.Split(raw, ",") {
			token = strings.TrimSpace(token)
			if token != "gres/gpu" && !strings.HasPrefix(token, "gres/gpu:") && !strings.HasPrefix(token, "gres/gpu=") {
				continue
			}
			parts := gpuRequestToken.FindStringSubmatch(token)
			if parts == nil {
				return unavailable()
			}
			count, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return unavailable()
			}
			key := scope + "/" + parts[1]
			if seen[key] || len(out) >= 64 {
				return unavailable()
			}
			seen[key] = true
			out = append(out, domain.GPURequest{Scope: scope, Type: parts[1], Count: count})
		}
	}
	return out, ""
}
