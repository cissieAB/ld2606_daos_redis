package main

import (
	"context"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
)

// main initializes the application: connects to Redis, starts the polling goroutine,
// and launches the HTTP server with WebSocket support.
func main() {
	initConfig()

	ctx := context.Background()
	topology, err := loadTopology(config.TopologyPath)
	if err != nil {
		errorLog("Failed to load topology from %s: %v", config.TopologyPath, err)
		os.Exit(1)
	}
	infoLog("Loaded %d topology nodes from %s", len(topology.Nodes), config.TopologyPath)

	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: "",
		DB:       config.RedisDB,
		Protocol: 2,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		errorLog("Failed to connect to Redis at %s: %v", config.RedisAddr, err)
	} else {
		infoLog("Connected to Redis at %s (db=%d)", config.RedisAddr, config.RedisDB)
	}

	initializeLatestData(ctx, rdb)

	go startRedisPoller(ctx, rdb)
	go handleMessages()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(topology, w, r)
	})
	http.HandleFunc("/latest", handleLatest)
	http.HandleFunc("/edge", func(w http.ResponseWriter, r *http.Request) {
		handleEdgeRequest(rdb, w, r)
	})
	http.HandleFunc("/history", handleHistory(func(
		ctx context.Context,
		start int,
		end int,
		limit int,
	) (HistoryResponse, error) {
		return loadHistory(ctx, rdb, start, end, limit)
	}))
	http.HandleFunc("/", handleRoot)

	infoLog("Starting server on %s (Debug: %v, Poll: %s)", config.ServerPort, config.Debug, config.PollInterval)
	if err := http.ListenAndServe(config.ServerPort, nil); err != nil {
		errorLog("HTTP server error: %v", err)
	}
}
