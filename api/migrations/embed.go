// Package migrations embeds the goose SQL files so the binary can migrate without files on disk.
package migrations

import "embed"

// FS holds every forward-only migration.
//
//go:embed *.sql
var FS embed.FS
