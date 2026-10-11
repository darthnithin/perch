package shortcuts

import "testing"

func TestTargetsUseCellsAndExcludeClippedActions(t *testing.T) {
	text := "\x1b[33mq 終了\x1b[m  esc 戻る"
	got := Targets(text, 16)
	if len(got) != 2 || got[0].X != 0 || got[0].Width != 6 || got[1].X != 8 || got[1].Width != 8 {
		t.Fatal("ANSI or wide characters changed hit geometry", got)
	}
	got = Targets(text, 15)
	if len(got) != 1 || got[0].Key != "q" {
		t.Fatal("clipped shortcut remained clickable", got)
	}
	if len(Targets(text, 0)) != 0 || len(Targets("Waiting for Hummingbird…", 40)) != 0 {
		t.Fatal("zero-width or prose acquired a shortcut")
	}
}

func TestDirectionalTargetsStaySeparate(t *testing.T) {
	got := Targets("↑↓ scroll · [ ] step  esc back", 40)
	if len(got) != 5 || got[0].Key != "↑" || got[1].Key != "↓" || got[2].Key != "[" || got[3].Key != "]" || got[4].Key != "esc" {
		t.Fatal(got)
	}
	if got[0].Width != 1 || got[1].X != got[0].X+1 || got[3].X != got[2].X+2 {
		t.Fatal("directions use prose as hit area", got)
	}
}
