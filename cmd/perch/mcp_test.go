package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Run the actual entrypoint in a subprocess, with stdout reserved for MCP.
func TestMCPProcess(t *testing.T) {
	if os.Getenv("PERCH_MCP_TEST_PROCESS") != "1" {
		return
	}
	args := []string{"mcp", "--demo"}
	if host := os.Getenv("PERCH_MCP_LIVE_HOST"); host != "" {
		args = []string{"mcp", "--host", host}
	}
	if err := run(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
func mcpProcess(t *testing.T, live bool) (context.Context, *mcp.ClientSession) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestMCPProcess$")
	// A developer's opt-in live variable must not change the offline test.
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "PERCH_MCP_TEST_PROCESS=") && (live || !strings.HasPrefix(v, "PERCH_MCP_LIVE_HOST=")) {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "PERCH_MCP_TEST_PROCESS=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	s, err := mcp.NewClient(&mcp.Implementation{Name: "perch-integration"}, nil).Connect(ctx, &mcp.CommandTransport{Command: cmd, TerminateDuration: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
		if cmd.ProcessState == nil || !cmd.ProcessState.Success() {
			t.Error("server failed to exit cleanly")
		}
		if stderr.Len() != 0 {
			t.Errorf("unexpected stderr: %s", stderr.String())
		}
	})
	return ctx, s
}
func TestMCPStdio(t *testing.T) {
	ctx, s := mcpProcess(t, false)
	list, err := s.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 15 {
		t.Fatal("unexpected tools")
	}
	for _, tool := range list.Tools {
		args := map[string]any{}
		switch tool.Name {
		case "perch_job", "perch_logs", "perch_script", "perch_resources":
			args["job_id"] = "9100_11"
		case "perch_module":
			args["name"] = "cuda/12.8.1"
		case "perch_guide":
			args["topic"] = "getting-started"
		}
		r, err := s.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if r.IsError != (tool.Name == "perch_doctor") {
			t.Fatalf("unexpected result from %s: %+v", tool.Name, r)
		}
	}
	resources, err := s.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range resources.Resources {
		if _, err = s.ReadResource(ctx, &mcp.ReadResourceParams{URI: r.URI}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.GetPrompt(ctx, &mcp.GetPromptParams{Name: "investigate_job", Arguments: map[string]string{"job_id": "9100_7"}}); err != nil {
		t.Fatal(err)
	}
	// Invalid arguments must not corrupt the channel; a subsequent call works.
	_, _ = s.CallTool(ctx, &mcp.CallToolParams{Name: "perch_job", Arguments: map[string]any{"job_id": 42}})
	r, err := s.CallTool(ctx, &mcp.CallToolParams{Name: "perch_jobs", Arguments: map[string]any{"limit": 1}})
	if err != nil || r.IsError {
		t.Fatalf("session lost after error: %v", err)
	}
}
func TestMCPLive(t *testing.T) {
	if os.Getenv("PERCH_MCP_LIVE_HOST") == "" {
		t.Skip("set PERCH_MCP_LIVE_HOST to opt into read-only Hummingbird checks")
	}
	ctx, s := mcpProcess(t, true)
	for _, name := range []string{"perch_jobs", "perch_nodes", "perch_priority", "perch_history", "perch_partitions", "perch_arrays", "perch_doctor", "perch_software", "perch_module"} {
		t.Run(name, func(t *testing.T) {
			args := map[string]any{}
			if name == "perch_software" {
				args["search"] = "cuda"
			}
			if name == "perch_module" {
				args["name"] = "singularity-ce/singularity-ce.4.1.4"
			}
			r, err := s.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil || r.IsError {
				t.Fatalf("%s failed: %v %+v", name, err, r)
			}
			if name != "perch_doctor" {
				raw, err := json.Marshal(r.StructuredContent)
				if err != nil {
					t.Fatal(err)
				}
				var data map[string]any
				if err = json.Unmarshal(raw, &data); err != nil {
					t.Fatal(err)
				}
				if data["source"] != "hummingbird" {
					t.Fatal("not live data")
				}
				if name == "perch_priority" {
					rows, ok := data["data"].([]any)
					if !ok {
						t.Fatal("priority response is not a list")
					}
					t.Logf("%d live priority records returned", len(rows))
				}
			}
			t.Log(name, "passed")
		})
	}
}
