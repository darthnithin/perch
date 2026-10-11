package hummingbird

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"perch/internal/domain"
)

// Node describes scheduler allocations, not OS use or immediate availability.
// UnallocatedMemory excludes specialized memory and can be negative when the
// controller reports allocations above configured capacity. Missing stays null.
type Node struct {
	Name              string        `json:"name"`
	Partitions        []string      `json:"partitions"`
	States            []string      `json:"states"`
	CPUs              *int64        `json:"cpus"`
	AllocatedCPUs     *int64        `json:"allocated_cpus"`
	IdleCPUs          *int64        `json:"idle_cpus"`
	Memory            *int64        `json:"configured_memory_bytes"`
	AllocatedMemory   *int64        `json:"allocated_memory_bytes"`
	ReservedMemory    *int64        `json:"system_reserved_memory_bytes"`
	UnallocatedMemory *int64        `json:"unallocated_memory_bytes"`
	GRES              string        `json:"gres"`
	GRESUsed          string        `json:"gres_used"`
	GPUs              []GPUResource `json:"gpus,omitempty"`
	GPUNote           string        `json:"gpu_note,omitempty"`
}

func (c *Client) Nodes(ctx context.Context) ([]Node, error) {
	r, err := c.Runner.Run(ctx, []string{"env", "-u", "SLURM_CLUSTERS", "-u", "SCONTROL_FEDERATION", "scontrol", "--local", "--json=" + Parser, "show", "nodes"})
	if err != nil {
		return nil, err
	}
	if err = commandError(r); err != nil {
		return nil, err
	}
	return ParseNodes(r.Stdout)
}

func ParseNodes(raw string) ([]Node, error) {
	if len(raw) > 8<<20 {
		return nil, fmt.Errorf("node response exceeds 8 MiB")
	}
	if _, err := check(raw, "nodes"); err != nil {
		return nil, err
	}
	var response struct {
		Nodes []struct {
			Name            string   `json:"name"`
			Partitions      []string `json:"partitions"`
			State           []string `json:"state"`
			CPUs            *int64   `json:"cpus"`
			AllocatedCPUs   *int64   `json:"alloc_cpus"`
			IdleCPUs        *int64   `json:"alloc_idle_cpus"`
			Memory          *int64   `json:"real_memory"`
			AllocatedMemory *int64   `json:"alloc_memory"`
			ReservedMemory  *int64   `json:"specialized_memory"`
			GRES            string   `json:"gres"`
			GRESUsed        string   `json:"gres_used"`
		}
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return nil, fmt.Errorf("invalid node response: %w", err)
	}
	if len(response.Nodes) > 10000 {
		return nil, fmt.Errorf("node response exceeds 10000 records")
	}
	rows := make([]Node, 0, len(response.Nodes))
	seen := map[string]bool{}
	for _, n := range response.Nodes {
		if !priorityLabel(n.Name) || seen[n.Name] {
			return nil, fmt.Errorf("invalid or duplicate node identity")
		}
		seen[n.Name] = true
		labels := [][]string{n.Partitions, n.State}
		for _, set := range labels {
			if len(set) > 128 {
				return nil, fmt.Errorf("too many node labels")
			}
			for _, label := range set {
				if !priorityLabel(label) {
					return nil, fmt.Errorf("invalid node label")
				}
			}
		}
		for _, v := range []*int64{n.CPUs, n.AllocatedCPUs, n.IdleCPUs, n.Memory, n.AllocatedMemory, n.ReservedMemory} {
			if v != nil && (*v < 0 || *v > math.MaxInt64/(1<<20)) {
				return nil, fmt.Errorf("invalid node resource value")
			}
		}
		if len(n.GRES) > 4096 || len(n.GRESUsed) > 4096 {
			return nil, fmt.Errorf("node GRES description too long")
		}
		row := Node{Name: n.Name, Partitions: append([]string{}, n.Partitions...), States: append([]string{}, n.State...), CPUs: n.CPUs, AllocatedCPUs: n.AllocatedCPUs, IdleCPUs: n.IdleCPUs, Memory: nodeMiB(n.Memory), AllocatedMemory: nodeMiB(n.AllocatedMemory), ReservedMemory: nodeMiB(n.ReservedMemory), GRES: domain.Label(n.GRES), GRESUsed: domain.Label(n.GRESUsed)}
		row.GPUs, row.GPUNote = nodeGPUs(n.GRES, n.GRESUsed)
		if row.Memory != nil && row.AllocatedMemory != nil && row.ReservedMemory != nil {
			// Subtract in MiB first to avoid overflowing when two valid byte values add.
			left := *n.Memory - *n.AllocatedMemory - *n.ReservedMemory
			if left > math.MinInt64/(1<<20) {
				v := left * (1 << 20)
				row.UnallocatedMemory = &v
			}
		}
		sort.Strings(row.Partitions)
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.Join(rows[i].Partitions, ",")+"/"+rows[i].Name < strings.Join(rows[j].Partitions, ",")+"/"+rows[j].Name
	})
	return rows, nil
}
func nodeMiB(v *int64) *int64 {
	if v == nil {
		return nil
	}
	n := *v * (1 << 20)
	return &n
}
