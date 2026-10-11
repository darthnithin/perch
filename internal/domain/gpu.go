package domain

import "fmt"

// GPURequest describes one scheduler constraint, never an allocation or a
// physical-card count. Constraints with different scopes/types may overlap.
type GPURequest struct {
	Scope string `json:"scope"`
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

func (g GPURequest) Label() string {
	kind := "GPU"
	if g.Count != 1 {
		kind = "GPUs"
	}
	if g.Type != "" {
		kind = Label(g.Type) + " " + kind
	}
	return fmt.Sprintf("%d %s per %s", g.Count, kind, Label(g.Scope))
}

func CopyGPURequests(source []GPURequest) []GPURequest {
	if source == nil {
		return nil
	}
	out := append([]GPURequest{}, source...)
	for i := range out {
		out[i].Scope, out[i].Type = Label(out[i].Scope), Label(out[i].Type)
	}
	return out
}
