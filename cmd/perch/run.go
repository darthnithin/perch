package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"perch/internal/diagnostics"
	"perch/internal/domain"
	"perch/internal/launch"
	"perch/internal/transport"
)

type modulesFlag []string

func (m *modulesFlag) String() string         { return strings.Join(*m, ", ") }
func (m *modulesFlag) Set(value string) error { *m = append(*m, value); return nil }

type runResult struct {
	Mode    string          `json:"mode"`
	Host    string          `json:"host"`
	Plan    launch.Plan     `json:"plan"`
	Check   *launch.Check   `json:"check,omitempty"`
	Receipt *launch.Receipt `json:"receipt,omitempty"`
}

func runWorkload(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("perch run", flag.ContinueOnError)
	flags.SetOutput(out)
	debug := flags.Bool("debug", false, "save private bounded operation metadata")
	debugRaw := flags.Bool("debug-raw", false, "with --debug, capture raw scheduler JSON; never submission stdin")
	host := flags.String("host", "hb.ucsc.edu", "SSH destination or config alias")
	flags.Bool("no-config", false, "ignore saved preferences for this invocation")
	local := flags.Bool("local", false, "submit through local Slurm, where permitted")
	name := flags.String("name", "perch-job", "job name")
	workdir := flags.String("workdir", "", "existing absolute working directory on the cluster (required)")
	partition := flags.String("partition", "", "one partition; omitted uses the scheduler default")
	array := flags.String("array", "", "task indices and optional concurrency: 0-99%4")
	afterOK := flags.String("afterok", "", "wait for these comma-separated job IDs to succeed; cancel if impossible")
	nodes := flags.Int("nodes", 1, "number of nodes per allocation")
	tasks := flags.Int("tasks-per-node", 1, "tasks per node; command runs once per task")
	launcher := flags.String("launcher", "auto", "auto, direct, srun (independent tasks) or mpirun (Open MPI)")
	cpus := flags.Int("cpus", 1, "CPUs per task")
	memory := flags.String("mem", "1G", "memory per node in MiB/GiB, such as 512M or 4G")
	duration := flags.String("time", "10m", "whole-minute limit, such as 30m or 2h")
	gpus := flags.Int("gpus", 0, "GPUs per node (select a suitable partition)")
	var modules modulesFlag
	flags.Var(&modules, "module", "load this exact module; repeat in prerequisite order")
	check := flags.Bool("check", false, "ask Slurm to validate without submitting")
	submit := flags.Bool("submit", false, "validate, then explicitly submit this command")
	asJSON := flags.Bool("json", false, "print the plan/check/receipt as JSON")
	timeout := flags.Duration("timeout", 45*time.Second, "deadline for validation and submission together")
	flags.Usage = func() { fmt.Fprint(out, runUsage); flags.PrintDefaults() }
	if e := flags.Parse(args); e != nil {
		return e
	}
	if e := applyPreferences(flags); e != nil {
		return e
	}
	if *host == "" || strings.HasPrefix(*host, "-") || domain.Label(*host) != *host || strings.ContainsAny(*host, " \t\r\n") {
		return fmt.Errorf("invalid SSH host or alias")
	}
	if *timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if *nodes < 1 || *tasks < 1 {
		return fmt.Errorf("nodes and tasks per node must be positive")
	}
	if *check && *submit {
		return fmt.Errorf("choose --check or --submit; submission includes validation")
	}
	if len(flags.Args()) == 0 {
		return fmt.Errorf("provide a command after --; use perch run --help for examples")
	}
	mem, e := launch.MemoryMiB(*memory)
	if e != nil {
		return e
	}
	mins, e := launch.Minutes(*duration)
	if e != nil {
		return e
	}
	p, e := launch.Build(launch.Spec{Name: *name, WorkDir: *workdir, Partition: *partition, Nodes: *nodes, TasksPerNode: *tasks, Launcher: *launcher, CPUs: *cpus, MemoryMiB: mem, Minutes: mins, GPUs: *gpus, Array: *array, AfterOK: *afterOK, Modules: []string(modules), Command: flags.Args()})
	if e != nil {
		return e
	}
	capture, e := startDiagnostics(*debug, *debugRaw)
	if e != nil {
		return e
	}
	defer finishDiagnostics(capture)
	result := runResult{Mode: "preview", Host: *host, Plan: p}
	if *local {
		result.Host = "local"
	}
	if !*check && !*submit {
		return writeRun(out, result, *asJSON)
	}
	var runner interface {
		transport.Runner
		launch.Runner
	}
	if *local {
		runner = transport.Local{}
	} else {
		remote, e := transport.NewSSH(*host)
		if e != nil {
			return e
		}
		defer remote.Close()
		runner = remote
	}
	if capture != nil {
		runner = &diagnostics.Runner{Base: runner, Store: capture}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	client := launch.Client{Runner: runner}
	validated, e := client.Check(ctx, p)
	if e != nil {
		return e
	}
	result.Check = &validated
	result.Mode = "checked"
	if *submit {
		// A failed validation never reaches the mutating call.
		client.Journal, e = launch.DefaultJournal()
		if e != nil {
			return fmt.Errorf("not submitted: recovery journal unavailable: %w", e)
		}
		client.Destination = result.Host
		receipt, e := client.Submit(ctx, p)
		if e != nil {
			if receipt.JobID != "" {
				result.Receipt, result.Mode = &receipt, "submitted"
				if writeErr := writeRun(out, result, *asJSON); writeErr != nil {
					return fmt.Errorf("%w; also failed to write receipt: %v", e, writeErr)
				}
			}
			return e
		}
		result.Receipt = &receipt
		result.Mode = "submitted"
	}
	if err := writeRun(out, result, *asJSON); err != nil {
		if result.Receipt != nil {
			return fmt.Errorf("job %s was submitted, but its receipt could not be written: %w; do not resubmit", result.Receipt.JobID, err)
		}
		return err
	}
	return nil
}
func writeRun(out io.Writer, r runResult, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(r)
	}
	p := r.Plan
	partition := p.Spec.Partition
	if partition == "" {
		partition = "scheduler default"
	}
	dependency := ""
	if p.Spec.AfterOK != "" {
		dependency = "After success  " + p.Spec.AfterOK + "\nIf a dependency cannot succeed, Slurm cancels this job.\n"
	}
	if _, e := fmt.Fprintf(out, "Perch · %s\n\nHost       %s\nDirectory  %s\nResources  %s\nPartition  %s\n%sOutput     %s\nErrors     %s\n\nSlurm arguments\n%s\n\nScript\n%s\n", r.Mode, r.Host, p.Spec.WorkDir, p.ResourceSummary(), partition, dependency, p.Stdout, p.Stderr, transport.QuoteArgs(p.Arguments), p.Script); e != nil {
		return e
	}
	if r.Receipt != nil {
		destination := "--host " + transport.Quote(r.Host)
		if r.Host == "local" {
			destination = "--local"
		}
		_, e := fmt.Fprintf(out, "Submitted job %s. Acceptance does not mean it has started.\nInspect: perch job %s %s\nSubmission comment: %s\n", r.Receipt.JobID, r.Receipt.JobID, destination, r.Receipt.Token)
		return e
	}
	if r.Check != nil {
		_, e := fmt.Fprintf(out, "Scheduler validation passed; no job was submitted.\n%s\n", r.Check.Message)
		return e
	}
	_, e := fmt.Fprintln(out, "Preview only. Add --check to validate on the cluster, or --submit to validate and submit.\nFiles and inputs must already exist on the cluster; Perch transfers only this script.")
	return e
}

const runUsage = `Prepare a Slurm workload. Preview is the default.

  perch run --workdir /scratch/your-id/project --cpus 2 --mem 4G --time 30m -- python3 train.py
  perch run --host your-id@hb.ucsc.edu --workdir /scratch/your-id/project --check -- python3 train.py
  perch run --host your-id@hb.ucsc.edu --workdir /scratch/your-id/project --submit -- python3 train.py
  perch run --workdir /scratch/your-id/project --array '0-99%4' -- bash -c 'python3 train.py --seed "$SLURM_ARRAY_TASK_ID"'
  perch run --workdir /scratch/your-id/project --afterok 12345 -- python3 summarize.py
  perch run --workdir /scratch/your-id/project --nodes 2 --tasks-per-node 2 -- hostname
  perch run --workdir /scratch/your-id/project --nodes 2 --launcher mpirun --module gnu15/15.2.0 --module openmpi5/5.0.10 -- ./mpi-program

Put Perch flags before -- and the exact executable/arguments after it.
Arguments are quoted literally. For a shell pipeline, explicitly use bash -lc '...'.
Remote paths refer to Hummingbird, not this computer. No files are uploaded.
Use repeated --module flags in prerequisite order. Login startup runs on the
compute node, followed by these module loads and your command.
CPUs apply per task; memory/GPUs apply per node. Each array element receives
the whole reviewed allocation. Auto uses direct for one task, srun otherwise.
srun here runs independent tasks with MPI disabled. For MPI, choose mpirun and
load the matching Open MPI modules. Validation checks allocation feasibility,
not application or MPI compatibility. --afterok waits for all
listed jobs (or all tasks of a parent array) to succeed; impossible dependencies
cancel the new job. Dependencies must still be known to the scheduler.

Options:
`
