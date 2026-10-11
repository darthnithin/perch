package ui

import (
	"perch/internal/domain"
	"time"
)

// Retain one derived hint, not a second log cache. It survives navigation only
// within the selected attempt, and never persists to disk.
type observedLogHint struct {
	attempt, stream string
	at              time.Time
	hint            domain.Diagnosis
}

func (m *Model) captureLogHint() {
	j, ok := m.selected()
	if !ok {
		m.logHint = observedLogHint{}
		return
	}
	stream := "stdout"
	if m.logErr {
		stream = "stderr"
	}
	m.logHint = observedLogHint{j.AttemptKey(), stream, m.now, domain.LogHint(m.logRaw)}
}

func (m *Model) jobDiagnosis(j domain.Job) domain.Diagnosis {
	d := domain.Diagnose(j, "")
	// Specific scheduler outcomes take precedence over text in a log.
	if j.State != "FAILED" || m.logHint.attempt != j.AttemptKey() || m.logHint.hint.Summary == "" {
		return d
	}
	d = m.logHint.hint
	d.Evidence = "Log hint, not a confirmed cause. " + d.Evidence +
		" · " + m.logHint.stream + " read " + m.logHint.at.Local().Format("02 Jan 15:04")
	return d
}
