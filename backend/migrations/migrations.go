// Package migrations embeds the SQL migrations. Files are named
// <version>_<snake_case>.sql, applied in lexical order and never edited once
// committed: every schema change is a new file.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
