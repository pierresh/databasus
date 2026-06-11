package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing/fstest"

	_ "github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"

	embeddedtools "databasus-backend/embedded_tools"
	"databasus-backend/internal/storage"
	"databasus-backend/migrations"
)

// extractToolsIfNeeded extracts the embedded client tool binaries (mariadb-dump,
// mysqldump, mongodump, etc.) to <exeDir>/assets/tools/win-x64/ on first run,
// and re-extracts whenever the exe itself is newer than the last extraction
// sentinel (i.e. after an update). If the binary was built without embedded
// tools (dev mode), this is a no-op.
func extractToolsIfNeeded(log *slog.Logger) error {
	entries, err := embeddedtools.FS.ReadDir("win-x64")
	if err != nil {
		return nil //nolint:nilerr // empty embed in dev mode — nothing to do
	}

	hasTools := false
	for _, e := range entries {
		if e.IsDir() {
			hasTools = true
			break
		}
	}
	if !hasTools {
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	exeInfo, err := os.Stat(exe)
	if err != nil {
		return fmt.Errorf("stat executable: %w", err)
	}

	targetDir := filepath.Join(filepath.Dir(exe), "assets", "tools", "win-x64")
	sentinelPath := filepath.Join(targetDir, ".extracted")

	if sentinelInfo, statErr := os.Stat(sentinelPath); statErr == nil {
		if !exeInfo.ModTime().After(sentinelInfo.ModTime()) {
			return nil
		}
		log.Info("new executable detected, re-extracting client tools")
	} else {
		log.Info("extracting client tools", "target", targetDir)
	}

	err = fs.WalkDir(embeddedtools.FS, "win-x64", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel := strings.TrimPrefix(path, "win-x64")
		rel = strings.TrimPrefix(rel, "/")
		destPath := filepath.Join(targetDir, filepath.FromSlash(rel))

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		data, readErr := embeddedtools.FS.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		return os.WriteFile(destPath, data, 0o755)
	})
	if err != nil {
		return fmt.Errorf("extract client tools: %w", err)
	}

	if writeErr := os.WriteFile(sentinelPath, []byte{}, 0o644); writeErr != nil {
		return fmt.Errorf("write extraction sentinel: %w", writeErr)
	}

	log.Info("client tools extracted successfully", "target", targetDir)

	return nil
}

func initStandaloneMode(log *slog.Logger) (func(), error) {
	if err := extractToolsIfNeeded(log); err != nil {
		return nil, err
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("could not resolve executable path: %w", err)
	}

	dataDir := filepath.Join(filepath.Dir(exe), "databasus-data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create data dir %s: %w", dataDir, err)
	}

	dbPath := filepath.Join(dataDir, "databasus.db")
	// WAL mode for better concurrency; foreign keys are enforced by default in PG so mirror that.
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", dbPath)

	log.Info("initialising SQLite database", "path", dbPath)

	storage.SetDSNOverride(dsn)

	if err := runSQLiteMigrations(log, dsn); err != nil {
		return nil, err
	}

	return func() {}, nil
}

func runSQLiteMigrations(log *slog.Logger, dsn string) error {
	log.Info("running database migrations")

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("could not open SQLite database: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	sqliteFS, err := buildSQLiteFS()
	if err != nil {
		return fmt.Errorf("could not build SQLite migrations: %w", err)
	}

	goose.SetBaseFS(sqliteFS)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("could not set goose dialect: %w", err)
	}

	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("database migrations failed: %w", err)
	}

	log.Info("database migrations completed")

	return nil
}

// buildSQLiteFS reads the embedded PostgreSQL migration files and returns an
// in-memory FS with each file transformed to SQLite-compatible SQL.
func buildSQLiteFS() (fs.FS, error) {
	entries, err := migrations.Files.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("could not read migrations: %w", err)
	}

	nullableMap := buildNullableMap()

	fsMap := make(fstest.MapFS)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("could not read migration %s: %w", entry.Name(), err)
		}

		transformed := transformPGtoSQLite(string(content))
		transformed = applyNullableToCreateTable(transformed, nullableMap)

		fsMap[entry.Name()] = &fstest.MapFile{
			Data: []byte(transformed),
		}
	}

	return fsMap, nil
}

// buildNullableMap scans all migration Up sections and collects every
// (table, column) pair targeted by ALTER TABLE … ALTER COLUMN … DROP NOT NULL.
// SQLite cannot drop NOT NULL after the fact, so we pre-remove it from the
// CREATE TABLE statement before running migrations.
func buildNullableMap() map[string]map[string]bool {
	nullable := make(map[string]map[string]bool)

	reBlock := regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(\w+)\b[^;]+DROP\s+NOT\s+NULL`)
	reColDrop := regexp.MustCompile(`(?i)ALTER\s+COLUMN\s+(\w+)\s+DROP\s+NOT\s+NULL`)

	entries, _ := migrations.Files.ReadDir(".")
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			continue
		}
		// Strip the Down section so we don't pick up SET NOT NULL reversals.
		text := string(data)
		if i := strings.Index(strings.ToUpper(text), "-- +GOOSE DOWN"); i >= 0 {
			text = text[:i]
		}
		for _, blockMatch := range reBlock.FindAllStringSubmatch(text, -1) {
			tableName := strings.ToLower(blockMatch[1])
			for _, colMatch := range reColDrop.FindAllStringSubmatch(blockMatch[0], -1) {
				colName := strings.ToLower(colMatch[1])
				if nullable[tableName] == nil {
					nullable[tableName] = make(map[string]bool)
				}
				nullable[tableName][colName] = true
			}
		}
	}
	return nullable
}

// applyNullableToCreateTable finds every CREATE TABLE block in sql and removes
// NOT NULL from columns listed in nullable[tableName]. It uses paren-counting
// so it handles nested parens (e.g. SQLite UUID expression defaults) correctly.
func applyNullableToCreateTable(sql string, nullable map[string]map[string]bool) string {
	reHeader := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(`)

	var out strings.Builder
	remaining := sql

	for {
		loc := reHeader.FindStringSubmatchIndex(remaining)
		if loc == nil {
			out.WriteString(remaining)
			break
		}

		tableName := strings.ToLower(remaining[loc[2]:loc[3]])
		out.WriteString(remaining[:loc[0]])

		// The regex ends with \(, so the opening paren is at loc[1]-1.
		depth := 0
		closeIdx := -1
		for i := loc[1] - 1; i < len(remaining); i++ {
			switch remaining[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					closeIdx = i
				}
			}
			if closeIdx >= 0 {
				break
			}
		}

		if closeIdx < 0 {
			out.WriteString(remaining[loc[0]:])
			break
		}

		tableBlock := remaining[loc[0] : closeIdx+1]
		if cols := nullable[tableName]; len(cols) > 0 {
			tableBlock = removeNotNullFromColumns(tableBlock, cols)
		}
		out.WriteString(tableBlock)
		remaining = remaining[closeIdx+1:]
	}

	return out.String()
}

// removeNotNullFromColumns strips " NOT NULL" from specific column definitions
// within a CREATE TABLE block. It matches lazily to stop at the first NOT NULL
// after the column name, leaving any subsequent DEFAULT clause intact.
func removeNotNullFromColumns(tableBlock string, cols map[string]bool) string {
	for col := range cols {
		colRe := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(col) + `\b([^,\n)]+?)\s+NOT\s+NULL`)
		tableBlock = colRe.ReplaceAllString(tableBlock, col+"$1")
	}
	return tableBlock
}

var (
	// gen_random_uuid() — replace with a SQLite-compatible UUID v4 expression
	// regardless of context (DEFAULT clause or SELECT value). In DEFAULT context
	// the result is DEFAULT (...), which SQLite accepts for expression defaults.
	// UUIDs in GORM operations are still generated in Go via the BeforeCreate
	// callback; the SQLite default is only a fallback for raw SQL inserts.
	reGenRandomUUID = regexp.MustCompile(`(?i)\bgen_random_uuid\(\)`)
	// now() / NOW() → CURRENT_TIMESTAMP everywhere (DEFAULT clause and SELECT).
	reDefaultNow = regexp.MustCompile(`(?i)\bnow\(\)`)
	// DEFERRABLE INITIALLY DEFERRED/IMMEDIATE — not supported by SQLite.
	reDeferrable = regexp.MustCompile(`(?i)\s+DEFERRABLE\s+INITIALLY\s+(?:DEFERRED|IMMEDIATE)`)
	// ALTER TABLE … ADD CONSTRAINT … FOREIGN KEY/UNIQUE — not supported by SQLite.
	reAlterAddConstraint = regexp.MustCompile(
		`(?i)ALTER\s+TABLE\s+\S+\s+ADD\s+CONSTRAINT\s+\S+\s+(?:FOREIGN\s+KEY|UNIQUE)[^;]*;`,
	)
	// ALTER TABLE … ALTER COLUMN — not supported by SQLite.
	reAlterColumn = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+\S+\s+ALTER\s+COLUMN[^;]*;`)
	// ALTER TABLE … DROP CONSTRAINT — not supported by SQLite.
	reDropConstraint = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+\S+\s+DROP\s+CONSTRAINT[^;]*;`)
	// PostgreSQL type-cast operator (expr::TYPE) — convert to CAST(expr AS TYPE).
	rePGCast = regexp.MustCompile(`(\w+)::(\w+)`)
	// CREATE INDEX (non-unique) — stripped so DROP COLUMN never fails because SQLite
	// refuses to drop a column while a non-unique index still references it.
	// CREATE UNIQUE INDEX is preserved intentionally for schema correctness.
	reCreateNonUniqueIndex = regexp.MustCompile(`(?im)\bCREATE\s+INDEX\s+[^;]+;`)
	// -- +goose StatementBegin/End — remove so goose falls back to semicolon-based
	// splitting, which lets each transformed statement execute individually.
	reGooseStatementMarker = regexp.MustCompile(`(?im)^--\s*\+goose\s+Statement(?:Begin|End)\s*$`)
	// ADD COLUMN IF NOT EXISTS → ADD COLUMN; SQLite 3.35 does not support IF NOT EXISTS here.
	reAddColumnIfNotExists = regexp.MustCompile(`(?i)\bADD\s+COLUMN\s+IF\s+NOT\s+EXISTS\b`)
	// DROP COLUMN IF EXISTS → DROP COLUMN.
	reDropColumnIfExists = regexp.MustCompile(`(?i)\bDROP\s+COLUMN\s+IF\s+EXISTS\b`)
	// PostgreSQL join-style UPDATE (UPDATE t alias SET … FROM t2 alias WHERE …) — strip;
	// it is always a data migration that is a no-op on a fresh database.
	reUpdateJoin = regexp.MustCompile(`(?im)^UPDATE\s+\w+\s+\w+\s*\nSET\b[^;]+\bFROM\b[^;]+WHERE[^;]+;`)

	// Multi-column ADD COLUMN blocks: ALTER TABLE <name>\n  ADD COLUMN a …,\n  ADD COLUMN b …;
	// SQLite only allows one ADD COLUMN per ALTER TABLE — split into individual statements.
	reMultiAddColumnBlock = regexp.MustCompile(
		`(?m)(ALTER\s+TABLE\s+(\w+)\s*\n)((?:\s+ADD\s+COLUMN\s+[^\n,]+,\s*\n)+\s+ADD\s+COLUMN\s+[^\n]+;)`,
	)
	// Multi-column DROP COLUMN blocks — same constraint applies.
	reMultiDropColumnBlock = regexp.MustCompile(
		`(?m)(ALTER\s+TABLE\s+(\w+)\s*\n)((?:\s+DROP\s+COLUMN\s+[^\n,]+,\s*\n)+\s+DROP\s+COLUMN\s+[^\n]+;)`,
	)
	// Clause extractors used inside the split functions.
	reAddColumnClause  = regexp.MustCompile(`(?i)ADD\s+COLUMN\s+([^\n,;]+)`)
	reDropColumnClause = regexp.MustCompile(`(?i)DROP\s+COLUMN\s+([^\n,;]+)`)
)

// splitMultiAddColumn splits "ALTER TABLE t\n ADD COLUMN a …,\n ADD COLUMN b …;"
// into individual "ALTER TABLE t ADD COLUMN x …;" statements for SQLite.
func splitMultiAddColumn(sql string) string {
	return reMultiAddColumnBlock.ReplaceAllStringFunc(sql, func(match string) string {
		sub := reMultiAddColumnBlock.FindStringSubmatch(match)
		if sub == nil {
			return match
		}
		tableName := sub[2]
		clauses := reAddColumnClause.FindAllStringSubmatch(sub[3], -1)
		stmts := make([]string, 0, len(clauses))
		for _, clause := range clauses {
			stmts = append(stmts, "ALTER TABLE "+tableName+" ADD COLUMN "+strings.TrimSpace(clause[1])+";")
		}
		return strings.Join(stmts, "\n")
	})
}

// splitMultiDropColumn splits "ALTER TABLE t\n DROP COLUMN a,\n DROP COLUMN b;"
// into individual "ALTER TABLE t DROP COLUMN x;" statements for SQLite.
func splitMultiDropColumn(sql string) string {
	return reMultiDropColumnBlock.ReplaceAllStringFunc(sql, func(match string) string {
		sub := reMultiDropColumnBlock.FindStringSubmatch(match)
		if sub == nil {
			return match
		}
		tableName := sub[2]
		clauses := reDropColumnClause.FindAllStringSubmatch(sub[3], -1)
		stmts := make([]string, 0, len(clauses))
		for _, clause := range clauses {
			stmts = append(stmts, "ALTER TABLE "+tableName+" DROP COLUMN "+strings.TrimSpace(clause[1])+";")
		}
		return strings.Join(stmts, "\n")
	})
}

// transformPGtoSQLite applies the minimal set of text replacements needed to
// make PostgreSQL DDL run on SQLite.
func transformPGtoSQLite(sql string) string {
	// Strip goose statement markers so each individual statement executes separately.
	sql = reGooseStatementMarker.ReplaceAllString(sql, "")
	// Normalise IF [NOT] EXISTS variants before splitting multi-column blocks.
	sql = reAddColumnIfNotExists.ReplaceAllString(sql, "ADD COLUMN")
	sql = reDropColumnIfExists.ReplaceAllString(sql, "DROP COLUMN")
	// Split multi-column ALTER TABLE clauses into individual statements.
	sql = splitMultiAddColumn(sql)
	sql = splitMultiDropColumn(sql)
	// Strip PG join-style UPDATE (data migration; no-op on a fresh database).
	sql = reUpdateJoin.ReplaceAllString(sql, "")
	// Replace gen_random_uuid() with a SQLite UUID v4 expression (parens make
	// it valid in both DEFAULT (...) and SELECT contexts).
	sql = reGenRandomUUID.ReplaceAllString(sql,
		"(lower(hex(randomblob(4)))||'-'||lower(hex(randomblob(2)))||'-4'||"+
			"substr(lower(hex(randomblob(2))),2)||'-'||"+
			"substr('89ab',abs(random())%4+1,1)||substr(lower(hex(randomblob(2))),2)||'-'||"+
			"lower(hex(randomblob(6))))")
	// Replace now() with the SQLite equivalent in all contexts.
	sql = reDefaultNow.ReplaceAllString(sql, "CURRENT_TIMESTAMP")
	// Strip unsupported DDL clauses.
	sql = reDeferrable.ReplaceAllString(sql, "")
	sql = reAlterAddConstraint.ReplaceAllString(sql, "")
	sql = reAlterColumn.ReplaceAllString(sql, "")
	sql = reDropConstraint.ReplaceAllString(sql, "")
	sql = reCreateNonUniqueIndex.ReplaceAllString(sql, "")
	// Convert PostgreSQL type-cast operator to SQLite CAST() syntax.
	sql = rePGCast.ReplaceAllString(sql, "CAST($1 AS $2)")
	// Type name aliases.
	sql = strings.ReplaceAll(sql, "TIMESTAMPTZ", "DATETIME")
	sql = strings.ReplaceAll(sql, "timestamptz", "DATETIME")
	sql = strings.ReplaceAll(sql, "DOUBLE PRECISION", "REAL")
	sql = strings.ReplaceAll(sql, "double precision", "REAL")
	return sql
}
