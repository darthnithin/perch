package domain

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// MaxTaskIndex excludes Slurm's INFINITE and NO_VAL uint32 sentinels.
const MaxTaskIndex = int64(4294967293)
const maxRangeBytes = 1 << 20
const maxRangeTerms = 8192
const maxRangeIntersections = 200000

type taskProgression struct{ first, last, step int64 }

func (p taskProgression) count() int64 { return (p.last-p.first)/p.step + 1 }
func (p taskProgression) contains(q taskProgression) bool {
	return q.first >= p.first && q.last <= p.last && (q.first-p.first)%p.step == 0 && (q.last-p.first)%p.step == 0 && (q.first == q.last || q.step%p.step == 0)
}

// RangeCount counts distinct task indices. It uses compact arithmetic
// progressions, not one allocation per task. Normal disjoint scheduler ranges
// are processed in sorted order; overlapping stepped ranges use bounded exact
// inclusion-exclusion. Excessive fragmentation/complexity returns an error,
// never a guessed or double-counted total.
func RangeCount(value string) (int64, error) {
	spans, err := parseTaskProgressions(value)
	if err != nil {
		return 0, err
	}
	return countProgressions(spans, maxRangeIntersections)
}

// RangeContains tests membership without expanding a potentially large array.
func RangeContains(value string, task int64) (bool, error) {
	spans, err := parseTaskProgressions(value)
	if err != nil {
		return false, err
	}
	for _, p := range spans {
		if task >= p.first && task <= p.last && (task-p.first)%p.step == 0 {
			return true, nil
		}
	}
	return false, nil
}

func decimal(s string) (int64, error) {
	if s == "" || strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, fmt.Errorf("invalid array number")
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseTaskProgressions(value string) ([]taskProgression, error) {
	if len(value) == 0 || len(value) > maxRangeBytes {
		return nil, fmt.Errorf("array range is empty or exceeds its byte limit")
	}
	if value[0] == '[' {
		end := strings.IndexByte(value, ']')
		if end < 0 || end+1 < len(value) && value[end+1] != '%' {
			return nil, fmt.Errorf("invalid array brackets")
		}
		value = value[1:end] + value[end+1:]
	}
	if strings.ContainsAny(value, "[]") {
		return nil, fmt.Errorf("invalid array brackets")
	}
	indices, throttle, limited := strings.Cut(value, "%")
	if limited {
		n, err := decimal(throttle)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid array concurrency")
		}
	}
	var spans []taskProgression
	for _, part := range strings.Split(indices, ",") {
		bounds, stride, stepped := strings.Cut(part, ":")
		lo, hi, ranged := strings.Cut(bounds, "-")
		first, err := decimal(lo)
		if err != nil || first > MaxTaskIndex {
			return nil, fmt.Errorf("invalid task index")
		}
		last, step := first, int64(1)
		if ranged {
			last, err = decimal(hi)
			if err != nil || last < first || last > MaxTaskIndex {
				return nil, fmt.Errorf("invalid array end")
			}
		}
		if stepped {
			if !ranged {
				return nil, fmt.Errorf("array stride requires a range")
			}
			step, err = decimal(stride)
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid array step")
			}
		}
		last = first + (last-first)/step*step
		if first == last {
			step = 1
		}
		spans = append(spans, taskProgression{first, last, step})
	}
	sort.Slice(spans, func(i, j int) bool {
		a, b := spans[i], spans[j]
		if a.first != b.first {
			return a.first < b.first
		}
		if a.last != b.last {
			return a.last > b.last
		}
		return a.step < b.step
	})
	return spans, nil
}

func countProgressions(spans []taskProgression, budget int) (int64, error) {
	terms := map[taskProgression]int64{}
	total, groupLast := int64(0), int64(-1)
	flush := func() error {
		// Intermediate signed terms can exceed the final union's size. Sum them
		// exactly before checking the bounded final result.
		var sum, product big.Int
		for p, coefficient := range terms {
			sum.Add(&sum, product.Mul(big.NewInt(p.count()), big.NewInt(coefficient)))
		}
		if !sum.IsInt64() || sum.Sign() < 0 || sum.Int64() > MaxTaskIndex+1-total {
			return fmt.Errorf("invalid array union count")
		}
		total += sum.Int64()
		clear(terms)
		return nil
	}
	for _, next := range spans {
		if next.first > groupLast {
			if err := flush(); err != nil {
				return 0, err
			}
			groupLast = next.last
		}
		groupLast = max(groupLast, next.last)
		// Every intersection term is itself a subset of an input progression,
		// even when its signed coefficient is negative.
		covered := false
		for p := range terms {
			budget--
			if budget < 0 {
				return 0, fmt.Errorf("array overlap exceeds computation limit")
			}
			if p.contains(next) {
				covered = true
			}
		}
		if covered {
			continue
		}
		changes := map[taskProgression]int64{next: 1}
		for p, coefficient := range terms {
			budget--
			if budget < 0 {
				return 0, fmt.Errorf("array overlap exceeds computation limit")
			}
			if intersection, ok := intersectProgressions(p, next); ok {
				if coefficient == math.MinInt64 || !addCoefficient(changes, intersection, -coefficient) {
					return 0, fmt.Errorf("array overlap exceeds coefficient limit")
				}
			}
		}
		for p, coefficient := range changes {
			if !addCoefficient(terms, p, coefficient) {
				return 0, fmt.Errorf("array overlap exceeds coefficient limit")
			}
		}
		if len(terms) > maxRangeTerms {
			return 0, fmt.Errorf("array overlap exceeds term limit")
		}
	}
	if err := flush(); err != nil {
		return 0, err
	}
	return total, nil
}

func addCoefficient(terms map[taskProgression]int64, p taskProgression, delta int64) bool {
	old := terms[p]
	if delta > 0 && old > math.MaxInt64-delta || delta < 0 && old < math.MinInt64-delta {
		return false
	}
	next := old + delta
	if next == 0 {
		delete(terms, p)
	} else {
		terms[p] = next
	}
	return true
}

func intersectProgressions(a, b taskProgression) (taskProgression, bool) {
	lo, hi := max(a.first, b.first), min(a.last, b.last)
	if lo > hi {
		return taskProgression{}, false
	}
	if a.contains(b) {
		return b, true
	}
	if b.contains(a) {
		return a, true
	}
	// Solve a.first + a.step*k == b.first (mod b.step). Big integers
	// avoid overflow when the least common multiple exceeds uint32 or int64.
	as, bs := big.NewInt(a.step), big.NewInt(b.step)
	var gcd, inverse big.Int
	gcd.GCD(&inverse, nil, as, bs)
	difference := big.NewInt(b.first - a.first)
	var remainder big.Int
	if remainder.Mod(difference, &gcd).Sign() != 0 {
		return taskProgression{}, false
	}
	modulus := new(big.Int).Quo(bs, &gcd)
	k := new(big.Int).Quo(difference, &gcd)
	k.Mul(k, &inverse).Mod(k, modulus)
	period := new(big.Int).Mul(as, modulus)
	first := new(big.Int).Mul(as, k)
	first.Add(first, big.NewInt(a.first)).Mod(first, period)
	if first.Cmp(big.NewInt(lo)) < 0 {
		delta := new(big.Int).Sub(big.NewInt(lo), first)
		delta.Add(delta, new(big.Int).Sub(period, big.NewInt(1))).Quo(delta, period)
		first.Add(first, delta.Mul(delta, period))
	}
	if first.Cmp(big.NewInt(hi)) > 0 {
		return taskProgression{}, false
	}
	start := first.Int64()
	if period.Cmp(big.NewInt(hi-start)) > 0 {
		return taskProgression{start, start, 1}, true
	}
	step := period.Int64()
	return taskProgression{start, start + (hi-start)/step*step, step}, true
}
