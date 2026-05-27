package cache_utils

import (
	"sync"
	"time"
)

// resettable is implemented by every CacheUtil instance so ClearAllCache can
// wipe all in-process caches in a single call (the same effect as the previous
// Redis FLUSHALL).
type resettable interface {
	clearAll()
}

var (
	globalCachesMu sync.Mutex
	globalCaches   []resettable
)

func registerCache(c resettable) {
	globalCachesMu.Lock()
	globalCaches = append(globalCaches, c)
	globalCachesMu.Unlock()
}

func clearAllRegisteredCaches() {
	globalCachesMu.Lock()
	caches := make([]resettable, len(globalCaches))
	copy(caches, globalCaches)
	globalCachesMu.Unlock()

	for _, c := range caches {
		c.clearAll()
	}
}

// -----------------------------------------------------------------------

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

// CacheUtil is a generic in-process key-value store with per-entry TTL.
// It replaces the previous Valkey-backed implementation; the public API
// (Get/Set/SetWithExpiration/SetIfAbsent/GetAndDelete/Invalidate) is unchanged.
type CacheUtil[T any] struct {
	mu      sync.RWMutex
	data    map[string]cacheEntry[T]
	prefix  string
	timeout time.Duration // kept for API symmetry
	expiry  time.Duration
}

func NewCacheUtil[T any](prefix string) *CacheUtil[T] {
	c := &CacheUtil[T]{
		data:    make(map[string]cacheEntry[T]),
		prefix:  prefix,
		timeout: DefaultCacheTimeout,
		expiry:  DefaultCacheExpiry,
	}
	registerCache(c)
	return c
}

func (c *CacheUtil[T]) Get(key string) *T {
	fullKey := c.prefix + key

	c.mu.RLock()
	entry, ok := c.data[fullKey]
	c.mu.RUnlock()

	if !ok {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.data, fullKey)
		c.mu.Unlock()
		return nil
	}

	value := entry.value
	return &value
}

func (c *CacheUtil[T]) Set(key string, item *T) {
	c.SetWithExpiration(key, item, c.expiry)
}

func (c *CacheUtil[T]) SetWithExpiration(key string, item *T, expiry time.Duration) {
	fullKey := c.prefix + key

	c.mu.Lock()
	c.data[fullKey] = cacheEntry[T]{
		value:     *item,
		expiresAt: time.Now().Add(expiry),
	}
	c.mu.Unlock()
}

// SetIfAbsent sets key to value only when no live entry exists for that key.
// It returns true if the value was stored (key was absent or expired), false if
// an existing live entry was found. The check and store are performed under the
// write lock so there is no TOCTOU race between callers.
func (c *CacheUtil[T]) SetIfAbsent(key string, item *T, expiry time.Duration) bool {
	fullKey := c.prefix + key
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.data[fullKey]; ok && now.Before(entry.expiresAt) {
		return false
	}

	c.data[fullKey] = cacheEntry[T]{
		value:     *item,
		expiresAt: now.Add(expiry),
	}
	return true
}

func (c *CacheUtil[T]) GetAndDelete(key string) *T {
	fullKey := c.prefix + key

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.data[fullKey]
	if !ok {
		return nil
	}

	delete(c.data, fullKey)

	if time.Now().After(entry.expiresAt) {
		return nil
	}

	value := entry.value
	return &value
}

func (c *CacheUtil[T]) Invalidate(key string) {
	c.mu.Lock()
	delete(c.data, c.prefix+key)
	c.mu.Unlock()
}

// ClearAll removes all entries. Used by tests to reset state between runs.
func (c *CacheUtil[T]) ClearAll() {
	c.clearAll()
}

// clearAll is the unexported implementation used by the resettable interface.
func (c *CacheUtil[T]) clearAll() {
	c.mu.Lock()
	c.data = make(map[string]cacheEntry[T])
	c.mu.Unlock()
}
