package app

import (
	"perch/internal/domain"
	"perch/internal/timewarning"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Retention = 8 * 24 * time.Hour

// Checkpoint retains identity, observed array totals and deduplication, never
// scripts, resource measurements or log content.
// Restored jobs are evidence of observation, not a fresh scheduler snapshot.
type Checkpoint struct {
	TimeWarnings              string               `json:"time_warnings,omitempty"`
	At                        time.Time            `json:"at"`
	Active                    []Observation        `json:"active"`
	Seen                      map[string]time.Time `json:"seen"`
	Arrays                    []ArrayObservation   `json:"arrays,omitempty"`
	Displays                  []ArrayDisplay       `json:"array_displays,omitempty"`
	ArrayDisplayLimitedUntil  time.Time            `json:"array_display_limited_until,omitempty"`
	ArrayTrackingLimitedUntil time.Time            `json:"array_tracking_limited_until,omitempty"`
}

type Observation struct {
	ID, Name, Cluster, User string
	Submit                  time.Time
	Restarts                int
	ArrayID                 int64
	TaskID                  *int64 `json:",omitempty"`
	TaskRange               string `json:",omitempty"`
}

func (s *Store) Checkpoint() Checkpoint {
	c := Checkpoint{TimeWarnings: s.warnings.String(), At: s.Snapshot.At, Seen: make(map[string]time.Time)}
	c.Arrays = s.arrayCheckpoint()
	c.Displays = s.displayCheckpoint()
	c.ArrayDisplayLimitedUntil = s.arrayDisplayLimitedUntil
	c.ArrayTrackingLimitedUntil = s.arrayTrackingLimitedUntil
	for _, j := range s.Snapshot.Jobs {
		if j.Terminal() {
			continue
		}
		c.Active = append(c.Active, Observation{ID: j.ID, ArrayID: j.ArrayID, TaskID: j.TaskID, TaskRange: j.TaskRange, Name: j.Name, Cluster: j.Cluster, User: j.User, Submit: j.Submit, Restarts: j.Restarts})
		if len(c.Active) == 5000 {
			break
		}
	}
	type entry struct {
		key string
		at  time.Time
	}
	var seen []entry
	urgent := map[string]int64{}
	for k, at := range s.Seen {
		if !at.After(c.At) && c.At.Sub(at) <= Retention {
			if base, threshold, ok := warningCheckpointKey(k); ok {
				if previous, exists := urgent[base]; !exists || threshold < previous {
					urgent[base] = threshold
				}
			}
		}
	}
	for k, at := range s.Seen {
		if base, threshold, ok := warningCheckpointKey(k); ok && urgent[base] != threshold {
			continue
		}
		if !at.After(c.At) && c.At.Sub(at) <= Retention {
			seen = append(seen, entry{k, at})
		}
	}
	sort.Slice(seen, func(i, j int) bool {
		if seen[i].at.Equal(seen[j].at) {
			return seen[i].key < seen[j].key
		}
		return seen[i].at.After(seen[j].at)
	})
	for _, e := range seen[:min(len(seen), 10000)] {
		c.Seen[e.key] = e.at
	}
	return c
}

func (s *Store) Restore(c Checkpoint, now time.Time) bool {
	if c.At.IsZero() || c.At.After(now) || now.Sub(c.At) > Retention || len(c.Active) > 5000 || len(c.Seen) > 10000 || !validateArrayCheckpoint(c) || !validArrayDisplays(c) {
		return false
	}
	previousWarnings := timewarning.Policy{}
	if c.TimeWarnings != "" {
		var err error
		previousWarnings, err = timewarning.Parse(c.TimeWarnings)
		if err != nil {
			return false
		}
	}
	s.baselineWarnings = previousWarnings.String() != s.warnings.String()
	s.revision++
	s.Seen = make(map[string]time.Time)
	s.arrayDisplays = make(map[int64]ArrayDisplay)
	s.MinimumTotals = make(map[int64]int64)
	if c.ArrayDisplayLimitedUntil.After(now) {
		s.arrayDisplayLimitedUntil = c.ArrayDisplayLimitedUntil
	}
	for _, d := range c.Displays {
		if now.Sub(d.ObservedAt) <= Retention {
			d.Retries = append([]ArrayRetry(nil), d.Retries...)
			s.arrayDisplays[d.ID] = d
			s.MinimumTotals[d.ID] = d.Minimum
		}
	}
	s.observedArrays = make(map[arrayAttempt]ArrayObservation)
	for _, observation := range c.Arrays {
		observation.Spans = append([]TaskSpan(nil), observation.Spans...)
		s.observedArrays[observation.key()] = observation
	}
	s.arrayTrackingLimitedUntil = c.ArrayTrackingLimitedUntil
	s.expireArrayObservations(now)
	for k, at := range c.Seen {
		if !at.After(now) && now.Sub(at) <= Retention {
			s.Seen[k] = at
		}
	}
	for _, o := range c.Active {
		j := domain.Job{ID: o.ID, Name: o.Name, Cluster: o.Cluster, User: o.User, Submit: o.Submit, Restarts: o.Restarts, ArrayID: o.ArrayID, TaskID: o.TaskID, TaskRange: o.TaskRange, State: "UNKNOWN", Reason: "Previously observed; waiting for a fresh scheduler response"}
		s.Snapshot.Jobs = append(s.Snapshot.Jobs, j)
		// Older checkpoints have only Active observations.
		if c.Arrays == nil {
			s.observeArray(j, c.At)
		}
	}
	s.initialized = true
	return true
}

// Default warning keys predate configurable thresholds and imply five minutes.
func warningCheckpointKey(key string) (string, int64, bool) {
	i := strings.LastIndex(key, "/time/")
	if i < 0 {
		return "", 0, false
	}
	parts := strings.Split(key[i+6:], "/")
	if len(parts) != 1 && (len(parts) != 3 || parts[1] != "threshold") {
		return "", 0, false
	}
	if limit, err := strconv.ParseInt(parts[0], 10, 64); err != nil || limit <= 0 {
		return "", 0, false
	}
	threshold := int64(300)
	if len(parts) == 3 {
		var err error
		threshold, err = strconv.ParseInt(parts[2], 10, 64)
		if err != nil || threshold <= 0 {
			return "", 0, false
		}
	}
	return key[:i+6] + parts[0], threshold, true
}
