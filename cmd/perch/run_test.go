package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunPreviewIsOfflineAndPreservesArguments(t *testing.T) {
	var output bytes.Buffer
	err := runWorkload([]string{"--host", "nonexistent.invalid", "--workdir", "/scratch/example", "--json", "--", "python3", "train.py", "--epochs", "5"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	var result runResult
	if e := json.Unmarshal(output.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	if result.Mode != "preview" || result.Check != nil || result.Receipt != nil {
		t.Fatal(result)
	}
	if strings.Join(result.Plan.Spec.Command, " ") != "python3 train.py --epochs 5" {
		t.Fatal(result.Plan.Spec.Command)
	}
}
func TestRunInvalidInputsDoNotConnect(t *testing.T) {
	for _, args := range [][]string{
		{"--workdir", "relative", "--submit", "--", "true"},
		{"--workdir", "/scratch/example", "--mem", "0G", "--submit", "--", "true"},
		{"--workdir", "/scratch/example", "--time", "0m", "--submit", "--", "true"},
		{"--workdir", "/scratch/example", "--check", "--submit", "--", "true"},
		{"--workdir", "/scratch/example", "--submit"},
		{"--workdir", "/scratch/example", "--nodes", "0", "--submit", "--", "true"},
		{"--workdir", "/scratch/example", "--tasks-per-node", "0", "--submit", "--", "true"},
		{"--workdir", "/scratch/example", "--nodes", "2", "--launcher", "direct", "--submit", "--", "true"},
	} {
		var out bytes.Buffer
		if err := runWorkload(args, &out); err == nil {
			t.Fatal(args)
		}
	}
}

func TestDistributedPreviewStaysOffline(t *testing.T) {
	for _, launcher := range []string{"auto", "mpirun"} {
		var output bytes.Buffer
		err := runWorkload([]string{"--host", "unreachable.invalid", "--workdir", "/scratch/example", "--nodes", "2", "--tasks-per-node", "3", "--cpus", "2", "--mem", "4G", "--launcher", launcher, "--json", "--", "./solver", "literal; argument"}, &output)
		if err != nil {
			t.Fatal(err)
		}
		var result runResult
		if err = json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		s := result.Plan.Spec
		if result.Mode != "preview" || result.Check != nil || result.Receipt != nil || s.Nodes != 2 || s.TasksPerNode != 3 || s.CPUs != 2 || s.MemoryMiB != 4096 || s.Command[1] != "literal; argument" {
			t.Fatal(result)
		}
		want := launcher
		if want == "auto" {
			want = "srun"
		}
		if s.Launcher != want {
			t.Fatal(s.Launcher)
		}
		output.Reset()
		if err = writeRun(&output, result, false); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), "Per allocation: 12 CPU · 8192 MiB") || !strings.Contains(output.String(), "Launcher: "+want) {
			t.Fatal(output.String())
		}
	}
}

func TestArrayAndDependencyPreviewRemainsOffline(t *testing.T) {
	var output bytes.Buffer
	err := runWorkload([]string{"--host", "unreachable.invalid", "--workdir", "/scratch/example", "--array", "0-9%2", "--afterok", "123", "--json", "--", "bash", "-c", "echo $SLURM_ARRAY_TASK_ID"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	var result runResult
	if err = json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Mode != "preview" || result.Plan.Array.Tasks != 10 || result.Plan.Array.Concurrent != 2 || result.Plan.Spec.AfterOK != "123" || !strings.Contains(result.Plan.Stdout, "%A_%a") {
		t.Fatal(result)
	}
	output.Reset()
	if err = writeRun(&output, result, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "10 array tasks") || !strings.Contains(output.String(), "If a dependency cannot succeed") {
		t.Fatal(output.String())
	}
}
