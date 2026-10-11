package app

import (
	"fmt"
	"sort"
	"time"

	"perch/internal/domain"
)

// ArrayDisplay retains a previously observed total, not inferred task IDs.
// Missing records are presented separately from scheduler jobs and notifications.
type ArrayDisplay struct {
	ID                             int64
	Name, Partition, User, Cluster string
	Submit, ObservedAt             time.Time
	Minimum                        int64
	Retries                        []ArrayRetry `json:",omitempty"`
}

// Submission time changes when Slurm requeues a member. Only retry identities
// connected to an older observed record are admitted into an existing array.
type ArrayRetry struct {
	Submit   time.Time
	Restarts int
}

const maxArrayRetries = 128

func (d ArrayDisplay) matches(j domain.Job) bool {
	if d.ID != j.ArrayID || d.User != j.User || d.Cluster != j.Cluster {
		return false
	}
	if d.Submit.Equal(j.Submit) {
		return true
	}
	for _, r := range d.Retries {
		if r.Submit.Equal(j.Submit) && r.Restarts == j.Restarts {
			return true
		}
	}
	return false
}

func sameArrayMember(a, b domain.Job) bool {
	return a.ArrayID > 0 && a.ArrayID == b.ArrayID && a.User == b.User && a.Cluster == b.Cluster &&
		a.RawID > 0 && a.RawID == b.RawID && a.TaskID != nil && b.TaskID != nil && *a.TaskID == *b.TaskID
}

func (s *Store) retryPredecessors(j domain.Job, previous map[string]domain.Job) []domain.Job {
	if j.Restarts <= 0 || j.TaskID == nil || j.RawID <= 0 || j.Submit.IsZero() {
		return nil
	}
	var out []domain.Job
	consider := func(old domain.Job) {
		if sameArrayMember(old, j) && old.Restarts < j.Restarts && !old.Submit.IsZero() && old.Submit.Before(j.Submit) {
			out = append(out, old)
		}
	}
	if old, ok := previous[j.ID]; ok {
		consider(old)
	}
	for _, old := range s.History {
		consider(old)
	}
	return out
}

func (s *Store) linkArrayRetry(d *ArrayDisplay, j domain.Job, previous map[string]domain.Job, at time.Time) bool {
	if d.matches(j) {
		return true
	}
	for _, old := range s.retryPredecessors(j, previous) {
		if d.matches(old) {
			if len(d.Retries) >= maxArrayRetries {
				s.arrayDisplayLimitedUntil = at.Add(Retention)
				return true // Keep the original total; unmatched retry remains unresolved.
			}
			d.Retries = append(d.Retries, ArrayRetry{j.Submit, j.Restarts})
			return true
		}
	}
	return false
}

func (s *Store) updateArrayDisplays(queue, jobs []domain.Job, previous map[string]domain.Job, at time.Time) {
	if s.arrayDisplays == nil {
		s.arrayDisplays = map[int64]ArrayDisplay{}
	}
	if !s.arrayDisplayLimitedUntil.After(at) {
		s.arrayDisplayLimitedUntil = time.Time{}
	}
	for id, d := range s.arrayDisplays {
		if d.ObservedAt.After(at) || at.Sub(d.ObservedAt) > Retention {
			delete(s.arrayDisplays, id)
			delete(s.MinimumTotals, id)
		}
	}
	// Process original submissions before their retries, independent of queue order.
	queue = append([]domain.Job(nil), queue...)
	sort.SliceStable(queue, func(i, j int) bool { return queue[i].Submit.Before(queue[j].Submit) })
	for _, j := range queue {
		if j.ArrayID <= 0 || j.TaskID == nil && j.TaskRange == "" {
			continue
		}
		d, found := s.arrayDisplays[j.ArrayID]
		if !found || !s.linkArrayRetry(&d, j, previous, at) {
			if !found && len(s.arrayDisplays) >= maxObservedArrays {
				s.arrayDisplayLimitedUntil = at.Add(Retention)
				continue
			}
			d = ArrayDisplay{ID: j.ArrayID, Name: j.Name, Partition: j.Partition, User: j.User, Cluster: j.Cluster, Submit: j.Submit}
			// On first connection, duplicate accounting records can establish the
			// original submission even when only a retried task remains queued.
			for _, old := range s.retryPredecessors(j, previous) {
				if old.Submit.Before(d.Submit) {
					d.Submit = old.Submit
				}
			}
			if !d.Submit.Equal(j.Submit) {
				d.Retries = []ArrayRetry{{j.Submit, j.Restarts}}
			}
			delete(s.MinimumTotals, j.ArrayID)
		}
		d.ObservedAt = at
		s.arrayDisplays[j.ArrayID] = d
	}
	// A task may retry and finish between polls. Accounting duplicates can
	// connect its new submission to an already observed member in that case.
	for _, j := range jobs {
		if d, ok := s.arrayDisplays[j.ArrayID]; ok && s.linkArrayRetry(&d, j, previous, at) {
			s.arrayDisplays[j.ArrayID] = d
		}
	}
	for _, j := range s.History {
		if j.ArrayID <= 0 || j.TaskRange == "" || j.Terminal() {
			continue
		}
		d, found := s.arrayDisplays[j.ArrayID]
		if !found {
			if len(s.arrayDisplays) >= maxObservedArrays {
				s.arrayDisplayLimitedUntil = at.Add(Retention)
				continue
			}
			d = ArrayDisplay{ID: j.ArrayID, Name: j.Name, Partition: j.Partition, User: j.User, Cluster: j.Cluster, Submit: j.Submit, ObservedAt: at}
		}
		if d.matches(j) {
			d.Minimum = max(d.Minimum, j.Count())
			s.arrayDisplays[j.ArrayID] = d
		}
	}
	// The normal reconciled snapshot has already removed parent records and
	// accounting tasks covered by a current collapsed queue range.
	groups := map[int64][]domain.Job{}
	identityConflicts := domain.ArraySubmissionConflicts(jobs)
	for _, j := range s.arraySummaryJobs(jobs) {
		if d, ok := s.arrayDisplays[j.ArrayID]; ok && d.matches(j) {
			groups[j.ArrayID] = append(groups[j.ArrayID], j)
		}
	}
	for id, d := range s.arrayDisplays {
		if identityConflicts[id] != "" {
			continue // Ambiguous evidence cannot retire the observed minimum.
		}
		arrays := domain.Arrays(groups[id])
		if len(arrays) > 0 {
			a := arrays[0]
			if a.SummaryError != "" {
				continue
			}
			d.Minimum = max(d.Minimum, a.Known)
			if a.Known >= d.Minimum && a.Running+a.Pending+a.Unresolved == 0 {
				// Once fully confirmed, an aging history window must not resurrect
				// the array as awaiting accounting.
				delete(s.arrayDisplays, id)
				delete(s.MinimumTotals, id)
				continue
			}
		}
		s.arrayDisplays[id] = d
		s.MinimumTotals[id] = d.Minimum
	}
}

func (s *Store) ArrayDisplayLimited() bool {
	return s.arrayDisplayLimitedUntil.After(s.Snapshot.At)
}

// ArrayGaps returns presentation-only rows for the gap between a previously
// observed total and current records. Counts are lower bounds: tasks unseen by
// Perch are not included. These rows never enter Snapshot, Seen or event logic.
func (s *Store) ArrayGaps() []domain.Job {
	counts := map[int64]int64{}
	invalid := map[int64]bool{}
	var jobs []domain.Job
	for _, j := range s.arraySummaryJobs(s.Snapshot.Jobs) {
		if d, ok := s.arrayDisplays[j.ArrayID]; ok && d.matches(j) && (j.TaskID != nil || j.TaskRange != "") {
			jobs = append(jobs, j)
		}
	}
	for _, a := range domain.Arrays(jobs) {
		counts[a.ID] = a.Known
		invalid[a.ID] = a.SummaryError != ""
	}
	var gaps []domain.Job
	for id, d := range s.arrayDisplays {
		if invalid[id] {
			continue
		}
		missing := d.Minimum - counts[id]
		if missing <= 0 {
			continue
		}
		gaps = append(gaps, domain.Job{ID: fmt.Sprintf("%d [?]", id), ArrayID: id, Name: d.Name, Partition: d.Partition, User: d.User, Cluster: d.Cluster, Submit: d.Submit, State: "UNKNOWN", AwaitingTasks: missing, Reason: "Previously observed array tasks are missing from the current queue and accounting records"})
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i].ArrayID > gaps[j].ArrayID })
	return gaps
}

func (s *Store) displayCheckpoint() []ArrayDisplay {
	result := make([]ArrayDisplay, 0, len(s.arrayDisplays))
	for _, d := range s.arrayDisplays {
		d.Retries = append([]ArrayRetry(nil), d.Retries...)
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func validArrayDisplays(c Checkpoint) bool {
	if len(c.Displays) > maxObservedArrays || c.ArrayDisplayLimitedUntil.After(c.At.Add(Retention)) {
		return false
	}
	seen := map[int64]bool{}
	for _, d := range c.Displays {
		if seen[d.ID] || d.ID <= 0 || d.Minimum <= 0 || d.Minimum > maxTaskIndex+1 || d.ObservedAt.IsZero() || d.ObservedAt.After(c.At) || len(d.Name) > 4096 || len(d.Partition) > 4096 || len(d.User) > 256 || len(d.Cluster) > 256 {
			return false
		}
		if len(d.Retries) > maxArrayRetries {
			return false
		}
		retries := map[ArrayRetry]bool{}
		for _, r := range d.Retries {
			if r.Restarts <= 0 || !r.Submit.After(d.Submit) || r.Submit.After(c.At) || retries[r] {
				return false
			}
			retries[r] = true
		}
		seen[d.ID] = true
	}
	return true
}

// arraySummaryJobs keeps older outcomes visible in History, but prevents an
// older task attempt from being counted again beside its newer queued range.
// Ranges stay compact; even a billion-task range is a membership check.
func (s *Store) arraySummaryJobs(jobs []domain.Job) []domain.Job {
	ranges := map[int64][]domain.Job{}
	for _, j := range jobs {
		if j.ArrayID > 0 && j.TaskRange != "" && j.State == "PENDING" {
			ranges[j.ArrayID] = append(ranges[j.ArrayID], j)
		}
	}
	observed := map[int64][]ArrayObservation{}
	for _, o := range s.observedArrays {
		observed[o.ArrayID] = append(observed[o.ArrayID], o)
	}
	out := make([]domain.Job, 0, len(jobs))
	for _, j := range jobs {
		superseded := false
		if j.TaskID != nil {
			for _, o := range observed[j.ArrayID] {
				newer := domain.Job{ID: j.ID, User: o.User, Cluster: o.Cluster, Submit: o.Submit, Restarts: o.Restarts}
				if !newer.AttemptAfter(j) {
					continue
				}
				for _, span := range o.Spans {
					if span.includes(*j.TaskID) {
						superseded = true
						break
					}
				}
				if superseded {
					break
				}
			}
			for _, q := range ranges[j.ArrayID] {
				q.ID = j.ID // Compare the same array member's attempts.
				if q.AttemptAfter(j) && inRange(*j.TaskID, q.TaskRange) {
					superseded = true
					break
				}
			}
		}
		if !superseded {
			out = append(out, j)
		}
	}
	return out
}
