// Package allocshell plans an explicit interactive step in an owned allocation.
// Preparing a plan is read-only; only the caller's foreground action executes it.
package allocshell

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
)

type Options struct {
	Node      string `json:"node,omitempty"`
	CPUs      int    `json:"cpus"`
	MemoryMiB int64  `json:"memory_mib"`
	Minutes   int    `json:"time_limit_minutes"`
}

type Plan struct {
	JobID        string    `json:"job_id"`
	AllocationID int64     `json:"allocation_id"`
	User         string    `json:"user"`
	Cluster      string    `json:"cluster"`
	Attempt      string    `json:"attempt"`
	Started      time.Time `json:"started"`
	Name         string    `json:"name"`
	Node         string    `json:"node"`
	Nodes        []string  `json:"allocated_nodes"`
	WorkDir      string    `json:"working_directory"`
	Options      Options   `json:"resources"`
	Arguments    []string  `json:"step_arguments"`
}

var singleID = regexp.MustCompile(`^[1-9][0-9]*(?:_[0-9]+)?$`)
var nodeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)

func Validate(id string, options Options) error {
	if len(id) > 40 || !singleID.MatchString(id) {
		return fmt.Errorf("shell requires one job ID or array task, such as 9100_11")
	}
	if options.Node != "" && !nodeName.MatchString(options.Node) {
		return fmt.Errorf("node must be one plain allocated hostname")
	}
	if options.CPUs < 1 || options.CPUs > 4096 || options.MemoryMiB < 1 || options.MemoryMiB > 1<<30 || options.Minutes < 1 || options.Minutes > 1440 {
		return fmt.Errorf("shell requires 1–4096 CPUs, positive memory up to 1 PiB, and 1–1440 minutes")
	}
	return nil
}

// Prepare always reads the live user-scoped queue. It never uses accounting,
// cached UI rows, or a collapsed array to authorize execution.
func Prepare(ctx context.Context, client *hummingbird.Client, id string, options Options) (Plan, error) {
	var empty Plan
	if err := Validate(id, options); err != nil {
		return empty, err
	}
	if err := client.Identify(ctx); err != nil {
		return empty, err
	}
	r, err := client.Runner.Run(ctx, []string{"squeue", "--json=" + hummingbird.Parser, "--user=" + client.User, "--jobs=" + id})
	if err != nil {
		return empty, err
	}
	if r.ExitCode != 0 {
		return empty, fmt.Errorf("could not inspect the current allocation (exit %d)", r.ExitCode)
	}
	jobs, err := hummingbird.ParseQueue(r.Stdout, time.Now())
	if err != nil {
		return empty, err
	}
	var found []domain.Job
	for _, job := range jobs {
		if job.ID == id || strconv.FormatInt(job.RawID, 10) == id {
			found = append(found, job)
		}
	}
	if len(found) != 1 {
		return empty, fmt.Errorf("select one currently running allocation or array task owned by you")
	}
	job := found[0]
	if job.User != client.User || job.State != "RUNNING" || job.RawID <= 0 || job.TaskRange != "" || job.ArrayID > 0 && job.TaskID == nil || job.AwaitingTasks != 0 || job.Submit.IsZero() || job.Start.IsZero() || job.Cluster == "" {
		return empty, fmt.Errorf("shell requires a fresh, identified RUNNING allocation owned by you")
	}
	if job.NodeCount < 1 || job.NodeCount > 4096 || job.Nodes == "" || domain.Label(job.Nodes) != job.Nodes {
		return empty, fmt.Errorf("allocation node list is unavailable")
	}
	if job.CPUs < options.CPUs {
		return empty, fmt.Errorf("shell requests more CPUs than this allocation owns")
	}
	if job.RequestedMemory != nil && options.MemoryMiB > *job.RequestedMemory/(1<<20) {
		return empty, fmt.Errorf("shell requests more memory than this allocation owns")
	}
	if !path.IsAbs(job.WorkDir) || len(job.WorkDir) > 4096 || domain.Label(job.WorkDir) != job.WorkDir {
		return empty, fmt.Errorf("allocation working directory is unavailable or not a safe absolute path")
	}
	r, err = client.Runner.Run(ctx, []string{"scontrol", "show", "hostnames", job.Nodes})
	if err != nil {
		return empty, err
	}
	if r.ExitCode != 0 {
		return empty, fmt.Errorf("could not resolve allocated nodes (exit %d)", r.ExitCode)
	}
	nodes := strings.Fields(r.Stdout)
	if len(nodes) != job.NodeCount {
		return empty, fmt.Errorf("allocation node count changed; inspect it again")
	}
	seen := map[string]bool{}
	for _, node := range nodes {
		if !nodeName.MatchString(node) || seen[node] {
			return empty, fmt.Errorf("invalid allocated node list")
		}
		seen[node] = true
	}
	node := options.Node
	if node == "" {
		node = nodes[0]
	}
	if !slices.Contains(nodes, node) {
		return empty, fmt.Errorf("node %s does not belong to this allocation", node)
	}
	r, err = client.Runner.Run(ctx, []string{"timeout", "--version"})
	if err != nil {
		return empty, err
	}
	if r.ExitCode != 0 || !strings.HasPrefix(r.Stdout, "timeout (GNU coreutils)") {
		return empty, fmt.Errorf("allocation shells require GNU timeout on the login host")
	}
	args := []string{"srun", "--jobid=" + strconv.FormatInt(job.RawID, 10), "--overlap", "--exact", "--nodes=1", "--ntasks=1", "--cpus-per-task=" + strconv.Itoa(options.CPUs), "--mem=" + strconv.FormatInt(options.MemoryMiB, 10) + "M", "--gres=none", "--mpi=none", "--export=ALL", "--immediate=5", "--time=" + strconv.Itoa(options.Minutes), "--nodelist=" + node, "--chdir=" + job.WorkDir, "--pty", "/bin/bash", "--noprofile", "--norc", "-i"}
	return Plan{JobID: job.ID, AllocationID: job.RawID, User: job.User, Cluster: job.Cluster, Attempt: job.AttemptKey(), Started: job.Start, Name: job.Name, Node: node, Nodes: nodes, WorkDir: job.WorkDir, Options: options, Arguments: args}, nil
}

// Revalidate pins the same attempt, node and directory just before handoff.
func Revalidate(ctx context.Context, client *hummingbird.Client, plan Plan) (Plan, error) {
	options := plan.Options
	options.Node = plan.Node
	next, err := Prepare(ctx, client, plan.JobID, options)
	if err != nil {
		return Plan{}, err
	}
	if next.AllocationID != plan.AllocationID || next.Attempt != plan.Attempt || !next.Started.Equal(plan.Started) || next.WorkDir != plan.WorkDir || !slices.Equal(next.Nodes, plan.Nodes) {
		return Plan{}, fmt.Errorf("allocation changed since preview; inspect it again before opening a shell")
	}
	return next, nil
}

// Command preserves the caller's ordinary environment, but removes inherited
// scheduler options which could change resource scope, redirect output or create
// a detached/container step. The selected batch process's environment is not
// available here. Startup files are deliberately not sourced again.
func (p Plan) Command() []string {
	const clean = `set -e; for v in ${!SLURM_@} ${!SRUN_@}; do unset "$v"; done; exec "$@"`
	args := []string{"env", "-u", "BASH_ENV", "-u", "ENV", "/bin/bash", "--noprofile", "--norc", "-c", clean, "perch-shell", "timeout", "--foreground", "--signal=TERM", "--kill-after=10s", strconv.Itoa(p.Options.Minutes*60) + "s"}
	return append(args, p.Arguments...)
}
