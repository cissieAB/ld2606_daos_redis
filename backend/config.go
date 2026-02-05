package main

import (
	"fmt"
	"os"
)

// Config holds the application configuration loaded from environment variables.
// All settings have sensible defaults and can be overridden via env vars.
type Config struct {
	Debug        bool
	RedisAddr    string
	RedisDB      int
	ServerPort   string
	RedisChannel string
}

// Global config instance
var config Config

// initConfig initializes the configuration from environment variables
func initConfig() {
	config = Config{
		Debug:        os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1",
		RedisAddr:    getEnv("REDIS_ADDR", "ejfat-6.jlab.org:6379"),
		RedisDB:      0,
		ServerPort:   getEnv("SERVER_PORT", ":8080"),
		RedisChannel: getEnv("REDIS_CHANNEL", "traffic_channel"),
	}
}

// getEnv gets an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// debugLog prints only when debug mode is enabled
func debugLog(format string, args ...interface{}) {
	if config.Debug {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}

// infoLog always prints important information
func infoLog(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

// errorLog always prints errors
func errorLog(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}
