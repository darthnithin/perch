package ui

import (
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestThemesPreserveContentAndAreModelLocal(t *testing.T) {
	m := demoModel(160, 44)
	warm := m.View().Content
	m.options.Theme = "graphite"
	cool := m.View().Content
	if warm == cool || ansi.Strip(warm) != ansi.Strip(cool) {
		t.Fatal("theme must change colors without changing content")
	}
	if got := color.RGBAModel.Convert(m.View().BackgroundColor); got != (color.RGBA{24, 28, 32, 255}) {
		t.Fatalf("wrong graphite canvas: %v", got)
	}
	if strings.Contains(cool, "48;2;24;28;32m") || strings.Contains(warm, "48;2;28;27;26m") {
		t.Fatal("canvas must not be repainted as per-cell backgrounds")
	}
	m.options.Theme = "warm"
	if m.View().Content != warm {
		t.Fatal("theme change leaked into shared styles")
	}
	m.options.Theme, m.options.NoColor = "graphite", true
	if strings.Contains(m.View().Content, "\x1b") {
		t.Fatal("theme ignored no-color preference")
	}
}
