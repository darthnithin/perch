package ui

import (
	"fmt"
	"perch/internal/query"
	"strconv"
	"strings"
)

func priorityContent(rows []query.Priority) string {
	lines := []string{muted.Render("Scores · not queue positions"), ""}
	if len(rows) == 0 {
		lines = append(lines, "No priority records reported for your account.", "Held or newly submitted jobs may not have factors yet.", "")
	}
	for _, r := range rows {
		lines = append(lines, accent.Bold(true).Render("#"+r.JobID)+muted.Render(" · "+r.Partition), ink.Bold(true).Render(fmt.Sprintf("%d", r.Score))+muted.Render(" priority"))
		zero := []string{}
		for _, f := range r.Factors {
			if f.Value == 0 {
				zero = append(zero, f.Name)
				continue
			}
			lines = append(lines, fmt.Sprintf("  %-14s %s", f.Name, strconv.FormatFloat(f.Value, 'f', -1, 64)))
		}
		if r.Nice != 0 {
			lines = append(lines, fmt.Sprintf("  Nice adjustment  %d", r.Nice))
		}
		if len(zero) > 0 {
			lines = append(lines, muted.Render("Zero: "+strings.Join(zero, ", ")))
		}
		lines = append(lines, "")
	}
	lines = append(lines, ink.Bold(true).Render("Reading the scores"), "Higher priority helps selection, but resources, holds, dependencies and reservations still matter.", "A zero contribution does not prove a factor is disabled. Numbers are weighted; nice is a separate adjustment. Displayed factors can round; the total is reported by Slurm.", "Allocation IDs can represent array records. Each partition has its own score.")
	return strings.Join(lines, "\n")
}
