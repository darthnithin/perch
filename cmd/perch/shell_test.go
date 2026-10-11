package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestShellRejectsInvalidFlagsBeforeConnecting(t *testing.T) {
	for _, args := range [][]string{
		{}, {"9100_[1-20]"}, {"9100_1;false"}, {"42", "--cpus=0"},
		{"42", "--time=25h"}, {"42", "--node=-node"}, {"42", "--mem=0"},
		{"42", "--open", "--json"}, {"42", "--timeout=0"},
	} {
		var out bytes.Buffer
		args = append(args, "--no-config")
		if err := runAllocationShell(args, &out); err == nil {
			t.Fatal("accepted", args)
		}
	}
}

func TestShellForegroundHelper(t *testing.T) {
	if os.Getenv("PERCH_SHELL_FOREGROUND_HELPER") != "yes" {
		return
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestForegroundDeadlineReapsChild(t *testing.T) {
	t.Setenv("PERCH_SHELL_FOREGROUND_HELPER", "yes")
	cmd := exec.Command(os.Args[0], "-test.run=^TestShellForegroundHelper$")
	start := time.Now()
	err := runForegroundShell(cmd, 200*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "time limit") || cmd.ProcessState == nil || time.Since(start) > 3*time.Second {
		t.Fatal(err, cmd.ProcessState, time.Since(start))
	}
}

func TestShellHelpExplainsSharedStepAndEnvironment(t *testing.T) {
	var out bytes.Buffer
	_ = runAllocationShell([]string{"--help"}, &out)
	for _, s := range []string{"shares resources", "--open", "batch script", "exit or Ctrl-D"} {
		if !strings.Contains(out.String(), s) {
			t.Fatal(out.String())
		}
	}
}
