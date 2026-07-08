package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func historicalEdgeDetail(
	ctx context.Context,
	rdb *redis.Client,
	src string,
	dest string,
	timestamp int,
) (EdgeDetail, bool, error) {
	key := fmt.Sprintf("packet:%s:%s:%d", dest, src, timestamp)
	fields, err := rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return EdgeDetail{}, false, err
	}
	if len(fields) == 0 {
		return EdgeDetail{}, false, nil
	}

	packet, err := docToPacket(redis.Document{ID: key, Fields: fields})
	if err != nil {
		return EdgeDetail{}, false, err
	}
	if packet.Src != src ||
		packet.Dest != dest ||
		packet.Timestamp != timestamp ||
		!validHistoryPacket(packet) {
		return EdgeDetail{}, false, nil
	}

	return generateEdgeDetail(packet), true, nil
}
