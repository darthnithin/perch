package domain

import (
	"fmt"
	"sort"
	"time"
)

// Individual task records intentionally refine aggregate ranges. Two terminal
// ranges of the same scoped attempt have no such ordering: their intersection
// cannot be assigned a state merely by sorting their displayed IDs.
func validateArrayRangeOutcomes(jobs []Job) error {
	type key struct {
		cluster, user string
		submit        time.Time
		restarts      int
	}
	type observation struct {
		span  taskProgression
		state string
	}
	groups := map[key][]observation{}
	states := map[key]map[string]bool{}
	seen := map[key]map[observation]bool{}
	for _, j := range jobs {
		if j.QueueObserved || !j.Terminal() || j.TaskRange == "" || j.TaskID != nil {
			continue
		}
		k := key{j.Cluster, j.User, j.Submit, j.Restarts}
		spans, err := parseTaskProgressions(j.TaskRange)
		if err != nil {
			return err
		}
		if states[k] == nil {
			states[k] = map[string]bool{}
			seen[k] = map[observation]bool{}
		}
		states[k][j.State] = true
		for _, span := range spans {
			o := observation{span, j.State}
			if seen[k][o] {
				continue
			}
			if len(groups[k]) >= maxRangeTerms {
				return fmt.Errorf("array outcome comparison exceeds fragment limit")
			}
			seen[k][o] = true
			groups[k] = append(groups[k], o)
		}
	}
	work := maxRangeIntersections
	for k, spans := range groups {
		if len(states[k]) < 2 {
			continue
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i].span.first < spans[j].span.first })
		var active []observation
		for _, current := range spans {
			retained := active[:0]
			for _, old := range active {
				work--
				if work < 0 {
					return fmt.Errorf("array outcome comparison exceeds overlap work limit")
				}
				if old.span.last < current.span.first {
					continue
				}
				retained = append(retained, old)
				if old.state == current.state {
					continue
				}
				if _, overlap := intersectProgressions(old.span, current.span); overlap {
					return fmt.Errorf("conflicting accounting outcomes in overlapping array ranges")
				}
			}
			active = append(retained, current)
		}
	}
	return nil
}
