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
	"perch/internal/launch"
)

func TestReceiptsSurviveRestartAndStaySeparateFromLiveJobs(t *testing.T) {
	j := &launch.Journal{Dir: filepath.Join(t.TempDir(), "receipts")}
	at := time.Now()
	s := launch.Submission{Destination: "user@cluster", Name: "saved-job", Started: at, Receipt: launch.Receipt{At: at, Token: "perch-000000000000000000000001", JobID: "42"}}
	if err := j.Save(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{120, 40}, {80, 24}, {48, 18}} {
		m := demoModel(size[0], size[1])
		m.options.Demo = false
		m.options.Host = "user@cluster"
		m.options.Journal = &launch.Journal{Dir: j.Dir}
		cmd := m.showReceipts()
		m.Update(cmd())
		if m.view != "receipts" || !strings.Contains(m.receiptsBody, "saved-job") || !strings.Contains(m.receiptsBody, "accepted · #42") {
			t.Fatal(m.view, m.receiptsBody)
		}
		if strings.Contains(m.receiptsBody, "running") || strings.Contains(m.receiptsBody, "completed") {
			t.Fatal("receipt invented job outcome")
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("overflow", size)
			}
		}
		if size[1] == 18 {
			wheel(m, 5, m.layout().bodyTop+1, tea.MouseWheelDown)
			if m.viewport.AtTop() {
				t.Fatal("receipt wheel ignored first content row")
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
		if !m.viewport.AtBottom() {
			t.Fatal("receipt scrolling")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.view == "receipts" {
			t.Fatal("escape failed")
		}
		if m.viewport.Height() != max(1, size[1]-16) {
			t.Fatal("receipt viewport height leaked into other views")
		}
		m.Close()
	}
}

func TestSubmissionSaveErrorsDoNotHideAcceptedJobOrInventAttempt(t *testing.T) {
	for _, id := range []string{"", "42"} {
		m := demoModel(120, 40)
		command(m, "/run")
		f := m.launchForm
		f.stage = "submitting"
		r := launch.Receipt{}
		if id != "" {
			r.JobID, r.Token = id, "perch-000000000000000000000001"
		}
		m.receiveLaunch(launchMsg{sequence: f.sequence, submit: true, receipt: r, err: errors.New("disk failure")})
		want := "review"
		if id != "" {
			want = "submitted"
		}
		if f.stage != want || f.problem != "disk failure" || f.receipt.JobID != id {
			t.Fatal(f.stage, f.problem, f.receipt)
		}
	}
}
