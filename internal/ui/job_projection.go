package ui

import (
	"time"

	"perch/internal/app"
	"perch/internal/domain"
)

// All slices are read-only projections of one reconciled store revision.
// Geometry, animation frames and log updates do not change task membership.
type jobProjection struct {
	store      *app.Store
	revision   uint64
	sourceHead *domain.Job
	sourceLen  int
	at         time.Time
	all, jobs  []domain.Job
	arrays     []domain.ArraySummary
	filtered   bool
	tab        int
	search     string
	sort       jobSort
}

func (m *Model) ensureJobProjection() {
	store := m.visibleStore()
	source := store.Snapshot.Jobs
	var head *domain.Job
	if len(source) > 0 {
		head = &source[0]
	}
	p := &m.projection
	// The slice identity also notices an explicitly replaced snapshot (for
	// example an empty initial view); in-place production updates use Revision.
	if p.store == store && p.revision == store.Revision() && p.sourceHead == head && p.sourceLen == len(source) && p.at.Equal(store.Snapshot.At) {
		return
	}
	*p = jobProjection{store: store, revision: store.Revision(), sourceHead: head, sourceLen: len(source), at: store.Snapshot.At}
	p.all = append(append([]domain.Job(nil), source...), store.ArrayGaps()...)
	p.arrays = store.Arrays()
}

func (m *Model) arraySummaries() []domain.ArraySummary {
	m.ensureJobProjection()
	return m.projection.arrays
}
