package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// createIndexIfNotExists creates a Redis Search index for traffic data if it doesn't exist.
func createIndexIfNotExists(ctx context.Context, rdb *redis.Client) error {
	// Check if index exists
	_, err := rdb.FTInfo(ctx, "idx:packets").Result()
	if err == nil {
		debugLog("Index 'idx:packets' already exists")
		return nil
	}

	_, err = rdb.FTCreate(
		ctx,
		"idx:packets",
		// Options:
		&redis.FTCreateOptions{
			OnHash: true,
			Prefix: []interface{}{"packet:"},
		},
		// Index schema fields:
		&redis.FieldSchema{
			FieldName: "timestamp",
			As:        "timestamp",
			FieldType: redis.SearchFieldTypeNumeric,
		},
		&redis.FieldSchema{
			FieldName: "total_bytes",
			As:        "total_bytes",
			FieldType: redis.SearchFieldTypeNumeric,
		},
	).Result()

	if err != nil {
		return err
	}

	infoLog("Index 'idx:packets' created successfully")
	return nil
}

// initializeLatestData attempts to fetch the latest data from Redis on startup.
// If no data exists, it initializes an empty structure.
func initializeLatestData(ctx context.Context, rdb *redis.Client) {
	err := createIndexIfNotExists(ctx, rdb)
	if err != nil {
		errorLog("Error creating index: %v", err)
		initializeEmptyLatest()
		return
	}

	// Get the max timestamp using aggregation
	aggOptions := redis.FTAggregateOptions{
		GroupBy: []redis.FTAggregateGroupBy{
			{
				Fields: []interface{}{"@timestamp"},
				Reduce: []redis.FTAggregateReducer{
					{
						Reducer: redis.SearchMax,
						Args:    []interface{}{"@timestamp"},
						As:      "max_timestamp",
					},
				},
			},
		},
	}

	aggResult, err := rdb.FTAggregateWithArgs(
		ctx,
		"idx:packets",
		"*",
		&aggOptions,
	).Result()

	if err != nil {
		debugLog("Error aggregating max timestamp: %v", err)
		initializeEmptyLatest()
		return
	}

	// Check if we have results
	if len(aggResult.Rows) == 0 {
		debugLog("No data found in index")
		initializeEmptyLatest()
		return
	}

	maxTimestampStr := aggResult.Rows[0].Fields["max_timestamp"]
	if maxTimestampStr == nil {
		debugLog("Max timestamp is nil")
		initializeEmptyLatest()
		return
	}

	// Parse timestamp string to int
	var timestamp int
	n, err := fmt.Sscanf(maxTimestampStr.(string), "%d", &timestamp)
	if err != nil || n != 1 {
		errorLog("Error parsing timestamp '%s': %v", maxTimestampStr, err)
		initializeEmptyLatest()
		return
	}

	debugLog("Max timestamp: %d", timestamp)

	// Get all packets with the max timestamp
	packets, err := rdb.FTSearchWithArgs(
		ctx,
		"idx:packets",
		fmt.Sprintf("@timestamp:[%d %d]", timestamp, timestamp),
		&redis.FTSearchOptions{},
	).Result()

	if err != nil {
		errorLog("Error searching for packets: %v", err)
		initializeEmptyLatest()
		return
	}

	debugLog("Found %d packets with timestamp %d", len(packets.Docs), timestamp)

	// Build the packets array with full packet data
	latestPackets := []interface{}{}
	for _, doc := range packets.Docs {
		// Store the entire packet as a map
		packetData := make(map[string]interface{})
		for field, value := range doc.Fields {
			packetData[field] = value
		}
		latestPackets = append(latestPackets, packetData)
	}

	// Update the latest structure
	latestMu.Lock()
	latest = latestData{
		Timestamp:   timestamp,
		PacketCount: len(packets.Docs),
		Packets:     latestPackets,
	}
	latestMu.Unlock()

	infoLog("Initialized with data: timestamp=%d packet_count=%d packets=%d",
		timestamp, len(packets.Docs), len(latestPackets))
}

// initializeEmptyLatest initializes the latest structure with empty values.
func initializeEmptyLatest() {
	latestMu.Lock()
	latest = latestData{
		Timestamp:   0,
		PacketCount: 0,
		Packets:     []interface{}{},
	}
	latestMu.Unlock()
	debugLog("Initialized with empty latest structure")
}

// startRedisSubscriber subscribes to a Redis pub/sub channel and
// forwards incoming messages to the broadcast channel.
func startRedisSubscriber(ctx context.Context, rdb *redis.Client) {
	// Subscribe to the channel from config
	pubsub := rdb.Subscribe(ctx, config.RedisChannel)

	// Wait for the subscription to be confirmed.
	_, err := pubsub.Receive(ctx)
	if err != nil {
		errorLog("Error receiving message from Redis: %v", err)
		return
	}
	// Channel that delivers published messages.
	ch := pubsub.Channel()

	infoLog("Subscribed to %s", config.RedisChannel)

	// Listen for incoming messages forever.
	for msg := range ch {
		// Send full payload to WebSocket broadcaster.
		broadcast <- msg.Payload
		
		// Unmarshal the payload to a trafficMessage struct.
		var payload trafficMessage
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			errorLog("Error decoding traffic payload: %v", err)
			continue
		}
		debugLog("Received: packet_count=%d timestamp=%d packets_len=%d", payload.PacketCount, payload.Timestamp, len(payload.Packets))

		// Update latest data based on timestamp.
		latestMu.Lock()
		if payload.Timestamp == latest.Timestamp {
			// Same timestamp: accumulate data.
			latest.PacketCount += payload.PacketCount
			latest.Packets = append(latest.Packets, payload.Packets...)
			debugLog("Accumulated: total packet_count=%d total_packets=%d", latest.PacketCount, len(latest.Packets))
		} else {
			// Different timestamp: replace with new data.
			latest = latestData{
				Timestamp:   payload.Timestamp,
				PacketCount: payload.PacketCount,
				Packets:     payload.Packets,
			}
			debugLog("Replaced: new timestamp=%d packet_count=%d packets=%d", latest.Timestamp, latest.PacketCount, len(latest.Packets))
		}
		latestMu.Unlock()
	}
}
