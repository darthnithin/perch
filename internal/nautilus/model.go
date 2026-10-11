// Package nautilus models Kubernetes workloads independently of Slurm jobs.
// It provides parsing, bounded lists and a credential-snapshot HTTPS reader.
// Experimental CLI queries, logs, namespace UI and explicit exec-plugin login
// are available. Live NRP authentication acceptance remains open.
package nautilus

import "time"

// Scope must identify the configured cluster, not merely its current context
// alias. Namespace is explicit even if the user's kubeconfig has a default.
type Scope struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
}

type Ref struct {
	Scope
	Kind string `json:"kind"`
	UID  string `json:"uid"`
	Name string `json:"name"`
}

// Identity excludes the display name. Kubernetes UID distinguishes a deleted
// object from a new one with the same name. Provider is fixed by this package.
type Identity struct {
	Scope
	Kind, UID string
}

func (r Ref) Identity() Identity { return Identity{r.Scope, r.Kind, r.UID} }

type Page[T any] struct {
	Items           []T    `json:"items"`
	ResourceVersion string `json:"resource_version"`
	Continue        string `json:"continue,omitempty"`
	Remaining       *int64 `json:"remaining,omitempty"`
}

type Condition struct {
	Type               string     `json:"type"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
	LastTransitionTime *time.Time `json:"lastTransitionTime,omitempty"`
}

type Job struct {
	Ref
	Created    *time.Time  `json:"created_at"`
	Deleted    *time.Time  `json:"deleted_at"`
	Started    *time.Time  `json:"started_at"`
	Completed  *time.Time  `json:"completed_at"`
	Conditions []Condition `json:"conditions"`
	// Active/Succeeded/Failed count Pods, not distinct completed indexes or
	// container restarts. Counts and desired parallelism are separate. Nil means the API
	// omitted a value, rather than an invented zero or completion target.
	Active           *int64 `json:"active_pods"`
	Succeeded        *int64 `json:"succeeded_pods"`
	Failed           *int64 `json:"failed_pods"`
	Ready            *int64 `json:"ready_pods"`
	Terminating      *int64 `json:"terminating_pods"`
	Completions      *int64 `json:"completions"`
	Parallelism      *int64 `json:"parallelism"`
	CompletionMode   string `json:"completion_mode"`
	CompletedIndexes string `json:"completed_indexes"`
	FailedIndexes    string `json:"failed_indexes"`
	SuspendRequested bool   `json:"suspend_requested"`
}

func (j Job) State() string {
	for _, pair := range [][2]string{{"Complete", "Complete"}, {"Failed", "Failed"}, {"FailureTarget", "Failing"}, {"SuccessCriteriaMet", "Completing"}} {
		if conditionTrue(j.Conditions, pair[0]) {
			return pair[1]
		}
	}
	if j.Deleted != nil {
		return "Deleting"
	}
	if conditionTrue(j.Conditions, "Suspended") {
		return "Suspended"
	}
	if j.Active != nil && *j.Active > 0 {
		return "Running"
	}
	return "Pending"
}

func (j Job) Terminal() bool { return j.State() == "Complete" || j.State() == "Failed" }

func conditionTrue(conditions []Condition, name string) bool {
	for _, c := range conditions {
		if c.Type == name && c.Status == "True" {
			return true
		}
	}
	return false
}

type Owner struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
	Controller bool   `json:"controller"`
}

type Pod struct {
	Ref
	Created         *time.Time  `json:"created_at"`
	Deleted         *time.Time  `json:"deleted_at"`
	Started         *time.Time  `json:"started_at"`
	Phase           string      `json:"phase"`
	Reason          string      `json:"reason"`
	Message         string      `json:"message"`
	Node            string      `json:"node"`
	CompletionIndex string      `json:"completion_index"`
	Owners          []Owner     `json:"owners"`
	Conditions      []Condition `json:"conditions"`
	Containers      []Container `json:"containers"`
}

type Container struct {
	Name string `json:"name"`
	// Role is app, init, sidecar (restartable init), or ephemeral.
	Role     string         `json:"role"`
	Image    string         `json:"image"`
	Ready    *bool          `json:"ready"`
	Started  *bool          `json:"started"`
	Restarts *int64         `json:"restarts"`
	State    ContainerState `json:"current"`
	Last     ContainerState `json:"previous"`
}

type ContainerState struct {
	// Unknown, Waiting, Running, or Terminated. Last describes the previous
	// container instance only; it is not the Pod's or Job's outcome.
	Phase    string     `json:"phase"`
	Reason   string     `json:"reason"`
	Message  string     `json:"message"`
	ExitCode *int64     `json:"exit_code"`
	Signal   *int64     `json:"signal"`
	Started  *time.Time `json:"started_at"`
	Finished *time.Time `json:"finished_at"`
}

// PodsForJob uses controller identity, never the mutable job-name label. Pods
// whose owner disappeared remain inspectable in the namespace Pod list.
func PodsForJob(j Job, pods []Pod) []Pod {
	var result []Pod
	for _, p := range pods {
		if p.Scope != j.Scope {
			continue
		}
		for _, o := range p.Owners {
			if o.Controller && o.APIVersion == "batch/v1" && o.Kind == "Job" && o.UID == j.UID && o.Name == j.Name {
				result = append(result, p)
				break
			}
		}
	}
	return result
}
