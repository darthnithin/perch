package bird

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestOctantGlyphsAreDistinctSingleCells(t *testing.T) {
	seen := map[rune]bool{}
	for mask, glyph := range octantGlyphs {
		if seen[glyph] {
			t.Fatalf("mask %d duplicates glyph %q", mask, glyph)
		}
		seen[glyph] = true
		if ansi.StringWidth(string(glyph)) != 1 {
			t.Fatalf("mask %d is not one layout cell", mask)
		}
	}
}
func TestOctantAnimationFootprintAndReducedMotion(t *testing.T) {
	for state := range posters {
		unique := map[string]bool{}
		static := strings.Join(OctantFrame(state, 0, true), "\n")
		for frame := 0; frame < 64; frame++ {
			rows := OctantFrame(state, frame, false)
			if len(rows) != 7 {
				t.Fatalf("%s has %d rows", state, len(rows))
			}
			for _, row := range rows {
				if ansi.StringWidth(row) != 15 {
					t.Fatalf("%s frame %d width %d", state, frame, ansi.StringWidth(row))
				}
				if strings.Contains(row, "[48;") {
					t.Fatal("cell background returned")
				}
			}
			unique[strings.Join(rows, "\n")] = true
			if strings.Join(OctantFrame(state, frame, true), "\n") != static {
				t.Fatalf("%s moves with reduced motion", state)
			}
		}
		if len(unique) < 2 {
			t.Fatalf("%s never moves", state)
		}
	}
}
func TestOctantCacheCannotBeMutatedByCaller(t *testing.T) {
	rows := OctantFrame("idle", 0, false)
	first := rows[0]
	rows[0] = "bad"
	if OctantFrame("idle", 0, false)[0] != first {
		t.Fatal("shared cache mutated")
	}
	if strings.Join(OctantFrame("unknown", -1, false), "\n") != strings.Join(OctantFrame("idle", 63, false), "\n") {
		t.Fatal("fallback/wrapping failed")
	}
}
