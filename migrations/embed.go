// Package migrations holds the SQL files applied at startup, in file-name order.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
