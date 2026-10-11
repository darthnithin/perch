// Package fixtures embeds deliberately scrubbed Hummingbird responses for demo
// mode. They contain no original job names, usernames, scripts, or output.
package fixtures

import _ "embed"

//go:embed queue.json
var Queue string

//go:embed accounting.json
var Accounting string

//go:embed partitions.json
var Partitions string

//go:embed nodes.json
var Nodes string
