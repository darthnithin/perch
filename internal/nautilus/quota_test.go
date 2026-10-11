package nautilus

import (
	"context"
	"encoding/json"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
	"testing"
)

func quotaPage(t *testing.T, hard, used, configured string) []byte {
	t.Helper()
	raw := map[string]any{"apiVersion": "v1", "kind": "ResourceQuotaList", "metadata": map[string]string{"resourceVersion": "qv"}, "items": []any{map[string]any{"metadata": map[string]string{"name": "test", "namespace": "research", "uid": "q1"}, "spec": map[string]any{"hard": map[string]string{"requests.cpu": configured}}, "status": map[string]any{"hard": map[string]string{"requests.cpu": hard}, "used": map[string]string{"requests.cpu": used}}}}}
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestQuotaExactArithmeticAndControllerLag(t *testing.T) {
	for _, tc := range []struct{ hard, used, configured, remaining, state string }{
		{"2", "1250m", "2000m", "750m", "within-limit"},
		{"1Gi", "512Mi", "1024Mi", "512Mi", "within-limit"},
		{"2", "3", "2", "-1", "over-limit"},
		{"0", "0", "0", "0", "at-limit"},
		{"4", "4", "4", "0", "at-limit"},
		{"1e3", "999", "1000", "1", "within-limit"},
		{"9007199254740993", "9007199254740992", "9007199254740993", "1", "within-limit"},
		{"4", "2", "8", "", "pending-update"},
	} {
		p, err := ParseQuotas(quotaPage(t, tc.hard, tc.used, tc.configured), fixtureScope)
		if err != nil {
			t.Fatal(tc, err)
		}
		r := p.Items[0].Resources[0]
		if r.State != tc.state {
			t.Fatal(tc, r)
		}
		if tc.remaining == "" {
			if r.Remaining != nil {
				t.Fatal("outdated headroom", r)
			}
			continue
		}
		want, _ := resource.ParseQuantity(tc.remaining)
		got, _ := resource.ParseQuantity(*r.Remaining)
		if got.Cmp(want) != 0 {
			t.Fatal(tc, r)
		}
	}
}

func TestQuotaFixturesPreserveScopesAndMissingUsage(t *testing.T) {
	p, err := ParseQuotas(fixture(t, "resourcequotas"), fixtureScope)
	if err != nil || len(p.Items) != 3 {
		t.Fatal(p, err)
	}
	if p.Items[1].ScopeDescription() != "PriorityClass In (interactive)" {
		t.Fatal(p.Items[1])
	}
	if p.Items[0].Resources[3].Name != "requests.nvidia.com/gpu" || p.Items[0].Resources[3].State != "at-limit" {
		t.Fatal(p.Items[0])
	}
	missing := p.Items[2].Resources[0]
	if missing.Used != nil || missing.Remaining != nil || missing.State != "unknown" {
		t.Fatal("invented missing usage", missing)
	}
	if p.Items[2].Resources[1].State != "pending-update" {
		t.Fatal("spec/status lag lost")
	}
}

func TestQuotaRejectsMalformedValuesWithoutPartialData(t *testing.T) {
	for _, v := range []string{"", "-1", "NaN", "Inf", "1GB", "1e1000000000", "1e-2147483648", "1\x1b[31m", strings.Repeat("1", 129)} {
		if p, err := ParseQuotas(quotaPage(t, "2", v, "2"), fixtureScope); err == nil || p.Items != nil {
			t.Fatalf("accepted %q", v)
		}
	}
	b := fixture(t, "resourcequotas")
	for _, pair := range [][2]string{{`"research"`, `"other"`}, {`"In"`, `"Bogus"`}, {`"requests.cpu": "32"`, `"requests.cpu": "32", "requests.cpu": "0"`}, {`"interactive"]`, `"bad\u001bname"]`}, {`"ResourceQuotaList"`, `"PodList"`}} {
		changed := strings.Replace(string(b), pair[0], pair[1], 1)
		if changed == string(b) {
			t.Fatal("ineffective corruption", pair)
		}
		if _, err := ParseQuotas([]byte(changed), fixtureScope); err == nil {
			t.Fatal("malformed accepted", pair)
		}
	}
}

func TestQuotaListPaginationAndIndependentDenial(t *testing.T) {
	calls := 0
	c := clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) {
		path, err := r.Path()
		if err != nil || !strings.HasPrefix(path, "/api/v1/namespaces/research/resourcequotas?") {
			t.Fatal(path, err)
		}
		calls++
		if calls == 1 {
			return fixturePage(t, Quotas, 0, 1, "qv", "next"), nil
		}
		if r.Continue != "next" {
			t.Fatal(r)
		}
		return fixturePage(t, Quotas, 1, 3, "qv", ""), nil
	}))
	r := c.ListQuotas(context.Background())
	if !r.OK() || len(r.Collection.Items) != 3 || calls != 2 {
		t.Fatal(r, calls)
	}
	c = clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) {
		if r.Resource == Quotas {
			return nil, queryError(Forbidden, "sensitive")
		}
		return fixture(t, string(r.Resource)), nil
	}))
	all := c.Refresh(context.Background())
	if !all.Jobs.OK() || !all.Pods.OK() || all.Quotas.OK() || all.Quotas.Error.Kind != Forbidden {
		t.Fatal(all)
	}
	if strings.Contains(all.Quotas.Error.Error(), "sensitive") {
		t.Fatal("raw message leaked")
	}
}

func TestQuotaDisplayUsesKnownUnitsWithoutRounding(t *testing.T) {
	for _, tc := range []struct{ name, value, want string }{
		{"requests.cpu", "7500m", "7.5"}, {"limits.cpu", "1m", "0.001"},
		{"requests.memory", "49152Mi", "48Gi"}, {"requests.storage", "2048Gi", "2Ti"},
		{"requests.memory", "129", "129"}, {"requests.example.com/accelerator", "1000m", "1000m"},
	} {
		if got := DisplayQuotaValue(tc.name, &tc.value); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	if DisplayQuotaValue("requests.cpu", nil) != "unknown" {
		t.Fatal("missing became zero")
	}
}
