package cache_utils

import "time"

const (
	DefaultCacheTimeout = 10 * time.Second
	DefaultCacheExpiry  = 10 * time.Minute
	DefaultQueueTimeout = 30 * time.Second
)
