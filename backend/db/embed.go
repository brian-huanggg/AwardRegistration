// Package db embeds the schema migrations (golang-migrate file format:
// NNNNNN_name.up.sql). Applied by cmd/migrate and by integration tests.
package db

import "embed"

// Migrations holds migration/*.up.sql.
//
//go:embed migration/*.sql
var Migrations embed.FS
