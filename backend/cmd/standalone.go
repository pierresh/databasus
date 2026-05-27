package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"

	"databasus-backend/internal/storage"
	"databasus-backend/migrations"
)

// initStandaloneMode starts an embedded PostgreSQL server that Databasus uses
// as its own internal metadata store (backup configs, schedules, credentials,
// run history). This is unrelated to which database engines users choose to
// back up — MySQL, MariaDB, MongoDB, etc. are all still supported as backup
// targets regardless of this internal database.
//
// The function injects the embedded-PG DSN into the storage layer and runs all
// schema migrations from the embedded SQL files. It must be called before
// config.GetEnv() is first invoked (which triggers lazy DB initialisation via
// storage.GetDb()). The returned cleanup function stops the embedded server on
// graceful exit.
func initStandaloneMode(log *slog.Logger) (func(), error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("could not resolve executable path: %w", err)
	}

	dataDir := filepath.Join(filepath.Dir(exe), "databasus-data", "postgres")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create postgres data dir %s: %w", dataDir, err)
	}

	log.Info("starting embedded PostgreSQL", "data_dir", dataDir)

	embeddedPG := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Username("databasus").
			Password("databasus").
			Database("databasus").
			DataPath(dataDir).
			Port(54321),
	)

	if err := embeddedPG.Start(); err != nil {
		return nil, fmt.Errorf("failed to start embedded PostgreSQL: %w", err)
	}

	const dsn = "host=localhost port=54321 user=databasus password=databasus dbname=databasus sslmode=disable"

	storage.SetDSNOverride(dsn)

	if err := runEmbeddedMigrations(log, dsn); err != nil {
		_ = embeddedPG.Stop()
		return nil, err
	}

	return func() {
		log.Info("stopping embedded PostgreSQL")
		if err := embeddedPG.Stop(); err != nil {
			log.Error("failed to stop embedded PostgreSQL", "error", err)
		}
	}, nil
}

func runEmbeddedMigrations(log *slog.Logger, dsn string) error {
	log.Info("running database migrations")

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("could not open database for migrations: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrations.Files)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("could not set goose dialect: %w", err)
	}

	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("database migrations failed: %w", err)
	}

	log.Info("database migrations completed")

	return nil
}
