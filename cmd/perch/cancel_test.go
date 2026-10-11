package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCancellationFlagsRejectBeforeConnecting(t *testing.T) {
	for _, args := range [][]string{{}, {"42", "43"}, {"42_0", "--array"}, {"42.1"}, {"42", "--timeout=0"}, {"42", "--timeout=2m"}, {"42", "--signal=KILL"}, {"42", "--demo"}} {
		var out bytes.Buffer
		if err := runCancellation(append(args, "--no-config"), &out); err == nil {
			t.Fatal(args)
		}
	}
}

func TestCancellationHelpDistinguishesArrayScopeAndReceipt(t *testing.T) {
	var out bytes.Buffer
	_ = runCancellation([]string{"--help"}, &out)
	for _, want := range []string{"--array", "--confirm", "siblings running", "Dependencies", "no automatic retry", "does not mean"} {
		if !strings.Contains(strings.ReplaceAll(out.String(), "\n", " "), want) {
			t.Fatal(out.String())
		}
	}
}
