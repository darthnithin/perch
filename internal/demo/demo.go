package demo

import (
	"fmt"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	fixtures "perch/testdata/hummingbird"
	"time"
)

// Snapshot is deterministic relative to the supplied clock. Live fixtures
// supply shapes and units; all scenario jobs and logs are synthetic.
func Snapshot(now time.Time, scenario int) hummingbird.Refresh {
	partitions, _ := hummingbird.ParsePartitions(fixtures.Partitions)
	history, _ := hummingbird.ParseAccounting(fixtures.Accounting)
	base := now.Truncate(24 * time.Hour).Add(-3 * time.Hour)
	var queue []domain.Job
	for i := 0; i < 12; i++ {
		task := int64(i)
		j := domain.Job{ID: fmt.Sprintf("9100_%d", i), RawID: int64(9101 + i), ArrayID: 9100, TaskID: &task, Name: "parameter-sweep", User: "researcher", Cluster: "hummingbird", Partition: "128x24", Submit: base, Eligible: base.Add(20 * time.Minute), Start: base.Add(time.Hour), CPUs: 2, NodeCount: 1, Limit: 2 * time.Hour, Elapsed: 38*time.Minute + time.Duration(i)*31*time.Second, State: "RUNNING", Nodes: "node-14", Stdout: "/home/researcher/experiments/sweep.out", WorkDir: "/home/researcher/experiments"}
		mem := int64(4 << 30)
		j.RequestedMemory = &mem
		if i < 7 {
			j.State = "COMPLETED"
			j.End = j.Start.Add(j.Elapsed)
			cpu := j.Elapsed.Seconds() * 1.84
			j.TotalCPU = &cpu
			rss := int64(875+i*30) << 20
			j.PeakRSS = &rss
			j.RSSScope = "largest recorded task peak (batch step)"
			history = append(history, j)
		} else if i == 7 {
			j.State = "OUT_OF_MEMORY"
			j.End = j.Start.Add(j.Elapsed)
			rss := int64(39) * (1 << 30) / 10
			j.PeakRSS = &rss
			j.RSSScope = "largest recorded task peak (batch step)"
			history = append(history, j)
		} else {
			queue = append(queue, j)
		}
	}
	queue = append(queue, domain.Job{ID: "9100_[12-39]", RawID: 9113, ArrayID: 9100, TaskRange: "12-39", Name: "parameter-sweep", User: "researcher", Cluster: "hummingbird", State: "PENDING", Reason: "Resources", Partition: "128x24", CPUs: 2, NodeCount: 1, Submit: base, Limit: 2 * time.Hour})
	queue = append(queue, domain.Job{ID: "9231", RawID: 9231, Name: "embedding-model", User: "researcher", Cluster: "hummingbird", State: "PENDING", Reason: "Priority", Partition: "96x24gpu4", CPUs: 8, NodeCount: 1, GPURequests: []domain.GPURequest{{Scope: "node", Count: 1}}, Submit: base.Add(time.Hour), EstimatedStart: now.Add(45 * time.Minute), Limit: 4 * time.Hour})
	if scenario%4 == 1 && len(queue) > 0 {
		j := queue[0]
		j.State = "COMPLETED"
		j.End = now
		history = append(history, j)
		queue = queue[1:]
	}
	if scenario%4 == 2 {
		for i := range queue {
			if queue[i].Running() {
				queue[i].Elapsed = queue[i].Limit - 3*time.Minute
			}
		}
	}
	if scenario%4 == 3 {
		queue = nil
	}
	domain.SortJobs(queue)
	return hummingbird.Refresh{At: now, Queue: queue, History: history, Partitions: partitions, QueueOK: true, HistoryOK: true, PartitionsOK: true, Errors: map[string]error{}}
}

func Log(j domain.Job) string {
	if j.State == "PENDING" {
		return "Output appears when this task starts.\n"
	}
	return fmt.Sprintf("[demo] %s · %s\n\nLoading experiment configuration…\nDataset ready: 48,000 observations\n\n  epoch   loss       validation\n  001     0.8412     0.7950\n  002     0.6127     0.5801\n  003     0.4382     0.4208\n  004     0.3124     0.3077\n\nCheckpoint saved.\nWaiting for the next batch…\n", j.ID, j.Name)
}

const Script = "#!/bin/bash\n#SBATCH --job-name=parameter-sweep\n#SBATCH --partition=128x24\n#SBATCH --array=0-39\n#SBATCH --cpus-per-task=2\n#SBATCH --mem=4G\n#SBATCH --time=02:00:00\n#SBATCH --output=logs/sweep-%A_%a.out\n\n# Synthetic demo script. Perch only displays it.\npython train.py --configuration \"$SLURM_ARRAY_TASK_ID\"\n"
