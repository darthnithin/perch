package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"perch/internal/query"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func connect(t *testing.T, options Options) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := New("test", options)
	a, b := mcp.NewInMemoryTransports()
	session, err := server.Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}
func call(t *testing.T, s *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, e := s.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func decode(t *testing.T, r *mcp.CallToolResult) map[string]any {
	t.Helper()
	if r.IsError {
		t.Fatalf("tool error: %+v", r.Content)
	}
	b, e := json.Marshal(r.StructuredContent)
	if e != nil {
		t.Fatal(e)
	}
	var out map[string]any
	if e = json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestMCPDiscoveryAndAllQueries(t *testing.T) {
	s := connect(t, Options{Service: query.Service{Demo: true}})
	ctx := context.Background()
	list, e := s.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(list.Tools) != 15 {
		t.Fatalf("got %d tools", len(list.Tools))
	}
	for _, tool := range list.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("missing tool metadata: %+v", tool)
		}
	}
	for _, name := range []string{"perch_jobs", "perch_nodes", "perch_priority", "perch_waits", "perch_history", "perch_partitions", "perch_arrays", "perch_job", "perch_resources", "perch_logs", "perch_script"} {
		args := map[string]any{}
		if name == "perch_job" || name == "perch_resources" || name == "perch_logs" || name == "perch_script" {
			args["job_id"] = "9100_11"
		}
		out := decode(t, call(t, s, name, args))
		if out["source"] != "demo" {
			t.Fatalf("unlabeled demo: %+v", out)
		}
	}
	resources, e := s.ListResources(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(resources.Resources) != 4 {
		t.Fatal(resources)
	}
	for _, r := range resources.Resources {
		got, e := s.ReadResource(ctx, &mcp.ReadResourceParams{URI: r.URI})
		if e != nil {
			t.Fatal(e)
		}
		if len(got.Contents) != 1 || len(got.Contents[0].Text) < 300 {
			t.Fatal("empty guide")
		}
	}
	for _, topic := range topics {
		out := decode(t, call(t, s, "perch_guide", map[string]any{"topic": topic.id}))
		if out["topic"] != topic.id {
			t.Fatal(out)
		}
	}
	prompts, e := s.ListPrompts(ctx, nil)
	if e != nil || len(prompts.Prompts) != 1 {
		t.Fatalf("prompts: %+v %v", prompts, e)
	}
	p, e := s.GetPrompt(ctx, &mcp.GetPromptParams{Name: "investigate_job", Arguments: map[string]string{"job_id": "9100_7"}})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(p.Messages[0].Content.(*mcp.TextContent).Text, "9100_7") {
		t.Fatal("missing job id")
	}
	if _, e = s.GetPrompt(ctx, &mcp.GetPromptParams{Name: "investigate_job", Arguments: map[string]string{"job_id": "12; touch x"}}); e == nil {
		t.Fatal("invalid job id in prompt")
	}
}
func TestPagingAndTextBounds(t *testing.T) {
	s := connect(t, Options{Service: query.Service{Demo: true}})
	first := decode(t, call(t, s, "perch_jobs", map[string]any{"limit": 2}))
	p := first["page"].(map[string]any)
	if p["total"] != float64(6) || p["next_offset"] != float64(2) || len(first["data"].([]any)) != 2 {
		t.Fatal(first)
	}
	last := decode(t, call(t, s, "perch_jobs", map[string]any{"offset": 5, "limit": 2}))
	if last["page"].(map[string]any)["next_offset"] != nil || len(last["data"].([]any)) != 1 {
		t.Fatal(last)
	}
	empty := decode(t, call(t, s, "perch_jobs", map[string]any{"offset": 999}))
	if len(empty["data"].([]any)) != 0 {
		t.Fatal(empty)
	}
	for _, name := range []string{"perch_logs", "perch_script"} {
		out := decode(t, call(t, s, name, map[string]any{"job_id": "9100_11", "max_chars": 20}))
		if out["text_truncated"] != true || len([]rune(out["data"].(map[string]any)["text"].(string))) != 20 {
			t.Fatal(out)
		}
	}
}
func TestMalformedRequestsAndOfflineGuidance(t *testing.T) {
	var connected atomic.Int32
	s := connect(t, Options{Connect: func(context.Context) error { connected.Add(1); return errors.New("SSH unavailable") }})
	for _, test := range []struct {
		name string
		args any
	}{
		{"perch_jobs", map[string]any{"limit": 201}},
		{"perch_jobs", map[string]any{"offset": -1}},
		{"perch_logs", map[string]any{"job_id": "12;whoami"}},
		{"perch_script", map[string]any{"job_id": "12", "max_chars": -1}},
		{"perch_guide", map[string]any{"topic": "../../README.md"}},
	} {
		if !call(t, s, test.name, test.args).IsError {
			t.Fatalf("accepted invalid args: %+v", test)
		}
	}
	if connected.Load() != 0 {
		t.Fatal("invalid input reached SSH")
	}
	// Schema validation can use protocol errors rather than isError. Either is failure.
	for _, args := range []any{map[string]any{"job_id": 123}, map[string]any{}, map[string]any{"job_id": "12", "shell": "whoami"}} {
		r, e := s.CallTool(context.Background(), &mcp.CallToolParams{Name: "perch_job", Arguments: args})
		if e == nil && !r.IsError {
			t.Fatal("invalid schema accepted")
		}
	}
	if !call(t, s, "perch_jobs", map[string]any{}).IsError {
		t.Fatal("SSH error hidden")
	}
	if connected.Load() != 1 {
		t.Fatal("missing connection attempt")
	}
	decode(t, call(t, s, "perch_guide", map[string]any{"topic": "perch"}))
	if connected.Load() != 1 {
		t.Fatal("offline guide tried SSH")
	}
}
func TestDeadlineWhileQueuedAndRecovery(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var connections atomic.Int32
	s := &server{gate: make(chan struct{}, 1), options: Options{Timeout: time.Second, Connect: func(context.Context) error { connections.Add(1); return nil }}}
	done := make(chan error, 1)
	go func() {
		done <- s.withConnection(context.Background(), func(ctx context.Context) error {
			close(entered)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.withConnection(ctx, func(context.Context) error { t.Error("cancelled operation executed"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := s.withConnection(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if connections.Load() != 2 {
		t.Fatal("cancelled queue entry authenticated")
	}
}
func TestMCPRequestCancellation(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	var calls atomic.Int32
	s := connect(t, Options{Timeout: time.Second, Connect: func(ctx context.Context) error {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			close(cancelled)
			return ctx.Err()
		}
		return errors.New("offline")
	}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.CallTool(ctx, &mcp.CallToolParams{Name: "perch_jobs", Arguments: map[string]any{}})
	}()
	<-started
	cancel()
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not reach backend")
	}
	<-done
	if !call(t, s, "perch_jobs", map[string]any{}).IsError {
		t.Fatal("lost error after cancellation")
	}
	decode(t, call(t, s, "perch_guide", map[string]any{"topic": "perch"}))
}

func TestSoftwareToolsSearchPaginationAndDetails(t *testing.T) {
	s := connect(t, Options{Service: query.Service{Demo: true}})
	out := decode(t, call(t, s, "perch_software", map[string]any{"search": "CUDA", "limit": 1}))
	data := out["data"].([]any)
	if len(data) != 1 || data[0].(map[string]any)["name"] != "cuda/12.8.1" {
		t.Fatal(out)
	}
	out = decode(t, call(t, s, "perch_software", map[string]any{"offset": 200}))
	if len(out["data"].([]any)) != 0 {
		t.Fatal("offset not applied", out)
	}
	out = decode(t, call(t, s, "perch_module", map[string]any{"name": "cuda/12.8.1", "max_chars": 10}))
	if out["text_truncated"] != true || len(out["data"].(map[string]any)["details"].(string)) != 10 {
		t.Fatal("module truncation failed", out)
	}
	result, e := s.CallTool(context.Background(), &mcp.CallToolParams{Name: "perch_module", Arguments: map[string]any{"name": "cuda;id"}})
	if e != nil || !result.IsError {
		t.Fatal("invalid module was not a tool error", result, e)
	}
}
