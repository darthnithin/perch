package launch

import (
	"fmt"
	"math"
)

func normalizeTopology(s Spec) (Spec, error) {
	// Zero preserves older callers' single-task defaults. CLI/form fields reject
	// explicit zero before calling Build.
	if s.Nodes == 0 {
		s.Nodes = 1
	}
	if s.TasksPerNode == 0 {
		s.TasksPerNode = 1
	}
	if s.Nodes < 1 || s.Nodes > 1024 {
		return s, invalidField("nodes", fmt.Errorf("nodes must be 1–1024; scheduler limits still apply"))
	}
	if s.TasksPerNode < 1 || s.TasksPerNode > 4096 {
		return s, invalidField("tasks", fmt.Errorf("tasks per node must be 1–4096; scheduler limits still apply"))
	}
	if s.Launcher == "" || s.Launcher == "auto" {
		s.Launcher = "direct"
		if s.Nodes*s.TasksPerNode > 1 {
			s.Launcher = "srun"
		}
	}
	switch s.Launcher {
	case "direct":
		if s.Nodes*s.TasksPerNode != 1 {
			return s, invalidField("launcher", fmt.Errorf("direct launch requires one node and one task; choose srun or mpirun"))
		}
	case "srun", "mpirun":
	default:
		return s, invalidField("launcher", fmt.Errorf("launcher must be auto, direct, srun or mpirun"))
	}
	return s, nil
}

func checkResourceTotals(s Spec, array Array) error {
	nodes := int64(s.Nodes) * max(int64(1), array.Concurrent)
	if s.MemoryMiB > math.MaxInt64/nodes {
		return invalidField("memory", fmt.Errorf("aggregate memory request is too large"))
	}
	return nil
}

func (s Spec) launchCommand() []string {
	var args []string
	switch s.Launcher {
	case "srun":
		args = []string{"srun", fmt.Sprintf("--nodes=%d", s.Nodes), fmt.Sprintf("--ntasks=%d", s.Nodes*s.TasksPerNode), fmt.Sprintf("--ntasks-per-node=%d", s.TasksPerNode), fmt.Sprintf("--cpus-per-task=%d", s.CPUs), "--mpi=none", "--kill-on-bad-exit=1", "--"}
	case "mpirun":
		args = []string{"mpirun", "--np", fmt.Sprint(s.Nodes * s.TasksPerNode), "--map-by", fmt.Sprintf("ppr:%d:node:PE=%d", s.TasksPerNode, s.CPUs), "--bind-to", "core", "--"}
	}
	return append(args, s.Command...)
}

func (p Plan) ResourceSummary() string {
	s := p.Spec
	tasks := int64(s.Nodes * s.TasksPerNode)
	nodes := int64(s.Nodes)
	allocation := fmt.Sprintf("%d nodes · %d tasks (%d/node) · %d CPU/task\nPer node: %d MiB memory · %d GPU\nPer allocation: %d CPU · %d MiB · %d GPU · %d minutes\nLauncher: %s", s.Nodes, tasks, s.TasksPerNode, s.CPUs, s.MemoryMiB, s.GPUs, tasks*int64(s.CPUs), nodes*s.MemoryMiB, nodes*int64(s.GPUs), s.Minutes, s.Launcher)
	if s.Launcher == "srun" {
		allocation += " · independent tasks; MPI disabled"
	}
	if s.Launcher == "mpirun" {
		allocation += " · Open MPI must be loaded; command runs once per rank"
	}
	if s.Array == "" {
		return allocation
	}
	throttle := "no array throttle"
	if p.Array.Throttled {
		throttle = fmt.Sprintf("at most %d at once", p.Array.Concurrent)
	}
	return fmt.Sprintf("%d array tasks · %s\nEach array task requests:\n%s\nConcurrent ceiling: %d CPU · %d MiB · %d GPU\nEach array task receives SLURM_ARRAY_TASK_ID. Scheduler limits still apply.", p.Array.Tasks, throttle, allocation, p.Array.Concurrent*tasks*int64(s.CPUs), p.Array.Concurrent*nodes*s.MemoryMiB, p.Array.Concurrent*nodes*int64(s.GPUs))
}
