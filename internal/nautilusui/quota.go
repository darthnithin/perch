package nautilusui

import (
	"fmt"
	"sort"
	"strings"

	"perch/internal/nautilus"
)

func (m *Model) quotaText() string {
	lines := []string{muted.Render("Used / limit · shared with collaborators"), ""}
	compact := m.height < 24
	if compact {
		lines = []string{muted.Render("Used / limit · shared quota")}
	}
	if m.quotasErr != nil {
		state := "Quota unavailable"
		if m.quotas != nil {
			state = "Stale quota snapshot"
		}
		lines = append(lines, warning.Render(state), m.quotasErr.Error(), "")
	}
	if m.quotas == nil {
		if m.quotasErr == nil {
			lines = append(lines, "Waiting for a quota snapshot.")
		}
		return strings.Join(lines, "\n")
	}
	quotas := append([]nautilus.Quota(nil), m.quotas.Items...)
	sort.Slice(quotas, func(i, j int) bool { return quotas[i].Name < quotas[j].Name })
	if len(quotas) == 0 {
		lines = append(lines, "No ResourceQuota objects reported.", "Other admission policies may still apply.")
	}
	for _, q := range quotas {
		scope := q.ScopeDescription()
		if len(q.Scopes) == 0 && len(q.Selectors) == 0 {
			scope = "Namespace-wide"
		}
		if compact {
			lines = append(lines, accent.Bold(true).Render(q.Name)+muted.Render(" · "+scope))
		} else {
			lines = append(lines, accent.Bold(true).Render(q.Name), muted.Render(scope))
		}
		if len(q.Resources) == 0 {
			lines = append(lines, "No resource limits reported.")
		}
		resources := append([]nautilus.QuotaResource(nil), q.Resources...)
		priority := func(s string) int {
			if s == "over-limit" || s == "at-limit" {
				return 0
			}
			return 1
		}
		sort.SliceStable(resources, func(i, j int) bool { return priority(resources[i].State) < priority(resources[j].State) })
		for _, r := range resources {
			state := ""
			switch r.State {
			case "at-limit":
				state = " · at limit"
			case "over-limit":
				state = " · over limit"
			case "pending-update":
				state = " · updating"
			case "unknown":
				state = " · incomplete"
			}
			value := fmt.Sprintf("%s / %s", nautilus.DisplayQuotaValue(r.Name, r.Used), nautilus.DisplayQuotaValue(r.Name, r.Hard))
			name := r.Name
			if label, ok := map[string]string{"requests.cpu": "CPU requests · cores", "limits.cpu": "CPU limits · cores", "requests.memory": "Memory requests", "limits.memory": "Memory limits", "requests.storage": "Storage requests", "pods": "Pods", "persistentvolumeclaims": "Volume claims", "requests.nvidia.com/gpu": "NVIDIA GPU requests"}[name]; ok {
				name = label
			}
			if m.width >= 80 && len(name) <= 29 {
				lines = append(lines, "  "+fit(name, 30)+ink.Render(value)+warning.Render(state))
			} else {
				lines = append(lines, "  "+name, "    "+ink.Render(value)+warning.Render(state))
			}
			if r.State == "pending-update" {
				lines = append(lines, muted.Render("    Configured limit: "+nautilus.DisplayQuotaValue(r.Name, r.Configured)))
			}
		}
		lines = append(lines, "")
	}
	lines = append(lines, muted.Render("Observed: "+stamp(m.quotas.ObservedAt)), "",
		"Reported quota usage, not live utilization or free hardware.",
		"Scoped quotas overlap; do not add them. Unknown usage stays unknown.",
		"Gi / Mi are binary units. Open a Job or Pod for container logs.")
	return strings.Join(lines, "\n")
}
