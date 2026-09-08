// Package main implements Redis polling for the real-time traffic data server.
package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// initializeLatestData seeds the live frame from the current Redis safety window on startup.
func initializeLatestData(ctx context.Context, rdb *redis.Client) {
	if err := ensureSearchIndex(ctx, rdb); err != nil {
		errorLog("Error ensuring search index: %v", err)
		initializeEmptyLatest()
		return
	}

	timestamp, packets, err := loadLiveFrame(ctx, rdb, int(time.Now().Unix()))
	if err != nil {
		errorLog("Error loading live frame: %v", err)
		initializeEmptyLatest()
		return
	}
	if timestamp == 0 {
		debugLog("No data found in live window")
		initializeEmptyLatest()
		return
	}

	replaceLatest(packets)
	latestMu.RLock()
	count := len(latest)
	latestMu.RUnlock()
	infoLog("Initialized live frame: %d pairs (timestamp=%d)", count, timestamp)
}

// startRedisPoller keeps the selected live frame current.
func startRedisPoller(ctx context.Context, rdb *redis.Client) {
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	infoLog("Polling Redis every %s for the selected live frame", config.PollInterval)

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
	timestamp, packets, err := loadLiveFrame(ctx, rdb, int(time.Now().Unix()))
	if err != nil {
		errorLog("Poll error: %v", err)
		return
	}

	if timestamp == 0 {
		if hasLatestPackets() {
			initializeEmptyLatest()
			broadcastSnapshot()
		}
		debugLog("Poll: no live frame in window")
		return
	}

	replaceLatest(packets)
	broadcastSnapshot()
	debugLog("Poll: live snapshot timestamp=%d pairs=%d", timestamp, len(packets))
}
