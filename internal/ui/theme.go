package ui

import "strings"

// Views are composed with the semantic warm palette, then translated as a
// whole. This keeps themes local to each model, including nested Bubbles and
// chart styles, without mutating package-global styles between instances.
var graphiteColors = strings.NewReplacer(
	"2;236;230;223m", "2;231;233;238m", // text
	"2;163;155;147m", "2;157;166;181m", // secondary
	"2;230;173;143m", "2;238;183;153m", // peach
	"2;168;202;154m", "2;166;210;165m", // success
	"2;223;192;131m", "2;230;197;145m", // waiting
	"2;230;157;145m", "2;234;158;152m", // failure
	"2;87;81;75m", "2;79;88;98m", // dividers
	"2;255;240;229m", "2;255;240;229m", // selected text
	"2;72;56;47m", "2;70;59;54m", // selected row
)

func (m *Model) applyTheme(content string) string {
	if m.options.Theme == "graphite" {
		return graphiteColors.Replace(content)
	}
	return content
}

func (m *Model) canvasColors() (string, string) {
	if m.options.Theme == "graphite" {
		return "#181C20", "#E7E9EE"
	}
	return background, "#ECE6DF"
}
