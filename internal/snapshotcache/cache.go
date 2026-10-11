// Package snapshotcache stores an optional, bounded last-known monitoring view.
// Cached rows are presentation data, never notification or scheduler evidence.
package snapshotcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"perch/internal/domain"
)

const MaxBytes = 8 << 20
const Retention = 24 * time.Hour
const maxJobs = 10000

type Scope struct{ Host, User, Cluster string }
type Entry struct {
	Version  int
	Scope    Scope
	Snapshot domain.Snapshot
	History  []domain.Job
}
type Store struct{ Dir string }

func DefaultStore() (*Store, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return &Store{Dir: filepath.Join(dir, "perch", "snapshot")}, nil
}
func (s Store) Path() string { return filepath.Join(s.Dir, "view.json") }

// Capture is an explicit whitelist: no paths, log text, scripts, raw responses,
// capabilities, credentials, steps or notification deduplication enter the file.
// Bounds are whole-view limits, never silent partial task-count truncation.
func Capture(scope Scope, snapshot domain.Snapshot, history []domain.Job) (Entry, error) {
	e := Entry{Version: 1, Scope: scope, Snapshot: domain.Snapshot{At: snapshot.At, HistoryAt: snapshot.HistoryAt, PartitionsAt: snapshot.PartitionsAt, User: scope.User, Cluster: scope.Cluster}}
	if len(snapshot.Jobs) > maxJobs || len(history) > maxJobs || len(snapshot.Partitions) > 128 {
		return Entry{}, fmt.Errorf("view exceeds cache record limit; previous cache retained")
	}
	e.Snapshot.Jobs = jobs(snapshot.Jobs)
	e.History = jobs(history)
	e.Snapshot.Partitions = append([]domain.Partition(nil), snapshot.Partitions...)
	for i := range e.Snapshot.Partitions {
		p := &e.Snapshot.Partitions[i]
		p.Name, p.State, p.Nodes = label(p.Name), label(p.State), label(p.Nodes)
		p.GRES, p.GRESUsed = label(p.GRES), label(p.GRESUsed)
	}
	return e, nil
}
func label(s string) string {
	s = domain.Label(s)
	r := []rune(s)
	if len(r) > 512 {
		return string(r[:512])
	}
	return s
}
func jobs(source []domain.Job) []domain.Job {
	out := make([]domain.Job, len(source))
	for i, j := range source {
		out[i] = domain.Job{QueueObserved: j.QueueObserved, AwaitingAccounting: j.AwaitingAccounting, ID: j.ID, RawID: j.RawID, ArrayID: j.ArrayID, TaskRange: j.TaskRange, Name: label(j.Name), State: label(j.State), Reason: label(j.Reason), Partition: label(j.Partition), Nodes: label(j.Nodes), User: j.User, Cluster: j.Cluster,
			Submit: j.Submit, Eligible: j.Eligible, Start: j.Start, End: j.End, EstimatedStart: j.EstimatedStart, Elapsed: j.Elapsed, Limit: j.Limit, CPUs: j.CPUs, NodeCount: j.NodeCount, Restarts: j.Restarts, RSSScope: label(j.RSSScope)}
		d := &out[i]
		d.TaskID = copyValue(j.TaskID)
		d.RequestedMemory = copyValue(j.RequestedMemory)
		d.AllocatedMemory = copyValue(j.AllocatedMemory)
		d.GPURequests = domain.CopyGPURequests(j.GPURequests)
		d.GPURequestNote = label(j.GPURequestNote)
		d.PeakRSS = copyValue(j.PeakRSS)
		d.TotalCPU = copyValue(j.TotalCPU)
		d.ExitCode = copyValue(j.ExitCode)
		d.Signal = copyValue(j.Signal)
	}
	return out
}
func copyValue[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
func (e Entry) valid(now time.Time) error {
	if e.Version != 1 || e.Scope.Host == "" || e.Scope.User == "" || e.Scope.Cluster == "" || e.Snapshot.User != e.Scope.User || e.Snapshot.Cluster != e.Scope.Cluster {
		return fmt.Errorf("cache identity or version is invalid")
	}
	for _, value := range []string{e.Scope.Host, e.Scope.User, e.Scope.Cluster} {
		if value != domain.Label(value) || len(value) > 512 {
			return fmt.Errorf("cache scope is invalid")
		}
	}
	for _, at := range []time.Time{e.Snapshot.At, e.Snapshot.HistoryAt, e.Snapshot.PartitionsAt} {
		if at.IsZero() || at.After(now) || now.Sub(at) > Retention {
			return fmt.Errorf("cached view is expired or has invalid timestamps")
		}
	}
	if len(e.Snapshot.Jobs) > maxJobs || len(e.History) > maxJobs || len(e.Snapshot.Partitions) > 128 {
		return fmt.Errorf("cache exceeds record limit")
	}
	for _, list := range [][]domain.Job{e.Snapshot.Jobs, e.History} {
		for _, j := range list {
			if len(j.ID) > 16384 || j.ID != domain.Label(j.ID) || len(j.TaskRange) > 16384 || j.TaskRange != domain.Label(j.TaskRange) {
				return fmt.Errorf("cached job identifier is invalid")
			}
			if j.TaskRange != "" {
				if _, err := domain.RangeCount(j.TaskRange); err != nil {
					return fmt.Errorf("cached task range is invalid")
				}
			}
			if j.ID == "" || j.User != "" && j.User != e.Scope.User || j.Cluster != "" && j.Cluster != e.Scope.Cluster {
				return fmt.Errorf("cached job identity does not match its scope")
			}
		}
	}
	return nil
}
func encode(e Entry, now time.Time) ([]byte, error) {
	if err := e.valid(now); err != nil {
		return nil, err
	}
	// Reapply the whitelist at the persistence boundary, including callers that
	// construct Entry directly. Never trust future fields on domain.Job implicitly.
	e, err := Capture(e.Scope, e.Snapshot, e.History)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(e)
	if len(raw) > MaxBytes {
		return nil, fmt.Errorf("view exceeds 8 MiB cache limit; previous cache retained")
	}
	return raw, err
}
