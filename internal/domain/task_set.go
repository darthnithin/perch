package domain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// taskSet stores disjoint progressions. Operations are bounded by fragmentation
// and work, rather than task count: a billion consecutive tasks stays one span.
type taskSet struct {
	spans []taskProgression
	work  int
}

func newTaskSet() *taskSet { return &taskSet{work: maxRangeIntersections} }

func (s *taskSet) add(spans []taskProgression) ([]taskProgression, error) {
	var added []taskProgression
	for _, span := range spans {
		pieces := []taskProgression{span}
		for _, old := range s.spans {
			if old.last < span.first || old.first > span.last {
				continue
			}
			var next []taskProgression
			for _, piece := range pieces {
				s.work--
				if s.work < 0 {
					return nil, fmt.Errorf("array selection exceeds overlap work limit")
				}
				parts, err := subtractProgression(piece, old)
				if err != nil {
					return nil, err
				}
				next = append(next, parts...)
				if len(next) > maxRangeTerms {
					return nil, fmt.Errorf("array selection exceeds fragment limit")
				}
			}
			pieces = next
			if len(pieces) == 0 {
				break
			}
		}
		if len(s.spans)+len(pieces) > maxRangeTerms {
			return nil, fmt.Errorf("array selection exceeds fragment limit")
		}
		s.spans = append(s.spans, pieces...)
		added = append(added, pieces...)
	}
	return added, nil
}

func subtractProgression(a, b taskProgression) ([]taskProgression, error) {
	i, ok := intersectProgressions(a, b)
	if !ok {
		return []taskProgression{a}, nil
	}
	var out []taskProgression
	appendSpan := func(first, last, step int64) {
		if first <= last {
			if first == last {
				step = 1
			}
			out = append(out, taskProgression{first, last, step})
		}
	}
	appendSpan(a.first, i.first-a.step, a.step)
	if i.count() > 1 && i.step > a.step {
		// Either describe each gap between removed tasks, or describe the
		// surviving residues. Choose the smaller representation.
		gaps, residues := i.count()-1, i.step/a.step-1
		if min(gaps, residues) > maxRangeTerms-2 {
			return nil, fmt.Errorf("array selection exceeds fragment limit")
		}
		if gaps <= residues {
			for n := i.first; n < i.last; n += i.step {
				appendSpan(n+a.step, n+i.step-a.step, a.step)
			}
		} else {
			for offset := a.step; offset < i.step; offset += a.step {
				appendSpan(i.first+offset, i.last-i.step+offset, i.step)
			}
		}
	}
	appendSpan(i.last+a.step, a.last, a.step)
	return out, nil
}

func progressionCount(spans []taskProgression) int64 {
	var n int64
	for _, p := range spans {
		n += p.count()
	}
	return n
}

// Format a disjoint selection without enumerating its members. Adjacent spans
// of the same stride are merged; two consecutive individual IDs retain the
// familiar comma form used by the existing copy command.
func formatTaskSelection(spans []taskProgression) string {
	spans = append([]taskProgression(nil), spans...)
	sort.Slice(spans, func(i, j int) bool { return spans[i].first < spans[j].first })
	var merged []taskProgression
	for _, p := range spans {
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			if last.step == p.step && last.last+last.step == p.first {
				last.last = p.last
				continue
			}
		}
		merged = append(merged, p)
	}
	var parts []string
	for _, p := range merged {
		s := strconv.FormatInt(p.first, 10)
		if p.count() == 2 && p.step == 1 {
			s += "," + strconv.FormatInt(p.last, 10)
		} else if p.first != p.last {
			s += "-" + strconv.FormatInt(p.last, 10)
			if p.step > 1 {
				s += ":" + strconv.FormatInt(p.step, 10)
			}
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ",")
}
