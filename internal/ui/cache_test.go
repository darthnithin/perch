package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/hummingbird"
	"perch/internal/snapshotcache"
)

func cachedModel(t *testing.T) *Model {
	t.Helper()
	now := time.Now()
	at := now.Add(-time.Hour)
	scope := snapshotcache.Scope{Host: "hb", User: "me", Cluster: "hb"}
	active := domain.Job{ID: "10", Name: "saved-running", State: "RUNNING", User: "me", Cluster: "hb", Submit: at.Add(-time.Hour)}
	finished := domain.Job{ID: "9", Name: "saved-finished", State: "COMPLETED", User: "me", Cluster: "hb", Submit: at.Add(-time.Hour)}
	snap := domain.Snapshot{At: at, HistoryAt: at, PartitionsAt: at, Jobs: []domain.Job{active, finished}, Partitions: []domain.Partition{{Name: "saved-partition", CPUs: 100, AllocatedCPUs: 20}}}
	e, err := snapshotcache.Capture(scope, snap, []domain.Job{finished})
	if err != nil {
		t.Fatal(err)
	}
	sink := &snapshotcache.Store{Dir: filepath.Join(t.TempDir(), "cache")}
	if err := sink.Save(context.Background(), e, now); err != nil {
		t.Fatal(err)
	}
	m := New(nil, Options{Host: "hb", Cache: sink, ReducedMotion: true})
	t.Cleanup(m.Close)
	return m
}
func TestCacheIsVisibleButNotLiveTrackingOrDetailAuthority(t *testing.T) {
	m := cachedModel(t)
	if m.cachedStore == nil || !m.store.Snapshot.At.IsZero() || len(m.store.Snapshot.Jobs) != 0 {
		t.Fatal("cache mixed into reconciliation")
	}
	if len(m.jobs()) != 1 || m.jobs()[0].Name != "saved-running" {
		t.Fatal("saved Jobs missing")
	}
	m.tab = 1
	if len(m.jobs()) != 1 || m.jobs()[0].Name != "saved-finished" {
		t.Fatal("saved History missing")
	}
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 24})
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "cached · disconnected") {
		t.Fatal("unlabelled cache", view)
	}
	m.tab = 0
	m.view = "logs"
	if m.fetchDetail() != nil || !strings.Contains(m.detailError, "Saved job") {
		t.Fatal("cached job initiated read")
	}
	if m.copyFailed() != nil || !strings.Contains(m.notice, "cached array") {
		t.Fatal("cache used for failed task copy")
	}
	if m.saveCache() != nil {
		t.Fatal("cache rewrote its own age")
	}
	if m.chipState() != "reconnecting" {
		t.Fatal("cached running state animated as current")
	}
	m.view = "details"
	m.Update(probeMsg{generation: m.generation, report: domain.Report{User: "other", Cluster: "hb"}})
	if m.cachedStore != nil || len(m.jobs()) != 0 || !strings.Contains(m.cacheStatus, "identity changed") {
		t.Fatal("cross-user cache remained visible")
	}
}
func TestCacheReplacedPerSectionWithoutEventsOrLostHistory(t *testing.T) {
	m := cachedModel(t)
	m.connected = true
	m.report = domain.Report{User: "me", Cluster: "hb"}
	m.verifyCache(m.report)
	job := m.jobs()[0]
	old := m.cachedStore.Snapshot.At
	job.Name = "fresh-running"
	apply := func(r hummingbird.Refresh) { m.Update(refreshMsg{generation: m.generation, result: r}) }
	apply(hummingbird.Refresh{At: m.now, QueueOK: true, Queue: []domain.Job{job}})
	if m.cachedStore == nil || m.jobs()[0].Name != "fresh-running" || len(m.store.Events) != 0 {
		t.Fatal("partial refresh or baseline event")
	}
	if !m.cachedDetailAllowed(job) {
		t.Fatal("fresh matching task blocked")
	}
	m.tab = 1
	if len(m.jobs()) != 1 || !m.visibleStore().Snapshot.HistoryAt.Equal(old) || m.cachedDetailAllowed(m.jobs()[0]) {
		t.Fatal("cached history lost or authorized a read")
	}
	finished := m.jobs()[0]
	finished.Name = "fresh-finished"
	apply(hummingbird.Refresh{At: m.now.Add(time.Second), QueueOK: true, Queue: []domain.Job{job}, HistoryOK: true, History: []domain.Job{finished}})
	if m.cachedStore == nil || m.jobs()[0].Name != "fresh-finished" {
		t.Fatal("partial history replacement")
	}
	apply(hummingbird.Refresh{At: m.now.Add(2 * time.Second), QueueOK: true, Queue: []domain.Job{job}, PartitionsOK: true, Partitions: []domain.Partition{{Name: "fresh-partition"}}})
	if m.cachedStore != nil || m.visibleStore() != m.store || strings.Contains(m.statusText(), "cached") {
		t.Fatal("fresh view did not take over")
	}
	if len(m.store.Events) != 0 {
		t.Fatal("cached observations generated events")
	}
}
func TestCacheIgnoredForDifferentHostAndDemo(t *testing.T) {
	m := cachedModel(t)
	for _, options := range []Options{{Host: "other", Cache: m.options.Cache}, {Host: "hb", Cache: m.options.Cache, Demo: true}, {Host: "hb"}} {
		n := New(nil, options)
		if n.cachedStore != nil {
			t.Fatal("wrong scope/demo cache loaded")
		}
		n.Close()
	}
}
func TestVerifiedScopeChangeDropsOldLiveData(t *testing.T) {
	m := cachedModel(t)
	m.cachedStore = nil
	m.liveScope = snapshotcache.Scope{Host: "hb", User: "me", Cluster: "hb"}
	m.store.Apply(hummingbird.Refresh{At: m.now, QueueOK: true, Queue: []domain.Job{{ID: "1", Name: "old-user", State: "RUNNING"}}})
	m.Update(probeMsg{generation: m.generation, report: domain.Report{User: "other", Cluster: "hb"}})
	if len(m.jobs()) != 0 || len(m.store.Events) != 0 {
		t.Fatal("old identity retained after reauthentication")
	}
}

func TestFailedCapabilityProbeStillDiscardsChangedIdentity(t *testing.T) {
	m := cachedModel(t)
	m.Update(probeMsg{generation: m.generation, report: domain.Report{User: "other"}, err: errors.New("scheduler unavailable")})
	if m.cachedStore != nil || len(m.jobs()) != 0 || m.report.User != "other" || m.connected {
		t.Fatal("old cache survived a known user change")
	}
}

func TestCachedJobsDoNotBecomeNewNotificationObservations(t *testing.T) {
	m := cachedModel(t)
	sender := &notificationRecorder{}
	m.options.Notifier = sender
	m.options.StateDir = filepath.Join(t.TempDir(), "notifications")
	m.connected = true
	m.report = domain.Report{User: "me", Cluster: "hb"}
	m.verifyCache(m.report)
	m.startNotifications()
	j := m.jobs()[0]
	j.State = "COMPLETED"
	r := hummingbird.Refresh{At: time.Now(), QueueOK: true, HistoryOK: true, PartitionsOK: true, History: []domain.Job{j}}
	_, cmd := m.Update(refreshMsg{generation: m.generation, result: r})
	runNotificationCommand(m, cmd)
	if len(sender.messages) != 0 || len(m.store.Events) != 0 {
		t.Fatal("cached running job generated a startup completion alert")
	}
}
