// Package main implements Redis polling for the real-time traffic data server.
package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// initializeLatestData seeds the materialized view and poll watermark from Redis on startup.
func initializeLatestData(ctx context.Context, rdb *redis.Client) {
	if err := ensureSearchIndex(ctx, rdb); err != nil {
		errorLog("Error ensuring search index: %v", err)
		initializeEmptyLatest()
		return
	}

	maxTs, err := maxTimestampFromIndex(ctx, rdb)
	if err != nil {
		errorLog("Error getting max timestamp: %v", err)
		initializeEmptyLatest()
		return
	}

	setStartingTimestamp(maxTs)
	if maxTs == 0 {
		debugLog("No data found in index")
		initializeEmptyLatest()
		return
	}

	docs, err := getNewPackets(ctx, rdb)
	if err != nil {
		debugLog("Error fetching initial data: %v", err)
		initializeEmptyLatest()
		return
	}
	if len(docs) == 0 {
		debugLog("No documents in initial poll window")
		initializeEmptyLatest()
		return
	}

	_, _ = applyDocuments(docs)
	latestMu.RLock()
	count := len(latest)
	latestMu.RUnlock()
	infoLog("Initialized materialized view: %d pairs (watermark=%d)", count, getStartingTimestamp())
}

// startRedisPoller keeps the materialized src:dest state current.
func startRedisPoller(ctx context.Context, rdb *redis.Client) {
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	infoLog("Polling Redis every %s for latest src:dest state", config.PollInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollRedisOnce(ctx, rdb)
		}
	}
}

func pollRedisOnce(ctx context.Context, rdb *redis.Client) {
	docs, err := getNewPackets(ctx, rdb)
	if err != nil {
		errorLog("Poll error: %v", err)
		return
	}

	if len(docs) == 0 {
		clearLatestIfRedisEmpty(ctx, rdb)
		debugLog("Poll: no documents in window (watermark=%d)", getStartingTimestamp())
		return
	}

	updates, pruned := applyDocuments(docs)
	if pruned {
		broadcastSnapshot()
		debugLog("Poll: stale pairs pruned; broadcast snapshot (watermark=%d)", getStartingTimestamp())
		return
	}

	if len(updates) == 0 {
		return
	}

	broadcastUpdates(updates)
	debugLog("Poll: %d updates (watermark=%d)", len(updates), getStartingTimestamp())
}

func clearLatestIfRedisEmpty(ctx context.Context, rdb *redis.Client) {
	maxTs, err := maxTimestampFromIndex(ctx, rdb)
	if err != nil || maxTs != 0 || !hasLatestPackets() {
		return
	}

	initializeEmptyLatest()
	broadcastSnapshot()
}
