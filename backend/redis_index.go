package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

const (
	searchIndexName = "idx:packets"
	searchLimit     = 10000
)

// ensureSearchIndex creates or migrates the RediSearch index for simulator v2 hashes.
func ensureSearchIndex(ctx context.Context, rdb *redis.Client) error {
	info, err := rdb.FTInfo(ctx, searchIndexName).Result()
	if err == nil {
		hasTimestamp := false
		for _, attr := range info.Attributes {
			if attr.Identifier == "timestamp" {
				hasTimestamp = true
				break
			}
		}
		if hasTimestamp {
			debugLog("Index '%s' already exists with timestamp", searchIndexName)
			return nil
		}
		infoLog("Dropping outdated index '%s' (missing timestamp field)", searchIndexName)
		if err := rdb.FTDropIndex(ctx, searchIndexName).Err(); err != nil {
			return fmt.Errorf("drop index: %w", err)
		}
	}

	_, err = rdb.FTCreate(
		ctx,
		searchIndexName,
		&redis.FTCreateOptions{
			OnHash: true,
			Prefix: []interface{}{"packet:"},
		},
		&redis.FieldSchema{
			FieldName: "timestamp",
			As:        "timestamp",
			FieldType: redis.SearchFieldTypeNumeric,
			Sortable:  true,
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

	infoLog("Index '%s' created successfully", searchIndexName)
	return nil
}

// isMissingIndexError reports whether err is RediSearch saying the index does not exist.
// Redis Stack 7.x says "no such index"; older RediSearch said "Unknown Index name".
func isMissingIndexError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such index") || strings.Contains(msg, "unknown index name")
}
