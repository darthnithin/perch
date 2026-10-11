package nautilus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type pageFunc func(context.Context, ListRequest) ([]byte, error)

func (f pageFunc) ReadPage(ctx context.Context, r ListRequest) ([]byte, error) { return f(ctx, r) }

func clientFor(t *testing.T, reader PageReader) *Client {
	t.Helper()
	c, err := NewClient(fixtureScope, reader)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func fixturePage(t *testing.T, resource Resource, start, end int, version, next string) []byte {
	t.Helper()
	var r map[string]any
	if err := json.Unmarshal(fixture(t, string(resource)), &r); err != nil {
		t.Fatal(err)
	}
	r["items"] = r["items"].([]any)[start:end]
	r["metadata"] = map[string]any{"resourceVersion": version}
	if next != "" {
		field(r, "metadata")["continue"] = next
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPaginatedJobsHaveExplicitScopeAndOpaqueContinuation(t *testing.T) {
	var requests []ListRequest
	token := "opaque/+=&limit=1&namespace=other?#"
	c := clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) {
		requests = append(requests, r)
		if r.Scope != fixtureScope || r.Resource != Jobs || r.Limit != listPageSize {
			t.Errorf("lost request scope: %+v", r)
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > listTimeout {
			t.Error("missing bounded deadline")
		}
		path, err := r.Path()
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(path)
		if err != nil {
			t.Fatal(err)
		}
		if u.Path != "/apis/batch/v1/namespaces/research/jobs" || u.Host != "" || u.Query().Get("limit") != "250" || len(u.Query()["continue"]) > 1 {
			t.Errorf("incorrect path: %s", path)
		}
		if len(requests) == 1 {
			if r.Continue != "" {
				t.Error("first page has continuation")
			}
			return fixturePage(t, Jobs, 0, 2, "snapshot:17", token), nil
		}
		if r.Continue != token || u.Query().Get("continue") != token || len(u.Query()) != 2 {
			t.Error("continuation token changed request parameters")
		}
		return fixturePage(t, Jobs, 2, 8, "snapshot:17", ""), nil
	}))
	result := c.ListJobs(context.Background())
	if !result.OK() || len(result.Collection.Items) != 8 || result.Collection.ResourceVersion != "snapshot:17" || result.Collection.ObservedAt.IsZero() || len(requests) != 2 {
		t.Fatalf("wrong collection: %+v", result)
	}
}

func TestInvalidRequestNeverBuildsClusterWideOrArbitraryPath(t *testing.T) {
	for _, req := range []ListRequest{
		{Scope: Scope{Cluster: "c", Namespace: ""}, Resource: Pods, Limit: 250},
		{Scope: Scope{Cluster: "c", Namespace: "../other"}, Resource: Pods, Limit: 250},
		{Scope: Scope{Cluster: "c", Namespace: "research?all=true"}, Resource: Pods, Limit: 250},
		{Scope: Scope{Cluster: "c", Namespace: "ALL"}, Resource: Pods, Limit: 250},
		{Scope: fixtureScope, Resource: Resource("secrets"), Limit: 250},
		{Scope: fixtureScope, Resource: Pods, Limit: 0},
		{Scope: fixtureScope, Resource: Pods, Limit: 1001},
		{Scope: fixtureScope, Resource: Pods, Limit: 250, Continue: strings.Repeat("x", maxContinueBytes+1)},
	} {
		if path, err := req.Path(); err == nil || path != "" {
			t.Errorf("unsafe request accepted: %+v", req)
		}
	}
	if _, err := NewClient(fixtureScope, nil); err == nil {
		t.Fatal("nil reader accepted")
	}
	if _, err := NewClient(Scope{Cluster: "x", Namespace: "../"}, pageFunc(nil)); err == nil {
		t.Fatal("invalid namespace accepted")
	}
	p, err := (ListRequest{Scope: fixtureScope, Resource: Pods, Limit: 250}).Path()
	if err != nil || p != "/api/v1/namespaces/research/pods?limit=250" {
		t.Fatalf("Pod path=%q %v", p, err)
	}
}

func TestRefreshAllowsIndependentResources(t *testing.T) {
	var mu sync.Mutex
	calls := map[Resource]int{}
	c := clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) {
		mu.Lock()
		calls[r.Resource]++
		mu.Unlock()
		if r.Resource == Jobs {
			return fixture(t, "jobs"), nil
		}
		return []byte(`{"apiVersion":"v1","kind":"Status","code":403,"message":"SENSITIVE_SERVER_MESSAGE"}`), nil
	}))
	r := c.Refresh(context.Background())
	if !r.Jobs.OK() || r.Pods.OK() || r.Pods.Collection != nil || r.Pods.Error.Kind != Forbidden || r.Scope != fixtureScope {
		t.Fatalf("partial failure lost: %+v", r)
	}
	if calls[Jobs] != 1 || calls[Pods] != 1 {
		t.Fatalf("unexpected calls: %+v", calls)
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "SENSITIVE_SERVER_MESSAGE") {
		t.Fatal("raw status leaked")
	}
}

func TestRefreshSeparateVersionsAndEmptySuccess(t *testing.T) {
	c := clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) {
		return fixturePage(t, r.Resource, 0, 0, string(r.Resource)+"-version", ""), nil
	}))
	r := c.Refresh(context.Background())
	if !r.Jobs.OK() || !r.Pods.OK() || r.Jobs.Collection.ResourceVersion == r.Pods.Collection.ResourceVersion || r.Jobs.Collection.Items == nil || r.Pods.Collection.Items == nil {
		t.Fatalf("valid independent empty snapshots lost: %+v", r)
	}
}

func TestIncompleteListsNeverReturnPartialSuccess(t *testing.T) {
	cases := []struct {
		name          string
		first, second []byte
		secondErr     error
		kind          ErrorKind
	}{
		{"version change", fixturePage(t, Jobs, 0, 1, "rv", "next"), fixturePage(t, Jobs, 1, 2, "different", ""), nil, InvalidResponse},
		{"duplicate UID", fixturePage(t, Jobs, 0, 1, "rv", "next"), fixturePage(t, Jobs, 0, 1, "rv", ""), nil, InvalidResponse},
		{"repeated token", fixturePage(t, Jobs, 0, 1, "rv", "next"), fixturePage(t, Jobs, 1, 2, "rv", "next"), nil, InvalidResponse},
		{"malformed next page", fixturePage(t, Jobs, 0, 1, "rv", "next"), []byte(`{}`), nil, InvalidResponse},
		{"next page expired", fixturePage(t, Jobs, 0, 1, "rv", "next"), []byte(`{"apiVersion":"v1","kind":"Status","code":410,"metadata":{"continue":"fresh-but-inconsistent"}}`), nil, Expired},
		{"reader failure", fixturePage(t, Jobs, 0, 1, "rv", "next"), nil, errors.New("SECRET_PLUGIN_OUTPUT"), ReadFailed},
		{"wrong namespace", fixturePage(t, Jobs, 0, 1, "rv", "next"), mutate(t, fixturePage(t, Jobs, 1, 2, "rv", ""), func(r map[string]any) { field(item(r, 0), "metadata")["namespace"] = "other" }), nil, InvalidResponse},
		{"same name new UID", fixturePage(t, Jobs, 0, 1, "rv", "next"), mutate(t, fixturePage(t, Jobs, 0, 1, "rv", ""), func(r map[string]any) { field(item(r, 0), "metadata")["uid"] = "new-uid" }), nil, InvalidResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) {
				calls++
				if calls == 1 {
					return tc.first, nil
				}
				return tc.second, tc.secondErr
			}))
			r := c.ListJobs(context.Background())
			if r.OK() || r.Collection != nil || r.Error == nil || r.Error.Kind != tc.kind || calls != 2 {
				t.Fatalf("partial response accepted or retried: %+v calls=%d", r, calls)
			}
			if strings.Contains(fmt.Sprintf("%+v", r.Error), "SECRET") {
				t.Fatal("reader output leaked")
			}
		})
	}
}

func TestListLimitsAndEmptyContinuationPages(t *testing.T) {
	for _, kind := range []string{"items", "bytes", "pages", "token", "page bytes", "remaining"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			c := clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) {
				calls++
				if kind == "token" {
					return fixturePage(t, Jobs, 0, 0, "rv", strings.Repeat("a", maxContinueBytes+1)), nil
				}
				if kind == "page bytes" {
					return []byte(strings.Repeat(" ", MaxPageBytes+1)), nil
				}
				if kind == "remaining" {
					return mutate(t, fixturePage(t, Jobs, 0, 0, "rv", ""), func(r map[string]any) { field(r, "metadata")["remainingItemCount"] = 1 }), nil
				}
				if kind == "pages" {
					return fixturePage(t, Jobs, 0, 0, "rv", fmt.Sprint(calls)), nil
				}
				if calls == 1 {
					return fixturePage(t, Jobs, 0, 1, "rv", "next"), nil
				}
				return fixturePage(t, Jobs, 1, 2, "rv", ""), nil
			}))
			want := LimitExceeded
			switch kind {
			case "items":
				c.limits.items = 1
			case "bytes":
				c.limits.bytes = len(fixturePage(t, Jobs, 0, 1, "rv", "next"))
			case "pages":
				c.limits.pages = 2
			case "remaining":
				want = InvalidResponse
			}
			r := c.ListJobs(context.Background())
			if r.Collection != nil || r.Error == nil || r.Error.Kind != want || calls > 2 {
				t.Fatalf("unbounded or incomplete result: %+v calls=%d", r, calls)
			}
		})
	}
	// Empty intermediate pages are possible. Continue until the token ends;
	// exact item limits do not preclude a final empty page.
	calls := 0
	c := clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) {
		calls++
		if calls == 1 {
			return fixturePage(t, Jobs, 0, 1, "rv", "next"), nil
		}
		if calls == 2 {
			return fixturePage(t, Jobs, 0, 0, "rv", "last"), nil
		}
		return fixturePage(t, Jobs, 0, 0, "rv", ""), nil
	}))
	c.limits.items = 1
	if r := c.ListJobs(context.Background()); !r.OK() || len(r.Collection.Items) != 1 || calls != 3 {
		t.Fatalf("empty continuation page lost: %+v", r)
	}
}

func TestCancellationAndDeadline(t *testing.T) {
	t.Run("before read", func(t *testing.T) {
		c := clientFor(t, pageFunc(func(context.Context, ListRequest) ([]byte, error) {
			t.Fatal("read after cancellation")
			return nil, nil
		}))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := c.ListPods(ctx)
		if r.Error == nil || r.Error.Kind != Cancelled || !errors.Is(r.Error, context.Canceled) {
			t.Fatalf("lost cancellation: %+v", r)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		c := clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() }))
		c.limits.timeout = 10 * time.Millisecond
		started := time.Now()
		r := c.ListJobs(context.Background())
		if r.Error == nil || r.Error.Kind != TimedOut || !errors.Is(r.Error, context.DeadlineExceeded) || time.Since(started) > time.Second {
			t.Fatalf("unbounded deadline: %+v", r)
		}
	})
	t.Run("late success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c := clientFor(t, pageFunc(func(context.Context, ListRequest) ([]byte, error) { cancel(); return fixture(t, "jobs"), nil }))
		r := c.ListJobs(ctx)
		if r.Collection != nil || r.Error == nil || r.Error.Kind != Cancelled {
			t.Fatal("late result survived cancellation")
		}
	})
}

func TestExpiredListRestartsOnlyOnNextExplicitRefresh(t *testing.T) {
	var tokens []string
	c := clientFor(t, pageFunc(func(ctx context.Context, r ListRequest) ([]byte, error) {
		tokens = append(tokens, r.Continue)
		switch len(tokens) {
		case 1:
			return fixturePage(t, Jobs, 0, 1, "old", "expired-token"), nil
		case 2:
			return nil, queryError(Expired, "SECRET: expired-token")
		default:
			return fixturePage(t, Jobs, 5, 6, "fresh", ""), nil
		}
	}))
	first := c.ListJobs(context.Background())
	if first.OK() || first.Error.Kind != Expired || len(tokens) != 2 {
		t.Fatal("expired list retried in place")
	}
	second := c.ListJobs(context.Background())
	if !second.OK() || second.Collection.ResourceVersion != "fresh" || !reflect.DeepEqual(tokens, []string{"", "expired-token", ""}) {
		t.Fatalf("fresh list reused stale token: %+v %+v", tokens, second)
	}
}

func TestAPIStatusClassificationDoesNotExposeMessages(t *testing.T) {
	for code, want := range map[int]ErrorKind{401: Unauthenticated, 403: Forbidden, 410: Expired, 429: Unavailable, 503: Unavailable, 404: NotFound} {
		data := []byte(fmt.Sprintf(`{"apiVersion":"v1","kind":"Status","code":%d,"message":"SECRET_URL_TOKEN","details":{"name":"SECRET_NAME"}}`, code))
		err := apiStatusError(data)
		if err == nil || err.Kind != want || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("bad status classification %d: %+v", code, err)
		}
	}
	if err := apiStatusError([]byte(`{"apiVersion":"v1","kind":"Status","code":401,"code":403}`)); err == nil || err.Kind != InvalidResponse {
		t.Fatal("ambiguous status accepted")
	}
}
