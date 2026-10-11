package hummingbird

import (
	"context"
	"fmt"
	"math"
	"perch/internal/transport"
	"sort"
	"strconv"
	"strings"
)

// Priority is a reported score for one allocation and partition, not a queue
// position. Values are already weighted, except Nice which is an adjustment.
// TRES stays labelled because sites can configure several resource factors.
type Priority struct {
	JobID     string           `json:"allocation_id"`
	Partition string           `json:"partition"`
	Score     uint64           `json:"score"`
	Factors   []PriorityFactor `json:"weighted_factors"`
	Nice      int64            `json:"nice_adjustment"`
}
type PriorityFactor struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// PriorityPolicy describes controller configuration observed with the report.
// A zero job factor alone is not evidence that a configured weight is zero.
type PriorityPolicy struct {
	FairshareWeight *uint64 `json:"fairshare_weight"`
	Note            string  `json:"note"`
}

func parsePriorityPolicy(r transport.Result) PriorityPolicy {
	unknown := PriorityPolicy{Note: "Fair-share priority weight unavailable; a zero job contribution does not establish that fair-share is disabled."}
	if r.ExitCode != 0 {
		return unknown
	}
	config := parseConfig(r.Stdout)
	if config["PriorityType"] != "priority/multifactor" {
		return unknown
	}
	weight, err := strconv.ParseUint(config["PriorityWeightFairShare"], 10, 32)
	if err != nil {
		return unknown
	}
	note := "Fair-share is one weighted priority factor; its value is not a queue position or start-time prediction."
	if weight == 0 {
		note = "Fair-share does not contribute to job priority on this cluster (configured weight 0)."
	}
	return PriorityPolicy{FairshareWeight: &weight, Note: note}
}

const priorityFormat = "%i|%u|%r|%Y|%S|%A|%B|%F|%J|%P|%Q|%N|%T"

func (c *Client) priorityArgs() ([]string, error) {
	if c.User == "" || strings.ContainsAny(c.User, " ,|\t\r\n\x00") {
		return nil, fmt.Errorf("identify the remote user before reading priority")
	}
	return []string{"env", "-u", "SPRIO_SIBLING", "-u", "SPRIO_FEDERATION", "-u", "SLURM_CLUSTERS", "sprio", "--local", "--noheader", "--user=" + c.User, "--format=" + priorityFormat}, nil
}

// PriorityWithPolicy reads both parts in one SSH batch, only on explicit report
// requests. Failure to read optional configuration does not erase valid scores.
func (c *Client) PriorityWithPolicy(ctx context.Context) ([]Priority, PriorityPolicy, error) {
	args, err := c.priorityArgs()
	if err != nil {
		return nil, PriorityPolicy{}, err
	}
	results, err := c.Runner.Batch(ctx, [][]string{args, {"scontrol", "show", "config"}})
	if err != nil {
		return nil, PriorityPolicy{}, err
	}
	if len(results) != 2 {
		return nil, PriorityPolicy{}, fmt.Errorf("incomplete priority response")
	}
	if err = commandError(results[0]); err != nil {
		return nil, PriorityPolicy{}, err
	}
	rows, err := parsePriority(results[0].Stdout, c.User)
	return rows, parsePriorityPolicy(results[1]), err
}

func (c *Client) Priority(ctx context.Context) ([]Priority, error) {
	args, err := c.priorityArgs()
	if err != nil {
		return nil, err
	}
	// Explicit local scope and format override user defaults. No recurring poll;
	// sprio contacts slurmctld and is only used by a requested report.
	r, err := c.Runner.Run(ctx, args)
	if err != nil {
		return nil, err
	}
	if err = commandError(r); err != nil {
		return nil, err
	}
	return parsePriority(r.Stdout, c.User)
}

func parsePriority(text, user string) ([]Priority, error) {
	invalid := func() ([]Priority, error) { return nil, fmt.Errorf("unrecognized priority response") }
	if len(text) > 2<<20 {
		return invalid()
	}
	out := []Priority{}
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) != 13 || len(out) >= 10000 {
			return invalid()
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		id, e := strconv.ParseUint(fields[0], 10, 63)
		if e != nil || id == 0 || strconv.FormatUint(id, 10) != fields[0] || fields[1] != user || !priorityLabel(fields[2]) {
			return invalid()
		}
		score, e := strconv.ParseUint(fields[3], 10, 32)
		if e != nil {
			return invalid()
		}
		nice, e := strconv.ParseInt(fields[11], 10, 64)
		if e != nil {
			return invalid()
		}
		row := Priority{JobID: fields[0], Partition: fields[2], Score: score, Nice: nice, Factors: []PriorityFactor{}}
		for i, name := range []string{"Admin", "Age", "Association", "Fairshare", "Job size", "Partition", "QOS"} {
			n, e := priorityNumber(fields[4+i])
			if e != nil {
				return invalid()
			}
			row.Factors = append(row.Factors, PriorityFactor{Name: name, Value: n})
		}
		if fields[12] != "" && fields[12] != "(null)" {
			tresSeen := map[string]bool{}
			for _, factor := range strings.Split(fields[12], ",") {
				name, value, ok := strings.Cut(factor, "=")
				if !ok || !priorityLabel(name) || tresSeen[name] {
					return invalid()
				}
				tresSeen[name] = true
				n, e := priorityNumber(value)
				if e != nil {
					return invalid()
				}
				row.Factors = append(row.Factors, PriorityFactor{Name: "TRES " + name, Value: n})
			}
		}
		key := row.JobID + "/" + row.Partition
		if seen[key] {
			return invalid()
		}
		seen[key] = true
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Partition != out[j].Partition {
			return out[i].Partition < out[j].Partition
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].JobID < out[j].JobID
	})
	return out, nil
}
func priorityLabel(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_./:-", c)) {
			return false
		}
	}
	return true
}
func priorityNumber(s string) (float64, error) {
	n, e := strconv.ParseFloat(s, 64)
	if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
		return 0, fmt.Errorf("invalid weighted factor")
	}
	return n, nil
}
