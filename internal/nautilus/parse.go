package nautilus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"perch/internal/domain"
)

const MaxPageBytes = 8 << 20
const MaxPageItems = 10000

type metadata struct {
	Name, Namespace, UID, ResourceVersion string
	CreationTimestamp, DeletionTimestamp  *time.Time
	OwnerReferences                       []Owner
	Annotations                           map[string]string
}

type rawJob struct {
	APIVersion, Kind string
	Metadata         metadata
	Spec             struct {
		Completions, Parallelism *int64
		CompletionMode           string
		Suspend                  bool
	}
	Status struct {
		Active, Succeeded, Failed, Ready, Terminating *int64
		StartTime, CompletionTime                     *time.Time
		Conditions                                    []Condition
		CompletedIndexes, FailedIndexes               string
	}
}

type rawPod struct {
	APIVersion, Kind string
	Metadata         metadata
	Spec             struct {
		NodeName                                        string
		Containers, InitContainers, EphemeralContainers []containerSpec
	}
	Status struct {
		Phase, Reason, Message                                               string
		StartTime                                                            *time.Time
		Conditions                                                           []Condition
		ContainerStatuses, InitContainerStatuses, EphemeralContainerStatuses []containerStatus
	}
}

type containerSpec struct{ Name, Image, RestartPolicy string }
type containerStatus struct {
	Name             string
	Ready, Started   *bool
	RestartCount     *int64
	State, LastState rawContainerState
}
type rawContainerState struct {
	Waiting    *struct{ Reason, Message string }
	Running    *struct{ StartedAt *time.Time }
	Terminated *struct {
		Reason, Message       string
		ExitCode, Signal      *int64
		StartedAt, FinishedAt *time.Time
	}
}

// ParseJobs parses one API page. Callers must follow Continue before treating
// this as the complete list. An error never represents a successful empty list.
func ParseJobs(data []byte, scope Scope) (Page[Job], error) {
	page, err := decodePage[rawJob](data, "JobList", "batch/v1", scope)
	if err != nil {
		return Page[Job]{}, err
	}
	out := Page[Job]{Items: make([]Job, 0, len(page.Items)), ResourceVersion: page.ResourceVersion, Continue: page.Continue, Remaining: page.Remaining}
	seen := make(map[Identity]bool)
	for i, raw := range page.Items {
		ref, err := objectRef(raw.Metadata, raw.Kind, raw.APIVersion, "Job", "batch/v1", scope)
		if err != nil || seen[ref.Identity()] {
			return Page[Job]{}, fmt.Errorf("invalid Job identity at item %d", i)
		}
		seen[ref.Identity()] = true
		s := raw.Status
		if !nonnegative(s.Active, s.Succeeded, s.Failed, s.Ready, s.Terminating, raw.Spec.Completions, raw.Spec.Parallelism) || cleanConditions(s.Conditions) != nil {
			return Page[Job]{}, fmt.Errorf("invalid Job status at item %d", i)
		}
		success := conditionTrue(s.Conditions, "Complete") || conditionTrue(s.Conditions, "SuccessCriteriaMet")
		failure := conditionTrue(s.Conditions, "Failed") || conditionTrue(s.Conditions, "FailureTarget")
		if success && failure {
			return Page[Job]{}, fmt.Errorf("conflicting Job outcomes at item %d", i)
		}
		j := Job{Ref: ref, Created: raw.Metadata.CreationTimestamp, Deleted: raw.Metadata.DeletionTimestamp,
			Started: s.StartTime, Completed: s.CompletionTime, Conditions: s.Conditions,
			Active: s.Active, Succeeded: s.Succeeded, Failed: s.Failed, Ready: s.Ready, Terminating: s.Terminating,
			Completions: raw.Spec.Completions, Parallelism: raw.Spec.Parallelism, CompletionMode: domain.Label(raw.Spec.CompletionMode),
			CompletedIndexes: s.CompletedIndexes, FailedIndexes: s.FailedIndexes, SuspendRequested: raw.Spec.Suspend}
		// Retain compact index ranges without expanding large indexed workloads.
		if !validIndexes(j.CompletedIndexes, j.Completions) || !validIndexes(j.FailedIndexes, j.Completions) {
			return Page[Job]{}, fmt.Errorf("invalid Job index ranges at item %d", i)
		}
		out.Items = append(out.Items, j)
	}
	return out, nil
}

func ParsePods(data []byte, scope Scope) (Page[Pod], error) {
	page, err := decodePage[rawPod](data, "PodList", "v1", scope)
	if err != nil {
		return Page[Pod]{}, err
	}
	out := Page[Pod]{Items: make([]Pod, 0, len(page.Items)), ResourceVersion: page.ResourceVersion, Continue: page.Continue, Remaining: page.Remaining}
	seen := make(map[Identity]bool)
	for i, raw := range page.Items {
		ref, err := objectRef(raw.Metadata, raw.Kind, raw.APIVersion, "Pod", "v1", scope)
		if err != nil || seen[ref.Identity()] {
			return Page[Pod]{}, fmt.Errorf("invalid Pod identity at item %d", i)
		}
		seen[ref.Identity()] = true
		if cleanConditions(raw.Status.Conditions) != nil || !validOwners(raw.Metadata.OwnerReferences) {
			return Page[Pod]{}, fmt.Errorf("invalid Pod status or owners at item %d", i)
		}
		p := Pod{Ref: ref, Created: raw.Metadata.CreationTimestamp, Deleted: raw.Metadata.DeletionTimestamp,
			Started: raw.Status.StartTime, Phase: domain.Label(raw.Status.Phase), Reason: domain.Label(raw.Status.Reason), Message: domain.Label(raw.Status.Message),
			Node: domain.Label(raw.Spec.NodeName), CompletionIndex: raw.Metadata.Annotations["batch.kubernetes.io/job-completion-index"],
			Owners: raw.Metadata.OwnerReferences, Conditions: raw.Status.Conditions}
		if p.Phase == "" {
			p.Phase = "Unknown"
		}
		if p.CompletionIndex != "" && !validIndex(p.CompletionIndex) {
			return Page[Pod]{}, fmt.Errorf("invalid Pod completion index at item %d", i)
		}
		names := make(map[string]bool)
		for _, group := range []struct {
			specs    []containerSpec
			statuses []containerStatus
			role     string
		}{
			{raw.Spec.InitContainers, raw.Status.InitContainerStatuses, "init"},
			{raw.Spec.Containers, raw.Status.ContainerStatuses, "app"},
			{raw.Spec.EphemeralContainers, raw.Status.EphemeralContainerStatuses, "ephemeral"},
		} {
			containers, err := normalizeContainers(group.specs, group.statuses, group.role, names)
			if err != nil {
				return Page[Pod]{}, fmt.Errorf("invalid Pod containers at item %d: %w", i, err)
			}
			p.Containers = append(p.Containers, containers...)
		}
		if len(raw.Spec.Containers) == 0 {
			return Page[Pod]{}, fmt.Errorf("missing Pod app containers at item %d", i)
		}
		out.Items = append(out.Items, p)
	}
	return out, nil
}

func decodePage[T any](data []byte, kind, version string, scope Scope) (Page[T], error) {
	if !safeIdentity(scope.Cluster) || !safeIdentity(scope.Namespace) {
		return Page[T]{}, errors.New("cluster and namespace scope are required")
	}
	if len(data) > MaxPageBytes {
		return Page[T]{}, errors.New("Kubernetes page exceeds byte limit")
	}
	if err := validateJSON(data); err != nil {
		return Page[T]{}, err
	}
	var raw struct {
		APIVersion, Kind string
		Metadata         struct {
			ResourceVersion, Continue string
			RemainingItemCount        *int64
		}
		Items *[]T
	}
	if json.Unmarshal(data, &raw) != nil {
		return Page[T]{}, errors.New("invalid Kubernetes list fields")
	}
	if raw.Kind != kind || raw.APIVersion != version || raw.Items == nil || raw.Metadata.ResourceVersion == "" || !nonnegative(raw.Metadata.RemainingItemCount) {
		return Page[T]{}, errors.New("unexpected Kubernetes list envelope")
	}
	if len(*raw.Items) > MaxPageItems {
		return Page[T]{}, errors.New("Kubernetes page exceeds item limit")
	}
	return Page[T]{*raw.Items, raw.Metadata.ResourceVersion, raw.Metadata.Continue, raw.Metadata.RemainingItemCount}, nil
}

func objectRef(m metadata, kind, version, expectedKind, expectedVersion string, scope Scope) (Ref, error) {
	// API list items commonly omit their own TypeMeta; the list supplies it.
	if kind != "" && kind != expectedKind || version != "" && version != expectedVersion || m.Namespace != scope.Namespace || !safeIdentity(m.UID) || !safeIdentity(m.Name) {
		return Ref{}, errors.New("invalid object identity")
	}
	return Ref{scope, expectedKind, m.UID, m.Name}, nil
}

func safeIdentity(s string) bool {
	return len(s) > 0 && len(s) <= 512 && !strings.ContainsAny(s, " \t\r\n") && strings.IndexFunc(s, unicode.IsControl) < 0 && domain.Label(s) == s
}

func nonnegative(values ...*int64) bool {
	for _, v := range values {
		if v != nil && *v < 0 {
			return false
		}
	}
	return true
}

func cleanConditions(conditions []Condition) error {
	seen := make(map[string]bool)
	for i := range conditions {
		c := &conditions[i]
		if !safeIdentity(c.Type) || seen[c.Type] || c.Status != "True" && c.Status != "False" && c.Status != "Unknown" {
			return errors.New("invalid condition")
		}
		seen[c.Type] = true
		c.Reason, c.Message = domain.Label(c.Reason), domain.Label(c.Message)
	}
	return nil
}

func validOwners(owners []Owner) bool {
	controllers := 0
	seen := make(map[string]bool)
	for _, o := range owners {
		if !safeIdentity(o.UID) || !safeIdentity(o.Name) || !safeIdentity(o.Kind) || !safeIdentity(o.APIVersion) || seen[o.UID] {
			return false
		}
		seen[o.UID] = true
		if o.Controller {
			controllers++
		}
	}
	return controllers <= 1
}

// Reject duplicate keys and excessively deep data before typed decoding, but
// accept unknown fields for forward compatibility. Error text never echoes
// server content (which can contain environment variables or credentials).
func validateJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return errors.New("JSON nesting limit")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := make(map[string]bool)
				for d.More() {
					key, err := d.Token()
					if err != nil {
						return err
					}
					k, ok := key.(string)
					if !ok || seen[k] {
						return errors.New("duplicate JSON field")
					}
					seen[k] = true
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			default:
				return errors.New("unexpected delimiter")
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if value(0) != nil {
		return errors.New("invalid or ambiguous Kubernetes JSON")
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing Kubernetes JSON data")
	}
	return nil
}
