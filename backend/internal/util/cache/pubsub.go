package cache_utils

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"databasus-backend/internal/util/logger"
)

// PubSubManager is a handle onto the global in-process broker.
// Each instance tracks the subscription IDs it owns so Close() removes only
// the handlers registered through that instance.
// A given channel may only be subscribed once per manager instance; a second
// call to Subscribe for the same channel returns an error.
type PubSubManager struct {
	mu       sync.Mutex
	subIDs   []string
	channels map[string]bool
	logger   *slog.Logger
}

func NewPubSubManager() *PubSubManager {
	return &PubSubManager{
		channels: make(map[string]bool),
		logger:   logger.GetLogger(),
	}
}

func (m *PubSubManager) Subscribe(
	_ context.Context,
	channel string,
	handler func(message string),
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.channels[channel] {
		return fmt.Errorf("already subscribed to channel: %s", channel)
	}

	id := globalBroker.subscribe(channel, handler)
	m.subIDs = append(m.subIDs, id)
	m.channels[channel] = true

	m.logger.Info("subscribed to channel", "channel", channel)
	return nil
}

func (m *PubSubManager) Publish(_ context.Context, channel, message string) error {
	globalBroker.publish(channel, message)
	return nil
}

func (m *PubSubManager) Close() error {
	m.mu.Lock()
	ids := m.subIDs
	m.subIDs = nil
	m.channels = make(map[string]bool)
	m.mu.Unlock()

	globalBroker.unsubscribe(ids)
	return nil
}
