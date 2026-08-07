// Package migrations embeds the plain-SQL migration files so any binary can
// apply them without shipping the repo alongside it. Files are applied once
// each, in filename order — keep the NNNN_ prefix monotonic.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
