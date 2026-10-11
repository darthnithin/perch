package app

import "perch/internal/domain"

func staleAccountingOverlaps(j domain.Job, history []domain.Job) bool {
	for _, h := range history {
		if !h.Terminal() {
			continue
		}
		if j.ID == h.ID && j.AttemptKey() == h.AttemptKey() {
			return true
		}
		if j.ArrayID == 0 || arrayKey(j) != arrayKey(h) {
			continue
		}
		if j.TaskRange != "" && h.TaskID != nil && inRange(*h.TaskID, j.TaskRange) {
			return true
		}
		if j.TaskID != nil && h.TaskRange != "" && inRange(*j.TaskID, h.TaskRange) {
			return true
		}
		if j.TaskRange != "" && h.TaskRange != "" {
			members, err := domain.IntersectRanges(j.TaskRange, h.TaskRange)
			// An unprovable intersection must not release a cached outcome.
			if err != nil || members != "" {
				return true
			}
		}
	}
	return false
}

// Queue disappearance is newer evidence than retained accounting. Keep the
// observed identity and compact membership, but do not invent its outcome.
func awaitingAccounting(j domain.Job) domain.Job {
	j.QueueObserved = false
	j.AwaitingAccounting = true
	j.State = "UNKNOWN"
	j.Reason = "No longer in the queue; awaiting a fresh accounting result"
	return j
}
