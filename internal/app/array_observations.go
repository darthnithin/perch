package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"perch/internal/domain"
)

const maxObservedArrays = 512
const maxObservedSpans = 8192
const maxArraySpans = 1024
const maxTaskIndex = domain.MaxTaskIndex

// TaskSpan is an arithmetic progression. It preserves billion-task ranges
// without allocating a task per index. Overlapping spans are membership evidence,
// not additive task counts or evidence of an outcome.
type TaskSpan struct {
	First int64 `json:"first"`
	Last  int64 `json:"last"`
	Step  int64 `json:"step"`
}

type ArrayObservation struct {
	ArrayID  int64      `json:"array_id"`
	Cluster  string     `json:"cluster"`
	User     string     `json:"user"`
	Submit   time.Time  `json:"submit"`
	Restarts int        `json:"restarts"`
	At       time.Time  `json:"observed_at"`
	Spans    []TaskSpan `json:"spans"`
}

type arrayAttempt struct {
	ID            int64
	Cluster, User string
	Submit        time.Time
	Restarts      int
}

func arrayKey(j domain.Job) arrayAttempt {
	return arrayAttempt{j.ArrayID, j.Cluster, j.User, j.Submit.UTC(), j.Restarts}
}
func (o ArrayObservation) key() arrayAttempt {
	return arrayAttempt{o.ArrayID, o.Cluster, o.User, o.Submit.UTC(), o.Restarts}
}
func (p TaskSpan) valid() bool {
	return p.First >= 0 && p.Last <= maxTaskIndex && p.Last >= p.First && p.Step > 0 && p.Step <= maxTaskIndex && (p.Last-p.First)%p.Step == 0
}
func (p TaskSpan) includes(id int64) bool {
	return id >= p.First && id <= p.Last && (id-p.First)%p.Step == 0
}
func (p TaskSpan) contains(q TaskSpan) bool {
	return p.includes(q.First) && p.includes(q.Last) && (q.First == q.Last || q.Step%p.Step == 0)
}

func parseSpans(value string) ([]TaskSpan, error) {
	if len(value) > 64<<10 || strings.Count(value, ",") >= maxArraySpans {
		return nil, fmt.Errorf("array observation is too fragmented")
	}
	if _, err := domain.RangeCount(value); err != nil {
		return nil, err
	}
	value, _, _ = strings.Cut(value, "%")
	value = strings.Trim(value, "[]")
	var result []TaskSpan
	for _, part := range strings.Split(value, ",") {
		span, stepText, stepped := strings.Cut(part, ":")
		lo, hi, ranged := strings.Cut(span, "-")
		first, _ := strconv.ParseInt(lo, 10, 64)
		last := first
		if ranged {
			last, _ = strconv.ParseInt(hi, 10, 64)
		}
		step := int64(1)
		if stepped {
			step, _ = strconv.ParseInt(stepText, 10, 64)
		}
		last = first + (last-first)/step*step
		if last == first {
			step = 1
		}
		p := TaskSpan{first, last, step}
		if !p.valid() {
			return nil, fmt.Errorf("invalid task range observation")
		}
		result = unionSpan(result, p)
	}
	return result, nil
}

// Remove contained spans and merge touching progressions with the same stride.
// Unlike rounding to a bounding interval, this never invents observed indices.
func unionSpan(spans []TaskSpan, next TaskSpan) []TaskSpan {
	for _, span := range spans {
		if span.contains(next) {
			return spans
		}
	}
	for i := 0; i < len(spans); {
		p := spans[i]
		merge := p.Step == next.Step && (p.First-next.First)%p.Step == 0 && p.First <= next.Last+next.Step && next.First <= p.Last+p.Step
		if next.contains(p) || merge {
			if merge {
				next.First = min(next.First, p.First)
				next.Last = max(next.Last, p.Last)
			}
			spans = append(spans[:i], spans[i+1:]...)
			i = 0 // an extended progression can now absorb an earlier one
		} else {
			i++
		}
	}
	return append(spans, next)
}

func (s *Store) expireArrayObservations(at time.Time) {
	for key, observation := range s.observedArrays {
		if observation.At.After(at) || at.Sub(observation.At) > Retention {
			delete(s.observedArrays, key)
		}
	}
	if !s.arrayTrackingLimitedUntil.After(at) {
		s.arrayTrackingLimitedUntil = time.Time{}
	}
}

func (s *Store) observeArray(j domain.Job, at time.Time) {
	if j.ArrayID <= 0 || j.TaskRange == "" {
		return
	}
	spans, err := parseSpans(j.TaskRange)
	if err != nil || j.Submit.IsZero() || j.Restarts < 0 || j.Cluster == "" || j.User == "" || len(j.Cluster) > 256 || len(j.User) > 256 {
		s.arrayTrackingLimitedUntil = at.Add(Retention)
		return
	}
	key := arrayKey(j)
	prior, exists := s.observedArrays[key]
	if !exists && len(s.observedArrays) >= maxObservedArrays {
		s.arrayTrackingLimitedUntil = at.Add(Retention)
		return
	}
	merged := append([]TaskSpan(nil), prior.Spans...)
	for _, span := range spans {
		merged = unionSpan(merged, span)
	}
	total := len(merged)
	for other, observation := range s.observedArrays {
		if other != key {
			total += len(observation.Spans)
		}
	}
	if len(merged) > maxArraySpans || total > maxObservedSpans {
		s.arrayTrackingLimitedUntil = at.Add(Retention)
		return
	}
	if s.observedArrays == nil {
		s.observedArrays = make(map[arrayAttempt]ArrayObservation)
	}
	s.observedArrays[key] = ArrayObservation{j.ArrayID, j.Cluster, j.User, j.Submit, j.Restarts, at, merged}
}

func (s *Store) observedArrayTask(j domain.Job) bool {
	if j.TaskID == nil {
		return false
	}
	observation, ok := s.observedArrays[arrayKey(j)]
	if !ok {
		return false
	}
	for _, span := range observation.Spans {
		if span.includes(*j.TaskID) {
			return true
		}
	}
	return false
}

// Forget confirmed tasks, not just their notification keys. Otherwise a large
// array could notify again after its oldest deduplication keys leave a bounded
// checkpoint. Subtract sorted observed outcomes without expanding a range.
func (s *Store) resolveArrayTasks(jobs []domain.Job, at time.Time) {
	resolved := map[arrayAttempt][]int64{}
	for _, j := range jobs {
		if j.Terminal() && j.TaskID != nil {
			key := arrayKey(j)
			if _, ok := s.observedArrays[key]; ok {
				resolved[key] = append(resolved[key], *j.TaskID)
			}
		}
	}
	for key, ids := range resolved {
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		observation := s.observedArrays[key]
		var remaining []TaskSpan
		fragmented := false
		for _, span := range observation.Spans {
			next := span.First
			for _, id := range ids {
				if id < next || !span.includes(id) {
					continue
				}
				if id > next {
					remaining = append(remaining, TaskSpan{next, id - span.Step, span.Step})
					if len(remaining) > maxArraySpans {
						fragmented = true
						break
					}
				}
				next = id + span.Step
			}
			if fragmented {
				break
			}
			if next <= span.Last {
				remaining = append(remaining, TaskSpan{next, span.Last, span.Step})
			}
			if len(remaining) > maxArraySpans {
				fragmented = true
				break
			}
		}
		if fragmented {
			delete(s.observedArrays, key)
			s.arrayTrackingLimitedUntil = at.Add(Retention)
			continue
		}
		if len(remaining) == 0 {
			delete(s.observedArrays, key)
			continue
		}
		observation.Spans = remaining
		s.observedArrays[key] = observation
	}
	// Fragmentation is evaluated after the whole accounting batch: a thousand
	// contiguous outcomes should leave one suffix, not transiently hit a limit.
	total := 0
	for _, observation := range s.observedArrays {
		total += len(observation.Spans)
	}
	for _, observation := range s.arrayCheckpoint() {
		if len(observation.Spans) > maxArraySpans || total > maxObservedSpans {
			delete(s.observedArrays, observation.key())
			total -= len(observation.Spans)
			s.arrayTrackingLimitedUntil = at.Add(Retention)
		}
	}
}

func (s *Store) ArrayTrackingLimited() bool { return s.arrayTrackingLimitedUntil.After(s.Snapshot.At) }

func (s *Store) arrayCheckpoint() []ArrayObservation {
	result := make([]ArrayObservation, 0, len(s.observedArrays))
	for _, observation := range s.observedArrays {
		observation.Spans = append([]TaskSpan(nil), observation.Spans...)
		sort.Slice(observation.Spans, func(i, j int) bool {
			a, b := observation.Spans[i], observation.Spans[j]
			if a.First != b.First {
				return a.First < b.First
			}
			if a.Last != b.Last {
				return a.Last < b.Last
			}
			return a.Step < b.Step
		})
		result = append(result, observation)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.ArrayID != b.ArrayID {
			return a.ArrayID < b.ArrayID
		}
		if a.Cluster != b.Cluster {
			return a.Cluster < b.Cluster
		}
		if a.User != b.User {
			return a.User < b.User
		}
		if !a.Submit.Equal(b.Submit) {
			return a.Submit.Before(b.Submit)
		}
		return a.Restarts < b.Restarts
	})
	return result
}

func validateArrayCheckpoint(c Checkpoint) bool {
	if len(c.Arrays) > maxObservedArrays || c.ArrayTrackingLimitedUntil.After(c.At.Add(Retention)) {
		return false
	}
	total := 0
	seen := map[arrayAttempt]bool{}
	for _, observation := range c.Arrays {
		total += len(observation.Spans)
		if seen[observation.key()] || observation.ArrayID <= 0 || observation.Submit.IsZero() || observation.Restarts < 0 || observation.Cluster == "" || observation.User == "" || len(observation.Cluster) > 256 || len(observation.User) > 256 || observation.At.IsZero() || observation.At.After(c.At) || len(observation.Spans) == 0 || len(observation.Spans) > maxArraySpans || total > maxObservedSpans {
			return false
		}
		seen[observation.key()] = true
		for _, span := range observation.Spans {
			if !span.valid() {
				return false
			}
		}
	}
	return true
}
