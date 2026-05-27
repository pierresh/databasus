package migrations

import "embed"

// Files holds all SQL migration files, embedded at build time.
//
//go:embed *.sql
var Files embed.FS
