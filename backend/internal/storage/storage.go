package storage

import (
	"os"
	"reflect"
	"sync"

	gormlite "github.com/glebarez/sqlite"
	"github.com/google/uuid"
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

	var (
		database *gorm.DB
		err      error
	)

	gormCfg := &gorm.Config{
		Logger: gormLogger.Default.LogMode(gormLogger.Silent),
	}

	if config.IsStandaloneMode() {
		database, err = gorm.Open(gormlite.Open(dsn), gormCfg)
		if err == nil {
			registerUUIDCallback(database)
		}
	} else {
		database, err = gorm.Open(postgres.Open(dsn), gormCfg)
	}

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

var uuidType = reflect.TypeFor[uuid.UUID]()

// registerUUIDCallback adds a BeforeCreate hook that auto-generates a UUID
// for any primary-key field of type uuid.UUID that is still the zero value.
// This replaces PostgreSQL's gen_random_uuid() default, which SQLite lacks.
func registerUUIDCallback(db *gorm.DB) {
	_ = db.Callback().Create().Before("gorm:create").Register("uuid:auto_generate", func(db *gorm.DB) {
		if db.Statement == nil || db.Statement.Schema == nil {
			return
		}

		for _, field := range db.Statement.Schema.PrimaryFields {
			if field.FieldType != uuidType {
				continue
			}

			if _, isZero := field.ValueOf(db.Statement.Context, db.Statement.ReflectValue); isZero {
				_ = field.Set(db.Statement.Context, db.Statement.ReflectValue, uuid.New())
			}
		}
	})
}
