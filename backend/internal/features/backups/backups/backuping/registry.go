package backuping

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
	backupSubmitChannel     = "backup:submit"
	backupCompletionChannel = "backup:completion"

	deadNodeThreshold     = 2 * time.Minute
	cleanupTickerInterval = 1 * time.Second
)

// BackupNodesRegistry coordinates the backup scheduler and backup worker
// nodes within a single process. It replaces the previous Valkey-backed
// implementation; the public API is unchanged so all callers compile as-is.
//
// In single-process deployments the "nodes" are goroutines in the same binary,
// so in-memory maps replace the distributed Redis/Valkey data structures.
type BackupNodesRegistry struct {
	nodesMu sync.RWMutex
	nodes   map[uuid.UUID]BackupNode

	countersMu sync.RWMutex
	counters   map[uuid.UUID]*atomic.Int64

	pubsubBackups     *cache_utils.PubSubManager
	pubsubCompletions *cache_utils.PubSubManager
	logger            *slog.Logger

	hasRun atomic.Bool
}

func (r *BackupNodesRegistry) Run(ctx context.Context) {
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

func (r *BackupNodesRegistry) GetAvailableNodes() ([]BackupNode, error) {
	threshold := time.Now().UTC().Add(-deadNodeThreshold)

	r.nodesMu.RLock()
	defer r.nodesMu.RUnlock()

	nodes := make([]BackupNode, 0)
	for _, node := range r.nodes {
		if node.LastHeartbeat.IsZero() || node.LastHeartbeat.Before(threshold) {
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (r *BackupNodesRegistry) GetBackupNodesStats() ([]BackupNodeStats, error) {
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

	stats := make([]BackupNodeStats, 0, len(liveIDs))
	for _, id := range liveIDs {
		count := int64(0)
		if counter, ok := r.counters[id]; ok {
			count = counter.Load()
		}
		stats = append(stats, BackupNodeStats{
			ID:            id,
			ActiveBackups: int(count),
		})
	}

	return stats, nil
}

func (r *BackupNodesRegistry) IncrementBackupsInProgress(nodeID uuid.UUID) error {
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

func (r *BackupNodesRegistry) DecrementBackupsInProgress(nodeID uuid.UUID) error {
	r.countersMu.RLock()
	counter, ok := r.counters[nodeID]
	r.countersMu.RUnlock()

	if !ok {
		return nil
	}

	newVal := counter.Add(-1)
	if newVal < 0 {
		counter.Store(0)
		r.logger.Warn("active backups counter went below 0, reset to 0", "nodeID", nodeID)
	}

	return nil
}

func (r *BackupNodesRegistry) HearthbeatNodeInRegistry(now time.Time, backupNode BackupNode) error {
	if now.IsZero() {
		return fmt.Errorf("cannot register node with zero heartbeat timestamp")
	}

	backupNode.LastHeartbeat = now

	r.nodesMu.Lock()
	r.nodes[backupNode.ID] = backupNode
	r.nodesMu.Unlock()

	return nil
}

func (r *BackupNodesRegistry) UnregisterNodeFromRegistry(backupNode BackupNode) error {
	r.nodesMu.Lock()
	delete(r.nodes, backupNode.ID)
	r.nodesMu.Unlock()

	r.countersMu.Lock()
	delete(r.counters, backupNode.ID)
	r.countersMu.Unlock()

	r.logger.Info("unregistered node from registry", "nodeID", backupNode.ID)
	return nil
}

func (r *BackupNodesRegistry) AssignBackupToNode(
	targetNodeID uuid.UUID,
	backupID uuid.UUID,
	isCallNotifier bool,
) error {
	message := BackupSubmitMessage{
		NodeID:         targetNodeID,
		BackupID:       backupID,
		IsCallNotifier: isCallNotifier,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal backup submit message: %w", err)
	}

	return r.pubsubBackups.Publish(context.Background(), backupSubmitChannel, string(messageJSON))
}

func (r *BackupNodesRegistry) SubscribeNodeForBackupsAssignment(
	nodeID uuid.UUID,
	handler func(backupID uuid.UUID, isCallNotifier bool),
) error {
	wrappedHandler := func(message string) {
		var msg BackupSubmitMessage
		if err := json.Unmarshal([]byte(message), &msg); err != nil {
			r.logger.Warn("failed to unmarshal backup submit message", "error", err)
			return
		}

		if msg.NodeID != nodeID {
			return
		}

		handler(msg.BackupID, msg.IsCallNotifier)
	}

	err := r.pubsubBackups.Subscribe(context.Background(), backupSubmitChannel, wrappedHandler)
	if err != nil {
		return fmt.Errorf("failed to subscribe to backup submit channel: %w", err)
	}

	r.logger.Info("subscribed to backup submit channel", "nodeID", nodeID)
	return nil
}

func (r *BackupNodesRegistry) UnsubscribeNodeForBackupsAssignments() error {
	if err := r.pubsubBackups.Close(); err != nil {
		return fmt.Errorf("failed to unsubscribe from backup submit channel: %w", err)
	}

	r.logger.Info("unsubscribed from backup submit channel")
	return nil
}

func (r *BackupNodesRegistry) PublishBackupCompletion(nodeID, backupID uuid.UUID) error {
	message := BackupCompletionMessage{
		NodeID:   nodeID,
		BackupID: backupID,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal backup completion message: %w", err)
	}

	return r.pubsubCompletions.Publish(context.Background(), backupCompletionChannel, string(messageJSON))
}

func (r *BackupNodesRegistry) SubscribeForBackupsCompletions(
	handler func(nodeID, backupID uuid.UUID),
) error {
	wrappedHandler := func(message string) {
		var msg BackupCompletionMessage
		if err := json.Unmarshal([]byte(message), &msg); err != nil {
			r.logger.Warn("failed to unmarshal backup completion message", "error", err)
			return
		}

		handler(msg.NodeID, msg.BackupID)
	}

	err := r.pubsubCompletions.Subscribe(context.Background(), backupCompletionChannel, wrappedHandler)
	if err != nil {
		return fmt.Errorf("failed to subscribe to backup completion channel: %w", err)
	}

	r.logger.Info("subscribed to backup completion channel")
	return nil
}

func (r *BackupNodesRegistry) UnsubscribeForBackupsCompletions() error {
	if err := r.pubsubCompletions.Close(); err != nil {
		return fmt.Errorf("failed to unsubscribe from backup completion channel: %w", err)
	}

	r.logger.Info("unsubscribed from backup completion channel")
	return nil
}

// cleanupDeadNodes removes nodes whose last heartbeat is older than deadNodeThreshold.
func (r *BackupNodesRegistry) cleanupDeadNodes() {
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

	r.logger.Info("cleaned up dead backup nodes", "count", len(deadIDs))
}
