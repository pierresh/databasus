package cache_utils

import (
	"sync"
	"time"
)

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

// CacheUtil is a generic in-process key-value store with per-entry TTL.
// It replaces the previous Valkey-backed implementation; the public API
// (Get/Set/SetWithExpiration/GetAndDelete/Invalidate) is unchanged.
type CacheUtil[T any] struct {
	mu      sync.RWMutex
	data    map[string]cacheEntry[T]
	prefix  string
	timeout time.Duration // kept for API symmetry
	expiry  time.Duration
}

func NewCacheUtil[T any](prefix string) *CacheUtil[T] {
	return &CacheUtil[T]{
		data:    make(map[string]cacheEntry[T]),
		prefix:  prefix,
		timeout: DefaultCacheTimeout,
		expiry:  DefaultCacheExpiry,
	}
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
	c.mu.Lock()
	c.data = make(map[string]cacheEntry[T])
	c.mu.Unlock()
}
