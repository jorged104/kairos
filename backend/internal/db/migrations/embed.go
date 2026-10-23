// Package migrations embeds the Goose migrations so tests can apply them.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
