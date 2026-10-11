// Package hummingbird normalizes the explicitly selected Slurm v0.0.44 schema.
// DTOs and scheduler units do not escape this package.
package hummingbird

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"perch/internal/domain"
)

const Parser = "v0.0.44"

var errAccountingOutcomeConflict = errors.New("conflicting accounting outcomes")

type number struct {
	Set, Infinite bool
	Number        int64
}

func (n number) value() *int64 {
	if !n.Set || n.Infinite || n.Number < 0 {
		return nil
	}
	v := n.Number
	return &v
}
func (n number) integer() int {
	if p := n.value(); p != nil {
		return int(*p)
	}
	return 0
}
func (n number) timestamp() time.Time {
	if p := n.value(); p != nil && *p > 0 {
		return timestamp(*p)
	}
	return time.Time{}
}
func (n number) minutes() time.Duration {
	if p := n.value(); p != nil && *p < math.MaxInt64/int64(time.Minute) {
		return time.Duration(*p) * time.Minute
	}
	return 0
}

type exitCode struct {
	Return number `json:"return_code"`
	Signal struct{ ID number }
}

func (e exitCode) values() (*int, *int) {
	var a, b *int
	if e.Return.value() != nil {
		v := e.Return.integer()
		a = &v
	}
	if e.Signal.ID.value() != nil {
		v := e.Signal.ID.integer()
		b = &v
	}
	return a, b
}

type cpuTime struct {
	Seconds      *int64
	Microseconds int64
}

func (t cpuTime) value() *float64 {
	if t.Seconds == nil || *t.Seconds < 0 || t.Microseconds < 0 {
		return nil
	}
	// sacct sums each step's seconds and microseconds independently. The
	// aggregate microsecond count can exceed one second without being invalid.
	v := float64(*t.Seconds) + float64(t.Microseconds)/1e6
	return &v
}

type tres struct {
	Type, Name string
	Count      int64
	Task       *json.Number
}
type metadata struct {
	Plugin struct {
		DataParser string `json:"data_parser"`
	}
	Slurm struct{ Release, Cluster string }
}
type envelope struct {
	Meta     metadata
	Errors   []json.RawMessage
	Warnings []json.RawMessage
}

func check(raw string, key string) (metadata, error) {
	var e envelope
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return metadata{}, fmt.Errorf("invalid Slurm JSON: %w", err)
	}
	if len(e.Errors) > 0 {
		return e.Meta, fmt.Errorf("Slurm returned %d query errors", len(e.Errors))
	}
	if e.Meta.Plugin.DataParser != "data_parser/"+Parser {
		return e.Meta, fmt.Errorf("unsupported Slurm parser %q; expected %s", domain.Label(e.Meta.Plugin.DataParser), Parser)
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal([]byte(raw), &fields)
	if value, ok := fields[key]; !ok || len(value) == 0 || value[0] != '[' {
		return e.Meta, fmt.Errorf("Slurm response is missing the %s array", key)
	}
	return e.Meta, nil
}

type queueJob struct {
	ID                              int64  `json:"job_id"`
	Array                           number `json:"array_job_id"`
	Task                            number `json:"array_task_id"`
	Range                           string `json:"array_task_string"`
	Name, Partition, Nodes, Cluster string
	User                            string   `json:"user_name"`
	State                           []string `json:"job_state"`
	Reason                          string   `json:"state_reason"`
	Submit                          number   `json:"submit_time"`
	Eligible                        number   `json:"eligible_time"`
	Start                           number   `json:"start_time"`
	End                             number   `json:"end_time"`
	Suspend                         number   `json:"suspend_time"`
	PreSuspend                      number   `json:"pre_sus_time"`
	Limit                           number   `json:"time_limit"`
	CPUs                            number
	NodeCount                       number   `json:"node_count"`
	MemCPU                          number   `json:"memory_per_cpu"`
	MemNode                         number   `json:"memory_per_node"`
	Restarts                        int      `json:"restart_cnt"`
	Exit                            exitCode `json:"exit_code"`
	Stdout                          string   `json:"standard_output"`
	Stderr                          string   `json:"standard_error"`
	ExpandedOut                     string   `json:"stdout_expanded"`
	ExpandedErr                     string   `json:"stderr_expanded"`
	WorkDir                         string   `json:"current_working_directory"`
	TRESPerJob                      string   `json:"tres_per_job"`
	TRESPerNode                     string   `json:"tres_per_node"`
	TRESPerSocket                   string   `json:"tres_per_socket"`
	TRESPerTask                     string   `json:"tres_per_task"`
}

func state(states []string) string {
	if len(states) == 0 {
		return "UNKNOWN"
	}
	return domain.Label(states[0])
}
func identity(raw, array int64, task *int64, taskRange string) (string, *int64, error) {
	if raw <= 0 {
		return "", nil, fmt.Errorf("invalid job ID")
	}
	if array <= 0 {
		return strconv.FormatInt(raw, 10), nil, nil
	}
	if taskRange != "" {
		if _, e := domain.RangeCount(taskRange); e != nil {
			return "", nil, e
		}
		return fmt.Sprintf("%d_[%s]", array, taskRange), nil, nil
	}
	// NO_VAL is sometimes emitted as a set value by Slurm.
	if task != nil && *task < 4294967294 {
		return fmt.Sprintf("%d_%d", array, *task), task, nil
	}
	return strconv.FormatInt(array, 10), nil, nil
}
func memory(perCPU, perNode number, cpus, nodes int) *int64 {
	var value *int64
	count := 0
	if value = perNode.value(); value != nil {
		count = nodes
	} else {
		value = perCPU.value()
		count = cpus
	}
	if value == nil || *value == 0 || count <= 0 || *value > math.MaxInt64/(1<<20)/int64(count) {
		return nil
	}
	v := *value * (1 << 20) * int64(count)
	return &v
}
func ParseQueue(raw string, now time.Time) ([]domain.Job, error) {
	meta, err := check(raw, "jobs")
	if err != nil {
		return nil, err
	}
	var response struct{ Jobs []queueJob }
	if err = json.Unmarshal([]byte(raw), &response); err != nil {
		return nil, err
	}
	jobs := make([]domain.Job, 0, len(response.Jobs))
	for _, q := range response.Jobs {
		// Validate before removing display brackets/throttling. Otherwise a bad
		// suffix would silently become a valid membership expression.
		if q.Range != "" {
			if _, err := domain.RangeCount(q.Range); err != nil {
				return nil, err
			}
			q.Range = strings.NewReplacer("[", "", "]", "").Replace(q.Range)
			// The throttle is not part of task membership or identity.
			q.Range, _, _ = strings.Cut(q.Range, "%")
		}
		id, task, e := identity(q.ID, int64(q.Array.integer()), q.Task.value(), q.Range)
		if e != nil {
			return nil, e
		}
		j := domain.Job{ID: id, RawID: q.ID, ArrayID: int64(q.Array.integer()), TaskID: task, TaskRange: q.Range, Name: domain.Label(q.Name), State: state(q.State), Reason: domain.Label(q.Reason), Partition: domain.Label(q.Partition), Nodes: domain.Label(q.Nodes), User: domain.Label(q.User), Cluster: domain.Label(q.Cluster), Submit: q.Submit.timestamp(), Eligible: q.Eligible.timestamp(), Start: q.Start.timestamp(), Limit: q.Limit.minutes(), CPUs: q.CPUs.integer(), NodeCount: q.NodeCount.integer(), Restarts: q.Restarts, Stdout: q.Stdout, Stderr: q.Stderr, WorkDir: q.WorkDir}
		if j.State == "PENDING" {
			j.EstimatedStart, j.Start = j.Start, time.Time{}
		}
		if j.Cluster == "" {
			j.Cluster = meta.Slurm.Cluster
		}
		if q.ExpandedOut != "" {
			j.Stdout = q.ExpandedOut
			j.StdoutExpanded = true
		}
		if q.ExpandedErr != "" {
			j.Stderr = q.ExpandedErr
			j.StderrExpanded = true
		}
		if j.Stderr == "" {
			j.Stderr = j.Stdout
			j.StderrExpanded = j.StdoutExpanded
		}
		j.RequestedMemory = memory(q.MemCPU, q.MemNode, j.CPUs, j.NodeCount)
		j.GPURequests, j.GPURequestNote = jobGPURequests(q.TRESPerJob, q.TRESPerNode, q.TRESPerSocket, q.TRESPerTask)
		if j.Terminal() {
			j.End = q.End.timestamp()
			j.ExitCode, j.Signal = q.Exit.values()
		}
		if j.State == "SUSPENDED" {
			j.Elapsed = time.Duration(q.PreSuspend.integer()) * time.Second
		} else if !j.Start.IsZero() && j.State != "PENDING" {
			end := now
			if j.Terminal() && !j.End.IsZero() {
				end = j.End
			}
			if suspended := q.Suspend.timestamp(); !suspended.IsZero() {
				j.Elapsed = end.Sub(suspended) + time.Duration(q.PreSuspend.integer())*time.Second
			} else {
				j.Elapsed = end.Sub(j.Start)
			}
			j.Elapsed = max(0, j.Elapsed)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

type accountingJob struct {
	ID                                    int64 `json:"job_id"`
	Name, Cluster, User, Partition, Nodes string
	Array                                 struct {
		ID     int64  `json:"job_id"`
		TaskID number `json:"task_id"`
		Task   string
	}
	State struct {
		Current []string
		Reason  string
	}
	Time struct {
		Elapsed                          int64
		Submission, Eligible, Start, End int64
		Limit                            number
		Total                            cpuTime
	}
	Required struct {
		CPUs    int
		MemCPU  number `json:"memory_per_cpu"`
		MemNode number `json:"memory_per_node"`
	}
	NodeCount      int      `json:"allocation_nodes"`
	Restarts       int      `json:"restart_cnt"`
	Exit           exitCode `json:"exit_code"`
	Stdout, Stderr string
	ExpandedOut    string `json:"stdout_expanded"`
	ExpandedErr    string `json:"stderr_expanded"`
	WorkDir        string `json:"working_directory"`
	TRES           struct{ Allocated, Requested []tres }
	Steps          []struct {
		Step  struct{ ID, Name string }
		State []string
		Tasks struct{ Count int }
		Time  struct{ Total cpuTime }
		TRES  struct{ Requested struct{ Max []tres } }
	}
}

func timestamp(n int64) time.Time {
	// Slurm 25.11 can emit raw NO_VAL32 for jobs cancelled before starting,
	// even though other fields use set/infinite wrappers. It is not a 2106 date.
	if n <= 0 || n == 4294967294 || n == 4294967295 {
		return time.Time{}
	}
	return time.Unix(n, 0)
}
func ParseAccounting(raw string) ([]domain.Job, error) {
	_, err := check(raw, "jobs")
	if err != nil {
		return nil, err
	}
	var response struct{ Jobs []accountingJob }
	if err = json.Unmarshal([]byte(raw), &response); err != nil {
		return nil, err
	}
	jobs := make([]domain.Job, 0, len(response.Jobs))
	// --duplicates is required for requeues, but two terminal outcomes for
	// the same scoped attempt cannot safely be resolved by response order.
	// Reject the accounting component before any consumer can emit an event,
	// copy failed tasks, or select a historical job from contradictory data.
	outcomes := make(map[string]string)
	for _, a := range response.Jobs {
		a.Array.Task, err = accountingTaskRange(a.Array.Task)
		if err != nil {
			return nil, err
		}
		id, task, e := identity(a.ID, a.Array.ID, a.Array.TaskID.value(), a.Array.Task)
		if e != nil {
			return nil, e
		}
		j := domain.Job{ID: id, RawID: a.ID, ArrayID: a.Array.ID, TaskID: task, TaskRange: a.Array.Task, Name: domain.Label(a.Name), State: state(a.State.Current), Reason: domain.Label(a.State.Reason), Partition: domain.Label(a.Partition), Nodes: domain.Label(a.Nodes), User: domain.Label(a.User), Cluster: domain.Label(a.Cluster), Submit: timestamp(a.Time.Submission), Eligible: timestamp(a.Time.Eligible), Start: timestamp(a.Time.Start), End: timestamp(a.Time.End), Elapsed: time.Duration(a.Time.Elapsed) * time.Second, Limit: a.Time.Limit.minutes(), CPUs: a.Required.CPUs, NodeCount: a.NodeCount, Restarts: a.Restarts, Stdout: a.Stdout, Stderr: a.Stderr, WorkDir: a.WorkDir}
		for _, t := range a.TRES.Allocated {
			if t.Type == "cpu" {
				j.CPUs = int(t.Count)
			}
			if t.Type == "mem" && t.Count > 0 && t.Count < math.MaxInt64/(1<<20) {
				v := t.Count * (1 << 20)
				j.AllocatedMemory = &v
			}
		}
		requestedCPUs, requestedNodes := a.Required.CPUs, a.NodeCount
		for _, t := range a.TRES.Requested {
			switch t.Type {
			case "cpu":
				requestedCPUs = int(t.Count)
			case "node":
				requestedNodes = int(t.Count)
			case "mem":
				if t.Count > 0 && t.Count < math.MaxInt64/(1<<20) {
					v := t.Count * (1 << 20)
					j.RequestedMemory = &v
				}
			}
		}
		if j.RequestedMemory == nil {
			j.RequestedMemory = memory(a.Required.MemCPU, a.Required.MemNode, requestedCPUs, requestedNodes)
		}
		if a.ExpandedOut != "" {
			j.Stdout = a.ExpandedOut
			j.StdoutExpanded = true
		}
		if a.ExpandedErr != "" {
			j.Stderr = a.ExpandedErr
			j.StderrExpanded = true
		}
		if j.Stderr == "" {
			j.Stderr = j.Stdout
			j.StderrExpanded = j.StdoutExpanded
		}
		if j.Terminal() {
			key := j.AttemptKey()
			if previous, ok := outcomes[key]; ok && previous != j.State {
				return nil, fmt.Errorf("%w for job %s; refresh history before relying on its outcome", errAccountingOutcomeConflict, j.ID)
			}
			outcomes[key] = j.State
			j.ExitCode, j.Signal = a.Exit.values()
			j.TotalCPU = a.Time.Total.value()
		}
		for _, s := range a.Steps {
			step := domain.Step{ID: domain.Label(s.Step.ID), State: state(s.State), Tasks: s.Tasks.Count, TotalCPU: s.Time.Total.value()}
			for _, t := range s.TRES.Requested.Max {
				if t.Type == "mem" && t.Count >= 0 {
					v := t.Count
					step.PeakRSS = &v
					step.RSSScope = "recorded RSS peak; task scope unavailable"
					if t.Task != nil {
						if task, err := t.Task.Int64(); err == nil {
							if task >= 0 && task < 4294967294 {
								step.RSSScope = "largest recorded task peak"
							} else if task == -1 {
								step.RSSScope = "largest recorded node peak"
							}
						}
					}
				}
			}
			if step.PeakRSS != nil && (j.PeakRSS == nil || *step.PeakRSS > *j.PeakRSS) {
				v := *step.PeakRSS
				j.PeakRSS = &v
				j.RSSScope = step.RSSScope + " (" + step.ID + ")"
			}
			j.Steps = append(j.Steps, step)
		}
		jobs = append(jobs, j)
	}
	if err := validateAccountingIdentities(jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func ParsePartitions(raw string) ([]domain.Partition, error) {
	_, err := check(raw, "sinfo")
	if err != nil {
		return nil, err
	}
	var response struct {
		Sinfo []struct {
			Partition struct {
				Name      string
				Nodes     struct{ Configured string }
				Partition struct{ State []string }
			}
			Nodes struct{ Total, Idle int }
			CPUs  struct{ Allocated, Idle, Total int }
			GRES  struct{ Total, Used string }
		}
	}
	if err = json.Unmarshal([]byte(raw), &response); err != nil {
		return nil, err
	}
	m := map[string]*domain.Partition{}
	for _, s := range response.Sinfo {
		name := domain.Label(s.Partition.Name)
		if name == "" {
			return nil, fmt.Errorf("partition name missing")
		}
		p := m[name]
		if p == nil {
			p = &domain.Partition{Name: name, State: state(s.Partition.Partition.State), Nodes: domain.Label(s.Partition.Nodes.Configured)}
			m[name] = p
		}
		p.CPUs += s.CPUs.Total
		p.AllocatedCPUs += s.CPUs.Allocated
		p.IdleCPUs += s.CPUs.Idle
		p.NodeCount += s.Nodes.Total
		p.IdleNodes += s.Nodes.Idle
		if s.GRES.Total != "" {
			if p.GRES != "" {
				p.GRES += "; "
			}
			p.GRES += domain.Label(s.GRES.Total)
		}
		if s.GRES.Used != "" {
			if p.GRESUsed != "" {
				p.GRESUsed += "; "
			}
			p.GRESUsed += domain.Label(s.GRES.Used)
		}
	}
	out := make([]domain.Partition, 0, len(m))
	for _, p := range m {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ResolvePath only expands filename tokens whose values are unambiguous for a
// single allocation. Per-node/task patterns require a future explicit chooser.
func ResolvePath(pattern string, j domain.Job) (string, error) {
	if strings.ContainsRune(pattern, 0) {
		return "", fmt.Errorf("invalid log path")
	}
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '%' {
			b.WriteByte(pattern[i])
			continue
		}
		i++
		width := 0
		for i < len(pattern) && pattern[i] >= '0' && pattern[i] <= '9' {
			width = width*10 + int(pattern[i]-'0')
			i++
			if width > 10 {
				return "", fmt.Errorf("invalid filename padding")
			}
		}
		if i >= len(pattern) {
			return "", fmt.Errorf("unfinished filename token")
		}
		var value string
		switch pattern[i] {
		case '%':
			value = "%"
		case 'j':
			value = strconv.FormatInt(j.RawID, 10)
		case 'A':
			n := j.ArrayID
			if n == 0 {
				n = j.RawID
			}
			value = strconv.FormatInt(n, 10)
		case 'a':
			if j.TaskID == nil {
				return "", fmt.Errorf("select an individual array task to read its output")
			}
			value = strconv.FormatInt(*j.TaskID, 10)
		case 'u':
			value = j.User
		case 'x':
			return "", fmt.Errorf("job-name filenames require a scheduler-expanded output path")
		default:
			return "", fmt.Errorf("filename token %%%c requires a specific node or task", pattern[i])
		}
		if width > len(value) {
			b.WriteString(strings.Repeat("0", width-len(value)))
		}
		b.WriteString(value)
	}
	path := b.String()
	if path == "" {
		return "", fmt.Errorf("no output path was recorded")
	}
	if !strings.HasPrefix(path, "/") {
		if j.WorkDir == "" {
			return "", fmt.Errorf("working directory is unknown")
		}
		path = strings.TrimSuffix(j.WorkDir, "/") + "/" + path
	}
	return path, nil
}
