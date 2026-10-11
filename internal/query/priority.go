package query

import (
	"context"
	"fmt"
	"perch/internal/hummingbird"
	"time"
)

type Priority = hummingbird.Priority
type PriorityPolicy = hummingbird.PriorityPolicy

type priorityBackend interface {
	Priority(context.Context) ([]Priority, error)
}

type priorityPolicyBackend interface {
	PriorityWithPolicy(context.Context) ([]Priority, hummingbird.PriorityPolicy, error)
}

func (s Service) priority(ctx context.Context) (Response, error) {
	out := Response{SchemaVersion: 1, Source: "hummingbird"}
	var rows []Priority
	if s.Demo {
		out.Source = "demo"
		weight := uint64(0)
		out.PriorityPolicy = &hummingbird.PriorityPolicy{FairshareWeight: &weight, Note: "Fair-share does not contribute to job priority on this cluster (configured weight 0)."}
		rows = []Priority{
			{JobID: "9231", Partition: "128x24", Score: 10010148, Nice: 0, Factors: []hummingbird.PriorityFactor{{Name: "Age", Value: 103}, {Name: "Job size", Value: 45}, {Name: "Partition", Value: 10000}, {Name: "QOS", Value: 10000000}, {Name: "Fairshare", Value: 0}, {Name: "Association", Value: 0}, {Name: "Admin", Value: 0}}},
			{JobID: "9100", Partition: "128x24", Score: 10010051, Nice: 0, Factors: []hummingbird.PriorityFactor{{Name: "Age", Value: 6}, {Name: "Job size", Value: 45}, {Name: "Partition", Value: 10000}, {Name: "QOS", Value: 10000000}, {Name: "Fairshare", Value: 0}, {Name: "Association", Value: 0}, {Name: "Admin", Value: 0}}},
		}
	} else {
		backend, ok := s.Backend.(priorityBackend)
		if !ok {
			return out, fmt.Errorf("priority factors unavailable on this backend")
		}
		var err error
		if enhanced, ok := s.Backend.(priorityPolicyBackend); ok {
			var policy hummingbird.PriorityPolicy
			rows, policy, err = enhanced.PriorityWithPolicy(ctx)
			out.PriorityPolicy = &policy
		} else {
			rows, err = backend.Priority(ctx)
		}
		if err != nil {
			return out, err
		}
	}
	out.At = time.Now()
	out.Data = rows
	return out, nil
}
