package main

import (
	"bytes"
	"encoding/json"
	"perch/internal/nautilus"
	"strings"
	"testing"
)

func TestNRPDemoScopeSemanticsAndJSON(t *testing.T) {
	for _, command := range []string{"jobs", "pods", "quotas", "job indexed", "pod retrying-replacement", "metrics retrying-replacement"} {
		var out bytes.Buffer
		args := append(strings.Fields(command), "--demo", "--json")
		if err := runNRP(args, &out); err != nil {
			t.Fatal(err)
		}
		var response map[string]any
		if err := json.Unmarshal(out.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response["schema_version"] != float64(1) || response["provider"] != "nautilus" || response["source"] != "synthetic-demo" {
			t.Fatal(response)
		}
		if response["scope"].(map[string]any)["namespace"] != "research" || response["resource_version"] == "" || response["observed_at"] == "" {
			t.Fatal("missing scope or observation evidence")
		}
		if command == "job indexed" {
			j := response["data"].(map[string]any)
			if j["completions"] != float64(1000000) || j["completed_indexes"] != "0-999989" || j["state"] != "Running" {
				t.Fatal("indexed semantics collapsed", j)
			}
			if j["created_at"] != nil {
				t.Fatal("invented missing timestamp")
			}
		}
		if command == "pod retrying-replacement" {
			pod := response["data"].(map[string]any)
			containers := pod["containers"].([]any)
			if len(containers) != 4 {
				t.Fatal("container roles lost")
			}
			var compute map[string]any
			for _, value := range containers {
				c := value.(map[string]any)
				if c["name"] == "compute" {
					compute = c
				}
			}
			if compute == nil {
				t.Fatal("missing compute container")
			}
			current := compute["current"].(map[string]any)
			previous := compute["previous"].(map[string]any)
			if current["phase"] != "Running" || previous["reason"] != "OOMKilled" {
				t.Fatal("previous failure confused with current state")
			}
		}
	}
}

func TestNRPValidationDoesNotReadConfiguration(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, args := range [][]string{{"jobs"}, {"pods", "--context=x"}, {"jobs", "--demo", "--namespace=other"}, {"jobs", "--demo", "--context=x"}, {"pods", "--demo", "--kubeconfig=/private/file"}, {"job", "--demo"}, {"job", "one", "two", "--demo"}, {"pod", "../escape", "--demo"}, {"jobs", "--all-namespaces"}, {"delete", "pod"}, {"jobs", "--demo", "--timeout=0"}, {"jobs", "--demo", "--timeout=1h"}} {
		var out bytes.Buffer
		err := runNRP(args, &out)
		if err == nil || strings.Contains(err.Error(), "kubectl") {
			t.Fatalf("bad validation for %v: %v", args, err)
		}
	}
	for _, command := range []string{"job nonexistent", "pod nonexistent"} {
		var out bytes.Buffer
		err := runNRP(append(strings.Fields(command), "--demo", "--json"), &out)
		if err == nil || out.Len() != 0 {
			t.Fatal("missing object produced a successful or partial result")
		}
	}
}

func TestNRPPlainDetailsKeepStateEvidenceSeparate(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	cases := map[string][]string{
		"quotas":                   {"compute", "PriorityClass In (interactive)", "7500m", "pending-update", "unknown", "not live utilization"},
		"jobs":                     {"synthetic data", "Namespace: research", "ACTIVE PODS", "Completing", "Failing", "unknown"},
		"job retrying":             {"State: Running", "Failed Pods", "UID: job-retry"},
		"pod retrying-replacement": {"Current", "Running", "Previous", "OOMKilled", "sidecar", "restarts: 2", "ImagePullBackOff"},
		"pod standalone":           {"ephemeral", "Unknown"},
		"pod pending":              {"Unschedulable", "Containers"},
	}
	for command, wants := range cases {
		var out bytes.Buffer
		if err := runNRP(append(strings.Fields(command), "--demo"), &out); err != nil {
			t.Fatal(err)
		}
		for _, want := range wants {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s missing %s:\n%s", command, want, out.String())
			}
		}
		if strings.Contains(out.String(), "\x1b") {
			t.Fatal("terminal control output")
		}
	}
}

func TestNRPPodDetailDoesNotTreatPreviousFailureAsCurrent(t *testing.T) {
	zero := int64(0)
	failure := int64(137)
	pod := nautilus.Pod{Phase: "Running", Containers: []nautilus.Container{{Name: "compute", State: nautilus.ContainerState{Phase: "Running"}, Last: nautilus.ContainerState{Phase: "Terminated", Reason: "OOMKilled", ExitCode: &failure}}}}
	if detail := nrpPodDetail(pod); detail != "—" {
		t.Fatal("previous failure relabelled current workload", detail)
	}
	pod.Containers[0].State = nautilus.ContainerState{Phase: "Terminated", ExitCode: &zero}
	if detail := nrpPodDetail(pod); detail != "—" {
		t.Fatal("successful exit flagged as failure", detail)
	}
	pod.Containers[0].State = nautilus.ContainerState{Phase: "Waiting", Reason: "ImagePullBackOff"}
	if detail := nrpPodDetail(pod); detail != "compute: ImagePullBackOff" {
		t.Fatal("current waiting evidence missing", detail)
	}
}

func TestNRPLogsDemoAndValidation(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out bytes.Buffer
	if err := runNRP([]string{"logs", "retrying-replacement", "--container=compute", "--previous", "--demo", "--json"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"previous": true`) || !strings.Contains(out.String(), "Previous instance of compute") {
		t.Fatal("demo logs lost previous-container meaning", out.String())
	}
	for _, args := range [][]string{
		{"logs", "sample", "--demo"}, {"logs", "sample", "--container=../bad", "--demo"},
		{"pods", "--previous", "--demo"}, {"jobs", "--container=compute", "--demo"},
		{"logs", "sample", "--container=compute", "--demo"},
		{"logs", "retrying-replacement", "--container=absent", "--demo"},
		{"logs", "pending", "--container=compute", "--demo"},
	} {
		out.Reset()
		if err := runNRP(args, &out); err == nil || out.Len() != 0 {
			t.Fatal("invalid/unavailable log produced output", args, out.String(), err)
		}
	}
}
