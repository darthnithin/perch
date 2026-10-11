package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"perch/internal/domain"
	"perch/internal/hummingbird"
)

func TestLogHintSurvivesInspectorNavigationButNotOtherAttempts(t *testing.T) {
	m := emptyModel(100, 36)
	defer m.Close()
	j := domain.Job{ID: "42", Name: "failed-test", State: "FAILED", User: "researcher", Cluster: "hb", Submit: m.now}
	m.store.Snapshot.Jobs = []domain.Job{j}
	m.tab = 1
	m.normalizeSelection()
	m.setView("logs")
	m.logErr = true
	receiveLog(m, "CUDA out of memory\n", true)
	at := m.logHint.at
	m.now = m.now.Add(time.Minute)
	m.Update(detailMsg{sequence: m.detailSequence, view: "logs", err: errors.New("read denied")})
	if !m.logHint.at.Equal(at) {
		t.Fatal("failed read relabelled old evidence")
	}
	m.setView("inspect")
	if m.logRaw != "" {
		t.Fatal("navigation retained the raw log")
	}
	text := ansi.Strip(m.details(j, 40))
	for _, want := range []string{"Possible GPU memory exhaustion", "not a confirmed", "stderr read"} {
		if !strings.Contains(text, want) {
			t.Fatal(text)
		}
	}
	if d := m.jobDiagnosis(j); d.Confirmed {
		t.Fatal("hint became confirmed")
	}
	for _, state := range []string{"RUNNING", "COMPLETED", "UNKNOWN", "CANCELLED", "TIMEOUT", "OUT_OF_MEMORY", "BOOT_FAIL", "PREEMPTED", "DEADLINE"} {
		changed := j
		changed.State = state
		if strings.Contains(m.jobDiagnosis(changed).Summary, "Possible") {
			t.Fatal("log overrode scheduler", state)
		}
	}
	saved := m.logHint
	for _, change := range []func(*domain.Job){func(j *domain.Job) { j.Restarts++ }, func(j *domain.Job) { j.ID = "43" }, func(j *domain.Job) { j.Submit = j.Submit.Add(time.Hour) }} {
		other := j
		change(&other)
		m.logHint = saved
		m.store.Snapshot.Jobs = []domain.Job{other}
		m.normalizeSelection()
		m.invalidateDetail()
		if m.logHint.attempt != "" || m.jobDiagnosis(other).Summary != "Job failed" {
			t.Fatal("other job/attempt inherited old hint")
		}
	}
}

func TestSuccessfulReadReplacesHintAndLateReadCannotRestoreIt(t *testing.T) {
	m := emptyModel(100, 36)
	defer m.Close()
	j := domain.Job{ID: "42", State: "FAILED", Submit: m.now}
	m.store.Snapshot.Jobs = []domain.Job{j}
	m.tab = 1
	m.normalizeSelection()
	m.setView("logs")
	receiveLog(m, "Traceback (most recent call last)\n", true)
	if m.logHint.hint.Summary == "" {
		t.Fatal("hint missing")
	}
	receiveLog(m, "", true)
	if m.logHint.hint.Summary != "" {
		t.Fatal("truncation kept old hint")
	}
	sequence := m.detailSequence
	m.setView("inspect")
	m.Update(detailMsg{sequence: sequence, view: "logs", chunk: hummingbird.LogChunk{Raw: "CUDA out of memory", Reset: true}})
	if m.logHint.hint.Summary != "" {
		t.Fatal("late read changed evidence")
	}
}
