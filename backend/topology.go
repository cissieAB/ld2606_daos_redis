package main

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const topologyNodesKey = "topology:nodes"

// loadTopology reads the complete static topology stored by traffic producers.
func loadTopology(ctx context.Context, rdb *redis.Client) (Topology, error) {
	ips, err := rdb.SMembers(ctx, topologyNodesKey).Result()
	if err != nil {
		return Topology{}, err
	}

	topology := Topology{Nodes: make(map[string]NodeMetadata, len(ips))}
	if len(ips) == 0 {
		return topology, nil
	}

	pipeline := rdb.Pipeline()
	commands := make(map[string]*redis.MapStringStringCmd, len(ips))
	for _, ip := range ips {
		commands[ip] = pipeline.HGetAll(ctx, "topology:node:"+ip)
	}

	if _, err := pipeline.Exec(ctx); err != nil {
		return Topology{}, err
	}

	for ip, command := range commands {
		fields := command.Val()
		metadataIP := fields["ip"]
		if metadataIP == "" {
			metadataIP = ip
		}

		topology.Nodes[ip] = NodeMetadata{
			IP:   metadataIP,
			Rack: fields["rack"],
		}
	}

	return topology, nil
}
