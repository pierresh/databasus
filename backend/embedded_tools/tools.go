package embeddedtools

import "embed"

// FS holds the win-x64 client tool binaries (mariadb-dump, mysqldump, mongodump, etc.)
// and their required DLLs. The directory is populated at build time by the
// build-windows Makefile target; at dev time only the placeholder .gitkeep is present.
//
//go:embed all:win-x64
var FS embed.FS
