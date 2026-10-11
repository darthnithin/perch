package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTopologyDefaultsAndResourceTotals(t *testing.T) {
	p, err := Build(spec())
	if err != nil || p.Spec.Nodes != 1 || p.Spec.TasksPerNode != 1 || p.Spec.Launcher != "direct" {
		t.Fatal(p, err)
	}
	s := spec()
	s.Nodes = 2
	s.TasksPerNode = 3
	s.Array = "0-9%4"
	s.GPUs = 1
	p, err = Build(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--nodes=2", "--ntasks=6", "--ntasks-per-node=3", "--cpus-per-task=2"} {
		if !strings.Contains(strings.Join(p.Arguments, " "), want) {
			t.Fatal(p.Arguments)
		}
	}
	for _, want := range []string{"6 tasks (3/node)", "Per node: 512 MiB memory · 1 GPU", "Per allocation: 12 CPU · 1024 MiB · 2 GPU", "Concurrent ceiling: 48 CPU · 4096 MiB · 8 GPU"} {
		if !strings.Contains(p.ResourceSummary(), want) {
			t.Fatal(p.ResourceSummary())
		}
	}
}

func TestTopologyRejectsInvalidOrOverflowingPlans(t *testing.T) {
	for _, change := range []func(*Spec){func(s *Spec) { s.Nodes = -1 }, func(s *Spec) { s.Nodes = 1025 }, func(s *Spec) { s.TasksPerNode = -1 }, func(s *Spec) { s.TasksPerNode = 4097 }, func(s *Spec) { s.Nodes = 2; s.Launcher = "direct" }, func(s *Spec) { s.Launcher = "mpirun --host evil" }, func(s *Spec) { s.Nodes = 1024; s.MemoryMiB = 1 << 40; s.Array = "0-99999" }} {
		s := spec()
		change(&s)
		if _, err := Build(s); err == nil {
			t.Fatal("accepted", s)
		}
	}
}

func TestLaunchersPreserveCommandArguments(t *testing.T) {
	for _, launcher := range []string{"srun", "mpirun"} {
		s := spec()
		s.Nodes = 2
		s.TasksPerNode = 2
		s.Launcher = launcher
		p, err := Build(s)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, launcher), []byte("#!/bin/sh\nprintf '%s\\000' \"$@\"\n"), 0700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash")
		cmd.Stdin = strings.NewReader(p.Script)
		cmd.Env = append(os.Environ(), "PATH="+dir+":/usr/bin:/bin")
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
		found := false
		for i, arg := range args {
			if arg == "--" {
				found = true
				if strings.Join(args[i+1:], "\x00") != strings.Join(s.Command, "\x00") {
					t.Fatal("changed command", args)
				}
				break
			}
		}
		if !found {
			t.Fatal("missing command delimiter", args)
		}
		if launcher == "mpirun" && !strings.Contains(string(out), "ppr:2:node:PE=2") {
			t.Fatal(args)
		}
		if launcher == "srun" && !strings.Contains(string(out), "--mpi=none") {
			t.Fatal(args)
		}
	}
}
