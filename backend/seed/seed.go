// Package seed embeds local development data. Never run against production.
package seed

import _ "embed"

//go:embed seed.sql
var SQL string
