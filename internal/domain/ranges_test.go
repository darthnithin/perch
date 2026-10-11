package domain

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func TestRangeUnionDistinctTasks(t *testing.T) {
	for _, tc := range []struct {
		expression string
		count      int64
	}{
		{"0-9,5-14", 15}, {"0-9,0-9,5", 10}, {"0-10:2,0-10:3", 8},
		{"0-10:2,1-10:2", 11}, {"[0-9,5-14%4]", 15}, {"[0-9,5-14]%4", 15},
		{"0-4000000000:2,0-4000000000:3", 2666666668},
		{"0-4294967293,1-4294967293:2,4294967293", 4294967294},
		{"1-4294967293:4294967291,2-4294967293:4294967290", 3},
		{"1-9:9223372036854775807", 1}, {"0-0:2,0", 1}, {"7-12:4,11", 2},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			got, err := RangeCount(tc.expression)
			if err != nil || got != tc.count {
				t.Fatalf("got %d, %v; want %d", got, err, tc.count)
			}
		})
	}
}

func TestRangeUnionRejectsMalformedAndReservedIndices(t *testing.T) {
	for _, s := range []string{"4294967294", "4294967295", "0-4294967294", "[[0-9]]", "[0-9", "0-9]", "[0-9]junk", "1:2", "+1", "0-9:+2", "0-9%", "0-9%0", "0-9%-1", "0-9%2%3", "[0-9%2]%3", "0,", "0,,1", "0-9:2:3", "0-9%bad", "0-9%9223372036854775808"} {
		if _, err := RangeCount(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	if _, err := RangeCount(strings.Repeat("0", maxRangeBytes+1)); err == nil {
		t.Fatal("unbounded expression")
	}
}

func TestRangeUnionMatchesEnumeratedOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(5718))
	for iteration := 0; iteration < 3000; iteration++ {
		var parts []string
		seen := map[int64]bool{}
		for part := 0; part < 1+rng.Intn(12); part++ {
			first := int64(rng.Intn(100))
			last := first + int64(rng.Intn(80))
			step := int64(1 + rng.Intn(25))
			parts = append(parts, fmt.Sprintf("%d-%d:%d", first, last, step))
			for n := first; n <= last; n += step {
				seen[n] = true
			}
		}
		expression := strings.Join(parts, ",")
		got, err := RangeCount(expression)
		if err != nil || got != int64(len(seen)) {
			t.Fatalf("%s: got %d, %v; oracle %d", expression, got, err, len(seen))
		}
		// Input order and duplicate terms cannot change the count.
		rng.Shuffle(len(parts), func(i, j int) { parts[i], parts[j] = parts[j], parts[i] })
		parts = append(parts, parts[0])
		if reordered, err := RangeCount(strings.Join(parts, ",")); err != nil || reordered != got {
			t.Fatalf("ordering or duplicate changed %s: %d %v", expression, reordered, err)
		}
	}
}

func TestProgressionIntersectionAtNumericLimits(t *testing.T) {
	// Large relatively-prime strides have an LCM above MaxInt64. Their one
	// common point must survive without overflowing the intermediate arithmetic.
	a := taskProgression{1, 4294967292, 4294967291}
	b := taskProgression{2, 4294967292, 4294967290}
	intersection, ok := intersectProgressions(a, b)
	if !ok || intersection != (taskProgression{4294967292, 4294967292, 1}) {
		t.Fatal(intersection, ok)
	}
	if _, ok := intersectProgressions(taskProgression{0, 10, 2}, taskProgression{1, 9, 2}); ok {
		t.Fatal("disjoint residues overlap")
	}
}

func TestRangeCountingWorkIsBoundedAndDisjointRangesStayCheap(t *testing.T) {
	spans, err := parseTaskProgressions("0-99999:2,0-99999:3,0-99999:5,0-99999:7")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := countProgressions(spans, 1); err == nil {
		t.Fatal("overlap work budget ignored")
	}
	// A canonical fragmented bitmap has disjoint intervals. Its count should
	// not require pairwise intersections or expanding the whole index space.
	var parts []string
	for i := 0; i < 20000; i++ {
		parts = append(parts, fmt.Sprint(i*100000))
	}
	spans, err = parseTaskProgressions(strings.Join(parts, ","))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := countProgressions(spans, 0); err != nil || got != 20000 {
		t.Fatalf("disjoint ranges need overlap work: %d %v", got, err)
	}
}

func FuzzRangeCount(f *testing.F) {
	for _, s := range []string{"0-9,5-14", "0-4000000000:2,0-4000000000:3", "[0-9]%4", "4294967294", "0-1:9223372036854775807"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n, err := RangeCount(s)
		if err == nil && (n < 1 || n > MaxTaskIndex+1) {
			t.Fatalf("invalid successful count %d for %q", n, s)
		}
	})
}

func BenchmarkRangeUnionBillionTasks(b *testing.B) {
	for b.Loop() {
		if _, err := RangeCount("0-4000000000:2,0-4000000000:3"); err != nil {
			b.Fatal(err)
		}
	}
}

func TestInvalidCollapsedRangeDoesNotInventOneTask(t *testing.T) {
	if got := (Job{TaskRange: "invalid"}).Count(); got != 0 {
		t.Fatalf("invented %d tasks", got)
	}
}

func TestRangeContainsWithoutExpansion(t *testing.T) {
	for _, tc := range []struct {
		expression string
		task       int64
		want       bool
	}{
		{"[0-4000000000:2]%4", 4000000000, true},
		{"[0-4000000000:2]%4", 3999999999, false},
		{"0-3,7,10-20:3", 7, true},
		{"0-3,7,10-20:3", 19, true},
		{"0-3,7,10-20:3", 20, false},
		{"0-10", -1, false},
	} {
		got, err := RangeContains(tc.expression, tc.task)
		if err != nil || got != tc.want {
			t.Fatal(tc, got, err)
		}
	}
	if _, err := RangeContains("0-3%garbage", 0); err == nil {
		t.Fatal("accepted malformed throttle")
	}
}
