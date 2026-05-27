package restoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	cache_utils "databasus-backend/internal/util/cache"
)

const (
	restoreSubmitChannel     = "restore:submit"
	restoreCompletionChannel = "restore:completion"

	deadNodeThreshold     = 2 * time.Minute
	cleanupTickerInterval = 1 * time.Second
)

// RestoreNodesRegistry coordinates the restore scheduler and restore worker
// nodes within a single process. It replaces the previous Valkey-backed
// implementation; the public API is unchanged so all callers compile as-is.
type RestoreNodesRegistry struct {
	nodesMu sync.RWMutex
	nodes   map[uuid.UUID]RestoreNode

	countersMu sync.RWMutex
	counters   map[uuid.UUID]*atomic.Int64

	pubsubRestores    *cache_utils.PubSubManager
	pubsubCompletions *cache_utils.PubSubManager
	logger            *slog.Logger

	hasRun atomic.Bool
}

func (r *RestoreNodesRegistry) Run(ctx context.Context) {
	if r.hasRun.Swap(true) {
		panic(fmt.Sprintf("%T.Run() called multiple times", r))
	}

	r.cleanupDeadNodes()

	ticker := time.NewTicker(cleanupTickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.cleanupDeadNodes()
		}
	}
}

func (r *RestoreNodesRegistry) GetAvailableNodes() ([]RestoreNode, error) {
	threshold := time.Now().UTC().Add(-deadNodeThreshold)

	r.nodesMu.RLock()
	defer r.nodesMu.RUnlock()

	nodes := make([]RestoreNode, 0)
	for _, node := range r.nodes {
		if node.LastHeartbeat.IsZero() || node.LastHeartbeat.Before(threshold) {
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (r *RestoreNodesRegistry) GetRestoreNodesStats() ([]RestoreNodeStats, error) {
	threshold := time.Now().UTC().Add(-deadNodeThreshold)

	r.nodesMu.RLock()
	liveIDs := make([]uuid.UUID, 0, len(r.nodes))
	for id, node := range r.nodes {
		if node.LastHeartbeat.IsZero() || node.LastHeartbeat.Before(threshold) {
			continue
		}
		liveIDs = append(liveIDs, id)
	}
	r.nodesMu.RUnlock()

	r.countersMu.RLock()
	defer r.countersMu.RUnlock()

	stats := make([]RestoreNodeStats, 0, len(liveIDs))
	for _, id := range liveIDs {
		count := int64(0)
		if counter, ok := r.counters[id]; ok {
			count = counter.Load()
		}
		stats = append(stats, RestoreNodeStats{
			ID:             id,
			ActiveRestores: int(count),
		})
	}

	return stats, nil
}

func (r *RestoreNodesRegistry) IncrementRestoresInProgress(nodeID uuid.UUID) error {
	r.countersMu.Lock()
	counter, ok := r.counters[nodeID]
	if !ok {
		counter = &atomic.Int64{}
		r.counters[nodeID] = counter
	}
	r.countersMu.Unlock()

	counter.Add(1)
	return nil
}

func (r *RestoreNodesRegistry) DecrementRestoresInProgress(nodeID uuid.UUID) error {
	r.countersMu.RLock()
	counter, ok := r.counters[nodeID]
	r.countersMu.RUnlock()

	if !ok {
		return nil
	}

	newVal := counter.Add(-1)
	if newVal < 0 {
		counter.Store(0)
		r.logger.Warn("active restores counter went below 0, reset to 0", "nodeID", nodeID)
	}

	return nil
}

func (r *RestoreNodesRegistry) HearthbeatNodeInRegistry(
	now time.Time,
	restoreNode RestoreNode,
) error {
	if now.IsZero() {
		return fmt.Errorf("cannot register node with zero heartbeat timestamp")
	}

	restoreNode.LastHeartbeat = now

	r.nodesMu.Lock()
	r.nodes[restoreNode.ID] = restoreNode
	r.nodesMu.Unlock()

	return nil
}

func (r *RestoreNodesRegistry) UnregisterNodeFromRegistry(restoreNode RestoreNode) error {
	r.nodesMu.Lock()
	delete(r.nodes, restoreNode.ID)
	r.nodesMu.Unlock()

	r.countersMu.Lock()
	delete(r.counters, restoreNode.ID)
	r.countersMu.Unlock()

	r.logger.Info("unregistered node from registry", "nodeID", restoreNode.ID)
	return nil
}

func (r *RestoreNodesRegistry) AssignRestoreToNode(
	targetNodeID uuid.UUID,
	restoreID uuid.UUID,
	isCallNotifier bool,
) error {
	message := RestoreSubmitMessage{
		NodeID:         targetNodeID,
		RestoreID:      restoreID,
		IsCallNotifier: isCallNotifier,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal restore submit message: %w", err)
	}

	return r.pubsubRestores.Publish(context.Background(), restoreSubmitChannel, string(messageJSON))
}

func (r *RestoreNodesRegistry) SubscribeNodeForRestoresAssignment(
	nodeID uuid.UUID,
	handler func(restoreID uuid.UUID, isCallNotifier bool),
) error {
	wrappedHandler := func(message string) {
		var msg RestoreSubmitMessage
		if err := json.Unmarshal([]byte(message), &msg); err != nil {
			r.logger.Warn("failed to unmarshal restore submit message", "error", err)
			return
		}

		if msg.NodeID != nodeID {
			return
		}

		handler(msg.RestoreID, msg.IsCallNotifier)
	}

	err := r.pubsubRestores.Subscribe(context.Background(), restoreSubmitChannel, wrappedHandler)
	if err != nil {
		return fmt.Errorf("failed to subscribe to restore submit channel: %w", err)
	}

	r.logger.Info("subscribed to restore submit channel", "nodeID", nodeID)
	return nil
}

func (r *RestoreNodesRegistry) UnsubscribeNodeForRestoresAssignments() error {
	if err := r.pubsubRestores.Close(); err != nil {
		return fmt.Errorf("failed to unsubscribe from restore submit channel: %w", err)
	}

	r.logger.Info("unsubscribed from restore submit channel")
	return nil
}

func (r *RestoreNodesRegistry) PublishRestoreCompletion(
	nodeID uuid.UUID,
	restoreID uuid.UUID,
) error {
	message := RestoreCompletionMessage{
		NodeID:    nodeID,
		RestoreID: restoreID,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal restore completion message: %w", err)
	}

	return r.pubsubCompletions.Publish(context.Background(), restoreCompletionChannel, string(messageJSON))
}

func (r *RestoreNodesRegistry) SubscribeForRestoresCompletions(
	handler func(nodeID, restoreID uuid.UUID),
) error {
	wrappedHandler := func(message string) {
		var msg RestoreCompletionMessage
		if err := json.Unmarshal([]byte(message), &msg); err != nil {
			r.logger.Warn("failed to unmarshal restore completion message", "error", err)
			return
		}

		handler(msg.NodeID, msg.RestoreID)
	}

	err := r.pubsubCompletions.Subscribe(context.Background(), restoreCompletionChannel, wrappedHandler)
	if err != nil {
		return fmt.Errorf("failed to subscribe to restore completion channel: %w", err)
	}

	r.logger.Info("subscribed to restore completion channel")
	return nil
}

func (r *RestoreNodesRegistry) UnsubscribeForRestoresCompletions() error {
	if err := r.pubsubCompletions.Close(); err != nil {
		return fmt.Errorf("failed to unsubscribe from restore completion channel: %w", err)
	}

	r.logger.Info("unsubscribed from restore completion channel")
	return nil
}

// cleanupDeadNodes removes nodes whose last heartbeat is older than deadNodeThreshold.
func (r *RestoreNodesRegistry) cleanupDeadNodes() {
	threshold := time.Now().UTC().Add(-deadNodeThreshold)

	r.nodesMu.Lock()
	var deadIDs []uuid.UUID
	for id, node := range r.nodes {
		if node.LastHeartbeat.IsZero() || node.LastHeartbeat.Before(threshold) {
			deadIDs = append(deadIDs, id)
			delete(r.nodes, id)
		}
	}
	r.nodesMu.Unlock()

	if len(deadIDs) == 0 {
		return
	}

	r.countersMu.Lock()
	for _, id := range deadIDs {
		delete(r.counters, id)
	}
	r.countersMu.Unlock()

	r.logger.Info("cleaned up dead restore nodes", "count", len(deadIDs))
}

// restoreDatabaseCache is a package-level in-memory cache replacing the
// Valkey-backed CacheUtil that previously stored per-restore DB metadata.
var restoreDatabaseCache = cache_utils.NewCacheUtil[RestoreDatabaseCache]("restore_db:")
