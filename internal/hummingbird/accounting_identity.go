package hummingbird

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"perch/internal/domain"
)

var errAccountingIdentityConflict = errors.New("ambiguous accounting identity")

// Consumers choose the newest accounting attempt only when submission/restart
// evidence establishes an order. Validate ambiguous identities before publishing
// the component, so historical logs, scripts and outcomes cannot depend on row
// order. Scope and attempt keys are structs, not delimiter-concatenated labels.
func validateAccountingIdentities(jobs []domain.Job) error {
	conflicts := domain.ArraySubmissionConflicts(jobs)
	if len(conflicts) > 0 {
		ids := make([]int64, 0, len(conflicts))
		for id := range conflicts {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return fmt.Errorf("%w for array %d: %s", errAccountingIdentityConflict, ids[0], conflicts[ids[0]])
	}
	type scope struct{ cluster, user, id string }
	type attempt struct {
		scope
		submit   time.Time
		restarts int
	}
	type times struct{ known, missing bool }
	submissions := make(map[scope]times)
	allocations := make(map[attempt]int64)
	for _, j := range jobs {
		s := scope{j.Cluster, j.User, j.ID}
		t := submissions[s]
		if j.Submit.IsZero() {
			t.missing = true
		} else {
			t.known = true
		}
		if t.known && t.missing {
			return fmt.Errorf("%w for job %s: submission time is missing on a duplicate record", errAccountingIdentityConflict, j.ID)
		}
		submissions[s] = t
		a := attempt{s, j.Submit, j.Restarts}
		if previous, ok := allocations[a]; ok && previous != j.RawID {
			return fmt.Errorf("%w for job %s: one attempt has different allocation IDs", errAccountingIdentityConflict, j.ID)
		}
		allocations[a] = j.RawID
	}
	return nil
}
