package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func liveWindowStart(now int) int {
	start := now - safetyWindow
	if start < 0 {
		return 0
	}
	return start
}

func getLiveTimestamp(ctx context.Context, rdb *redis.Client, now int) (int, bool, error) {
	start := liveWindowStart(now)
	query := fmt.Sprintf("@timestamp:[%d %d]", start, now)
	result, err := rdb.FTAggregateWithArgs(
		ctx,
		searchIndexName,
		query,
		&redis.FTAggregateOptions{
			GroupBy: []redis.FTAggregateGroupBy{{
				Fields: []interface{}{"@timestamp"},
			}},
			SortBy: []redis.FTAggregateSortBy{{
				FieldName: "@timestamp",
				Asc:       false,
			}},
			LimitOffset: 0,
			Limit:       1,
		},
	).Result()
	if err != nil {
		return 0, false, fmt.Errorf("query live timestamp: %w", err)
	}
	if len(result.Rows) == 0 {
		return 0, false, nil
	}

	timestamp, ok := parseIntField(result.Rows[0].Fields["timestamp"])
	if !ok {
		return 0, false, fmt.Errorf("invalid live timestamp: %v", result.Rows[0].Fields["timestamp"])
	}
	return timestamp, true, nil
}

func loadLiveFrame(ctx context.Context, rdb *redis.Client, now int) (int, []Packet, error) {
	timestamp, ok, err := getLiveTimestamp(ctx, rdb, now)
	if err != nil || !ok {
		return 0, nil, err
	}

	packets, err := getHistoryPackets(ctx, rdb, timestamp, timestamp)
	if err != nil {
		return 0, nil, err
	}
	return timestamp, packets, nil
}
