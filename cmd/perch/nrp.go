package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"perch/internal/domain"
	"perch/internal/nautilus"
	"perch/internal/nautilusui"
	"perch/internal/ui"
)

const nrpUsage = `Perch · experimental Nautilus workspace and queries

Usage:
  perch nrp prepare [flags] -- COMMAND [ARG...]   Review or export a local Job draft
  perch nrp validate JOB.json [flags]   Server Job dry-run and quota snapshot; no submission
  perch nrp ui [flags]        Open the experimental namespace workspace
  perch nrp jobs [flags]       Namespace Jobs, including collaborators' workloads
  perch nrp job NAME [flags]   Inspect one Job's observed controller state
  perch nrp quotas [flags]     Namespace ResourceQuota usage and limits
  perch nrp metrics POD [flags]   On-demand container CPU/memory sample
  perch nrp pods [flags]       Namespace Pods
  perch nrp pod NAME [flags]   Inspect containers, current/previous state and owners
  perch nrp logs POD --container NAME [flags]   Read a bounded container-log tail

Use --demo for synthetic offline data. Live queries require explicit --context
and --namespace, plus kubectl to read the selected configuration. No current
context is changed. Use --login in a terminal to explicitly run the selected
credential plugin (it may open a browser). Otherwise only static token/client-
certificate credentials are used. Quotas are reported limits, not free hardware.
prepare is offline. validate uses a server dry-run; it creates no Job or Pod.
Live submission and other workload actions remain unfinished.

Flags:
`

type nrpResponse struct {
	Version         int            `json:"schema_version"`
	Provider        string         `json:"provider"`
	Source          string         `json:"source"`
	Context         string         `json:"context,omitempty"`
	Scope           nautilus.Scope `json:"scope"`
	Command         string         `json:"command"`
	ResourceVersion string         `json:"resource_version,omitempty"`
	ObservedAt      time.Time      `json:"observed_at"`
	Data            any            `json:"data"`
}
type nrpJob struct {
	nautilus.Job
	State string `json:"state"`
}

func runNRP(args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "prepare" {
		return runNRPPrepare(args[1:], out)
	}
	flags := flag.NewFlagSet("perch nrp", flag.ContinueOnError)
	flags.SetOutput(out)
	demo := flags.Bool("demo", false, "synthetic offline namespace; no kubectl or network")
	login := flags.Bool("login", false, "explicitly run the selected credential plugin once in the foreground")
	noMouse := flags.Bool("no-mouse", false, "disable mouse capture in the namespace workspace")
	container := flags.String("container", "", "explicit container name for logs")
	previous := flags.Bool("previous", false, "read the previous terminated container instance's logs")
	contextName := flags.String("context", "", "explicit Kubernetes context")
	namespace := flags.String("namespace", "", "explicit namespace (never all namespaces)")
	kubeconfig := flags.String("kubeconfig", "", "optional kubeconfig file; otherwise kubectl's normal lookup")
	asJSON := flags.Bool("json", false, "versioned machine-readable output")
	timeout := flags.Duration("timeout", 45*time.Second, "maximum total query time (default 2m30s with --login)")
	flags.Usage = func() { fmt.Fprint(out, nrpUsage); flags.PrintDefaults() }
	ordered, err := interspersed(flags, args)
	if err != nil {
		return err
	}
	if err = flags.Parse(ordered); err != nil {
		return err
	}
	positional := flags.Args()
	if len(positional) == 0 || len(positional) == 1 && positional[0] == "help" {
		flags.Usage()
		return nil
	}
	command := positional[0]
	name := ""
	var manifest []byte
	switch command {
	case "validate":
		if len(positional) != 2 || *demo {
			return fmt.Errorf("nrp validate requires one Job JSON file and a live --context and --namespace; no demo validation")
		}
		manifest, err = readNRPJobManifest(positional[1], *namespace)
		if err != nil {
			return err
		}
	case "jobs", "pods", "quotas", "ui":
		if len(positional) != 1 {
			return fmt.Errorf("nrp %s takes no names", command)
		}
	case "job", "pod", "logs", "metrics":
		if len(positional) != 2 {
			return fmt.Errorf("nrp %s requires exactly one name", command)
		}
		name = positional[1]
		if !validNRPName(name) {
			return fmt.Errorf("invalid Kubernetes resource name")
		}
	default:
		return fmt.Errorf("unknown NRP command; use ui, jobs, job, pods, pod, quotas, metrics, logs, prepare or validate")
	}
	if command == "logs" {
		if !validNRPName(*container) || strings.Contains(*container, ".") || len(*container) > 63 {
			return fmt.Errorf("logs requires an explicit valid --container NAME")
		}
	} else if *container != "" || *previous {
		return fmt.Errorf("--container and --previous are only available with nrp logs")
	}
	if *login {
		supplied := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "timeout" {
				supplied = true
			}
		})
		if !supplied {
			*timeout = 150 * time.Second
		}
	}
	if *timeout <= 0 || *timeout > 5*time.Minute {
		return fmt.Errorf("--timeout must be positive and at most 5m")
	}
	if *demo && (*contextName != "" || *namespace != "" || *kubeconfig != "" || *login) {
		return fmt.Errorf("--demo uses its own synthetic research namespace; omit connection flags")
	}
	if !*demo && (*contextName == "" || *namespace == "") {
		return fmt.Errorf("NRP queries require --context and --namespace; use --demo to explore offline")
	}
	if *login && !term.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("--login requires an interactive terminal; background queries never start sign-in")
	}
	if command == "ui" && (*asJSON || !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd())) {
		return fmt.Errorf("nrp ui requires a terminal and does not support --json; use jobs or pods for scripted queries")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	lifetime := ctx
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	var client *nautilus.Client
	var closeConnection func()
	source := "synthetic-demo"
	if *demo {
		client = nautilus.NewDemoClient()
	} else {
		var conn *nautilus.Connection
		var err error
		if *login {
			conn, err = openNRPWithLogin(ctx, *contextName, *namespace, *kubeconfig)
		} else {
			conn, err = nautilus.OpenKubeconfig(ctx, *contextName, *namespace, *kubeconfig)
		}
		if err != nil {
			return err
		}
		defer conn.Close()
		closeConnection = conn.Close
		client = conn.Client
		source = "kubernetes-api"
	}
	if command == "ui" {
		cancel() // The startup budget does not limit the workspace lifetime.
		options := nautilusui.Options{Context: *contextName, Demo: *demo, NoMouse: *noMouse, PointerStyle: ui.DetectPointerStyle(os.Getenv), Close: closeConnection}
		if !*demo {
			options.Connect = func(ctx context.Context) (*nautilus.Connection, error) {
				return openNRPWithLogin(ctx, *contextName, *namespace, *kubeconfig)
			}
		}
		model := nautilusui.New(lifetime, client, options)
		defer model.Close()
		defer func() {
			if reset := model.PointerReset(); reset != "" {
				fmt.Fprint(out, reset)
			}
		}()
		_, err := tea.NewProgram(model, tea.WithContext(lifetime), tea.WithOutput(out)).Run()
		return err
	}
	response := nrpResponse{Version: 1, Provider: "nautilus", Source: source, Context: *contextName, Scope: client.Scope(), Command: command}
	switch command {
	case "validate":
		result, err := client.ValidateJob(ctx, manifest)
		if err != nil {
			return err
		}
		response.Data, response.ObservedAt = result, result.ObservedAt
		return writeNRPValidation(out, response, result, *asJSON)
	case "metrics":
		pod, err := client.GetPod(ctx, name)
		if err != nil {
			return err
		}
		metrics, err := client.ReadMetrics(ctx, pod.Ref)
		if err != nil {
			return err
		}
		response.Data, response.ObservedAt = metrics, metrics.ObservedAt
	case "quotas":
		result := client.ListQuotas(ctx)
		if !result.OK() {
			return result.Error
		}
		sort.Slice(result.Collection.Items, func(i, j int) bool { return result.Collection.Items[i].Name < result.Collection.Items[j].Name })
		response.Data, response.ResourceVersion, response.ObservedAt = result.Collection.Items, result.Collection.ResourceVersion, result.Collection.ObservedAt
	case "logs":
		pod, err := client.GetPod(ctx, name)
		if err != nil {
			return err
		}
		logs, err := client.ReadLogs(ctx, nautilus.LogRequest{Pod: pod.Ref, Container: *container, Previous: *previous})
		if err != nil {
			return err
		}
		if !*asJSON {
			_, err := io.WriteString(out, logs.Text)
			return err
		}
		response.Data, response.ObservedAt = logs, logs.ObservedAt
	case "jobs", "job":
		result := client.ListJobs(ctx)
		if !result.OK() {
			return result.Error
		}
		response.ResourceVersion = result.Collection.ResourceVersion
		response.ObservedAt = result.Collection.ObservedAt
		jobs := make([]nrpJob, 0, len(result.Collection.Items))
		for _, j := range result.Collection.Items {
			jobs = append(jobs, nrpJob{j, j.State()})
		}
		sort.Slice(jobs, func(i, j int) bool { return jobs[i].Name < jobs[j].Name })
		response.Data = jobs
		if command == "job" {
			found := false
			for _, j := range jobs {
				if j.Name == name {
					response.Data = j
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Job not present in the current namespace snapshot; absence is not a completion result")
			}
		}
	case "pods", "pod":
		result := client.ListPods(ctx)
		if !result.OK() {
			return result.Error
		}
		response.ResourceVersion = result.Collection.ResourceVersion
		response.ObservedAt = result.Collection.ObservedAt
		pods := result.Collection.Items
		sort.Slice(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })
		response.Data = pods
		if command == "pod" {
			found := false
			for _, p := range pods {
				if p.Name == name {
					response.Data = p
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("Pod not present in the current namespace snapshot; logs and history may have expired")
			}
		}
	}
	if *asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(response)
	}
	return writeNRP(out, response)
}

// A foreground plugin may disable echo or otherwise change the requesting
// terminal. Restore its original modes even after failed or cancelled sign-in.
func openNRPWithLogin(ctx context.Context, contextName, namespace, kubeconfig string) (conn *nautilus.Connection, err error) {
	fd := os.Stdin.Fd()
	state, err := term.GetState(fd)
	if err != nil {
		return nil, fmt.Errorf("unable to save terminal settings before sign-in")
	}
	defer func() {
		if term.Restore(fd, state) != nil {
			if conn != nil {
				conn.Close()
				conn = nil
			}
			err = fmt.Errorf("unable to restore terminal settings after sign-in; run reset in your terminal")
		}
	}()
	return nautilus.OpenKubeconfigWithLogin(ctx, contextName, namespace, kubeconfig, nautilus.LoginIO{Stdin: os.Stdin, Stderr: os.Stderr})
}

func nrpCount(n *int64) string {
	if n == nil {
		return "unknown"
	}
	return fmt.Sprint(*n)
}
func nrpReady(b *bool) string {
	if b == nil {
		return "unknown"
	}
	if *b {
		return "yes"
	}
	return "no"
}
func writeNRP(out io.Writer, r nrpResponse) error {
	var buffer strings.Builder
	contextLabel := domain.Label(r.Context)
	if r.Source == "synthetic-demo" {
		contextLabel = "offline demo · synthetic data"
	}
	fmt.Fprintf(&buffer, "Nautilus · %s\nNamespace: %s\n\n", contextLabel, r.Scope.Namespace)
	table := tabwriter.NewWriter(&buffer, 0, 4, 2, ' ', 0)
	switch data := r.Data.(type) {
	case nautilus.MetricsSnapshot:
		fmt.Fprintf(table, "%s · container resources\nSample: %s · window %s\n", data.Pod.Name, data.Timestamp.Format(time.RFC3339), data.Window)
		if data.Stale {
			fmt.Fprintln(table, "STALE sample · not current usage")
		}
		if len(data.Containers) == 0 {
			fmt.Fprintln(table, "No container measurements reported.")
		}
		fmt.Fprintln(table, "CONTAINER\tCPU CORES\tMEMORY WORKING SET\tATTRIBUTION")
		for _, c := range data.Containers {
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", c.Name, nautilus.QuotaValue(c.CPUCores), nautilus.DisplayQuotaValue("memory", c.MemoryBytes), c.Attribution)
		}
		fmt.Fprintln(table, "\nCPU is an average over the sample window; memory is an estimated working set, not peak usage.\nMissing values are unknown. This snapshot does not measure GPUs or Job-wide utilization.")
	case []nautilus.Quota:
		if len(data) == 0 {
			fmt.Fprintln(table, "No ResourceQuota objects reported. Other admission policies may still apply.")
		}
		for _, q := range data {
			fmt.Fprintf(table, "%s\nScope: %s\n", q.Name, q.ScopeDescription())
			fmt.Fprintln(table, "RESOURCE\tUSED\tLIMIT\tREMAINING\tSTATUS")
			for _, r := range q.Resources {
				fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", r.Name, nautilus.QuotaValue(r.Used), nautilus.QuotaValue(r.Hard), nautilus.QuotaValue(r.Remaining), r.State)
				if r.State == "pending-update" {
					fmt.Fprintf(table, "  Configured limit: %s; controller status has not caught up.\n", nautilus.QuotaValue(r.Configured))
				}
			}
			fmt.Fprintln(table)
		}
		fmt.Fprintln(table, "Reported quota usage, not live utilization or free hardware. Scoped quotas overlap; do not add them.\nCPU is in cores (m = 1/1000); memory/storage use Kubernetes quantities (Gi, Mi).")
	case []nrpJob:
		fmt.Fprintln(table, "JOB\tSTATE\tACTIVE PODS\tSUCCEEDED PODS\tFAILED PODS")
		for _, j := range data {
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", j.Name, j.State, nrpCount(j.Active), nrpCount(j.Succeeded), nrpCount(j.Failed))
		}
		if len(data) == 0 {
			fmt.Fprintln(table, "No Jobs in this namespace snapshot.")
		}
		fmt.Fprintln(table, "\nPod counts are not distinct completed indexes. Use nrp job NAME for details.")
	case nrpJob:
		j := data
		fmt.Fprintf(table, "%s\nUID: %s\nState: %s\n\n", j.Name, j.UID, j.State)
		fmt.Fprintf(table, "Active Pods\t%s\nSucceeded Pods\t%s\nFailed Pods\t%s\nReady Pods\t%s\nCompletions requested\t%s\nParallelism\t%s\n", nrpCount(j.Active), nrpCount(j.Succeeded), nrpCount(j.Failed), nrpCount(j.Ready), nrpCount(j.Completions), nrpCount(j.Parallelism))
		if j.CompletionMode != "" {
			fmt.Fprintf(table, "Completion mode\t%s\n", j.CompletionMode)
		}
		if j.CompletedIndexes != "" {
			fmt.Fprintf(table, "Completed indexes\t%s\n", j.CompletedIndexes)
		}
		if j.FailedIndexes != "" {
			fmt.Fprintf(table, "Failed indexes\t%s\n", j.FailedIndexes)
		}
		fmt.Fprintf(table, "Suspend requested\t%t\n", j.SuspendRequested)
		writeNRPConditions(table, j.Conditions)
	case []nautilus.Pod:
		fmt.Fprintln(table, "POD\tPHASE\tCONTAINERS\tSTATUS DETAIL")
		for _, p := range data {
			fmt.Fprintf(table, "%s\t%s\t%d\t%s\n", p.Name, p.Phase, len(p.Containers), nrpPodDetail(p))
		}
		if len(data) == 0 {
			fmt.Fprintln(table, "No Pods in this namespace snapshot.")
		}
		fmt.Fprintln(table, "\nUse nrp pod NAME for container readiness, restarts and current/previous states.")
	case nautilus.Pod:
		node := data.Node
		if node == "" {
			node = "unassigned"
		}
		fmt.Fprintf(table, "%s\nUID: %s\nPhase: %s\nNode: %s\n", data.Name, data.UID, data.Phase, node)
		if data.Deleted != nil {
			fmt.Fprintln(table, "Deletion requested; no completion inferred.")
		}
		if data.CompletionIndex != "" {
			fmt.Fprintf(table, "Completion index: %s\n", data.CompletionIndex)
		}
		if data.Reason != "" || data.Message != "" {
			fmt.Fprintf(table, "%s %s\n", data.Reason, data.Message)
		}
		fmt.Fprintln(table, "\nContainers")
		for _, c := range data.Containers {
			fmt.Fprintf(table, "\n%s (%s)\tready: %s · restarts: %s\nImage\t%s\n", c.Name, c.Role, nrpReady(c.Ready), nrpCount(c.Restarts), c.Image)
			writeNRPContainer(table, "Current", c.State)
			if c.Last.Phase != "Unknown" {
				writeNRPContainer(table, "Previous", c.Last)
			}
		}
		if len(data.Owners) > 0 {
			fmt.Fprintln(table, "\nOwner references (controller identity, not labels)")
		}
		for _, o := range data.Owners {
			fmt.Fprintf(table, "%s %s\tUID %s · controller: %t\n", o.Kind, o.Name, o.UID, o.Controller)
		}
		writeNRPConditions(table, data.Conditions)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(&buffer, "\nObserved %s · namespace snapshot, not durable history.\n", r.ObservedAt.UTC().Format(time.RFC3339))
	_, err := io.WriteString(out, buffer.String())
	return err
}
func writeNRPConditions(w io.Writer, conditions []nautilus.Condition) {
	if len(conditions) > 0 {
		fmt.Fprintln(w, "\nConditions")
	}
	for _, c := range conditions {
		fmt.Fprintf(w, "%s\t%s", c.Type, c.Status)
		if c.Reason != "" {
			fmt.Fprintf(w, " · %s", c.Reason)
		}
		fmt.Fprintln(w)
		if c.Message != "" {
			fmt.Fprintln(w, c.Message)
		}
	}
}
func writeNRPContainer(w io.Writer, label string, s nautilus.ContainerState) {
	fmt.Fprintf(w, "%s\t%s", label, s.Phase)
	if s.Reason != "" {
		fmt.Fprintf(w, " · %s", s.Reason)
	}
	if s.ExitCode != nil {
		fmt.Fprintf(w, " · exit %d", *s.ExitCode)
	}
	fmt.Fprintln(w)
	if s.Message != "" {
		fmt.Fprintln(w, s.Message)
	}
}

func nrpPodDetail(p nautilus.Pod) string {
	if p.Deleted != nil {
		return "Deletion requested"
	}
	if p.Reason != "" {
		return p.Reason
	}
	for _, c := range p.Conditions {
		if c.Type == "PodScheduled" && c.Status == "False" {
			if c.Reason != "" {
				return c.Reason
			}
			return "Not scheduled"
		}
	}
	var reasons []string
	for _, c := range p.Containers {
		if c.State.Phase == "Waiting" && c.State.Reason != "" {
			reasons = append(reasons, c.Name+": "+c.State.Reason)
		}
		if c.State.Phase == "Terminated" && c.State.ExitCode != nil && *c.State.ExitCode != 0 {
			reason := c.State.Reason
			if reason == "" {
				reason = fmt.Sprintf("exit %d", *c.State.ExitCode)
			}
			reasons = append(reasons, c.Name+": "+reason)
		}
	}
	if len(reasons) == 0 {
		return "—"
	}
	return strings.Join(reasons, "; ")
}

func validNRPName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if len(part) == 0 || len(part) > 63 {
			return false
		}
		for i, ch := range part {
			if ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' {
				continue
			}
			if ch != '-' || i == 0 || i == len(part)-1 {
				return false
			}
		}
	}
	return true
}
