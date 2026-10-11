package hummingbird

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"perch/internal/domain"
	"perch/internal/transport"
)

func TestScriptCommandAndContent(t *testing.T) {
	for _, state := range []string{"RUNNING", "COMPLETED"} {
		t.Run(state, func(t *testing.T) {
			r := &fakeRunner{results: []transport.Result{{Stdout: "#!/bin/sh\n\techo NONE\n\x1b]52;c;unsafe\a"}}}
			c := Client{Runner: r}
			got, err := c.Script(context.Background(), domain.Job{ID: "9100_7", State: state})
			if err != nil || got != "#!/bin/sh\n\techo NONE\n" {
				t.Fatal(got, err)
			}
			want := []string{"scontrol", "write", "batch_script", "9100_7", "-"}
			if state == "COMPLETED" {
				want = []string{"sacct", "--batch-script", "--noheader", "--jobs=9100_7"}
			}
			if !reflect.DeepEqual(r.commands, [][]string{want}) {
				t.Fatal(r.commands)
			}
		})
	}
}

func TestScriptUnavailableAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result transport.Result
		err    error
		want   string
	}{
		{"empty", transport.Result{}, nil, "no batch script is available"},
		{"none", transport.Result{Stdout: "NONE\n"}, nil, "no batch script is available"},
		{"whitespace", transport.Result{Stdout: " \n\t"}, nil, "no batch script is available"},
		{"failed exit", transport.Result{Stdout: "partial script", Stderr: "script access denied", ExitCode: 1}, nil, "script access denied"},
		{"real runner failure", transport.Result{Stdout: "partial script", Stderr: "\x1b[31mscript access denied\x1b[0m", ExitCode: 1}, &transport.Error{Kind: transport.CommandFailed, Message: "Command failed"}, "script access denied"},
		{"network", transport.Result{Stdout: "partial script", Stderr: "untrusted", ExitCode: 255}, &transport.Error{Kind: transport.Network, Message: "SSH connection failed"}, "SSH connection failed"},
		{"oversized", transport.Result{Stdout: strings.Repeat("x", (256<<10)+1)}, nil, "256 KiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRunner{results: []transport.Result{tc.result}, err: tc.err}
			got, err := (&Client{Runner: r}).Script(context.Background(), domain.Job{ID: "9100_7", State: "COMPLETED"})
			if got != "" || err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "\x1b") {
				t.Fatal(got, err)
			}
			if tc.name == "network" && !errors.Is(err, tc.err) {
				t.Fatal("lost transport error classification")
			}
		})
	}
}
