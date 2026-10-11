package hummingbird

import (
	"strings"
	"testing"
)

func TestAccountingBitmap(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"0x2", "1"}, {"0x3", "0-1"}, {"0x32", "1,4-5"}, {"0x18001", "0,15-16"},
		{"0xFFFF", "0-15"}, {"0x0000a", "1,3"}, {"0x0", ""}, {"1-5:2", "1-5:2"}, {"", ""},
	} {
		got, err := accountingTaskRange(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("%s: %q %v, want %q", tc.in, got, err, tc.want)
		}
	}
	for _, value := range []string{"0x", "0xgg", "0x1 2", "0x-1", "0x" + strings.Repeat("f", (1<<20)+1)} {
		if _, err := accountingTaskRange(value); err == nil {
			t.Errorf("accepted invalid bitmap %q", value[:min(len(value), 20)])
		}
	}
	jobs, err := ParseAccounting(fixture(t, "accounting-bitmap"))
	if err != nil || len(jobs) != 1 {
		t.Fatalf("%v %+v", err, jobs)
	}
	j := jobs[0]
	if j.ID != "9400_[1]" || j.TaskRange != "1" || j.TaskID != nil || j.Count() != 1 || j.State != "CANCELLED" {
		t.Fatalf("bitmap did not retain collapsed task membership: %+v", j)
	}
}

func TestAccountingRequestsRequeueDuplicates(t *testing.T) {
	args := strings.Join(accountingArgs("researcher", "now-7days"), " ")
	for _, required := range []string{"--duplicates", "--user=researcher", "--starttime=now-7days", "--endtime=now", "--state="} {
		if !strings.Contains(args, required) {
			t.Fatalf("missing %s in %s", required, args)
		}
	}
}
