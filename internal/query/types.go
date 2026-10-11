package query

import (
	"perch/internal/domain"
	"time"
)

// Public JSON uses explicit units and null for unavailable measurements.
type Job struct {
	Restarts             int                 `json:"restart_count"`
	ID                   string              `json:"id"`
	ArrayID              int64               `json:"array_id,omitempty"`
	TaskID               *int64              `json:"task_id,omitempty"`
	TaskRange            string              `json:"task_range,omitempty"`
	Name                 string              `json:"name"`
	State                string              `json:"state"`
	Reason               string              `json:"reason,omitempty"`
	Partition            string              `json:"partition"`
	Nodes                string              `json:"nodes,omitempty"`
	User                 string              `json:"user"`
	Cluster              string              `json:"cluster,omitempty"`
	Submit               *time.Time          `json:"submitted_at"`
	Start                *time.Time          `json:"started_at"`
	EstimatedStart       *time.Time          `json:"estimated_start_at"`
	End                  *time.Time          `json:"ended_at"`
	ElapsedSeconds       float64             `json:"elapsed_seconds"`
	TimeLimitSeconds     *float64            `json:"time_limit_seconds"`
	CPUs                 int                 `json:"cpus"`
	RequestedMemoryBytes *int64              `json:"requested_memory_bytes"`
	AllocatedMemoryBytes *int64              `json:"allocated_memory_bytes"`
	GPURequests          []domain.GPURequest `json:"gpu_requests"`
	GPURequestNote       string              `json:"gpu_request_note,omitempty"`
	TotalCPUSeconds      *float64            `json:"total_cpu_seconds"`
	PeakRSSBytes         *int64              `json:"peak_rss_bytes"`
	RSSScope             string              `json:"rss_scope,omitempty"`
	CPUEfficiencyPercent *float64            `json:"cpu_efficiency_percent"`
	CPUEfficiencyNote    string              `json:"cpu_efficiency_note,omitempty"`
	ExitCode             *int                `json:"exit_code"`
	Signal               *int                `json:"signal"`
	Stdout               string              `json:"stdout_pattern,omitempty"`
	Stderr               string              `json:"stderr_pattern,omitempty"`
	WorkDir              string              `json:"working_directory,omitempty"`
}

func timestamp(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
func job(j domain.Job) Job {
	var limit, efficiency *float64
	if j.Limit > 0 {
		v := j.Limit.Seconds()
		limit = &v
	}
	if v, ok := j.Efficiency(); ok {
		efficiency = &v
	}
	return Job{GPURequests: domain.CopyGPURequests(j.GPURequests), GPURequestNote: domain.Label(j.GPURequestNote), Restarts: j.Restarts, ID: j.ID, ArrayID: j.ArrayID, TaskID: j.TaskID, TaskRange: j.TaskRange, Name: domain.Label(j.Name), State: domain.Label(j.State), Reason: domain.Label(j.Reason), Partition: domain.Label(j.Partition), Nodes: domain.Label(j.Nodes), User: domain.Label(j.User), Cluster: domain.Label(j.Cluster), Submit: timestamp(j.Submit), Start: timestamp(j.Start), EstimatedStart: timestamp(j.EstimatedStart), End: timestamp(j.End), ElapsedSeconds: j.Elapsed.Seconds(), TimeLimitSeconds: limit, CPUs: j.CPUs, RequestedMemoryBytes: j.RequestedMemory, AllocatedMemoryBytes: j.AllocatedMemory, TotalCPUSeconds: j.TotalCPU, PeakRSSBytes: j.PeakRSS, RSSScope: domain.Label(j.RSSScope), CPUEfficiencyPercent: efficiency, CPUEfficiencyNote: j.EfficiencyNote(), ExitCode: j.ExitCode, Signal: j.Signal, Stdout: domain.Label(j.Stdout), Stderr: domain.Label(j.Stderr), WorkDir: domain.Label(j.WorkDir)}
}
func Jobs(jobs []domain.Job) []Job {
	out := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, job(j))
	}
	return out
}

// Accounting step measurements are separate observations, not additive peaks.
type Step struct {
	ID              string   `json:"id"`
	State           string   `json:"state"`
	Tasks           *int     `json:"tasks"`
	PeakRSSBytes    *int64   `json:"peak_rss_bytes"`
	RSSScope        string   `json:"rss_scope,omitempty"`
	TotalCPUSeconds *float64 `json:"total_cpu_seconds"`
}

func steps(values []domain.Step) []Step {
	out := make([]Step, 0, len(values))
	for _, s := range values {
		var tasks *int
		if s.Tasks > 0 {
			n := s.Tasks
			tasks = &n
		}
		out = append(out, Step{ID: domain.Label(s.ID), State: domain.Label(s.State), Tasks: tasks,
			PeakRSSBytes: s.PeakRSS, RSSScope: domain.Label(s.RSSScope), TotalCPUSeconds: s.TotalCPU})
	}
	return out
}

type Partition struct {
	Name          string `json:"name"`
	State         string `json:"state"`
	Nodes         string `json:"nodes"`
	GRES          string `json:"gres"`
	GRESUsed      string `json:"gres_used"`
	CPUs          int    `json:"cpus"`
	AllocatedCPUs int    `json:"allocated_cpus"`
	IdleCPUs      int    `json:"idle_cpus"`
	NodeCount     int    `json:"node_count"`
	IdleNodes     int    `json:"idle_nodes"`
}

func partition(p domain.Partition) Partition {
	return Partition{domain.Label(p.Name), domain.Label(p.State), domain.Label(p.Nodes), domain.Label(p.GRES), domain.Label(p.GRESUsed), p.CPUs, p.AllocatedCPUs, p.IdleCPUs, p.NodeCount, p.IdleNodes}
}

type Array struct {
	ID          int64   `json:"id"`
	Observed    int64   `json:"observed_tasks"`
	Completed   int64   `json:"completed"`
	Failed      int64   `json:"failed"`
	Cancelled   int64   `json:"cancelled"`
	Running     int64   `json:"running"`
	Pending     int64   `json:"pending"`
	Unresolved  int64   `json:"unresolved"`
	FailedTasks []int64 `json:"failed_task_ids"`
	FailedSpec  string  `json:"failed_array_spec,omitempty"`
}

func array(a domain.ArraySummary) Array {
	ids := append([]int64{}, a.FailedTasks...)
	return Array{a.ID, a.Known, a.Completed, a.Failed, a.Cancelled, a.Running, a.Pending, a.Unresolved, ids, a.FailedSpec()}
}
