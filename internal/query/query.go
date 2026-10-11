// Package query provides the shared, read-only interface for the CLI and agents.
package query

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"perch/internal/app"
	"perch/internal/demo"
	"perch/internal/domain"
	"perch/internal/hummingbird"
)

type Backend interface {
	Refresh(context.Context, bool, bool) (hummingbird.Refresh, error)
	ReadLog(context.Context, domain.Job, bool, hummingbird.LogCursor) (hummingbird.LogChunk, error)
	Script(context.Context, domain.Job) (string, error)
	LiveResources(context.Context, domain.Job) (hummingbird.LiveSample, error)
}

type Service struct {
	Backend Backend
	Demo    bool
}
type Request struct {
	Command, ID string
	Stderr      bool
}
type Response struct {
	SchemaVersion  int                         `json:"schema_version"`
	Source         string                      `json:"source"`
	At             time.Time                   `json:"at"`
	HistoryDays    int                         `json:"history_days,omitempty"`
	Data           any                         `json:"data"`
	PriorityPolicy *hummingbird.PriorityPolicy `json:"priority_policy,omitempty"`
}

type Text struct {
	JobID          string `json:"job_id"`
	Stream         string `json:"stream,omitempty"`
	Path           string `json:"path,omitempty"`
	Text           string `json:"text"`
	FileSizeBytes  int64  `json:"file_size_bytes,omitempty"`
	TailLimitBytes int    `json:"tail_limit_bytes,omitempty"`
}

type LiveStep struct {
	ID                    string   `json:"id"`
	PeakRSSBytes          *int64   `json:"peak_rss_bytes"`
	AverageTaskCPUSeconds *float64 `json:"average_task_cpu_seconds"`
}

type Resources struct {
	LiveSteps                 []LiveStep `json:"live_steps"`
	Job                       Job        `json:"job"`
	Steps                     []Step     `json:"steps"`
	SampledAt                 *time.Time `json:"sampled_at,omitempty"`
	LivePeakRSSBytes          *int64     `json:"live_peak_rss_bytes"`
	LiveAverageTaskCPUSeconds *float64   `json:"live_average_task_cpu_seconds"`
	LiveScope                 string     `json:"live_scope,omitempty"`
}

var singleID = regexp.MustCompile(`^[1-9][0-9]*(?:_[0-9]+)?$`)

func Validate(r Request) error {
	switch r.Command {
	case "software":
		if len(r.ID) > 160 || strings.ContainsAny(r.ID, "\x00\r\n\t") {
			return fmt.Errorf("software search must be at most 160 characters on one line")
		}
	case "module":
		if !hummingbird.ValidModuleName(r.ID) {
			return fmt.Errorf("module requires an exact name or name/version")
		}
	case "jobs", "history", "partitions", "arrays", "waits", "priority", "nodes":
		if r.ID != "" {
			return fmt.Errorf("%s does not take a job ID", r.Command)
		}
	case "job", "logs", "script", "resources":
		if !singleID.MatchString(r.ID) {
			return fmt.Errorf("%s requires one job ID or array task (for example 9100_11)", r.Command)
		}
	default:
		return fmt.Errorf("unknown query %q", r.Command)
	}
	if r.Stderr && r.Command != "logs" {
		return fmt.Errorf("--stderr is only supported by logs")
	}
	return nil
}

func (s Service) Execute(ctx context.Context, req Request) (Response, error) {
	out := Response{SchemaVersion: 1, Source: "hummingbird"}
	if err := Validate(req); err != nil {
		return out, err
	}
	if req.Command == "nodes" {
		return s.nodes(ctx)
	}
	if req.Command == "priority" {
		return s.priority(ctx)
	}
	if req.Command == "software" || req.Command == "module" {
		return s.software(ctx, req)
	}
	history := req.Command != "jobs" && req.Command != "partitions"
	partitions := req.Command == "partitions"
	var refresh hummingbird.Refresh
	var err error
	if s.Demo {
		out.Source = "demo"
		refresh = demo.Snapshot(time.Now(), 0)
	} else {
		if s.Backend == nil {
			return out, fmt.Errorf("no cluster connection")
		}
		refresh, err = s.Backend.Refresh(ctx, history, partitions)
	}
	if err != nil {
		return out, err
	}
	out.At = refresh.At
	if history {
		out.HistoryDays = 7
	}
	require := func(ok bool, name string) error {
		if ok {
			return nil
		}
		if e := refresh.Errors[name]; e != nil {
			return fmt.Errorf("%s: %w", name, e)
		}
		return fmt.Errorf("%s response unavailable", name)
	}
	switch req.Command {
	case "jobs":
		if err = require(refresh.QueueOK, "jobs"); err != nil {
			return out, err
		}
		domain.SortJobs(refresh.Queue)
		out.Data = Jobs(refresh.Queue)
	case "waits":
		if err = require(refresh.HistoryOK, "history"); err != nil {
			return out, err
		}
		out.Data = Waits(refresh.History, refresh.At)
	case "history":
		if err = require(refresh.HistoryOK, "history"); err != nil {
			return out, err
		}
		domain.SortJobs(refresh.History)
		out.Data = Jobs(refresh.History)
	case "partitions":
		if err = require(refresh.PartitionsOK, "partitions"); err != nil {
			return out, err
		}
		values := make([]Partition, 0, len(refresh.Partitions))
		for _, p := range refresh.Partitions {
			values = append(values, partition(p))
		}
		out.Data = values
	case "arrays":
		if err = require(refresh.QueueOK, "jobs"); err != nil {
			return out, err
		}
		if err = require(refresh.HistoryOK, "history"); err != nil {
			return out, err
		}
		store := app.NewStore()
		store.Apply(refresh)
		values := make([]Array, 0)
		for _, a := range store.Arrays() {
			if a.SummaryError != "" {
				return out, fmt.Errorf("array %d: %s", a.ID, a.SummaryError)
			}
			values = append(values, array(a))
		}
		out.Data = values
	default:
		// Prefer the scheduler's current state; accounting can lag behind it.
		var selected *domain.Job
		if refresh.QueueOK {
			for _, j := range refresh.Queue {
				if j.ID == req.ID {
					copy := j
					selected = &copy
					break
				}
			}
		}
		if selected == nil {
			if err = require(refresh.QueueOK, "jobs"); err != nil {
				return out, err
			}
			if err = require(refresh.HistoryOK, "history"); err != nil {
				return out, err
			}
			for _, j := range refresh.History {
				if j.ID == req.ID {
					copy := j
					if !copy.Terminal() {
						copy.State = "UNKNOWN"
						copy.Reason = "Absent from the queue; accounting has not confirmed an outcome"
					}
					if selected == nil || copy.AttemptAfter(*selected) {
						selected = &copy
					}
				}
			}
		}
		if selected == nil {
			return out, fmt.Errorf("job %s is not in your queue or the last seven days of accounting", req.ID)
		}
		j := *selected
		switch req.Command {
		case "job":
			out.Data = job(j)
		case "logs":
			stream := "stdout"
			if req.Stderr {
				stream = "stderr"
			}
			if s.Demo {
				out.Data = Text{JobID: j.ID, Stream: stream, Text: demo.Log(j), TailLimitBytes: 262144}
				break
			}
			chunk, e := s.Backend.ReadLog(ctx, j, req.Stderr, hummingbird.LogCursor{})
			if e != nil {
				return out, e
			}
			out.Data = Text{JobID: j.ID, Stream: stream, Path: chunk.Path, Text: domain.Clean(chunk.Raw), FileSizeBytes: chunk.Size, TailLimitBytes: 262144}
		case "script":
			text := demo.Script
			if !s.Demo {
				text, err = s.Backend.Script(ctx, j)
				if err != nil {
					return out, err
				}
			}
			out.Data = Text{JobID: j.ID, Text: domain.Clean(text)}
		case "resources":
			resources := Resources{Job: job(j), Steps: steps(j.Steps), LiveSteps: []LiveStep{}}
			if j.Running() {
				if s.Demo {
					resources.LiveScope = "Demo: live measurements unavailable"
				} else {
					sample, e := s.Backend.LiveResources(ctx, j)
					if e != nil {
						return out, e
					}
					resources.SampledAt = &sample.At
					resources.LivePeakRSSBytes = sample.PeakRSS
					resources.LiveAverageTaskCPUSeconds = sample.CPUSeconds
					resources.LiveScope = sample.Scope
					for _, step := range sample.Steps {
						resources.LiveSteps = append(resources.LiveSteps, LiveStep{ID: domain.Label(step.ID), PeakRSSBytes: step.PeakRSS, AverageTaskCPUSeconds: step.CPUSeconds})
					}
				}
			}
			out.Data = resources
		}
	}
	return out, nil
}
