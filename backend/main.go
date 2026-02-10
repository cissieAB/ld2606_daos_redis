package main

import (
	"context"
	"net/http"

	"github.com/redis/go-redis/v9"
)

// main is the entry point for the traffic backend server.
// It initializes Redis connection, starts pub/sub subscriber,
// launches WebSocket broadcaster, and starts the HTTP server.
func main() {
	// Initialize configuration
	initConfig()

	// Root context for Redis operations.
	ctx := context.Background()

	// Create Redis client.
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: "",
		DB:       config.RedisDB,
		Protocol: 2,
	})

	// Initialize latest data structure on server startup.
	initializeLatestData(ctx, rdb)

	// Start Redis subscriber and WebSocket broadcaster in background.
	go startRedisSubscriber(ctx, rdb)
	go handleMessages()

	// Register HTTP handlers
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/latest", handleLatest)

	// Start the HTTP server.
	infoLog("Starting server on %s (Debug: %v)", config.ServerPort, config.Debug)
	if err := http.ListenAndServe(config.ServerPort, nil); err != nil {
		errorLog("HTTP server error: %v", err)
	}
}
