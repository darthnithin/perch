package launch

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/transport"
)

// Opt-in only: compile on one compute CPU, then two independent tasks on one
// node and a two-rank MPI collective across two nodes. No login-node compute.
func TestHummingbirdTopologyIntegration(t *testing.T) {
	host, root := os.Getenv("PERCH_TOPOLOGY_LIVE_HOST"), os.Getenv("PERCH_TOPOLOGY_LIVE_WORKDIR")
	if host == "" || root == "" {
		t.Skip("set PERCH_TOPOLOGY_LIVE_HOST and PERCH_TOPOLOGY_LIVE_WORKDIR for small compute jobs")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ssh, err := transport.NewSSH(host)
	if err != nil {
		t.Fatal(err)
	}
	defer ssh.Close()
	monitor := &hummingbird.Client{Runner: ssh, Host: host}
	if err = monitor.Identify(ctx); err != nil {
		t.Fatal(err)
	}
	directory, err := ssh.Run(ctx, []string{"mktemp", "-d", root + "/perch-topology.XXXXXXXX"})
	if err != nil || directory.ExitCode != 0 {
		t.Fatal(directory, err)
	}
	dir := strings.TrimSpace(directory.Stdout)
	if !strings.HasPrefix(dir, root+"/perch-topology.") || strings.ContainsAny(dir, "\r\n") {
		t.Fatal("invalid test directory")
	}
	t.Log("synthetic workspace", dir)
	client := Client{Runner: ssh}
	run := func(s Spec) (domain.Job, string) {
		t.Helper()
		p, err := Build(s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = client.Check(ctx, p); err != nil {
			t.Fatal(err)
		}
		receipt, err := client.Submit(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(s.Name, receipt.JobID)
		for {
			r, err := monitor.Refresh(ctx, true, false)
			if err != nil {
				t.Fatal(err)
			}
			if !r.HistoryOK {
				t.Fatal(r.Errors)
			}
			for _, j := range r.History {
				if j.ID != receipt.JobID || !j.Terminal() {
					continue
				}
				log, err := monitor.ReadLog(ctx, j, false, hummingbird.LogCursor{})
				if err != nil {
					t.Fatal(err)
				}
				if j.State != "COMPLETED" {
					stderr, _ := monitor.ReadLog(ctx, j, true, hummingbird.LogCursor{})
					t.Fatalf("%s %s: stdout=%s stderr=%s", j.ID, j.State, log.Raw, stderr.Raw)
				}
				return j, log.Raw
			}
			select {
			case <-ctx.Done():
				t.Fatalf("job %s outcome not observed: %v; do not resubmit", receipt.JobID, ctx.Err())
			case <-time.After(10 * time.Second):
			}
		}
	}
	s := Spec{Name: "perch-tasks-smoke", WorkDir: dir, Nodes: 1, TasksPerNode: 2, CPUs: 1, MemoryMiB: 256, Minutes: 1, Launcher: "srun", Command: []string{"/bin/bash", "-c", `printf 'TASK=%s\n' "$SLURM_PROCID"`}}
	_, out := run(s)
	if !strings.Contains(out, "TASK=0\n") || !strings.Contains(out, "TASK=1\n") || len(strings.Fields(out)) != 2 {
		t.Fatal("task launch", out)
	}
	s.Name = "perch-mpi-build"
	s.TasksPerNode = 1
	s.Launcher = "direct"
	s.Minutes = 2
	s.Modules = []string{"gnu15/15.2.0", "openmpi5/5.0.10"}
	source := `#include <mpi.h>
#include <stdio.h>
int main(int argc,char **argv){
 MPI_Init(&argc,&argv);
 int rank,size,sum,len; char host[MPI_MAX_PROCESSOR_NAME];
 MPI_Comm_rank(MPI_COMM_WORLD,&rank); MPI_Comm_size(MPI_COMM_WORLD,&size);
 MPI_Allreduce(&rank,&sum,1,MPI_INT,MPI_SUM,MPI_COMM_WORLD);
 MPI_Get_processor_name(host,&len);
 printf("RANK=%d SIZE=%d SUM=%d HOST=%s\n",rank,size,sum,host);
 MPI_Finalize(); return (size==2 && sum==1)?0:3;
}
`
	s.Command = []string{"/bin/bash", "-c", "printf %s " + transport.Quote(base64.StdEncoding.EncodeToString([]byte(source))) + " | base64 -d > hello.c; mpicc hello.c -o hello"}
	run(s)
	s.Name = "perch-mpi-smoke"
	s.Nodes = 2
	s.Minutes = 1
	s.Launcher = "mpirun"
	s.Command = []string{dir + "/hello"}
	job, out := run(s)
	hosts := map[string]bool{}
	ranks := map[int]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var rank, size, sum int
		var node string
		if n, err := fmt.Sscanf(line, "RANK=%d SIZE=%d SUM=%d HOST=%s", &rank, &size, &sum, &node); err != nil || n != 4 || size != 2 || sum != 1 {
			t.Fatal("MPI collective output", line)
		}
		ranks[rank] = true
		hosts[node] = true
	}
	if !ranks[0] || !ranks[1] || len(hosts) != 2 || job.NodeCount != 2 {
		t.Fatal("MPI did not span two nodes", out, job.NodeCount)
	}
	t.Log("verified two independent tasks and a two-node MPI_Allreduce; synthetic sources and logs retained in", dir)
}
