package domain

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func TestRangeSetOperationsMatchIndependentMembership(t *testing.T) {
	rng := rand.New(rand.NewSource(82155))
	for trial := 0; trial < 500; trial++ {
		makeRange := func() (string, map[int64]bool) {
			first, step := int64(rng.Intn(50)), int64(rng.Intn(6)+1)
			last := first + int64(rng.Intn(20))*step
			set := map[int64]bool{}
			for i := first; i <= last; i += step {
				set[i] = true
			}
			return fmt.Sprintf("%d-%d:%d", first, last, step), set
		}
		a, x := makeRange()
		b, y := makeRange()
		for _, difference := range []bool{false, true} {
			want := map[int64]bool{}
			for n := range x {
				if y[n] != difference {
					want[n] = true
				}
			}
			var got string
			var err error
			if difference {
				got, err = SubtractRanges(a, b)
			} else {
				got, err = IntersectRanges(a, b)
			}
			if err != nil {
				t.Fatal(err)
			}
			actual := map[int64]bool{}
			if got != "" {
				spans, e := parseTaskProgressions(got)
				if e != nil {
					t.Fatal(e)
				}
				actual = enumerateSpans(spans)
			}
			if !reflect.DeepEqual(actual, want) {
				t.Fatalf("%s, %s, difference%t: %s", a, b, difference, got)
			}
		}
	}
}

func TestRangeSetOperationsStayCompactAndFailWithoutPartialData(t *testing.T) {
	got, err := SubtractRanges("0-4000000000", "0-4000000000:2")
	if err != nil || got != "1-3999999999:2" {
		t.Fatal(got, err)
	}
	got, err = IntersectRanges("0-4000000000:2", "1-3999999999:2")
	if err != nil || got != "" {
		t.Fatal(got, err)
	}
	for _, fn := range []func(string, string) (string, error){SubtractRanges, IntersectRanges} {
		if got, err := fn("0-10", "invalid"); err == nil || got != "" {
			t.Fatal("invalid range returned data", got, err)
		}
	}
}
