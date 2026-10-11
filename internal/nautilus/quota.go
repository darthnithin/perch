package nautilus

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Quota is one independently enforced ResourceQuota. Overlapping or scoped
// quotas must never be summed into a namespace-wide allowance.
type Quota struct {
	Ref
	Scopes    []string        `json:"scopes"`
	Selectors []QuotaSelector `json:"selectors"`
	Resources []QuotaResource `json:"resources"`
}

type QuotaSelector struct {
	ScopeName string   `json:"scopeName"`
	Operator  string   `json:"operator"`
	Values    []string `json:"values"`
}

// Quantities retain Kubernetes units. Nil means absent, including unknown
// usage; zero is a real reported value. Remaining is a difference within this
// quota only, never available hardware or a promise of workload admission.
type QuotaResource struct {
	Name       string  `json:"name"`
	Configured *string `json:"configured"`
	Hard       *string `json:"hard"`
	Used       *string `json:"used"`
	Remaining  *string `json:"remaining"`
	State      string  `json:"state"`
}

type rawQuota struct {
	APIVersion, Kind string
	Metadata         metadata
	Spec             struct {
		Hard          map[string]string
		Scopes        []string
		ScopeSelector struct{ MatchExpressions []QuotaSelector }
	}
	Status struct{ Hard, Used map[string]string }
}

var quantityExponent = regexp.MustCompile(`[eE]([+-]?[0-9]+)$`)

func quotaQuantity(s string) (resource.Quantity, error) {
	// Keep untrusted decimal exponents bounded before exact arithmetic. Normal
	// Kubernetes resource magnitudes fit comfortably inside this envelope.
	if len(s) == 0 || len(s) > 128 {
		return resource.Quantity{}, fmt.Errorf("invalid quota quantity")
	}
	if match := quantityExponent.FindStringSubmatch(s); match != nil {
		n, err := strconv.Atoi(match[1])
		if err != nil || n < -30 || n > 30 {
			return resource.Quantity{}, fmt.Errorf("quota exponent out of range")
		}
	}
	q, err := resource.ParseQuantity(s)
	if err != nil || q.Sign() < 0 {
		return resource.Quantity{}, fmt.Errorf("invalid quota quantity")
	}
	return q, nil
}

func quotaLabel(s string) bool {
	return s != "" && len(s) <= 512 && safeIdentity(s) && !strings.ContainsAny(s, " \t\r\n")
}

func ParseQuotas(data []byte, scope Scope) (Page[Quota], error) {
	page, err := decodePage[rawQuota](data, "ResourceQuotaList", "v1", scope)
	if err != nil {
		return Page[Quota]{}, err
	}
	out := Page[Quota]{Items: make([]Quota, 0, len(page.Items)), ResourceVersion: page.ResourceVersion, Continue: page.Continue, Remaining: page.Remaining}
	seen := make(map[Identity]bool)
	for i, raw := range page.Items {
		ref, err := objectRef(raw.Metadata, raw.Kind, raw.APIVersion, "ResourceQuota", "v1", scope)
		if err != nil || seen[ref.Identity()] {
			return Page[Quota]{}, fmt.Errorf("invalid quota identity at item %d", i)
		}
		seen[ref.Identity()] = true
		q := Quota{Ref: ref, Scopes: raw.Spec.Scopes, Selectors: raw.Spec.ScopeSelector.MatchExpressions, Resources: []QuotaResource{}}
		for _, s := range q.Scopes {
			if !quotaLabel(s) {
				return Page[Quota]{}, fmt.Errorf("invalid quota scope")
			}
		}
		for _, s := range q.Selectors {
			if !quotaLabel(s.ScopeName) {
				return Page[Quota]{}, fmt.Errorf("invalid quota selector")
			}
			switch s.Operator {
			case "In", "NotIn":
				if len(s.Values) == 0 {
					return Page[Quota]{}, fmt.Errorf("missing quota selector values")
				}
			case "", "Exists", "DoesNotExist":
				if len(s.Values) != 0 {
					return Page[Quota]{}, fmt.Errorf("unexpected quota selector values")
				}
			default:
				return Page[Quota]{}, fmt.Errorf("invalid quota selector operator")
			}
			for _, v := range s.Values {
				if !quotaLabel(v) {
					return Page[Quota]{}, fmt.Errorf("invalid quota selector value")
				}
			}
		}
		names := map[string]bool{}
		for _, values := range []map[string]string{raw.Spec.Hard, raw.Status.Hard, raw.Status.Used} {
			for name, value := range values {
				if !quotaLabel(name) {
					return Page[Quota]{}, fmt.Errorf("invalid quota resource")
				}
				if _, err := quotaQuantity(value); err != nil {
					return Page[Quota]{}, err
				}
				names[name] = true
			}
		}
		for name := range names {
			entry := QuotaResource{Name: name, State: "unknown"}
			for _, pair := range []struct {
				values map[string]string
				dst    **string
			}{{raw.Spec.Hard, &entry.Configured}, {raw.Status.Hard, &entry.Hard}, {raw.Status.Used, &entry.Used}} {
				if value, ok := pair.values[name]; ok {
					*pair.dst = &value
				}
			}
			if entry.Hard != nil && entry.Used != nil {
				hard, _ := quotaQuantity(*entry.Hard)
				used, _ := quotaQuantity(*entry.Used)
				entry.State = "within-limit"
				switch used.Cmp(hard) {
				case 0:
					entry.State = "at-limit"
				case 1:
					entry.State = "over-limit"
				}
				hard.Sub(used)
				remaining := hard.String()
				entry.Remaining = &remaining
			}
			// Spec may have changed before the quota controller updated status.
			// Do not advertise old headroom as usable while limits disagree.
			if (entry.Configured == nil) != (entry.Hard == nil) {
				entry.State, entry.Remaining = "pending-update", nil
			} else if entry.Configured != nil {
				configured, _ := quotaQuantity(*entry.Configured)
				hard, _ := quotaQuantity(*entry.Hard)
				if configured.Cmp(hard) != 0 {
					entry.State, entry.Remaining = "pending-update", nil
				}
			}
			q.Resources = append(q.Resources, entry)
		}
		sort.Slice(q.Resources, func(i, j int) bool { return q.Resources[i].Name < q.Resources[j].Name })
		out.Items = append(out.Items, q)
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Name < out.Items[j].Name })
	return out, nil
}

func (c *Client) ListQuotas(ctx context.Context) Result[Quota] {
	return list(ctx, c, Quotas, ParseQuotas, func(q Quota) Ref { return q.Ref })
}

func (q Quota) ScopeDescription() string {
	parts := append([]string(nil), q.Scopes...)
	for _, s := range q.Selectors {
		op := s.Operator
		if op == "" {
			op = "Exists"
		}
		part := s.ScopeName + " " + op
		if len(s.Values) != 0 {
			part += " (" + strings.Join(s.Values, ", ") + ")"
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "All matching namespace resources"
	}
	return strings.Join(parts, " AND ")
}

func QuotaValue(value *string) string {
	if value == nil {
		return "unknown"
	}
	return *value
}

// DisplayQuotaValue converts only known CPU/byte resources. Unknown extended
// resources keep their API quantity and name; no unit is guessed for them.
func DisplayQuotaValue(name string, value *string) string {
	if value == nil {
		return "unknown"
	}
	q, err := quotaQuantity(*value)
	if err != nil {
		return "unknown"
	}
	base := strings.TrimPrefix(strings.TrimPrefix(name, "requests."), "limits.")
	if base == "cpu" {
		s := q.AsDec().String()
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
		return s
	}
	if base == "memory" || base == "storage" || base == "ephemeral-storage" {
		if n, ok := q.AsInt64(); ok && n > 0 {
			for _, unit := range []struct {
				size   int64
				suffix string
			}{{1 << 40, "Ti"}, {1 << 30, "Gi"}, {1 << 20, "Mi"}, {1 << 10, "Ki"}} {
				if n%unit.size == 0 {
					return strconv.FormatInt(n/unit.size, 10) + unit.suffix
				}
			}
		}
	}
	return *value
}
