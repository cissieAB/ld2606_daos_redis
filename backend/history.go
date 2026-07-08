package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	defaultHistoryLimit = 60
	maxHistoryLimit     = 120
)

var historyReturnFields = []redis.FTSearchReturn{
	{FieldName: "timestamp"},
	{FieldName: "source_ip"},
	{FieldName: "dest_ip"},
	{FieldName: "samples_per_second"},
	{FieldName: "udp_packets"},
	{FieldName: "udp_bytes"},
	{FieldName: "tcp_packets"},
	{FieldName: "tcp_bytes"},
}

func loadHistory(
	ctx context.Context,
	rdb *redis.Client,
	start int,
	end int,
	limit int,
) (HistoryResponse, error) {
	timestamps, err := getHistoryTimestamps(ctx, rdb, start, end, limit+1)
	if err != nil {
		return HistoryResponse{}, err
	}

	response := HistoryResponse{
		Start:  start,
		End:    end,
		Frames: []HistoryFrame{},
	}
	if len(timestamps) == 0 {
		return response, nil
	}

	timestamps, response.HasMore, response.NextStart = paginateHistoryTimestamps(
		timestamps,
		limit,
	)

	packets, err := getHistoryPackets(
		ctx,
		rdb,
		timestamps[0],
		timestamps[len(timestamps)-1],
	)
	if err != nil {
		return HistoryResponse{}, err
	}

	response.Frames = buildHistoryFrames(timestamps, packets)
	return response, nil
}

func paginateHistoryTimestamps(timestamps []int, limit int) ([]int, bool, *int) {
	if len(timestamps) <= limit {
		return timestamps, false, nil
	}

	selected := timestamps[:limit]
	nextStart := selected[len(selected)-1] + 1
	return selected, true, &nextStart
}

func getHistoryTimestamps(
	ctx context.Context,
	rdb *redis.Client,
	start int,
	end int,
	limit int,
) ([]int, error) {
	query := fmt.Sprintf("@timestamp:[%d %d]", start, end)
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
				Asc:       true,
			}},
			LimitOffset: 0,
			Limit:       limit,
		},
	).Result()
	if err != nil {
		return nil, fmt.Errorf("query history timestamps: %w", err)
	}

	timestamps := make([]int, 0, len(result.Rows))
	for _, row := range result.Rows {
		timestamp, ok := parseIntField(row.Fields["timestamp"])
		if !ok {
			return nil, fmt.Errorf("invalid history timestamp: %v", row.Fields["timestamp"])
		}
		timestamps = append(timestamps, timestamp)
	}

	return timestamps, nil
}

func getHistoryPackets(
	ctx context.Context,
	rdb *redis.Client,
	start int,
	end int,
) ([]Packet, error) {
	query := fmt.Sprintf("@timestamp:[%d %d]", start, end)
	packets := make([]Packet, 0)
	offset := 0

	for {
		result, err := rdb.FTSearchWithArgs(
			ctx,
			searchIndexName,
			query,
			&redis.FTSearchOptions{
				Return:      historyReturnFields,
				LimitOffset: offset,
				Limit:       searchLimit,
			},
		).Result()
		if err != nil {
			return nil, fmt.Errorf("query history packets: %w", err)
		}

		for _, doc := range result.Docs {
			packet, err := docToPacket(doc)
			if err != nil {
				return nil, fmt.Errorf("decode history packet: %w", err)
			}
			packets = append(packets, packet)
		}

		if len(result.Docs) < searchLimit {
			break
		}
		offset += searchLimit
	}

	return packets, nil
}

func buildHistoryFrames(timestamps []int, packets []Packet) []HistoryFrame {
	framesByTimestamp := make(map[int]map[string]PacketSummary, len(timestamps))
	invalidTimestamps := make(map[int]bool)
	for _, timestamp := range timestamps {
		framesByTimestamp[timestamp] = make(map[string]PacketSummary)
	}

	for _, packet := range packets {
		frame, ok := framesByTimestamp[packet.Timestamp]
		if !ok {
			continue
		}
		if !validHistoryPacket(packet) {
			invalidTimestamps[packet.Timestamp] = true
			continue
		}
		frame[pairKey(packet.Src, packet.Dest)] = generateEdgeSummary(packet)
	}

	frames := make([]HistoryFrame, 0, len(timestamps))
	for _, timestamp := range timestamps {
		data := framesByTimestamp[timestamp]
		if len(data) == 0 || invalidTimestamps[timestamp] {
			continue
		}
		frames = append(frames, HistoryFrame{
			Timestamp: timestamp,
			Data:      data,
		})
	}

	return frames
}

func validHistoryPacket(packet Packet) bool {
	samples := packet.SamplesPerSecond
	return packet.Src != "" &&
		packet.Dest != "" &&
		samples > 0 &&
		len(packet.UDPPackets) == samples &&
		len(packet.UDPBytes) == samples &&
		len(packet.TCPPackets) == samples &&
		len(packet.TCPBytes) == samples
}
