package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	"perch/internal/hummingbird"
)

type Software = hummingbird.Software
type ModuleInfo = hummingbird.ModuleInfo

type softwareBackend interface {
	Software(context.Context) ([]Software, error)
	Module(context.Context, string) (ModuleInfo, error)
}

func (s Service) software(ctx context.Context, req Request) (Response, error) {
	out := Response{SchemaVersion: 1, Source: "hummingbird", At: time.Now()}
	var modules []Software
	var err error
	backend, ok := s.Backend.(softwareBackend)
	if !s.Demo && !ok {
		return out, fmt.Errorf("software discovery unavailable on this backend")
	}
	if s.Demo {
		out.Source = "demo"
	}
	if req.Command == "module" {
		if s.Demo {
			if req.ID != "cuda/12.8.1" && req.ID != "openmpi5/5.0.10" {
				return out, fmt.Errorf("demo details available for cuda/12.8.1 and openmpi5/5.0.10")
			}
			details := "Synthetic demo module. Verify requirements on your cluster."
			if strings.HasPrefix(req.ID, "openmpi5/") {
				details += "\nLoad the compatible compiler family before loading this MPI module."
			}
			out.Data = ModuleInfo{Name: req.ID, Details: details, LoadCommand: "module load " + req.ID}
		} else {
			out.Data, err = backend.Module(ctx, req.ID)
		}
		out.At = time.Now()
		return out, err
	}
	if s.Demo {
		modules = []Software{{Name: "cuda/12.8.1", Available: true}, {Name: "gnu14/14.2.0", Available: true}, {Name: "miniconda3/3.13", Available: true}, {Name: "openmpi5/5.0.10"}, {Name: "singularity-ce/singularity-ce.4.1.4", Available: true}}
	} else {
		modules, err = backend.Software(ctx)
	}
	if err != nil {
		return out, err
	}
	filtered := make([]Software, 0)
	for _, m := range modules {
		if strings.Contains(strings.ToLower(m.Name), strings.ToLower(req.ID)) {
			filtered = append(filtered, m)
		}
	}
	out.At = time.Now()
	out.Data = filtered
	return out, nil
}
