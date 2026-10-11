// Package timewarning defines scheduler-measurement warning thresholds.
package timewarning

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"perch/internal/domain"
)

// A zero Policy preserves the built-in five-minute warning. Configured off is
// distinct from zero, and thresholds are immutable outside this package.
type Policy struct {
	configured bool
	thresholds []time.Duration
}

func Parse(value string) (Policy, error) {
	p := Policy{configured: true}
	if value == "off" {
		return p, nil
	}
	parts := strings.Split(value, ",")
	if len(parts) > 5 {
		return Policy{}, fmt.Errorf("choose at most five time-warning thresholds")
	}
	for _, part := range parts {
		d, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil || d < time.Minute || d > 24*time.Hour || d%time.Minute != 0 {
			return Policy{}, fmt.Errorf("time warnings need whole minutes from 1m to 24h, such as 15m,5m,1m, or off")
		}
		if slices.Contains(p.thresholds, d) {
			return Policy{}, fmt.Errorf("time-warning thresholds must be distinct")
		}
		p.thresholds = append(p.thresholds, d)
	}
	slices.Sort(p.thresholds)
	slices.Reverse(p.thresholds)
	return p, nil
}

func (p Policy) thresholdsOrDefault() []time.Duration {
	if !p.configured {
		return []time.Duration{5 * time.Minute}
	}
	return p.thresholds
}

func (p Policy) String() string {
	var values []string
	for _, d := range p.thresholdsOrDefault() {
		values = append(values, strconv.Itoa(int(d/time.Minute))+"m")
	}
	if len(values) == 0 {
		return "off"
	}
	return strings.Join(values, ",")
}

// Crossed is based only on the latest measured elapsed time, never a local
// countdown. A threshold applies only to limits at least twice its length.
func (p Policy) Crossed(j domain.Job) []time.Duration {
	remaining, ok := j.Remaining()
	if !ok || j.Elapsed < 0 {
		return nil
	}
	var out []time.Duration
	for _, d := range p.thresholdsOrDefault() {
		if j.Limit/2 >= d && remaining <= d {
			out = append(out, d)
		}
	}
	return out
}

func (p Policy) Near(j domain.Job) bool { return len(p.Crossed(j)) > 0 }
