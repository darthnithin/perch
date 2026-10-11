package query

import (
	"sort"
	"time"

	"perch/internal/domain"
)

// WaitHistory describes started allocations, not a forecast of pending jobs.
// Array tasks count separately. Percentiles use the nearest-rank method.
type WaitHistory struct {
	Partitions []PartitionWait `json:"partitions"`
	Excluded   int             `json:"excluded_records"`
	Notes      string          `json:"notes"`
}
type WaitDistribution struct {
	Samples       int     `json:"samples"`
	MedianSeconds float64 `json:"median_seconds"`
	P90Seconds    float64 `json:"p90_seconds"`
}
type PartitionWait struct {
	Partition       string            `json:"partition"`
	ArrayTasks      int               `json:"array_tasks"`
	SubmitToStart   WaitDistribution  `json:"submit_to_start"`
	EligibleToStart *WaitDistribution `json:"eligible_to_start"`
}

const WaitNotes = "Your starts in the last 7 days, from available accounting. Each array task counts separately; large arrays can dominate. Submit-to-start includes holds and dependencies. Eligible-to-start begins when eligible, but can include other delays. Restarted, unstarted, duplicate, aggregate and invalid records are excluded. Historical waits are not start-time predictions or comparisons of equivalent resource requests."

func Waits(jobs []domain.Job, at time.Time) WaitHistory {
	report := WaitHistory{Partitions: []PartitionWait{}, Notes: WaitNotes}
	type samples struct {
		submit, eligible []float64
		tasks            int
	}
	groups := map[string]*samples{}
	// Conflicting duplicate attempts are omitted, rather than selecting arbitrary timestamps.
	counts := map[string]int{}
	for _, j := range jobs {
		counts[j.AttemptKey()]++
	}
	for _, j := range jobs {
		if counts[j.AttemptKey()] != 1 || j.Restarts != 0 || j.AwaitingTasks > 0 || j.TaskRange != "" ||
			(j.ArrayID > 0 && j.TaskID == nil) || !singleID.MatchString(j.ID) || j.Partition == "" ||
			j.State == "PENDING" || j.State == "REQUEUED" || j.State == "RESIZING" ||
			j.Submit.IsZero() || j.Start.IsZero() || j.Start.Before(j.Submit) || j.Start.After(at) || j.Start.Before(at.Add(-7*24*time.Hour)) {
			report.Excluded++
			continue
		}
		key := domain.Label(j.Partition)
		g := groups[key]
		if g == nil {
			g = &samples{}
			groups[key] = g
		}
		g.submit = append(g.submit, j.Start.Sub(j.Submit).Seconds())
		if j.TaskID != nil {
			g.tasks++
		}
		if !j.Eligible.IsZero() && !j.Eligible.Before(j.Submit) && !j.Eligible.After(j.Start) {
			g.eligible = append(g.eligible, j.Start.Sub(j.Eligible).Seconds())
		}
	}
	for name, g := range groups {
		p := PartitionWait{Partition: name, ArrayTasks: g.tasks, SubmitToStart: distribution(g.submit)}
		if len(g.eligible) > 0 {
			d := distribution(g.eligible)
			p.EligibleToStart = &d
		}
		report.Partitions = append(report.Partitions, p)
	}
	sort.Slice(report.Partitions, func(i, j int) bool { return report.Partitions[i].Partition < report.Partitions[j].Partition })
	return report
}
func distribution(values []float64) WaitDistribution {
	sort.Float64s(values)
	n := len(values)
	median := values[n/2]
	if n%2 == 0 {
		median = (values[n/2-1] + median) / 2
	}
	return WaitDistribution{Samples: n, MedianSeconds: median, P90Seconds: values[(9*n+9)/10-1]}
}
