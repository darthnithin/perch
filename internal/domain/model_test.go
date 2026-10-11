package domain

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestRangeCount(t *testing.T) {
	for _, tc := range []struct {
		s string
		n int64
	}{{"0-999999", 1000000}, {"[0-99]%4", 100}, {"1-9:2,12,15-18", 10}, {"0", 1}} {
		got, err := RangeCount(tc.s)
		if err != nil || got != tc.n {
			t.Errorf("%q: %d %v", tc.s, got, err)
		}
	}
	for _, s := range []string{"", "-1", "9-1", "1-4:0", "4294967295", "1;exit"} {
		if _, err := RangeCount(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
func TestSanitizeTerminalOutput(t *testing.T) {
	input := "before\x1b[31mred\x1b[0m\x1b]52;c;c2VjcmV0\x07 after\x1bPsecret\x1b\\\n\tα\u202eevil\x00\x7f\u009b2Jend"
	got := Clean(input)
	want := "beforered after\n\tαevilend"
	if got != want {
		t.Fatalf("%q != %q", got, want)
	}
	if got := Clean("safe\x1b]52;unterminated"); got != "safe" {
		t.Fatal(got)
	}
}
func TestEfficiencyScopeAndMissing(t *testing.T) {
	cpu := 490.282971
	j := Job{CPUs: 1, Elapsed: 494 * time.Second, TotalCPU: &cpu}
	e, ok := j.Efficiency()
	if !ok || e < 99.24 || e > 99.25 {
		t.Fatal(e, ok)
	}
	j.CPUs = 0
	if _, ok = j.Efficiency(); ok {
		t.Fatal("zero allocation reported efficiency")
	}
	j = Job{State: "RUNNING"}
	if _, ok = j.Remaining(); ok {
		t.Fatal("missing limit became deadline")
	}
}

func TestEfficiencyRejectsInvalidAndRequeuedDenominators(t *testing.T) {
	for _, cpu := range []float64{-1, math.NaN(), math.Inf(1)} {
		if _, ok := (Job{CPUs: 1, Elapsed: time.Second, TotalCPU: &cpu}).Efficiency(); ok {
			t.Fatal("invalid CPU ratio", cpu)
		}
	}
	cpu := 50.0
	j := Job{CPUs: 1, Elapsed: time.Minute, TotalCPU: &cpu, Restarts: 1}
	if _, ok := j.Efficiency(); ok || j.EfficiencyNote() == "" {
		t.Fatal("requeued ratio lacked qualification")
	}
	j.Restarts = 0
	j.State = "TIMEOUT"
	if _, ok := j.Efficiency(); !ok || !strings.Contains(j.EfficiencyNote(), "child processes") {
		t.Fatal("interrupted accounting missing caveat")
	}
}
func TestDiagnosisEvidence(t *testing.T) {
	if hint := LogHint("safe\x1b]0;CUDA out of memory\a"); hint.Summary != "" {
		t.Fatal("hidden terminal command became log evidence")
	}
	if d := Diagnose(Job{State: "RUNNING"}, "CUDA out of memory"); d.Summary != "" {
		t.Fatal("log alone became failed job")
	}
	d := Diagnose(Job{State: "FAILED"}, "CUDA out of memory")
	if d.Confirmed || !strings.Contains(d.Summary, "Possible") {
		t.Fatal(d)
	}
	d = Diagnose(Job{State: "TIMEOUT"}, "")
	if !d.Confirmed {
		t.Fatal(d)
	}
}

func TestFailedTaskSpecSortsDeduplicatesAndCompresses(t *testing.T) {
	a := ArraySummary{FailedTasks: []int64{42, 3, 17, 3, 5, 4, -1, 4294967294}}
	if got := a.FailedSpec(); got != "--array=3-5,17,42" {
		t.Fatal(got)
	}
	if a.FailedTasks[0] != 42 {
		t.Fatal("copy preparation mutated scheduler data")
	}
}
