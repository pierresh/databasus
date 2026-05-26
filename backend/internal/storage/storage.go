package storage

import (
	"os"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"

	"databasus-backend/internal/config"
	"databasus-backend/internal/util/logger"
)

var log = logger.GetLogger()

var db *gorm.DB

var initDb = sync.OnceFunc(loadDbs)

// dsnOverride is set by SetDSNOverride before GetDb() is ever called.
// In standalone mode main() starts embedded Postgres, obtains the DSN, then
// calls SetDSNOverride so that the lazy DB initialisation uses the right DSN.
var dsnOverride string

// SetDSNOverride must be called before GetDb() is first invoked.  It is used
// by standalone mode to supply the embedded-Postgres connection string after
// the embedded server has started, without requiring a DATABASE_DSN env var.
func SetDSNOverride(dsn string) {
	dsnOverride = dsn
}

func GetDb() *gorm.DB {
	initDb()
	return db
}

func loadDbs() {
	LoadMainDb()
}

func LoadMainDb() {
	dsn := dsnOverride
	if dsn == "" {
		dsn = config.GetEnv().DatabaseDsn
	}

	log.Info("connecting to database...")

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		log.Error("error connecting to database", "error", err)
		os.Exit(1)
	}

	sqlDB, err := database.DB()
	if err != nil {
		log.Error("error getting underlying sql.DB", "error", err)
		os.Exit(1)
	}

	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(10)

	db = database

	log.Info("main database connected successfully")
}
