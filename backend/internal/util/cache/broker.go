package cache_utils

import (
	"sync"

	"github.com/google/uuid"
)

// inMemoryBroker is a single-process pub/sub message broker that replaces
// Valkey pub/sub. All subscribers in the same process share one global broker.
type inMemoryBroker struct {
	mu   sync.RWMutex
	subs map[string][]brokerEntry // channel → registered handlers
}

type brokerEntry struct {
	id      string
	handler func(string)
}

var globalBroker = &inMemoryBroker{
	subs: make(map[string][]brokerEntry),
}

// publish calls every registered handler for the channel, each in its own
// goroutine to match Valkey's asynchronous delivery semantics.
func (b *inMemoryBroker) publish(channel, message string) {
	b.mu.RLock()
	handlers := make([]brokerEntry, len(b.subs[channel]))
	copy(handlers, b.subs[channel])
	b.mu.RUnlock()

	for _, entry := range handlers {
		go entry.handler(message)
	}
}

// subscribe registers handler for channel and returns an opaque subscription ID
// that can be passed to unsubscribe to remove just this handler.
func (b *inMemoryBroker) subscribe(channel string, handler func(string)) string {
	id := uuid.New().String()

	b.mu.Lock()
	b.subs[channel] = append(b.subs[channel], brokerEntry{id, handler})
	b.mu.Unlock()

	return id
}

// unsubscribe removes all handlers whose IDs appear in ids.
func (b *inMemoryBroker) unsubscribe(ids []string) {
	if len(ids) == 0 {
		return
	}

	remove := make(map[string]bool, len(ids))
	for _, id := range ids {
		remove[id] = true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for channel, entries := range b.subs {
		kept := entries[:0]
		for _, entry := range entries {
			if !remove[entry.id] {
				kept = append(kept, entry)
			}
		}
		b.subs[channel] = kept
	}
}

// ResetBroker clears all subscriptions. Intended for tests only.
func ResetBroker() {
	globalBroker.mu.Lock()
	globalBroker.subs = make(map[string][]brokerEntry)
	globalBroker.mu.Unlock()
}

// ClearAllCache resets the in-process pub/sub broker. It replaces the previous
// Valkey-backed ClearAllCache that flushed the Redis keyspace; tests call it
// between runs to guarantee a clean subscription state.
func ClearAllCache() error {
	ResetBroker()
	return nil
}
