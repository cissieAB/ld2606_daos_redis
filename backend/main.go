package main

import (
	"context"
	"net/http"

	"github.com/redis/go-redis/v9"
)

// main initializes the application: connects to Redis, starts the polling goroutine,
// and launches the HTTP server with WebSocket support.
func main() {
	initConfig()

	ctx := context.Background()

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

	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/latest", handleLatest)

	infoLog("Starting server on %s (Debug: %v, Poll: %s)", config.ServerPort, config.Debug, config.PollInterval)
	if err := http.ListenAndServe(config.ServerPort, nil); err != nil {
		errorLog("HTTP server error: %v", err)
	}
}
