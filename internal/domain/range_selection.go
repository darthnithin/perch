package domain

import "fmt"

// IntersectRanges returns a bounded, compact union of common task indices.
// Empty strings represent empty selections; no individual tasks are allocated.
func IntersectRanges(a, b string) (string, error) {
	if a == "" || b == "" {
		return "", nil
	}
	x, err := parseTaskProgressions(a)
	if err != nil {
		return "", err
	}
	y, err := parseTaskProgressions(b)
	if err != nil {
		return "", err
	}
	set := newTaskSet()
	work := maxRangeIntersections
	for _, p := range x {
		for _, q := range y {
			work--
			if work < 0 {
				return "", fmt.Errorf("array intersection exceeds work limit")
			}
			if common, ok := intersectProgressions(p, q); ok {
				if _, err := set.add([]taskProgression{common}); err != nil {
					return "", err
				}
			}
		}
	}
	return formatTaskSelection(set.spans), nil
}

// SubtractRanges returns a without b, with the same fragmentation/work bounds
// used for failed-task selections. An error never returns a partial selection.
func SubtractRanges(a, b string) (string, error) {
	if a == "" {
		return "", nil
	}
	x, err := parseTaskProgressions(a)
	if err != nil {
		return "", err
	}
	set := newTaskSet()
	if b != "" {
		y, err := parseTaskProgressions(b)
		if err != nil {
			return "", err
		}
		if _, err = set.add(y); err != nil {
			return "", err
		}
	}
	remaining, err := set.add(x)
	if err != nil {
		return "", err
	}
	return formatTaskSelection(remaining), nil
}
