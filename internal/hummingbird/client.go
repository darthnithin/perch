package hummingbird

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"perch/internal/domain"
	"perch/internal/transport"
)

type Client struct {
	Runner     transport.Runner
	User, Host string
}
type Refresh struct {
	Queue, History                   []domain.Job
	Partitions                       []domain.Partition
	QueueOK, HistoryOK, PartitionsOK bool
	Errors                           map[string]error
	At                               time.Time
}

// sacct without --state omits jobs that never became eligible (including
// cancelled dependencies). Select every Slurm base state explicitly: "all" is
// not a valid sacct state. Queue records still take precedence in reconciliation.
const accountingStates = "PENDING,RUNNING,SUSPENDED,COMPLETED,CANCELLED,FAILED,TIMEOUT,NODE_FAIL,PREEMPTED,BOOT_FAIL,DEADLINE,OUT_OF_MEMORY"

func accountingArgs(user, window string) []string {
	return []string{"sacct", "--duplicates", "--json=" + Parser, "--user=" + user, "--starttime=" + window, "--endtime=now", "--state=" + accountingStates}
}

func commandError(result transport.Result) error {
	if result.ExitCode == 0 {
		return nil
	}
	return &transport.Error{Kind: transport.CommandFailed, Message: fmt.Sprintf("Query failed (exit %d): %s", result.ExitCode, truncate(domain.Label(result.Stderr), 220))}
}
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
func capResult(name string, r transport.Result) domain.Capability {
	status, detail := "supported", "Query succeeded"
	if r.ExitCode != 0 {
		status = "temporary failure"
		detail = truncate(domain.Label(r.Stderr), 160)
		low := strings.ToLower(r.Stderr)
		if r.ExitCode == 127 || strings.Contains(low, "not supported") || strings.Contains(low, "disabled") {
			status = "unsupported"
		}
		if strings.Contains(low, "permission denied") || strings.Contains(low, "access denied") {
			status = "permission denied"
		}
	}
	return domain.Capability{Name: name, Status: status, Detail: detail}
}

// Probe runs at connection time, not on the refresh clock. Optional failures
// remain explicit capabilities; a working queue must not hide a broken history.
// Identify scopes one-shot queries without repeating the full doctor probe.
func (c *Client) Identify(ctx context.Context) error {
	r, err := c.Runner.Run(ctx, []string{"id", "-un"})
	if err != nil {
		return err
	}
	if err = commandError(r); err != nil {
		return err
	}
	user := strings.TrimSpace(r.Stdout)
	if user == "" || strings.ContainsAny(user, " \x00\r\n\t") {
		return fmt.Errorf("invalid remote username")
	}
	c.User = user
	return nil
}

func (c *Client) Probe(ctx context.Context) (domain.Report, error) {
	r := domain.Report{Host: c.Host, Parser: Parser, At: time.Now()}
	first, e := c.Runner.Batch(ctx, [][]string{{"id", "-un"}, {"squeue", "--version"}, {"scontrol", "show", "config"}})
	if e != nil {
		return r, e
	}
	if len(first) != 3 {
		return r, fmt.Errorf("incomplete capability response")
	}
	if e = commandError(first[0]); e != nil {
		return r, e
	}
	user := strings.TrimSpace(first[0].Stdout)
	if user == "" || strings.ContainsAny(user, "\x00\r\n") {
		return r, fmt.Errorf("invalid remote username")
	}
	r.User = user
	if e = commandError(first[1]); e != nil {
		return r, e
	}
	r.Version = domain.Label(first[1].Stdout)
	config := parseConfig(first[2].Stdout)
	second, e := c.Runner.Batch(ctx, [][]string{
		{"squeue", "--json=" + Parser, "--user=" + user},
		accountingArgs(user, "now-1days"),
		{"sinfo", "--json=" + Parser},
		{"sh", "-c", "command -v sstat"},
	})
	if e != nil {
		return r, e
	}
	if len(second) != 4 {
		return r, fmt.Errorf("incomplete capability response")
	}
	names := []string{"jobs", "history", "partitions", "live resources"}
	for i, v := range second {
		cap := capResult(names[i], v)
		if v.ExitCode == 0 {
			switch i {
			case 0:
				jobs, err := ParseQueue(v.Stdout, time.Now())
				if err != nil {
					cap.Status = "unsupported"
					cap.Detail = err.Error()
				} else if len(jobs) > 0 {
					r.Cluster = jobs[0].Cluster
				}
			case 1:
				_, err := ParseAccounting(v.Stdout)
				if err != nil {
					cap.Status = "unsupported"
					if errors.Is(err, errAccountingOutcomeConflict) || errors.Is(err, errAccountingIdentityConflict) {
						cap.Status = "invalid response"
					}
					cap.Detail = err.Error()
				}
			case 2:
				_, err := ParsePartitions(v.Stdout)
				if err != nil {
					cap.Status = "unsupported"
					cap.Detail = err.Error()
				}
			case 3:
				cap.Status = "unverified"
				cap.Detail = "sstat is installed; measurements depend on the selected running job"
			}
		}
		r.Capabilities = append(r.Capabilities, cap)
	}
	if r.Cluster == "" {
		r.Cluster = config["ClusterName"]
	}
	addConfig := func(name, key, expected, detail string) {
		status := "unverified"
		value := config[key]
		if first[2].ExitCode == 0 && value != "" {
			status = "unsupported"
			if value == expected {
				status = "supported"
			}
		}
		r.Capabilities = append(r.Capabilities, domain.Capability{Name: name, Status: status, Detail: detail + value})
	}
	addConfig("start estimates", "SchedulerType", "sched/backfill", "Scheduler: ")
	addConfig("priority", "PriorityType", "priority/multifactor", "Priority plugin: ")
	addConfig("resource accounting", "JobAcctGatherType", "jobacct_gather/cgroup", "Accounting collector: ")
	arrayStatus, arrayDetail := "unverified", "Array limits were not returned by the scheduler"
	if size, err := strconv.ParseInt(config["MaxArraySize"], 10, 64); err == nil && first[2].ExitCode == 0 {
		arrayStatus, arrayDetail = "unsupported", "Job arrays are disabled"
		if size > 0 {
			arrayStatus, arrayDetail = "supported", fmt.Sprintf("Task indices 0–%d; account and partition limits also apply", size-1)
		}
	}
	r.Capabilities = append(r.Capabilities, domain.Capability{Name: "job arrays", Status: arrayStatus, Detail: arrayDetail})
	status := "unverified"
	if first[2].ExitCode == 0 {
		status = "unsupported"
		if strings.Contains(config["AccountingStoreFlags"], "job_script") {
			status = "supported"
		}
	}
	r.Capabilities = append(r.Capabilities, domain.Capability{Name: "historical scripts", Status: status, Detail: "AccountingStoreFlags: " + config["AccountingStoreFlags"]})
	r.Capabilities = append(r.Capabilities, domain.Capability{Name: "fairshare", Status: "unverified", Detail: "Configured priority weight: " + config["PriorityWeightFairShare"] + "; a zero weight does not affect priority"})
	r.Capabilities = append(r.Capabilities, domain.Capability{Name: "storage quota", Status: "unverified", Detail: "Personal BeeGFS quota retrieval has not been validated"})
	if !r.Supported("jobs") {
		return r, fmt.Errorf("personal jobs are unavailable; see doctor capability details")
	}
	return r, nil
}
func parseConfig(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

func (c *Client) Refresh(ctx context.Context, history, partitions bool) (Refresh, error) {
	r := Refresh{At: time.Now(), Errors: map[string]error{}}
	if c.User == "" {
		return r, fmt.Errorf("connect before refreshing")
	}
	commands := [][]string{{"squeue", "--json=" + Parser, "--user=" + c.User}}
	if history {
		commands = append(commands, accountingArgs(c.User, "now-7days"))
	}
	if partitions {
		commands = append(commands, []string{"sinfo", "--json=" + Parser})
	}
	results, e := c.Runner.Batch(ctx, commands)
	if e != nil {
		return r, e
	}
	if len(results) != len(commands) {
		return r, fmt.Errorf("incomplete refresh response")
	}
	r.At = time.Now()
	if e = commandError(results[0]); e == nil {
		r.Queue, e = ParseQueue(results[0].Stdout, r.At)
	}
	r.QueueOK = e == nil
	if e != nil {
		r.Errors["jobs"] = e
	}
	i := 1
	if history {
		if e = commandError(results[i]); e == nil {
			r.History, e = ParseAccounting(results[i].Stdout)
		}
		r.HistoryOK = e == nil
		if e != nil {
			r.Errors["history"] = e
		}
		i++
	}
	if partitions {
		if e = commandError(results[i]); e == nil {
			r.Partitions, e = ParsePartitions(results[i].Stdout)
		}
		r.PartitionsOK = e == nil
		if e != nil {
			r.Errors["partitions"] = e
		}
	}
	return r, nil
}

var jobID = regexp.MustCompile(`^[1-9][0-9]*(?:_[0-9]+)?$`)

func (c *Client) Script(ctx context.Context, j domain.Job) (string, error) {
	if !jobID.MatchString(j.ID) {
		return "", fmt.Errorf("select one allocation or array task")
	}
	args := []string{"scontrol", "write", "batch_script", j.ID, "-"}
	if j.Terminal() {
		args = []string{"sacct", "--batch-script", "--noheader", "--jobs=" + j.ID}
	}
	r, e := c.Runner.Run(ctx, args)
	if e == nil || transport.IsKind(e, transport.CommandFailed) && r.ExitCode != 0 {
		e = commandError(r)
	}
	if e != nil {
		return "", e
	}
	if len(r.Stdout) > 256<<10 {
		return "", fmt.Errorf("submitted script exceeds the 256 KiB viewer limit")
	}
	script := domain.Clean(r.Stdout)
	if strings.TrimSpace(script) == "" || j.Terminal() && strings.TrimSpace(script) == "NONE" {
		return "", fmt.Errorf("no batch script is available for this job; it may not have used one or the cluster may not retain it")
	}
	return script, nil
}

func (c *Client) LoginCommand() *exec.Cmd {
	if s, ok := c.Runner.(interface{ LoginCommand() *exec.Cmd }); ok {
		return s.LoginCommand()
	}
	return nil
}

type LogCursor struct {
	Identity string
	Offset   int64
}
type LogChunk struct {
	Cursor LogCursor
	Raw    string
	Reset  bool
	Size   int64
	Path   string
}

// ReadLog uses an open file descriptor for both stat and read, so a rename
// between them cannot mix two files. Only regular files may be read. Initial
// reads and rotations start at the bounded tail; later reads are incremental.
const logScript = `p=$1
old=$2
offset=$3
test -f "$p" || { printf 'Output is missing or is not a regular file\n' >&2; exit 66; }
exec 3<"$p" || exit 77
metadata=$(stat -Lc '%d:%i %s' /proc/self/fd/3) || exit 74
identity=${metadata% *}
size=${metadata##* }
start=$offset
reset=0
if [ "$identity" != "$old" ] || [ "$offset" -gt "$size" ]; then
  reset=1
  start=$((size - 262144))
  if [ "$start" -lt 0 ]; then start=0; fi
fi
count=$((size-start))
if [ "$count" -gt 262144 ]; then count=262144; fi
printf '%s %s %s %s\n' "$identity" "$size" "$start" "$reset"
dd bs=65536 iflag=skip_bytes,count_bytes skip="$start" count="$count" status=none <&3
`

func (c *Client) ReadLog(ctx context.Context, j domain.Job, stderr bool, previous LogCursor) (LogChunk, error) {
	pattern := j.Stdout
	expanded := j.StdoutExpanded
	if stderr {
		pattern = j.Stderr
		expanded = j.StderrExpanded
	}
	if pattern == "" {
		pattern = j.Stdout
		expanded = j.StdoutExpanded
	}
	path := pattern
	var e error
	if expanded {
		if !strings.HasPrefix(path, "/") || strings.ContainsRune(path, 0) {
			return LogChunk{}, fmt.Errorf("invalid expanded output path")
		}
	} else {
		path, e = ResolvePath(pattern, j)
	}
	if e != nil {
		return LogChunk{}, e
	}
	if previous.Offset < 0 {
		return LogChunk{}, fmt.Errorf("invalid log cursor")
	}
	r, e := c.Runner.Run(ctx, []string{"sh", "-c", logScript, "perch-log", path, previous.Identity, strconv.FormatInt(previous.Offset, 10)})
	if e != nil {
		if transport.IsKind(e, transport.CommandFailed) {
			return LogChunk{}, commandError(r)
		}
		return LogChunk{}, e
	}
	header, raw, ok := strings.Cut(r.Stdout, "\n")
	if !ok {
		return LogChunk{}, fmt.Errorf("invalid log response")
	}
	var identity string
	var size, start int64
	var reset int
	if n, err := fmt.Sscanf(header, "%s %d %d %d", &identity, &size, &start, &reset); err != nil || n != 4 || size < 0 || start < 0 || start > size || len(raw) > 262144 {
		return LogChunk{}, fmt.Errorf("invalid log metadata")
	}
	return LogChunk{Cursor: LogCursor{Identity: identity, Offset: start + int64(len(raw))}, Raw: raw, Reset: reset == 1, Size: size, Path: path}, nil
}

type LiveStep struct {
	ID         string
	PeakRSS    *int64
	CPUSeconds *float64
}

type LiveSample struct {
	Steps      []LiveStep
	At         time.Time
	PeakRSS    *int64
	CPUSeconds *float64
	Scope      string
}

func (c *Client) LiveResources(ctx context.Context, j domain.Job) (LiveSample, error) {
	s := LiveSample{At: time.Now(), Scope: "batch step; largest task RSS and average task CPU time"}
	if !j.Running() || j.RawID <= 0 || j.TaskRange != "" {
		return s, fmt.Errorf("select a running allocation")
	}
	r, e := c.Runner.Run(ctx, []string{"sstat", "--allsteps", "--jobs=" + strconv.FormatInt(j.RawID, 10), "--noheader", "--parsable2", "--format=JobID,MaxRSS,AveCPU"})
	if e == nil {
		e = commandError(r)
	}
	if e != nil {
		return s, e
	}
	steps, err := parseLiveSteps(r.Stdout, j.RawID)
	if err != nil {
		return s, err
	}
	s.Steps = steps
	for _, step := range steps {
		if step.ID == strconv.FormatInt(j.RawID, 10)+".batch" {
			s.PeakRSS, s.CPUSeconds = step.PeakRSS, step.CPUSeconds
		}
	}
	return s, nil
}

var liveStepSuffix = regexp.MustCompile(`^(?:[0-9]+|batch|extern)$`)

func parseLiveSteps(raw string, jobID int64) ([]LiveStep, error) {
	steps := []LiveStep{}
	seen := map[string]bool{}
	prefix := strconv.FormatInt(jobID, 10) + "."
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) > 256 {
			return nil, fmt.Errorf("live step row exceeds 256 bytes")
		}
		parts := strings.Split(strings.TrimSpace(line), "|")
		// Accept a trailing separator as well as parsable2 output.
		if len(parts) == 4 && parts[3] == "" {
			parts = parts[:3]
		}
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid live step response")
		}
		id := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(id, prefix) || !liveStepSuffix.MatchString(strings.TrimPrefix(id, prefix)) {
			return nil, fmt.Errorf("unexpected live step identity")
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate live step identity")
		}
		if len(steps) >= 1024 {
			return nil, fmt.Errorf("live step response exceeds 1024 steps")
		}
		seen[id] = true
		steps = append(steps, LiveStep{ID: id, PeakRSS: parseMemory(parts[1]), CPUSeconds: parseClock(parts[2])})
	}
	return steps, nil
}

func parseMemory(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	mult := float64(1)
	switch raw[len(raw)-1] {
	case 'K', 'k':
		mult = 1 << 10
	case 'M', 'm':
		mult = 1 << 20
	case 'G', 'g':
		mult = 1 << 30
	case 'T', 't':
		mult = 1 << 40
	}
	if mult != 1 {
		raw = raw[:len(raw)-1]
	}
	n, e := strconv.ParseFloat(raw, 64)
	if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n*mult >= float64(1<<63) {
		return nil
	}
	v := int64(n * mult)
	return &v
}
func parseClock(raw string) *float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	days := float64(0)
	if a, b, ok := strings.Cut(raw, "-"); ok {
		v, e := strconv.ParseFloat(a, 64)
		if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return nil
		}
		days = v
		raw = b
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return nil
	}
	n := float64(0)
	for _, p := range parts {
		v, e := strconv.ParseFloat(p, 64)
		if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return nil
		}
		n = n*60 + v
	}
	n += days * 86400
	// sstat can format INFINITE64 / CPU_TIME_ADJ as an enormous duration
	// (observed on Hummingbird extern steps). It is not measured CPU time.
	if math.IsInf(n, 0) || n >= float64(uint64(math.MaxUint64)/1000) {
		return nil
	}
	return &n
}
