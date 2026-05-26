package cache_utils

import (
	"sync"
	"time"
)

type windowEntry struct {
	mu         sync.Mutex
	timestamps []time.Time
}

// RateLimiter is an in-process sliding-window rate limiter.
// It replaces the previous Valkey-backed implementation; the public API
// (CheckLimit) is unchanged. Unlike the distributed implementation this one
// is per-process only — adequate for single-binary standalone deployments.
type RateLimiter struct {
	mu      sync.RWMutex
	windows map[string]*windowEntry
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		windows: make(map[string]*windowEntry),
	}
}

// CheckLimit returns true when the request is allowed (under the limit).
// identifier is typically an IP or user ID; endpoint is a stable name for
// the route being guarded.
func (r *RateLimiter) CheckLimit(
	identifier string,
	endpoint string,
	maxRequests int,
	windowDuration time.Duration,
) (bool, error) {
	key := endpoint + ":" + identifier
	now := time.Now()
	cutoff := now.Add(-windowDuration)

	r.mu.RLock()
	entry, exists := r.windows[key]
	r.mu.RUnlock()

	if !exists {
		r.mu.Lock()
		// Re-check under write lock to avoid duplicate creation.
		entry, exists = r.windows[key]
		if !exists {
			entry = &windowEntry{}
			r.windows[key] = entry
		}
		r.mu.Unlock()
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Drop expired timestamps.
	valid := entry.timestamps[:0]
	for _, ts := range entry.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	entry.timestamps = append(valid, now)

	return len(entry.timestamps) <= maxRequests, nil
}
