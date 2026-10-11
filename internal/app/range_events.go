package app

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"perch/internal/domain"
)

func observationMembers(o ArrayObservation) string {
	parts := make([]string, 0, len(o.Spans))
	for _, span := range o.Spans {
		if span.First == span.Last {
			parts = append(parts, fmt.Sprint(span.First))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d:%d", span.First, span.Last, span.Step))
		}
	}
	return strings.Join(parts, ",")
}

// Only the previously observed, unresolved portion of a confirmed range can
// generate an event. Removing that portion from the checkpointed ledger also
// prevents later expanded accounting rows from notifying for the same tasks.
func (s *Store) rangeOutcome(j domain.Job, members string, at time.Time) *Event {
	o, ok := s.observedArrays[arrayKey(j)]
	if !ok || members == "" {
		return nil
	}
	observed := observationMembers(o)
	confirmed, err := domain.IntersectRanges(observed, members)
	if err != nil {
		s.arrayTrackingLimitedUntil = at.Add(Retention)
		return nil
	}
	if confirmed == "" {
		return nil
	}
	remaining, err := domain.SubtractRanges(observed, confirmed)
	if err != nil {
		s.arrayTrackingLimitedUntil = at.Add(Retention)
		return nil
	}
	var spans []TaskSpan
	if remaining != "" {
		spans, err = parseSpans(remaining)
		if err != nil {
			s.arrayTrackingLimitedUntil = at.Add(Retention)
			return nil
		}
	}
	count, err := domain.RangeCount(confirmed)
	if err != nil || count <= 0 {
		return nil
	}
	if len(spans) == 0 {
		delete(s.observedArrays, arrayKey(j))
	} else {
		o.Spans = spans
		s.observedArrays[arrayKey(j)] = o
	}
	// Older versions stored one outcome key for an entire range.
	if _, legacy := s.Seen[j.AttemptKey()+"/outcome"]; legacy || !s.initialized {
		return nil
	}
	key := fmt.Sprintf("%s/members/%x/outcome", j.AttemptKey(), sha256.Sum256([]byte(confirmed)))
	if _, seen := s.Seen[key]; seen {
		return nil
	}
	s.Seen[key] = at
	kind := "finished"
	if j.Failed() {
		kind = "failed"
	} else if j.State == "CANCELLED" {
		kind = "cancelled"
	}
	return &Event{Key: key, Kind: kind, JobID: j.ID, Name: j.Name, State: j.State, Attempt: j.AttemptKey(), ArrayID: j.ArrayID, At: at, TaskCount: count}
}
