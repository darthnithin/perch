// Package cli formats the shared query results without terminal control codes.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"perch/internal/domain"
	"perch/internal/query"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

func Write(w io.Writer, response query.Response, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(response)
	}
	if text, ok := response.Data.(query.Text); ok {
		_, err := io.WriteString(w, text.Text)
		return err
	}
	t := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	switch data := response.Data.(type) {
	case []query.Node:
		fmt.Fprintln(t, "NODE / PARTITIONS\tSTATE\tCPU ALLOC/TOTAL\tCPU IDLE\tMEM ALLOC/TOTAL\tMEM UNALLOCATED\tSYSTEM RESERVED")
		for _, n := range data {
			fmt.Fprintf(t, "%s / %s\t%s\t%s / %s\t%s\t%s / %s\t%s\t%s\n", n.Name, strings.Join(n.Partitions, ","), strings.Join(n.States, "+"), nodeCount(n.AllocatedCPUs), nodeCount(n.CPUs), nodeCount(n.IdleCPUs), nodeMemory(n.AllocatedMemory), nodeMemory(n.Memory), nodeMemory(n.UnallocatedMemory), nodeMemory(n.ReservedMemory))
			if n.GRES != "" {
				for _, gpu := range n.GPUs {
					kind := gpu.Type
					if kind == "" {
						kind = "untyped"
					}
					fmt.Fprintf(t, "  GPU %s: %s / %d allocated; %s unallocated\n", kind, nodeCount(gpu.Allocated), gpu.Configured, nodeCount(gpu.Unallocated))
				}
				if n.GPUNote != "" {
					fmt.Fprintln(t, "  "+n.GPUNote)
				}
				fmt.Fprintf(t, "  GRES %s; allocated %s\n", n.GRES, n.GRESUsed)
			}
		}
		if len(data) == 0 {
			fmt.Fprintln(t, "No node records reported.")
		}
		fmt.Fprintln(t, "Scheduler allocations, not utilization or a start-time guarantee.\nUnallocated memory excludes job allocations and system-reserved memory; it is not OS free memory.\n? means unavailable. Node flags and scheduling policy still apply.")
	case []query.Priority:
		if policy := response.PriorityPolicy; policy != nil {
			fmt.Fprintln(t, policy.Note)
			if policy.FairshareWeight != nil {
				fmt.Fprintf(t, "Configured fair-share weight: %d\n\n", *policy.FairshareWeight)
			}
		}
		if len(data) == 0 {
			fmt.Fprintln(t, "No priority records reported for this user.")
		}
		for _, p := range data {
			fmt.Fprintf(t, "Allocation %s · %s · score %d\nFACTOR\tWEIGHTED SCORE\n", p.JobID, p.Partition, p.Score)
			for _, f := range p.Factors {
				fmt.Fprintf(t, "%s\t%s\n", f.Name, strconv.FormatFloat(f.Value, 'f', -1, 64))
			}
			fmt.Fprintf(t, "Nice adjustment\t%d\n\n", p.Nice)
		}
		fmt.Fprintln(t, "Reported priority factors, not queue positions or start-time guarantees.\nA zero contribution does not by itself mean a factor is disabled.\nAllocation IDs may represent array records. Multiple partitions are separate scores.")
	case query.WaitHistory:
		fmt.Fprintln(t, "PARTITION\tSTARTS\tARRAY TASKS\tSUBMIT MEDIAN / P90\tELIGIBLE MEDIAN / P90\tELIGIBLE SAMPLES")
		for _, p := range data.Partitions {
			eligible, samples := "unavailable", 0
			if d := p.EligibleToStart; d != nil {
				eligible = duration(d.MedianSeconds) + " / " + duration(d.P90Seconds)
				samples = d.Samples
			}
			fmt.Fprintf(t, "%s\t%d\t%d\t%s / %s\t%s\t%d\n", p.Partition, p.SubmitToStart.Samples, p.ArrayTasks, duration(p.SubmitToStart.MedianSeconds), duration(p.SubmitToStart.P90Seconds), eligible, samples)
		}
		if len(data.Partitions) == 0 {
			fmt.Fprintln(t, "No usable started jobs in this window.")
		}
		fmt.Fprintf(t, "\n%d records excluded. %s\n", data.Excluded, data.Notes)
	case []query.Software:
		fmt.Fprintln(t, "MODULE\tLOGIN ENVIRONMENT")
		for _, m := range data {
			status := "Check prerequisites"
			if m.Available {
				status = "Available"
			}
			fmt.Fprintf(t, "%s\t%s\n", m.Name, status)
		}
		if len(data) == 0 {
			fmt.Fprintln(t, "No modules match this search.")
		}
		fmt.Fprintln(t, "\nAvailable refers to this query's login environment; it does not guarantee workload compatibility.")
		fmt.Fprintln(t, "Use perch module NAME/VERSION for prerequisites. Nothing has been loaded.")
	case query.ModuleInfo:
		fmt.Fprintf(t, "%s\n\n%s\n\nAfter satisfying the prerequisites above, use in your cluster shell or batch script:\n  %s\n\nThis command is shown for review; Perch has not run it.\n", data.Name, data.Details, data.LoadCommand)

	case []query.Job:
		fmt.Fprintln(t, "JOB\tNAME\tSTATE\tPARTITION\tELAPSED\tREASON")
		for _, j := range data {
			fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\t%s\n", j.ID, j.Name, j.State, j.Partition, duration(j.ElapsedSeconds), j.Reason)
		}
		if len(data) == 0 {
			fmt.Fprintln(t, "No jobs in this view.")
		}
	case query.Job:
		writeJob(t, data, response.At)
	case []query.Partition:
		fmt.Fprintln(t, "PARTITION\tSTATE\tCPUS ALLOC/TOTAL\tIDLE CPUS\tNODES\tGRES")
		for _, p := range data {
			fmt.Fprintf(t, "%s\t%s\t%d/%d\t%d\t%d\t%s\n", p.Name, p.State, p.AllocatedCPUs, p.CPUs, p.IdleCPUs, p.NodeCount, p.GRES)
		}
	case []query.Array:
		fmt.Fprintln(t, "ARRAY\tOBSERVED\tDONE\tFAILED\tRUNNING\tQUEUED\tUNRESOLVED\tFAILED TASK SPEC")
		for _, a := range data {
			fmt.Fprintf(t, "%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", a.ID, a.Observed, a.Completed, a.Failed, a.Running, a.Pending, a.Unresolved, a.FailedSpec)
		}
		if len(data) == 0 {
			fmt.Fprintln(t, "No arrays observed.")
		}
	case query.Resources:
		writeJob(t, data.Job, response.At)
		fmt.Fprintf(t, "\nRecorded peak RSS\t%s\nRSS scope\t%s\nCPU efficiency\t%s\n", memory(data.Job.PeakRSSBytes), data.Job.RSSScope, percent(data.Job.CPUEfficiencyPercent))
		if data.Job.CPUEfficiencyNote != "" {
			fmt.Fprintf(t, "Accounting note\t%s\n", data.Job.CPUEfficiencyNote)
		}
		if data.LiveScope != "" {
			fmt.Fprintf(t, "Live peak RSS\t%s\nLive scope\t%s\n", memory(data.LivePeakRSSBytes), data.LiveScope)
			cpu := "unavailable"
			if data.LiveAverageTaskCPUSeconds != nil {
				cpu = fmt.Sprintf("%.3f s", *data.LiveAverageTaskCPUSeconds)
			}
			fmt.Fprintf(t, "Live average task CPU time\t%s\n", cpu)
		}
		if len(data.LiveSteps) > 0 {
			fmt.Fprintln(t, "\nLive steps · task peaks and average task CPU time, not job totals")
			fmt.Fprintln(t, "STEP\tPEAK RSS\tAVERAGE TASK CPU TIME")
			for _, step := range data.LiveSteps {
				cpu := "unavailable"
				if step.AverageTaskCPUSeconds != nil {
					cpu = fmt.Sprintf("%.3f s", *step.AverageTaskCPUSeconds)
				}
				fmt.Fprintf(t, "%s\t%s\t%s\n", step.ID, memory(step.PeakRSSBytes), cpu)
			}
		}
		if len(data.Steps) > 0 {
			fmt.Fprintln(t, "\nRecorded steps · peaks are not summed")
			fmt.Fprintln(t, "STEP\tSTATE\tTASKS\tCPU TIME\tPEAK RSS\tRSS SCOPE")
			for _, s := range data.Steps {
				tasks, cpu := "unavailable", "unavailable"
				if s.Tasks != nil {
					tasks = fmt.Sprint(*s.Tasks)
				}
				if s.TotalCPUSeconds != nil {
					cpu = fmt.Sprintf("%.3f s", *s.TotalCPUSeconds)
				}
				fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\t%s\n", s.ID, s.State, tasks, cpu, memory(s.PeakRSSBytes), s.RSSScope)
			}
		} else {
			fmt.Fprintln(t, "\nNo step accounting records observed.")
		}
	default:
		return fmt.Errorf("unsupported output type %T", response.Data)
	}
	return t.Flush()
}
func writeJob(w io.Writer, j query.Job, at time.Time) {
	if j.Restarts > 0 {
		fmt.Fprintf(w, "Restarts\t%d\n", j.Restarts)
	}
	fmt.Fprintf(w, "Job\t%s\nName\t%s\nState\t%s\nPartition\t%s\nNodes\t%s\nElapsed\t%s\nCPUs\t%d\nRequested memory\t%s\n", j.ID, j.Name, j.State, j.Partition, j.Nodes, duration(j.ElapsedSeconds), j.CPUs, memory(j.RequestedMemoryBytes))
	for _, request := range j.GPURequests {
		fmt.Fprintf(w, "GPU request\t%s\n", request.Label())
	}
	if j.GPURequestNote != "" {
		fmt.Fprintf(w, "GPU request\t%s\n", j.GPURequestNote)
	}
	if j.AllocatedMemoryBytes != nil {
		fmt.Fprintf(w, "Allocated memory\t%s\n", memory(j.AllocatedMemoryBytes))
	}
	if j.Reason != "" {
		fmt.Fprintf(w, "Reason\t%s\n", j.Reason)
	}
	if j.State == "PENDING" {
		if j.EstimatedStart == nil {
			fmt.Fprintln(w, "Start estimate\tunavailable")
		} else {
			note := "tentative; may change"
			if !at.IsZero() && !j.EstimatedStart.After(at) {
				note = "past estimate; still queued at observation"
			}
			fmt.Fprintf(w, "Start estimate\t%s (%s)\n", j.EstimatedStart.Format(time.RFC3339), note)
		}
	}
	if j.TimeLimitSeconds != nil {
		fmt.Fprintf(w, "Time limit\t%s\n", duration(*j.TimeLimitSeconds))
	}
	if j.ExitCode != nil {
		fmt.Fprintf(w, "Exit code\t%d\n", *j.ExitCode)
	}
}
func duration(s float64) string { return domain.Duration(time.Duration(s * float64(time.Second))) }
func memory(n *int64) string {
	if n == nil {
		return "unavailable"
	}
	return domain.Bytes(*n)
}
func percent(n *float64) string {
	if n == nil {
		return "unavailable"
	}
	return fmt.Sprintf("%.1f%%", *n)
}

func nodeCount(v *int64) string {
	if v == nil {
		return "?"
	}
	return strconv.FormatInt(*v, 10)
}
func nodeMemory(v *int64) string {
	if v == nil {
		return "?"
	}
	if *v < 0 {
		return "-" + domain.Bytes(-*v)
	}
	return domain.Bytes(*v)
}
