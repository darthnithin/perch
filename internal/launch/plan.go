// Package launch prepares and explicitly submits reviewed Slurm workloads.
package launch

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"perch/internal/hummingbird"
	"perch/internal/transport"
)

type Spec struct {
	Nodes        int      `json:"nodes"`
	TasksPerNode int      `json:"tasks_per_node"`
	Launcher     string   `json:"launcher"`
	Array        string   `json:"array,omitempty"`
	AfterOK      string   `json:"after_ok,omitempty"`
	Name         string   `json:"name"`
	WorkDir      string   `json:"working_directory"`
	Partition    string   `json:"partition,omitempty"`
	CPUs         int      `json:"cpus"`
	MemoryMiB    int64    `json:"memory_mib"`
	Minutes      int64    `json:"time_limit_minutes"`
	GPUs         int      `json:"gpus"`
	Modules      []string `json:"modules"`
	Command      []string `json:"command"`
}
type Plan struct {
	Array     Array    `json:"array"`
	Spec      Spec     `json:"spec"`
	Arguments []string `json:"sbatch_arguments"`
	Script    string   `json:"script"`
	Stdout    string   `json:"stdout_pattern"`
	Stderr    string   `json:"stderr_pattern"`
}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,79}$`)

func MemoryMiB(value string) (int64, error) {
	r := regexp.MustCompile(`^([1-9][0-9]*)([MG])$`).FindStringSubmatch(strings.ToUpper(value))
	if r == nil {
		return 0, fmt.Errorf("memory must be positive MiB or GiB, for example 512M or 4G")
	}
	n, e := strconv.ParseInt(r[1], 10, 64)
	if e != nil || n > 1<<30 {
		return 0, fmt.Errorf("memory request is too large")
	}
	if r[2] == "G" {
		n *= 1024
	}
	return n, nil
}
func Minutes(value string) (int64, error) {
	d, e := time.ParseDuration(value)
	if e != nil || d < time.Minute || d%time.Minute != 0 {
		return 0, fmt.Errorf("time must be a positive whole-minute duration, for example 10m or 2h30m")
	}
	return int64(d / time.Minute), nil
}
func cleanText(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func Build(s Spec) (Plan, error) {
	p := Plan{Spec: s}
	var err error
	s, err = normalizeTopology(s)
	if err != nil {
		return p, err
	}
	if !namePattern.MatchString(s.Name) {
		return p, invalidField("name", fmt.Errorf("name must be 1–80 letters, digits, dots, underscores or hyphens, starting with a letter or digit"))
	}
	if !path.IsAbs(s.WorkDir) || !cleanText(s.WorkDir) || len(s.WorkDir) > 4096 {
		return p, invalidField("workdir", fmt.Errorf("workdir must be an absolute cluster path without control characters"))
	}
	if s.Partition != "" && !namePattern.MatchString(s.Partition) {
		return p, invalidField("partition", fmt.Errorf("use one exact partition name"))
	}
	if s.CPUs < 1 || s.CPUs > 4096 {
		return p, invalidField("cpus", fmt.Errorf("CPUs per task must be 1–4096"))
	}
	if s.MemoryMiB < 1 || s.MemoryMiB > 1<<40 {
		return p, invalidField("memory", fmt.Errorf("memory must be positive and at most 1099511627776 MiB"))
	}
	if s.Minutes < 1 || s.Minutes > 525600 {
		return p, invalidField("time", fmt.Errorf("time must be 1–525600 whole minutes"))
	}
	if s.GPUs < 0 || s.GPUs > 64 {
		return p, invalidField("gpus", fmt.Errorf("GPUs per node must be 0–64"))
	}
	if len(s.Command) == 0 || s.Command[0] == "" || strings.HasPrefix(s.Command[0], "-") {
		return p, invalidField("command", fmt.Errorf("provide an executable after --"))
	}
	total := 0
	for _, arg := range s.Command {
		total += len(arg)
		if !cleanText(arg) {
			return p, invalidField("command", fmt.Errorf("command arguments cannot contain control characters"))
		}
	}
	if total > 32768 || len(s.Command) > 512 {
		return p, invalidField("command", fmt.Errorf("command exceeds the 32 KiB or 512-argument limit"))
	}
	if len(s.Modules) > 32 {
		return p, invalidField("modules", fmt.Errorf("at most 32 modules may be requested"))
	}
	for _, module := range s.Modules {
		if !hummingbird.ValidModuleName(module) {
			return p, invalidField("modules", fmt.Errorf("invalid module %q", module))
		}
	}
	array, err := ParseArray(s.Array)
	if err != nil {
		return p, invalidField("array", err)
	}
	dependencies, err := Dependencies(s.AfterOK)
	if err != nil {
		return p, invalidField("afterok", err)
	}
	s.Array = array.Expression
	s.AfterOK = strings.Join(dependencies, ",")
	p.Spec = s
	p.Array = array
	if err := checkResourceTotals(s, array); err != nil {
		return p, err
	}
	// Percent escaping is specific to Slurm filename patterns, not --chdir.
	outputDir := strings.ReplaceAll(path.Clean(s.WorkDir), "%", "%%")
	outputName := "perch-%j"
	if s.Array != "" {
		outputName = "perch-%A_%a"
	}
	p.Stdout = path.Join(outputDir, outputName+".out")
	p.Stderr = path.Join(outputDir, outputName+".err")
	p.Arguments = []string{"--job-name=" + s.Name, "--chdir=" + s.WorkDir, fmt.Sprintf("--nodes=%d", s.Nodes), fmt.Sprintf("--ntasks=%d", s.Nodes*s.TasksPerNode), fmt.Sprintf("--ntasks-per-node=%d", s.TasksPerNode), fmt.Sprintf("--cpus-per-task=%d", s.CPUs), fmt.Sprintf("--mem=%dM", s.MemoryMiB), fmt.Sprintf("--time=%d", s.Minutes), "--output=" + p.Stdout, "--error=" + p.Stderr, "--open-mode=append", "--export=ALL", "--input=/dev/null"}
	if s.Array != "" {
		p.Arguments = append(p.Arguments, "--array="+s.Array)
	}
	if len(dependencies) > 0 {
		p.Arguments = append(p.Arguments, "--dependency=afterok:"+strings.Join(dependencies, ":"), "--kill-on-invalid-dep=yes")
	}
	if s.Partition != "" {
		p.Arguments = append(p.Arguments, "--partition="+s.Partition)
	}
	if s.GPUs > 0 {
		p.Arguments = append(p.Arguments, fmt.Sprintf("--gres=gpu:%d", s.GPUs))
	}
	var body strings.Builder
	body.WriteString("#!/bin/bash -l\n# Generated by Perch. Resources are supplied as sbatch arguments.\nset -eo pipefail\numask 077\n")
	if len(s.Modules) > 0 {
		body.WriteString("module load " + transport.QuoteArgs(s.Modules) + "\n")
	}
	body.WriteString("export OMP_NUM_THREADS=\"${SLURM_CPUS_PER_TASK:-1}\"\nexec " + transport.QuoteArgs(s.launchCommand()) + "\n")
	p.Script = body.String()
	return p, nil
}
