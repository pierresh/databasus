package restoring

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	backups_services "databasus-backend/internal/features/backups/backups/services"
	backups_config "databasus-backend/internal/features/backups/config"
	"databasus-backend/internal/features/databases"
	restores_core "databasus-backend/internal/features/restores/core"
	"databasus-backend/internal/features/restores/usecases"
	"databasus-backend/internal/features/storages"
	tasks_cancellation "databasus-backend/internal/features/tasks/cancellation"
	cache_utils "databasus-backend/internal/util/cache"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
)

var restoreRepository = &restores_core.RestoreRepository{}

var restoreNodesRegistry = &RestoreNodesRegistry{
	nodesMu:           sync.RWMutex{},
	nodes:             make(map[uuid.UUID]RestoreNode),
	countersMu:        sync.RWMutex{},
	counters:          make(map[uuid.UUID]*atomic.Int64),
	pubsubRestores:    cache_utils.NewPubSubManager(),
	pubsubCompletions: cache_utils.NewPubSubManager(),
	logger:            logger.GetLogger(),
}

var restoreCancelManager = tasks_cancellation.GetTaskCancelManager()

var restorerNode = &RestorerNode{
	uuid.New(),
	databases.GetDatabaseService(),
	backups_services.GetBackupService(),
	encryption.GetFieldEncryptor(),
	restoreRepository,
	backups_config.GetBackupConfigService(),
	storages.GetStorageService(),
	restoreNodesRegistry,
	logger.GetLogger(),
	usecases.GetRestoreBackupUsecase(),
	restoreDatabaseCache,
	restoreCancelManager,
	time.Time{},
	atomic.Bool{},
}

var restoresScheduler = &RestoresScheduler{
	restoreRepository,
	backups_services.GetBackupService(),
	storages.GetStorageService(),
	backups_config.GetBackupConfigService(),
	restoreNodesRegistry,
	time.Now().UTC(),
	logger.GetLogger(),
	make(map[uuid.UUID]RestoreToNodeRelation),
	restorerNode,
	restoreDatabaseCache,
	uuid.Nil,
	atomic.Bool{},
}

func GetRestoresScheduler() *RestoresScheduler {
	return restoresScheduler
}

func GetRestorerNode() *RestorerNode {
	return restorerNode
}

func GetRestoreNodesRegistry() *RestoreNodesRegistry {
	return restoreNodesRegistry
}
