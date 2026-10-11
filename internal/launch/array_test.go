package launch

import (
	"strings"
	"testing"
)

func TestArraySyntaxAndExactResourceCounts(t *testing.T) {
	for _, c := range []struct {
		in, normalized    string
		tasks, concurrent int64
	}{
		{"0-99%4", "0-99%4", 100, 4}, {"1,3,5-15:2", "1,3,5-15:2", 8, 8},
		{"0-8:2,1-9:2%3", "0-8:2,1-9:2%3", 10, 3}, {"002%8", "2%8", 1, 1}, {"0", "0", 1, 1},
	} {
		got, err := ParseArray(c.in)
		if err != nil || got.Expression != c.normalized || got.Tasks != c.tasks || got.Concurrent != c.concurrent {
			t.Fatal(c, got, err)
		}
	}
	for _, value := range []string{"0-1,1", "0-100000", "-1", "0-9%0", "0-9%2%4", "9-1", "1-9:0", "1:2", "1,", " 1", "4294967294", "9999999999999999999999", "0-9%9999999999999999999999", "1;touch /tmp/x", "0-9\n"} {
		if _, err := ParseArray(value); err == nil {
			t.Fatal("accepted invalid array", value)
		}
	}
	s := spec()
	s.Array = "0-99%4"
	s.GPUs = 1
	s.AfterOK = "123,456_2"
	s.WorkDir = "/scratch/100%ready"
	p, err := Build(s)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(p.Arguments, " ")
	for _, v := range []string{"--array=0-99%4", "--dependency=afterok:123:456_2", "--kill-on-invalid-dep=yes", "--cpus-per-task=2", "--mem=512M"} {
		if !strings.Contains(args, v) {
			t.Fatal(args)
		}
	}
	if p.Stdout != "/scratch/100%%ready/perch-%A_%a.out" || p.Stderr != "/scratch/100%%ready/perch-%A_%a.err" {
		t.Fatal(p)
	}
	if !strings.Contains(p.ResourceSummary(), "Concurrent ceiling: 8 CPU · 2048 MiB · 4 GPU") {
		t.Fatal(p.ResourceSummary())
	}
}

func TestDependencyInputIsOnlyIDs(t *testing.T) {
	for _, input := range []string{"afterok:12", "12:13", "12?13", "12_1-5", "-1", "0", "12,", "12\n13", "12;true"} {
		if _, err := Dependencies(input); err == nil {
			t.Fatal(input)
		}
	}
	ids, err := Dependencies("12,12,13_0")
	if err != nil || strings.Join(ids, ",") != "12,13_0" {
		t.Fatal(ids, err)
	}
}
