package ui

import (
	"fmt"
	"perch/internal/domain"
	"perch/internal/query"
	"strconv"
	"strings"
)

func nodeNumber(v *int64) string {
	if v == nil {
		return "?"
	}
	return strconv.FormatInt(*v, 10)
}

func (m *Model) refreshNodeBody() {
	rows := m.nodeRows
	if m.nodesGPUOnly {
		rows = nil
		for _, n := range m.nodeRows {
			// Keep unparsed GPU descriptions visible for inspection too.
			if len(n.GPUs) > 0 || n.GPUNote != "" || strings.Contains(n.GRES, "gpu") {
				rows = append(rows, n)
			}
		}
	}
	m.softwareBody = nodesContent(rows)
	if m.nodesGPUOnly {
		m.softwareBody = muted.Render(fmt.Sprintf("GPU nodes · %d of %d", len(rows), len(m.nodeRows))) + "\n\n" + m.softwareBody
	}
	m.softwareBody += "\n\n" + muted.Render("Observed: "+m.reportAt.Local().Format("Jan 2 15:04:05")+" · on demand")
}
func nodeBytes(v *int64) string {
	if v == nil {
		return "?"
	}
	if *v < 0 {
		return "−" + domain.Bytes(-*v)
	}
	return domain.Bytes(*v)
}
func nodesContent(rows []query.Node) string {
	lines := []string{}
	if len(rows) == 0 {
		lines = append(lines, "No node records reported.", "")
	}
	for _, n := range rows {
		parts := strings.Join(n.Partitions, ", ")
		if parts == "" {
			parts = "partition not reported"
		}
		state := strings.ToLower(strings.Join(n.States, " · "))
		if state == "" {
			state = "state not reported"
		}
		lines = append(lines, accent.Bold(true).Render(n.Name)+muted.Render(" · "+parts+" · "+state))
		for _, gpu := range n.GPUs {
			kind := gpu.Type
			if kind == "" {
				kind = "untyped"
			}
			lines = append(lines, fmt.Sprintf("GPU  %s · %s / %d allocated", kind, nodeNumber(gpu.Allocated), gpu.Configured), "     "+nodeNumber(gpu.Unallocated)+" unallocated")
		}
		if n.GPUNote != "" {
			lines = append(lines, yellow.Render(n.GPUNote))
		}
		lines = append(lines, fmt.Sprintf("CPU  %s / %s allocated · %s idle", nodeNumber(n.AllocatedCPUs), nodeNumber(n.CPUs), nodeNumber(n.IdleCPUs)), fmt.Sprintf("RAM  %s / %s allocated", nodeBytes(n.AllocatedMemory), nodeBytes(n.Memory)))
		remaining := nodeBytes(n.UnallocatedMemory) + " unallocated"
		if n.UnallocatedMemory == nil {
			remaining = "Unallocated memory not reported"
		}
		if n.UnallocatedMemory != nil && *n.UnallocatedMemory <= 0 {
			remaining = yellow.Render(remaining)
		} else {
			remaining = ink.Render(remaining)
		}
		lines = append(lines, "     "+remaining)
		if n.ReservedMemory != nil && *n.ReservedMemory > 0 {
			lines = append(lines, muted.Render("     "+nodeBytes(n.ReservedMemory)+" system-reserved"))
		}
		if n.GRES != "" {
			used := n.GRESUsed
			if used == "" {
				used = "not reported"
			}
			lines = append(lines, "GRES "+n.GRES, "Used "+used)
		}
		lines = append(lines, "")
	}
	lines = append(lines, ink.Bold(true).Render("Reading capacity"), "Unallocated memory subtracts job allocations and system-reserved memory from configured memory. It is not OS free memory.", "Idle CPUs and unallocated memory do not guarantee a start: node flags, reservations, priority and job requirements still apply.", "GRES describes assigned resources, not GPU utilization. ? means unavailable.")
	return strings.Join(lines, "\n")
}
