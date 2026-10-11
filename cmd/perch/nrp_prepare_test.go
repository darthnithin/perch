package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"perch/internal/nautilus"
	"strings"
	"testing"
)

func prepareFlags() []string {
	return []string{"prepare", "--name", "analysis", "--namespace", "research", "--image", "python:3.13-slim", "--cpu", "500m", "--memory", "512Mi", "--ephemeral-storage", "1Gi", "--time", "5m"}
}
func TestNRPPrepareOfflineReviewAndExport(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("KUBECONFIG", "/does-not-exist")
	for _, mode := range []string{"review", "--json", "--manifest"} {
		t.Run(mode, func(t *testing.T) {
			args := prepareFlags()
			if mode != "review" {
				args = append(args, mode)
			}
			args = append(args, "--", "python", "-c", "print('$(HOME)')", "--json", "\x1b[2J")
			var out bytes.Buffer
			if err := runNRP(args, &out); err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(out.Bytes(), []byte{27}) {
				t.Fatal("unsafe terminal escape")
			}
			switch mode {
			case "review":
				for _, want := range []string{"Local review · not submitted", "500m", "No persistent volume", "--manifest"} {
					if !strings.Contains(out.String(), want) {
						t.Fatal(out.String())
					}
				}
			case "--json":
				var d nautilus.Draft
				if err := json.Unmarshal(out.Bytes(), &d); err != nil || d.Source != "local-draft" || d.Command[3] != "--json" {
					t.Fatal(d, err)
				}
			case "--manifest":
				var j nautilus.JobManifest
				if err := json.Unmarshal(out.Bytes(), &j); err != nil || j.Kind != "Job" || j.Spec.Template.Spec.Containers[0].Args[1] != "print('$$(HOME)')" {
					t.Fatal(j, err)
				}
			}
		})
	}
}
func TestNRPPrepareFlagBoundaries(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, tail := range [][]string{{"--", ""}, {"python"}, {"--json", "--manifest", "--", "python"}, {"--context", "prod", "--", "python"}, {"--login", "--", "python"}, {"--demo", "--", "python"}, {"unexpected", "--", "python"}, {"--"}} {
		var out bytes.Buffer
		if err := runNRP(append(prepareFlags(), tail...), &out); err == nil {
			t.Fatal("accepted invalid flags", tail)
		}
	}
	var out bytes.Buffer
	if err := runNRP([]string{"prepare", "--help"}, &out); !errors.Is(err, flag.ErrHelp) || !strings.Contains(out.String(), "-- COMMAND") {
		t.Fatal(err, out.String())
	}
}
