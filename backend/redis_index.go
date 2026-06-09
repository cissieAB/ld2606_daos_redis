package main

import (
	"context"
	"fmt"

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

func maxTimestampFromIndex(ctx context.Context, rdb *redis.Client) (int, error) {
	aggResult, err := rdb.FTAggregateWithArgs(
		ctx,
		searchIndexName,
		"*",
		&redis.FTAggregateOptions{
			GroupBy: []redis.FTAggregateGroupBy{
				{
					Fields: []interface{}{},
					Reduce: []redis.FTAggregateReducer{
						{
							Reducer: redis.SearchMax,
							Args:    []interface{}{"@timestamp"},
							As:      "max_timestamp",
						},
					},
				},
			},
		},
	).Result()
	if err != nil {
		return 0, err
	}
	if len(aggResult.Rows) == 0 {
		return 0, nil
	}

	tsVal := aggResult.Rows[0].Fields["max_timestamp"]
	if tsVal == nil {
		return 0, nil
	}
	ts, ok := parseIntField(tsVal)
	if !ok {
		return 0, fmt.Errorf("invalid max_timestamp value: %v", tsVal)
	}
	return ts, nil
}

func getNewPackets(ctx context.Context, rdb *redis.Client) ([]redis.Document, error) {
	since := pollSinceTimestamp()
	query := fmt.Sprintf("@timestamp:[%d +inf]", since)

	var docs []redis.Document
	offset := 0

	for {
		result, err := rdb.FTSearchWithArgs(
			ctx,
			searchIndexName,
			query,
			&redis.FTSearchOptions{
				LimitOffset: offset,
				Limit:       searchLimit,
				SortBy: []redis.FTSearchSortBy{
					{
						FieldName: "timestamp",
						Asc:       false,
					},
				},
			},
		).Result()
		if err != nil {
			return nil, fmt.Errorf("search packets since %d: %w", since, err)
		}

		docs = append(docs, result.Docs...)

		if len(result.Docs) < searchLimit {
			break
		}
		offset += searchLimit
	}

	return docs, nil
}
