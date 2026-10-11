// Package app owns reconciled snapshots and observed events. It has no terminal
// rendering or transport side effects.
package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/timewarning"
)

type Event struct {
	TaskCount                     int64
	WarningThreshold, Remaining   time.Duration
	Key, Kind, JobID, Name, State string
	Attempt                       string
	ArrayID                       int64
	At                            time.Time
}
type Store struct {
	// Snapshot and derived evidence are read-only outside Apply/Restore.
	// Revision lets presentation caches retain derived data between updates.
	baselineWarnings          bool
	warnings                  timewarning.Policy
	revision                  uint64
	Snapshot                  domain.Snapshot
	History                   []domain.Job
	Events                    []Event
	MinimumTotals             map[int64]int64
	arrayDisplays             map[int64]ArrayDisplay
	arrayDisplayLimitedUntil  time.Time
	Seen                      map[string]time.Time
	initialized               bool
	observedArrays            map[arrayAttempt]ArrayObservation
	arrayTrackingLimitedUntil time.Time
}

func NewStore() *Store { return &Store{MinimumTotals: map[int64]int64{}, Seen: map[string]time.Time{}} }

func (s *Store) Revision() uint64 { return s.revision }

func inRange(task int64, s string) bool {
	if i := strings.IndexByte(s, '%'); i >= 0 {
		s = s[:i]
	}
	s = strings.Trim(s, "[]")
	for _, part := range strings.Split(s, ",") {
		step := int64(1)
		if a, b, ok := strings.Cut(part, ":"); ok {
			part = a
			step, _ = strconv.ParseInt(b, 10, 64)
		}
		a, b, rangePart := strings.Cut(part, "-")
		lo, e := strconv.ParseInt(a, 10, 64)
		if e != nil {
			continue
		}
		hi := lo
		if rangePart {
			hi, _ = strconv.ParseInt(b, 10, 64)
		}
		if step > 0 && task >= lo && task <= hi && (task-lo)%step == 0 {
			return true
		}
	}
	return false
}
func covered(j domain.Job, queue []domain.Job) bool {
	if j.ArrayID == 0 || j.TaskID == nil {
		return false
	}
	for _, q := range queue {
		if arrayKey(q) == arrayKey(j) && q.TaskRange != "" && inRange(*j.TaskID, q.TaskRange) {
			return true
		}
	}
	return false
}
func (s *Store) Apply(r hummingbird.Refresh) []Event {
	s.revision++
	if r.HistoryOK {
		s.History = r.History
		s.Snapshot.HistoryAt = r.At
	}
	if r.PartitionsOK {
		s.Snapshot.Partitions = r.Partitions
		s.Snapshot.PartitionsAt = r.At
	}
	// A partial queue failure never makes any previous job disappear.
	if !r.QueueOK {
		return nil
	}
	s.expireArrayObservations(r.At)
	previous := map[string]domain.Job{}
	var unfinished []domain.Job
	for _, j := range s.Snapshot.Jobs {
		previous[j.ID] = j
		if !j.Terminal() {
			unfinished = append(unfinished, j)
		}
	}
	current := map[string]domain.Job{}
	for _, j := range s.History {
		j.QueueObserved, j.AwaitingAccounting = false, false
		if !r.HistoryOK && covered(j, unfinished) {
			j = awaitingAccounting(j)
		}
		// A pending accounting bitmap can lag the queue's split into running
		// tasks and smaller ranges. Retain its size as a lower bound below,
		// never as another job alongside the authoritative queue records.
		if j.TaskRange != "" && !j.Terminal() {
			continue
		}
		if covered(j, r.Queue) {
			continue
		}
		if !j.Terminal() {
			j.State = "UNKNOWN"
			j.Reason = "Awaiting scheduler/accounting confirmation"
		}
		if existing, ok := current[j.ID]; !ok || j.AttemptAfter(existing) {
			current[j.ID] = j
		}
	}
	queued := map[string]bool{}
	for _, j := range r.Queue {
		j.QueueObserved, j.AwaitingAccounting = true, false
		queued[j.ID] = true
		current[j.ID] = j
	}
	for id, j := range previous {
		if covered(j, r.Queue) || queued[id] {
			continue
		}
		j.QueueObserved = false
		if !r.HistoryOK && !j.Terminal() && staleAccountingOverlaps(j, s.History) {
			current[id] = awaitingAccounting(j)
			continue
		}
		if j.TaskRange != "" {
			continue
		}
		j.AwaitingAccounting = false
		if candidate, exists := current[id]; exists {
			if candidate.AttemptKey() == j.AttemptKey() {
				// Accounting can lag even a previously confirmed terminal row.
				if j.Terminal() && !candidate.Terminal() {
					current[id] = j
				}
				continue
			}
			if candidate.AttemptAfter(j) {
				continue
			}
			if j.Terminal() && !j.AttemptAfter(candidate) {
				continue
			}
		} else if j.Terminal() {
			continue
		} // Let history age out normally.
		if !j.Terminal() {
			j.State = "UNKNOWN"
			j.Reason = "No longer in the queue; accounting has not confirmed an outcome"
		}
		current[id] = j
	}

	// sacct may emit array parents in addition to task records; they are not
	// additional tasks and must not inflate array progress.
	members := map[int64]bool{}
	for _, j := range current {
		if j.ArrayID > 0 && (j.TaskID != nil || j.TaskRange != "") {
			members[j.ArrayID] = true
		}
	}
	jobs := make([]domain.Job, 0, len(current))
	// Derive aggregate winners from the same bounded reconciliation used by
	// progress and failed-task copying, before generating any range events.
	var candidates []domain.Job
	for _, j := range current {
		candidates = append(candidates, j)
	}
	rangeMembers := map[string]string{}
	identityConflicts := domain.ArraySubmissionConflicts(candidates)
	for _, a := range domain.Arrays(s.arraySummaryJobs(candidates)) {
		if identityConflicts[a.ID] != "" {
			continue
		}
		for _, outcome := range a.RangeOutcomes {
			rangeMembers[outcome.Job.AttemptKey()] = outcome.Members
		}
	}
	var events []Event
	for _, j := range current {
		if j.ArrayID > 0 && j.TaskID == nil && j.TaskRange == "" && members[j.ArrayID] {
			continue
		}
		jobs = append(jobs, j)
		old, known := previous[j.ID]
		if known && old.AttemptKey() != j.AttemptKey() {
			known = false
		}
		// A short array task can start and finish between polls. Its previous
		// observation may be a collapsed pending range, rather than a task row.
		if !known && s.observedArrayTask(j) {
			old, known = j, true
			old.State = "UNKNOWN" // prior observation proves membership, not outcome
		}
		if j.Terminal() && j.TaskRange != "" && j.TaskID == nil {
			if r.HistoryOK || j.QueueObserved {
				if event := s.rangeOutcome(j, rangeMembers[j.AttemptKey()], r.At); event != nil {
					events = append(events, *event)
				}
			}
		} else if j.Terminal() {
			key := j.AttemptKey() + "/outcome"
			_, seen := s.Seen[key]
			if s.initialized && known && !old.Terminal() && old.AttemptKey() == j.AttemptKey() && !seen {
				kind := "finished"
				if j.Failed() {
					kind = "failed"
				}
				if j.State == "CANCELLED" {
					kind = "cancelled"
				}
				events = append(events, Event{Key: key, Kind: kind, JobID: j.ID, Name: j.Name, State: j.State, Attempt: j.AttemptKey(), ArrayID: j.ArrayID, At: r.At})
			}
			s.Seen[key] = r.At
		}
		var warning *Event
		crossed := s.warnings.Crossed(j)
		for _, threshold := range crossed {
			key := warningKey(j, threshold)
			seen := false
			// A more urgent retained threshold also covers earlier warnings.
			// Checkpoints compact these keys so arrays do not multiply storage
			// by the number of configured thresholds.
			for _, urgent := range crossed {
				if urgent <= threshold {
					if _, ok := s.Seen[warningKey(j, urgent)]; ok {
						seen = true
						break
					}
				}
			}
			if !seen && s.initialized && !s.baselineWarnings {
				remaining, _ := j.Remaining()
				warning = &Event{Key: key, Kind: "time warning", JobID: j.ID, Name: j.Name, State: j.State, Attempt: j.AttemptKey(), ArrayID: j.ArrayID, At: r.At, WarningThreshold: threshold, Remaining: remaining}
			}
			s.Seen[key] = r.At
		}
		// Descending thresholds: only the most urgent newly crossed warning is
		// emitted, while all crossed thresholds are marked seen for catch-up.
		if warning != nil {
			events = append(events, *warning)
		}
	}
	domain.SortJobs(jobs)
	s.Snapshot.Jobs = jobs
	s.Snapshot.At = r.At
	for _, j := range r.Queue {
		if !j.Terminal() {
			s.observeArray(j, r.At)
		}
	}
	s.resolveArrayTasks(jobs, r.At)
	s.updateArrayDisplays(r.Queue, jobs, previous, r.At)
	s.initialized = true
	s.baselineWarnings = false
	sort.Slice(events, func(i, j int) bool { return events[i].Key < events[j].Key })
	s.Events = append(s.Events, events...)
	if len(s.Events) > 100 {
		s.Events = s.Events[len(s.Events)-100:]
	}
	for key, at := range s.Seen {
		if r.At.Sub(at) > 8*24*time.Hour {
			delete(s.Seen, key)
		}
	}
	return events
}
func (s *Store) Arrays() []domain.ArraySummary {
	identityConflicts := domain.ArraySubmissionConflicts(s.Snapshot.Jobs)
	var jobs []domain.Job
	for _, j := range s.Snapshot.Jobs {
		if d, ok := s.arrayDisplays[j.ArrayID]; ok && !d.matches(j) {
			continue
		}
		jobs = append(jobs, j)
	}
	arrays := domain.Arrays(s.arraySummaryJobs(jobs))
	present := map[int64]bool{}
	for i := range arrays {
		a := &arrays[i]
		present[a.ID] = true
		if err := identityConflicts[a.ID]; err != "" {
			*a = domain.ArraySummary{ID: a.ID, SummaryError: err}
			continue
		}
		if minimum := s.MinimumTotals[a.ID]; minimum > a.Known && a.SummaryError == "" {
			a.Unresolved += minimum - a.Known
			a.Known = minimum
		}
	}
	for id, err := range identityConflicts {
		if !present[id] {
			arrays = append(arrays, domain.ArraySummary{ID: id, SummaryError: err})
			present[id] = true
		}
	}
	for id, minimum := range s.MinimumTotals {
		if !present[id] && minimum > 0 {
			arrays = append(arrays, domain.ArraySummary{ID: id, Known: minimum, Unresolved: minimum})
		}
	}
	sort.Slice(arrays, func(i, j int) bool { return arrays[i].ID > arrays[j].ID })
	return arrays
}

func warningKey(j domain.Job, threshold time.Duration) string {
	key := fmt.Sprintf("%s/time/%d", j.AttemptKey(), int64(j.Limit.Seconds()))
	// Retain the old default key so existing notification checkpoints dedupe.
	if threshold != 5*time.Minute {
		key += fmt.Sprintf("/threshold/%d", int64(threshold.Seconds()))
	}
	return key
}

// SetTimeWarnings baselines already-crossed thresholds from the displayed
// snapshot. Editing a preference does not create retroactive activity/alerts.
func (s *Store) SetTimeWarnings(policy timewarning.Policy) {
	s.warnings = policy
	for _, j := range s.Snapshot.Jobs {
		for _, threshold := range policy.Crossed(j) {
			s.Seen[warningKey(j, threshold)] = s.Snapshot.At
		}
	}
}
