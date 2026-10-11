// Package fixtures embeds synthetic API-shaped workloads, never live NRP data.
package fixtures

import _ "embed"

//go:embed jobs.json
var Jobs []byte

//go:embed pods.json
var Pods []byte

//go:embed resourcequotas.json
var Quotas []byte
