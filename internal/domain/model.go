package domain

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Job struct {
	// QueueObserved marks membership in the store's last successful queue
	// snapshot. It only controls presentation precedence, never action authority.
	QueueObserved bool `json:"queue_observed,omitempty"`
	// AwaitingAccounting preserves a newer observation of an unfinished job
	// over retained accounting until another successful accounting read.
	AwaitingAccounting bool `json:"awaiting_accounting,omitempty"`
	// AwaitingTasks is used only by presentation rows for missing array records.
	// Such a row is not a scheduler job and has no individual task IDs.
	AwaitingTasks                                        int64 `json:"-"`
	StdoutExpanded, StderrExpanded                       bool
	ID                                                   string `json:"id"`
	RawID                                                int64  `json:"raw_id"`
	ArrayID                                              int64  `json:"array_id,omitempty"`
	TaskID                                               *int64 `json:"task_id,omitempty"`
	TaskRange                                            string `json:"task_range,omitempty"`
	Name, State, Reason, Partition, Nodes, User, Cluster string
	Submit, Eligible, Start, End                         time.Time
	EstimatedStart                                       time.Time
	Elapsed, Limit                                       time.Duration
	CPUs, NodeCount, Restarts                            int
	RequestedMemory                                      *int64
	AllocatedMemory                                      *int64
	GPURequests                                          []GPURequest
	GPURequestNote                                       string
	TotalCPU                                             *float64
	PeakRSS                                              *int64
	RSSScope                                             string
	ExitCode, Signal                                     *int
	Stdout, Stderr, WorkDir                              string
	Steps                                                []Step
}

type Step struct {
	ID, State string
	Tasks     int
	PeakRSS   *int64
	RSSScope  string
	TotalCPU  *float64
}

func (j Job) Terminal() bool {
	switch j.State {
	case "COMPLETED", "FAILED", "CANCELLED", "TIMEOUT", "OUT_OF_MEMORY", "NODE_FAIL", "BOOT_FAIL", "DEADLINE", "PREEMPTED":
		return true
	}
	return false
}
func (j Job) Failed() bool  { return j.Terminal() && j.State != "COMPLETED" && j.State != "CANCELLED" }
func (j Job) Running() bool { return j.State == "RUNNING" || j.State == "COMPLETING" }
func (j Job) AttemptKey() string {
	return fmt.Sprintf("%s/%s/%s/%d/%d", j.Cluster, j.User, j.ID, j.Submit.Unix(), j.Restarts)
}
func (j Job) Efficiency() (float64, bool) {
	if j.TotalCPU == nil || j.Elapsed <= 0 || j.CPUs <= 0 || j.Restarts > 0 || *j.TotalCPU < 0 || math.IsNaN(*j.TotalCPU) || math.IsInf(*j.TotalCPU, 0) {
		return 0, false
	}
	return *j.TotalCPU / (j.Elapsed.Seconds() * float64(j.CPUs)) * 100, true
}

func (j Job) EfficiencyNote() string {
	if j.Restarts > 0 {
		return "Efficiency unavailable: CPU and elapsed time may span different attempts."
	}
	if j.Elapsed <= 0 {
		return ""
	}
	if j.Signal != nil && *j.Signal > 0 {
		return "Interrupted job: recorded CPU time may omit child processes."
	}
	switch j.State {
	case "CANCELLED", "TIMEOUT", "OUT_OF_MEMORY", "PREEMPTED", "NODE_FAIL", "BOOT_FAIL", "DEADLINE":
		return "Interrupted job: recorded CPU time may omit child processes."
	}
	return ""
}
func (j Job) Remaining() (time.Duration, bool) {
	if j.Limit <= 0 || !j.Running() {
		return 0, false
	}
	return max(0, j.Limit-j.Elapsed), true
}
func (j Job) Count() int64 {
	if j.AwaitingTasks > 0 {
		return 0 // presentation summary, never an additional scheduler task
	}
	if j.TaskRange != "" {
		n, err := RangeCount(j.TaskRange)
		if err == nil {
			return n
		}
		return 0 // Invalid range is not evidence of one task; parsers reject it.
	}
	return 1
}

type Partition struct {
	Name, State, Nodes, GRES, GRESUsed                  string
	CPUs, AllocatedCPUs, IdleCPUs, NodeCount, IdleNodes int
}
type Snapshot struct {
	Jobs                        []Job
	Partitions                  []Partition
	At, HistoryAt, PartitionsAt time.Time
	User, Cluster               string
	Warnings                    []string
}
type Capability struct{ Name, Status, Detail string }
type Report struct {
	Host, User, Cluster, Version, Parser string
	Capabilities                         []Capability
	At                                   time.Time
}

func (r Report) Supported(name string) bool {
	for _, c := range r.Capabilities {
		if c.Name == name {
			return c.Status == "supported"
		}
	}
	return false
}

type ArraySummary struct {
	RangeOutcomes                                                     []ArrayRangeOutcome `json:"-"`
	ID                                                                int64
	Known, Completed, Failed, Cancelled, Running, Pending, Unresolved int64
	FailedTasks                                                       []int64
	SummaryError                                                      string
	failedSpec                                                        string
}

// ArrayRangeOutcome is the disjoint membership assigned to an aggregate record
// after current queue and more specific observations take precedence.
type ArrayRangeOutcome struct {
	Job     Job
	Members string
}

func Arrays(jobs []Job) []ArraySummary {
	groups := map[int64][]Job{}
	for _, j := range jobs {
		if j.ArrayID > 0 && (j.TaskID != nil || j.TaskRange != "") {
			groups[j.ArrayID] = append(groups[j.ArrayID], j)
		}
	}
	var out []ArraySummary
	for id, group := range groups {
		out = append(out, summarizeArray(id, group))
	}
	sort.Slice(out, func(i, k int) bool { return out[i].ID > out[k].ID })
	return out
}

// Each task contributes once. Current queue observations win over accounting,
// then newer attempts, then individual observations over aggregate ranges.
// The store has already removed reused families.
func summarizeArray(id int64, jobs []Job) ArraySummary {
	if err := ArraySubmissionConflicts(jobs)[id]; err != "" {
		return ArraySummary{ID: id, SummaryError: err}
	}
	if err := validateArrayRangeOutcomes(jobs); err != nil {
		return ArraySummary{ID: id, SummaryError: err.Error()}
	}
	sort.SliceStable(jobs, func(i, k int) bool {
		a, b := jobs[i], jobs[k]
		if a.QueueObserved != b.QueueObserved {
			return a.QueueObserved
		}
		if a.AwaitingAccounting != b.AwaitingAccounting {
			return a.AwaitingAccounting
		}
		if !a.Submit.Equal(b.Submit) {
			return a.Submit.After(b.Submit)
		}
		if a.Restarts != b.Restarts {
			return a.Restarts > b.Restarts
		}
		if (a.TaskID != nil) != (b.TaskID != nil) {
			return a.TaskID != nil
		}
		return a.ID < b.ID
	})
	a := ArraySummary{ID: id}
	occupied := newTaskSet()
	individualOnly := true
	for _, j := range jobs {
		individualOnly = individualOnly && j.TaskID != nil
	}
	seenTasks := map[int64]bool{}
	var failed []taskProgression
	for _, j := range jobs {
		expr := j.TaskRange
		if j.TaskID != nil {
			expr = strconv.FormatInt(*j.TaskID, 10)
		}
		spans, err := parseTaskProgressions(expr)
		if err == nil && individualOnly {
			if seenTasks[*j.TaskID] {
				continue
			}
			seenTasks[*j.TaskID] = true
		} else if err == nil {
			spans, err = occupied.add(spans)
		}
		if err != nil {
			// Never expose a partial selection as if it covered all failures.
			return ArraySummary{ID: id, SummaryError: err.Error()}
		}
		n := progressionCount(spans)
		if n > 0 && j.TaskRange != "" && j.TaskID == nil && j.Terminal() {
			a.RangeOutcomes = append(a.RangeOutcomes, ArrayRangeOutcome{Job: j, Members: formatTaskSelection(spans)})
		}
		a.Known += n
		switch {
		case j.State == "COMPLETED":
			a.Completed += n
		case j.Failed():
			a.Failed += n
			failed = append(failed, spans...)
			if j.TaskID != nil && n > 0 {
				a.FailedTasks = append(a.FailedTasks, *j.TaskID)
			}
		case j.State == "CANCELLED":
			a.Cancelled += n
		case j.Running():
			a.Running += n
		case j.State == "PENDING":
			a.Pending += n
		default:
			a.Unresolved += n
		}
	}
	sort.Slice(a.FailedTasks, func(i, k int) bool { return a.FailedTasks[i] < a.FailedTasks[k] })
	if expr := formatTaskSelection(failed); expr != "" {
		a.failedSpec = "--array=" + expr
	}
	return a
}
func (a ArraySummary) FailedSpec() string {
	if a.SummaryError != "" {
		return ""
	}
	if a.failedSpec != "" {
		return a.failedSpec
	}
	ids := append([]int64(nil), a.FailedTasks...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var unique []int64
	for _, id := range ids {
		if id < 0 || id >= 4294967294 {
			continue
		}
		if len(unique) == 0 || unique[len(unique)-1] != id {
			unique = append(unique, id)
		}
	}
	var parts []string
	for i := 0; i < len(unique); {
		end := i
		for end+1 < len(unique) && unique[end+1] == unique[end]+1 {
			end++
		}
		if end-i >= 2 {
			parts = append(parts, fmt.Sprintf("%d-%d", unique[i], unique[end]))
		} else {
			for k := i; k <= end; k++ {
				parts = append(parts, strconv.FormatInt(unique[k], 10))
			}
		}
		i = end + 1
	}
	if len(parts) == 0 {
		return ""
	}
	return "--array=" + strings.Join(parts, ",")
}

func SortJobs(jobs []Job) {
	rank := func(j Job) int {
		if j.Running() {
			return 0
		}
		if j.State == "PENDING" {
			return 1
		}
		if !j.Terminal() {
			return 2
		}
		if j.Failed() {
			return 3
		}
		return 4
	}
	sort.SliceStable(jobs, func(i, k int) bool {
		a, b := jobs[i], jobs[k]
		if rank(a) != rank(b) {
			return rank(a) < rank(b)
		}
		if !a.Submit.Equal(b.Submit) {
			return a.Submit.After(b.Submit)
		}
		if a.RawID != b.RawID {
			return a.RawID > b.RawID
		}
		return a.ID > b.ID
	})
}

func Duration(d time.Duration) string {
	if d < 0 {
		return "—"
	}
	s := int64(d.Seconds())
	if s >= 86400 {
		return fmt.Sprintf("%dd %dh", s/86400, s%86400/3600)
	}
	if s >= 3600 {
		return fmt.Sprintf("%dh %02dm", s/3600, s%3600/60)
	}
	return fmt.Sprintf("%dm %02ds", s/60, s%60)
}
func Bytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n)
	u := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	i := 0
	for v >= 1024 && i < len(u)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", v, u[i])
}
