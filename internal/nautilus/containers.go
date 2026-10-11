package nautilus

import (
	"errors"
	"perch/internal/domain"
)

func normalizeContainers(specs []containerSpec, statuses []containerStatus, role string, names map[string]bool) ([]Container, error) {
	byName := make(map[string]containerStatus)
	for _, status := range statuses {
		if !safeIdentity(status.Name) || !nonnegative(status.RestartCount) {
			return nil, errors.New("invalid container status")
		}
		if _, exists := byName[status.Name]; exists {
			return nil, errors.New("duplicate container status")
		}
		byName[status.Name] = status
	}
	var out []Container
	for _, spec := range specs {
		if !safeIdentity(spec.Name) || names[spec.Name] {
			return nil, errors.New("invalid or duplicate container name")
		}
		names[spec.Name] = true
		status := byName[spec.Name]
		delete(byName, spec.Name)
		state, err := normalizeState(status.State)
		if err != nil {
			return nil, err
		}
		last, err := normalizeState(status.LastState)
		if err != nil {
			return nil, err
		}
		containerRole := role
		if role == "init" && spec.RestartPolicy == "Always" {
			containerRole = "sidecar"
		}
		out = append(out, Container{Name: spec.Name, Role: containerRole, Image: domain.Label(spec.Image),
			Ready: status.Ready, Started: status.Started, Restarts: status.RestartCount, State: state, Last: last})
	}
	if len(byName) != 0 {
		return nil, errors.New("status has no matching container spec")
	}
	return out, nil
}

func normalizeState(raw rawContainerState) (ContainerState, error) {
	out := ContainerState{Phase: "Unknown"}
	count := 0
	if w := raw.Waiting; w != nil {
		count++
		out = ContainerState{Phase: "Waiting", Reason: domain.Label(w.Reason), Message: domain.Label(w.Message)}
	}
	if r := raw.Running; r != nil {
		count++
		out = ContainerState{Phase: "Running", Started: r.StartedAt}
	}
	if t := raw.Terminated; t != nil {
		count++
		if t.ExitCode == nil || !nonnegative(t.Signal) {
			return ContainerState{}, errors.New("invalid terminated container status")
		}
		out = ContainerState{Phase: "Terminated", Reason: domain.Label(t.Reason), Message: domain.Label(t.Message),
			ExitCode: t.ExitCode, Signal: t.Signal, Started: t.StartedAt, Finished: t.FinishedAt}
	}
	if count > 1 {
		return ContainerState{}, errors.New("conflicting container states")
	}
	return out, nil
}
