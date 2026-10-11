package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"perch/internal/nautilus"
)

func runNRPPrepare(args []string, out io.Writer) error {
	f := flag.NewFlagSet("perch nrp prepare", flag.ContinueOnError)
	f.SetOutput(out)
	var o nautilus.PrepareOptions
	f.StringVar(&o.Name, "name", "", "Job name (required)")
	f.StringVar(&o.Namespace, "namespace", "", "destination namespace (required; no kubeconfig lookup)")
	f.StringVar(&o.Image, "image", "", "container image containing your code, preferably pinned to a digest (required)")
	f.StringVar(&o.CPU, "cpu", "", "CPU request and limit, e.g. 500m or 2 (required)")
	f.StringVar(&o.Memory, "memory", "", "memory request and limit, e.g. 2Gi (required)")
	f.StringVar(&o.EphemeralStorage, "ephemeral-storage", "", "local storage request and limit, e.g. 1Gi (required)")
	f.DurationVar(&o.Deadline, "time", 0, "Job deadline including Pod startup, e.g. 30m (required)")
	f.IntVar(&o.GPUs, "gpus", 0, "GPUs per Pod; requires --gpu-resource")
	f.StringVar(&o.GPUResource, "gpu-resource", "", "explicit resource key, e.g. nvidia.com/gpu; access is unverified")
	f.BoolVar(&o.Opportunistic, "opportunistic", false, "use preemptible opportunistic priority")
	f.StringVar(&o.PVC, "pvc", "", "existing persistent volume claim in the destination namespace")
	f.StringVar(&o.MountPath, "mount-path", "", "absolute path for that volume inside the container")
	f.StringVar(&o.WorkDir, "workdir", "", "working directory inside the container, not on this Mac")
	asJSON := f.Bool("json", false, "versioned draft with review notes and manifest")
	manifest := f.Bool("manifest", false, "only the Kubernetes Job JSON, suitable for file redirection")
	f.Usage = func() {
		fmt.Fprint(out, `Perch · prepare a Nautilus workload

Usage: perch nrp prepare [flags] -- COMMAND [ARG...]

Review a finite batch Job locally. No login, cluster request or submission.
Your command runs inside the image. Use an existing PVC to retain results.
Arguments after -- are preserved; shell syntax requires an explicit shell.

Example:
  perch nrp prepare --name analysis --namespace research --image python:3.13-slim \
    --cpu 1 --memory 512Mi --ephemeral-storage 1Gi --time 5m \
    -- python -c 'print(sum(i*i for i in range(1000000)))'

Add --manifest before -- to export Job JSON, or --json for the full review.
Check an export with: perch nrp validate job.json --context NAME --namespace NAME
This server dry-run creates no workload. Real NRP access remains an acceptance gate.

Flags:
`)
		f.PrintDefaults()
	}
	if len(args) == 0 {
		f.Usage()
		return nil
	}
	// A required delimiter prevents container options from being interpreted
	// as Perch flags (including --json, --namespace and future action flags).
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	flagArgs := args
	if sep >= 0 {
		flagArgs = args[:sep]
	}
	if err := f.Parse(flagArgs); err != nil {
		return err
	}
	if len(f.Args()) != 0 || sep < 0 {
		return fmt.Errorf("put all Perch flags before -- and the container command after it; see perch nrp prepare --help")
	}
	if *asJSON && *manifest {
		return fmt.Errorf("choose --json for the full draft or --manifest for just the Job")
	}
	o.Command = args[sep+1:]
	draft, err := nautilus.Prepare(o)
	if err != nil {
		return err
	}
	if *asJSON || *manifest {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if *manifest {
			return enc.Encode(draft.Manifest)
		}
		return enc.Encode(draft)
	}
	var b strings.Builder
	c := draft.Manifest.Spec.Template.Spec.Containers[0]
	fmt.Fprintf(&b, "Perch · Nautilus job draft\n\n%s / %s\nLocal review · not submitted\n\nImage       %s\nCPU         %s\nMemory      %s\nLocal disk  %s\nDeadline    %s (includes Pod startup)\nRetries     0\n", o.Namespace, o.Name, o.Image, c.Resources.Requests["cpu"], c.Resources.Requests["memory"], c.Resources.Requests["ephemeral-storage"], (time.Duration(draft.Manifest.Spec.ActiveDeadlineSeconds) * time.Second).String())
	if o.GPUs > 0 {
		fmt.Fprintf(&b, "GPU         %d × %s\n", o.GPUs, o.GPUResource)
	}
	if o.PVC != "" {
		fmt.Fprintf(&b, "Storage     %s → %s\n", o.PVC, o.MountPath)
	}
	if o.WorkDir != "" {
		fmt.Fprintf(&b, "Workdir     %s\n", o.WorkDir)
	}
	// JSON quoting keeps control characters and multiline commands visible
	// without turning a review into an executable terminal escape sequence.
	argv, _ := json.Marshal(draft.Command)
	fmt.Fprintf(&b, "\nCommand argv\n%s\n\n", argv)
	for _, note := range draft.Notes {
		fmt.Fprintf(&b, "• %s\n", note)
	}
	b.WriteString("\nExport: repeat with --manifest before -- and redirect to job.json.\n")
	return writeTerminalText(out, b.String())
}
