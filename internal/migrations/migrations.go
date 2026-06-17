// Package migrations embeds the kacho-geo goose SQL migrations (schema kacho_geo).
// Source of truth — this directory. Never edit an applied migration; add a new one.
package migrations

import "embed"

// FS — embedded kacho-geo migrations (goose format).
//
//go:embed *.sql
var FS embed.FS
