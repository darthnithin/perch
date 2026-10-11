package bird

import "image/color"

// Perched keeps the idle placement API; terminal artwork has no cell backgrounds
// and therefore does not depend on the canvas color.
func Perched(_ color.Color) []string { return OctantFrame("idle", 0, true) }
