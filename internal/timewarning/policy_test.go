package timewarning

import (
	"perch/internal/domain"
	"testing"
	"time"
)

func TestParseAndFiniteMeasuredTime(t *testing.T) {
	for _, bad := range []string{"", "-1m", "0m", "1s", "90s", "25h", "1m,1m", "1m,2m,3m,4m,5m,6m", "off,5m", "NaN"} {
		if _, err := Parse(bad); err == nil {
			t.Fatal("accepted", bad)
		}
	}
	p, err := Parse("1m, 1h,5m")
	if err != nil || p.String() != "60m,5m,1m" {
		t.Fatal(p, err)
	}
	j := domain.Job{State: "RUNNING", Limit: 2 * time.Hour, Elapsed: 119 * time.Minute}
	if len(p.Crossed(j)) != 3 {
		t.Fatal(p.Crossed(j))
	}
	for _, mutate := range []func(*domain.Job){func(j *domain.Job) { j.Limit = 0 }, func(j *domain.Job) { j.Elapsed = -1 }, func(j *domain.Job) { j.State = "PENDING" }, func(j *domain.Job) { j.State = "UNKNOWN" }, func(j *domain.Job) { j.State = "COMPLETED" }} {
		copy := j
		mutate(&copy)
		if p.Near(copy) {
			t.Fatal("unsupported measurement warns", copy)
		}
	}
	j.Limit, j.Elapsed = 90*time.Second, 80*time.Second
	if p.Near(j) {
		t.Fatal("threshold too large for short job")
	}
	off, _ := Parse("off")
	j.Limit, j.Elapsed = 2*time.Hour, 2*time.Hour
	if off.Near(j) || off.String() != "off" {
		t.Fatal("off warns")
	}
	var defaults Policy
	if defaults.String() != "5m" || !defaults.Near(j) {
		t.Fatal("default changed")
	}
}
