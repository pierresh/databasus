package backups_download

import (
	"errors"
	"time"

	"github.com/google/uuid"

	cache_utils "databasus-backend/internal/util/cache"
)

const (
	downloadLockPrefix     = "backup_download_lock:"
	downloadLockTTL        = 5 * time.Second
	downloadLockValue      = "1"
	downloadHeartbeatDelay = 3 * time.Second
)

var ErrDownloadAlreadyInProgress = errors.New("download already in progress for this user")

type DownloadTracker struct {
	cache *cache_utils.CacheUtil[string]
}

func NewDownloadTracker() *DownloadTracker {
	return &DownloadTracker{
		cache: cache_utils.NewCacheUtil[string](downloadLockPrefix),
	}
}

func (t *DownloadTracker) AcquireDownloadLock(userID uuid.UUID) error {
	key := userID.String()

	if t.cache.Get(key) != nil {
		return ErrDownloadAlreadyInProgress
	}

	value := downloadLockValue
	t.cache.SetWithExpiration(key, &value, downloadLockTTL)

	return nil
}

func (t *DownloadTracker) RefreshDownloadLock(userID uuid.UUID) {
	key := userID.String()
	value := downloadLockValue
	t.cache.SetWithExpiration(key, &value, downloadLockTTL)
}

func (t *DownloadTracker) ReleaseDownloadLock(userID uuid.UUID) {
	t.cache.Invalidate(userID.String())
}

func (t *DownloadTracker) IsDownloadInProgress(userID uuid.UUID) bool {
	return t.cache.Get(userID.String()) != nil
}

func GetDownloadHeartbeatInterval() time.Duration {
	return downloadHeartbeatDelay
}
