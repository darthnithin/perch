package domain

import (
	"fmt"
	"sort"
	"strconv"
)

// ArraySubmissionConflicts checks membership before consumers filter older
// families or choose a winning range. Missing time is not the oldest attempt.
// Disjoint task sets need no relative ordering and remain usable.
func ArraySubmissionConflicts(jobs []Job) map[int64]string {
	type scope struct {
		id            int64
		cluster, user string
	}
	groups := map[scope][]Job{}
	for _, j := range jobs {
		if j.ArrayID > 0 && (j.TaskID != nil || j.TaskRange != "") {
			k := scope{j.ArrayID, j.Cluster, j.User}
			groups[k] = append(groups[k], j)
		}
	}
	out := map[int64]string{}
	for k, group := range groups {
		if err := mixedSubmissionOverlap(group); err != nil {
			out[k.id] = err.Error()
		}
	}
	return out
}

func mixedSubmissionOverlap(jobs []Job) error {
	known, missing := false, false
	for _, j := range jobs {
		known = known || !j.Submit.IsZero()
		missing = missing || j.Submit.IsZero()
	}
	if !known || !missing {
		return nil
	}
	type observation struct {
		span    taskProgression
		missing bool
	}
	var spans []observation
	seen := map[observation]bool{}
	for _, j := range jobs {
		expr := j.TaskRange
		if j.TaskID != nil {
			expr = strconv.FormatInt(*j.TaskID, 10)
		}
		parts, err := parseTaskProgressions(expr)
		if err != nil {
			return err
		}
		for _, span := range parts {
			o := observation{span, j.Submit.IsZero()}
			if seen[o] {
				continue
			}
			if len(spans) >= maxRangeTerms {
				return fmt.Errorf("array identity comparison exceeds fragment limit")
			}
			seen[o] = true
			spans = append(spans, o)
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].span.first < spans[j].span.first })
	work := maxRangeIntersections
	var active []observation
	for _, current := range spans {
		retained := active[:0]
		for _, old := range active {
			work--
			if work < 0 {
				return fmt.Errorf("array identity comparison exceeds overlap work limit")
			}
			if old.span.last < current.span.first {
				continue
			}
			retained = append(retained, old)
			if old.missing == current.missing {
				continue
			}
			if _, overlap := intersectProgressions(old.span, current.span); overlap {
				return fmt.Errorf("ambiguous array identity: overlapping members mix known and missing submission times")
			}
		}
		active = append(retained, current)
	}
	return nil
}
