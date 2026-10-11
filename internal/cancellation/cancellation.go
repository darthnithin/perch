// Package cancellation resolves explicit cancellation scopes from the live
// queue. It never uses cached rows or accounting to authorize a mutation.
package cancellation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/transport"
)

const PreviewLifetime = 2 * time.Minute

type Target struct {
	ID    string `json:"id"`
	Array bool   `json:"whole_array"`
}

type Preview struct {
	Target  Target    `json:"target"`
	Host    string    `json:"host"`
	User    string    `json:"user"`
	Cluster string    `json:"cluster"`
	Scope   string    `json:"scope"`
	Name    string    `json:"name"`
	Tasks   int64     `json:"observed_tasks"`
	States  []string  `json:"states"`
	At      time.Time `json:"at"`
	Expires time.Time `json:"expires"`
}

// Plan is opaque and usable once. View returns a detached presentation value;
// editing displayed fields cannot change the target authorized by the plan.
type Plan struct {
	preview     Preview
	fingerprint string
	records     []identity
	used        atomic.Bool
}

// MatchesSelection prevents a UI row for an old attempt from silently opening
// a review for a reused ID. The review still presents freshly observed state.
func (p *Plan) MatchesSelection(j domain.Job) bool {
	if p == nil || p.preview.User != j.User || p.preview.Cluster != j.Cluster {
		return false
	}
	for _, r := range p.records {
		if r.ID == j.ID && r.RawID == j.RawID && r.Submit.Equal(j.Submit) && r.Restarts == j.Restarts {
			return true
		}
	}
	return false
}

func (p *Plan) View() Preview {
	v := p.preview
	v.States = slices.Clone(v.States)
	return v
}

type Receipt struct {
	Status  string    `json:"status"` // requested, unconfirmed, or unknown
	Target  Target    `json:"target"`
	Host    string    `json:"host"`
	Cluster string    `json:"cluster"`
	User    string    `json:"user"`
	At      time.Time `json:"at"`
	Detail  string    `json:"detail"`
}

var idPattern = regexp.MustCompile(`^[1-9][0-9]*(?:_(?:0|[1-9][0-9]*))?$`)
var identityPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

func Validate(target Target) error {
	if len(target.ID) > 32 || !idPattern.MatchString(target.ID) {
		return fmt.Errorf("provide one job ID or array task (such as 9100_4); steps and ranges are not cancellation targets")
	}
	parts := strings.Split(target.ID, "_")
	for _, part := range parts {
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil || n > domain.MaxTaskIndex {
			return fmt.Errorf("cancellation ID is outside Slurm's supported range")
		}
	}
	if target.Array && len(parts) != 1 {
		return fmt.Errorf("--array requires the parent array ID, without a task suffix")
	}
	return nil
}

// Clean removes inherited client filters and mutations such as SCANCEL_SIGNAL,
// SCANCEL_BATCH, SCANCEL_HURRY, and cluster overrides. All argv remain literal.
func clean(args []string) []string {
	const script = `set -e; for v in ${!SCANCEL_@} ${!SQUEUE_@}; do unset "$v"; done; unset SLURM_CLUSTERS; exec "$@"`
	return append([]string{"env", "-u", "BASH_ENV", "-u", "ENV", "/bin/bash", "--noprofile", "--norc", "-c", script, "perch-cancel"}, args...)
}

type optionalNumber struct {
	Set      bool
	Infinite bool
	Number   int64
}

// These fields are intentionally checked before using the general display
// parser, which need not preserve unsupported heterogeneous/federation scope.
type scopeRecord struct {
	Array     *optionalNumber `json:"array_job_id"`
	Task      *optionalNumber `json:"array_task_id"`
	Het       *optionalNumber `json:"het_job_id"`
	HetOffset *optionalNumber `json:"het_job_offset"`
	HetSet    *string         `json:"het_job_id_set"`
	Origin    *string         `json:"federation_origin"`
	Active    *string         `json:"federation_siblings_active"`
	Viable    *string         `json:"federation_siblings_viable"`
	Restarts  *int            `json:"restart_cnt"`
	States    []string        `json:"job_state"`
	User      string          `json:"user_name"`
	Cluster   string          `json:"cluster"`
}

type identity struct {
	ID            string
	RawID         int64
	Range         string
	Submit, Start time.Time
	Restarts      int
	State, Name   string
}

func Prepare(ctx context.Context, client *hummingbird.Client, target Target) (*Plan, error) {
	if err := Validate(target); err != nil {
		return nil, err
	}
	if client.Host == "" {
		return nil, fmt.Errorf("cancellation requires an explicit connection identity")
	}
	if err := client.Identify(ctx); err != nil {
		return nil, err
	}
	if !identityPattern.MatchString(client.User) {
		return nil, fmt.Errorf("unsupported remote username")
	}
	parent, suffix, task := strings.Cut(target.ID, "_")
	parentID, _ := strconv.ParseInt(parent, 10, 64)
	taskID, _ := strconv.ParseInt(suffix, 10, 64)
	// Inspect the parent, including grouped tasks. --local and the clean wrapper
	// prevent inherited SQUEUE_* filters from hiding part of a whole-array scope.
	r, err := client.Runner.Run(ctx, clean([]string{"squeue", "--json=" + hummingbird.Parser, "--local", "--user=" + client.User, "--jobs=" + parent}))
	if err != nil {
		return nil, err
	}
	if r.ExitCode != 0 {
		return nil, fmt.Errorf("could not inspect cancellation scope (exit %d)", r.ExitCode)
	}
	jobs, err := hummingbird.ParseQueue(r.Stdout, time.Now())
	if err != nil {
		return nil, err
	}
	var raw struct {
		Meta struct {
			Slurm struct{ Cluster, Release string }
		}
		Warnings []json.RawMessage
		Jobs     []scopeRecord
	}
	if err := json.Unmarshal([]byte(r.Stdout), &raw); err != nil {
		return nil, err
	}
	for _, warning := range raw.Warnings {
		var w struct{ Description string }
		if json.Unmarshal(warning, &w) != nil || len(jobs) != 0 || w.Description != "Zero jobs to dump" {
			return nil, fmt.Errorf("queue scope is incomplete; refresh before reviewing cancellation")
		}
	}
	if !identityPattern.MatchString(raw.Meta.Slurm.Cluster) {
		return nil, fmt.Errorf("queue scope is incomplete or its cluster identity is unavailable")
	}
	v := Preview{Target: target, Host: client.Host, User: client.User, Cluster: raw.Meta.Slurm.Cluster, Scope: "job", At: time.Now()}
	v.Expires = v.At.Add(PreviewLifetime)
	if task {
		v.Scope = "task"
	} else if target.Array {
		v.Scope = "array"
	}
	var selected []identity
	var ranges []string
	var summed int64
	states := map[string]int64{}
	for i, job := range jobs {
		if job.RawID != parentID && job.ArrayID != parentID {
			continue
		}
		if !task && !target.Array && job.ArrayID > 0 {
			return nil, fmt.Errorf("%s belongs to an array; use ARRAY_TASK for one task or the parent ID with --array for every remaining task", target.ID)
		}
		if (task || target.Array) && job.ArrayID != parentID {
			continue
		}
		if task {
			match := job.TaskID != nil && *job.TaskID == taskID
			if job.TaskRange != "" {
				match, err = domain.RangeContains(job.TaskRange, taskID)
				if err != nil {
					return nil, err
				}
			}
			if !match {
				continue
			}
		}
		guard := raw.Jobs[i]
		if guard.Array == nil || guard.Task == nil || guard.Het == nil || guard.HetOffset == nil || guard.HetSet == nil || guard.Origin == nil || guard.Active == nil || guard.Viable == nil || guard.Restarts == nil {
			return nil, fmt.Errorf("scheduler response lacks the fields needed to verify cancellation scope")
		}
		if guard.Array.Infinite || guard.Array.Set && (guard.Array.Number < 0 || guard.Array.Number > domain.MaxTaskIndex) || guard.Task.Infinite || guard.Task.Set && guard.Task.Number < 0 || job.RawID > domain.MaxTaskIndex || job.ArrayID == 0 && (job.TaskRange != "" || guard.Task.Set && guard.Task.Number <= domain.MaxTaskIndex) {
			return nil, fmt.Errorf("scheduler returned inconsistent job or array identity")
		}
		if guard.Het.Infinite || guard.Het.Set && guard.Het.Number != 0 || guard.HetOffset.Infinite || guard.HetOffset.Set && guard.HetOffset.Number != 0 || *guard.HetSet != "" || *guard.Origin != "" || *guard.Active != "" || *guard.Viable != "" {
			return nil, fmt.Errorf("heterogeneous and federated cancellation is not supported")
		}
		if guard.User != client.User || job.User != client.User || job.Cluster != v.Cluster || guard.Cluster != "" && guard.Cluster != v.Cluster || job.Submit.IsZero() || *guard.Restarts < 0 {
			return nil, fmt.Errorf("cancellation requires a fresh owned job with a known cluster and attempt")
		}
		if len(guard.States) != 1 || !slices.Contains([]string{"PENDING", "RUNNING", "SUSPENDED"}, job.State) {
			return nil, fmt.Errorf("job is %s; refresh before reviewing cancellation", job.State)
		}
		if job.State != "PENDING" && job.Start.IsZero() {
			return nil, fmt.Errorf("job start identity is missing")
		}
		if task && job.TaskRange != "" && raw.Meta.Slurm.Release != "25.11.4" {
			return nil, fmt.Errorf("grouped-task cancellation has only been verified on Slurm 25.11.4")
		}
		count := int64(1)
		expr := ""
		if target.Array {
			expr = job.TaskRange
			if expr == "" && job.TaskID != nil {
				expr = strconv.FormatInt(*job.TaskID, 10)
			}
			if expr == "" {
				return nil, fmt.Errorf("array task membership is unavailable")
			}
			count, err = domain.RangeCount(expr)
			if err != nil {
				return nil, err
			}
			ranges = append(ranges, expr)
		}
		summed += count
		states[job.State] += count
		selected = append(selected, identity{ID: job.ID, RawID: job.RawID, Range: job.TaskRange, Submit: job.Submit, Start: job.Start, Restarts: job.Restarts, State: job.State, Name: job.Name})
		if v.Name == "" {
			v.Name = job.Name
		} else if v.Name != job.Name {
			v.Name = "Multiple job names"
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("target is absent from your current queue; no cancellation was sent")
	}
	if !target.Array && len(selected) != 1 {
		return nil, fmt.Errorf("target resolves to overlapping or ambiguous task records")
	}
	if target.Array {
		count, err := domain.RangeCount(strings.Join(ranges, ","))
		if err != nil {
			return nil, err
		}
		if count != summed {
			return nil, fmt.Errorf("array records overlap; refresh before reviewing cancellation")
		}
	}
	v.Tasks = summed
	for _, state := range []string{"RUNNING", "PENDING", "SUSPENDED"} {
		if n := states[state]; n > 0 {
			v.States = append(v.States, fmt.Sprintf("%d %s", n, state))
		}
	}
	slices.SortFunc(selected, func(a, b identity) int { return strings.Compare(a.ID, b.ID) })
	encoded, err := json.Marshal(struct {
		Host, User, Cluster string
		Target              Target
		Records             []identity
	}{v.Host, v.User, v.Cluster, target, selected})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(encoded)
	return &Plan{preview: v, fingerprint: hex.EncodeToString(hash[:]), records: selected}, nil
}

// Execute consumes the review, rechecks identity and scope, then sends at most
// one mutation. A transport error is an unknown outcome, never an auto-retry.
// Scheduler reads and cancellation are not an atomic compare-and-cancel API.
func Execute(ctx context.Context, client *hummingbird.Client, p *Plan) (*Receipt, error) {
	if p == nil || !p.used.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("cancellation review was already used or is unavailable; inspect the queue before a new review")
	}
	if time.Now().After(p.preview.Expires) {
		return nil, fmt.Errorf("cancellation review expired; preview the target again")
	}
	if client.Host != p.preview.Host {
		return nil, fmt.Errorf("connection changed since review")
	}
	next, err := Prepare(ctx, client, p.preview.Target)
	if err != nil {
		return nil, err
	}
	if next.fingerprint != p.fingerprint {
		return nil, fmt.Errorf("job attempt or scope changed since review; preview the target again")
	}
	if time.Now().After(p.preview.Expires) {
		return nil, fmt.Errorf("cancellation review expired during validation; preview the target again")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	v := p.preview
	args := []string{"scancel", "--ctld", "--clusters=" + v.Cluster, "--user=" + v.User}
	if v.Scope != "array" {
		_, state, _ := strings.Cut(v.States[0], " ")
		args = append(args, "--state="+state)
	}
	args = append(args, "--", v.Target.ID)
	receipt := &Receipt{Status: "unknown", Target: v.Target, Host: v.Host, User: v.User, Cluster: v.Cluster, At: time.Now(), Detail: "Outcome unknown. Inspect queue and history before considering another request."}
	r, err := client.Runner.Run(ctx, clean(args))
	if err != nil && !transport.IsKind(err, transport.CommandFailed) {
		return receipt, fmt.Errorf("cancellation outcome unknown: %w; inspect queue and history before trying again", err)
	}
	if err != nil || r.ExitCode != 0 {
		receipt.Status = "unconfirmed"
		detail := []rune(domain.Label(r.Stderr))
		if len(detail) > 300 {
			detail = detail[:300]
		}
		receipt.Detail = fmt.Sprintf("scancel exited %d: %s. Some array tasks may have been affected; inspect queue and history.", r.ExitCode, string(detail))
		return receipt, fmt.Errorf("cancellation was not confirmed (exit %d); inspect queue and history", r.ExitCode)
	}
	receipt.Status = "requested"
	receipt.Detail = "Cancellation requested. This confirms acceptance, not that all processes stopped. Check queue and history for scheduler state."
	return receipt, nil
}
