// Package main implements a real-time traffic data server.
// It connects to Redis, polls for packet data, maintains a materialized view of
// the latest packets per source:destination pair, and serves this data via HTTP
// and WebSocket endpoints for real-time updates.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Debug        bool
	RedisAddr    string
	RedisDB      int
	ServerPort   string
	PollInterval time.Duration
}

var config Config

func initConfig() {
	pollInterval := time.Second
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			pollInterval = d
		}
	}

	redisDB := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			redisDB = n
		}
	}

	config = Config{
		Debug:        os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1",
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		RedisDB:      redisDB,
		ServerPort:   getEnv("SERVER_PORT", ":8080"),
		PollInterval: pollInterval,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// debugLog prints debug-level messages (only when DEBUG=true).
func debugLog(format string, args ...interface{}) {
	if config.Debug {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}

// infoLog prints informational messages.
func infoLog(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

// errorLog prints error messages.
func errorLog(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}
