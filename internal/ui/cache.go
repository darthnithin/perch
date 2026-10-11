package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"perch/internal/app"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/snapshotcache"
)

type cacheMsg struct {
	at  time.Time
	err error
}

// The cached store is display-only. Notifications, refresh cadence and event
// reconciliation always use m.store, which starts from live/alert evidence.
func (m *Model) visibleStore() *app.Store {
	if m.cachedStore != nil {
		return m.cachedStore
	}
	return m.store
}
func (m *Model) loadCache() {
	if m.options.Cache == nil || m.options.Demo {
		return
	}
	m.cacheStatus = "Cache enabled · " + m.options.Cache.Path()
	entry, err := m.options.Cache.Load(m.options.Host, m.now)
	if err != nil {
		m.cacheStatus = "Cache unavailable: " + domain.Label(err.Error())
		return
	}
	if entry == nil {
		return
	}
	m.cachedStore = app.NewStore()
	m.cachedStore.Snapshot, m.cachedStore.History = entry.Snapshot, entry.History
	m.cachedScope = entry.Scope
	m.cachedSavedAt = entry.Snapshot.At
	m.cacheStatus = "Saved view · " + entry.Scope.User + " @ " + entry.Scope.Cluster + " · " + m.options.Cache.Path()
}

// A successfully returned username is already enough to reject another user's
// cached view, even if a later capability check fails. Missing fields never
// establish a match and cannot authorize reads.
func (m *Model) rejectChangedIdentity(report domain.Report) {
	differs := func(scope snapshotcache.Scope) bool {
		return report.User != "" && scope.User != "" && report.User != scope.User || report.Cluster != "" && scope.Cluster != "" && report.Cluster != scope.Cluster
	}
	changed := false
	if m.cachedStore != nil && differs(m.cachedScope) {
		m.cachedStore = nil
		m.cacheVerified = false
		m.cacheStatus = "Saved view discarded: signed-in identity changed"
		changed = true
	}
	if differs(m.liveScope) {
		m.store = app.NewStore()
		m.liveScope = snapshotcache.Scope{}
		changed = true
	}
	if changed {
		m.invalidateDetail()
		m.selectedID = ""
		m.waitsAt = time.Time{}
	}
}
func (m *Model) verifyCache(report domain.Report) {
	if m.cachedStore == nil {
		return
	}
	scope := snapshotcache.Scope{Host: m.options.Host, User: report.User, Cluster: report.Cluster}
	if scope != m.cachedScope {
		m.cachedStore = nil
		m.invalidateDetail()
		m.selectedID = ""
		m.waitsAt = time.Time{}
		m.cacheStatus = "Saved view discarded: signed-in identity changed"
		return
	}
	m.cacheVerified = true
}
func (m *Model) updateCacheView(r hummingbird.Refresh) {
	if m.cachedStore == nil || !m.cacheVerified {
		return
	}
	m.cachedStore.Apply(r) // returned events are deliberately ignored
	m.cacheFresh[0] = m.cacheFresh[0] || r.QueueOK
	m.cacheFresh[1] = m.cacheFresh[1] || r.HistoryOK
	m.cacheFresh[2] = m.cacheFresh[2] || r.PartitionsOK
	if m.cacheFresh == [3]bool{true, true, true} {
		m.cachedStore = nil
	}
}
func (m *Model) saveCache() tea.Cmd {
	if m.options.Cache == nil || m.options.Demo || m.cachedStore != nil {
		return nil
	}
	snap := m.store.Snapshot
	if snap.At.IsZero() || snap.HistoryAt.IsZero() || snap.PartitionsAt.IsZero() {
		m.cacheStatus = "Cache waiting for successful jobs, history and partition reads"
		return nil
	}
	scope := snapshotcache.Scope{Host: m.options.Host, User: m.report.User, Cluster: m.report.Cluster}
	entry, err := snapshotcache.Capture(scope, snap, m.store.History)
	if err != nil {
		m.cacheStatus = "Cache not updated: " + domain.Label(err.Error())
		return nil
	}
	sink, parent, now := m.options.Cache, m.notificationContext, time.Now()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, 2*time.Second)
		defer cancel()
		return cacheMsg{at: snap.At, err: sink.Save(ctx, entry, now)}
	}
}
func (m *Model) cachedDetailAllowed(j domain.Job) bool {
	if m.cachedStore == nil {
		return true
	}
	if !m.connected || !m.cacheVerified || m.store.Snapshot.At.IsZero() {
		return false
	}
	for _, fresh := range m.store.Snapshot.Jobs {
		if fresh.ID == j.ID && fresh.AttemptKey() == j.AttemptKey() && fresh.State != "UNKNOWN" {
			return true
		}
	}
	return false
}
func (m *Model) cacheDescription() string {
	if m.options.Cache == nil {
		return "Snapshot cache off · enable with --cache"
	}
	text := m.cacheStatus
	if !strings.Contains(text, m.options.Cache.Path()) {
		text += "\n" + m.options.Cache.Path()
	}
	if m.cachedStore != nil {
		text = fmt.Sprintf("Saved view for %s @ %s · %s\nLive data has not replaced every section. Cached states and measurements may have changed.\n%s", m.cachedScope.User, m.cachedScope.Cluster, m.cachedSavedAt.Local().Format("Jan 2 15:04"), text)
	}
	return text
}
