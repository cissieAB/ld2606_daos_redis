package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

// trafficMessage represents incoming messages from Redis.
type trafficMessage struct {
	PacketCount int           `json:"packet_count"`
	Timestamp   int           `json:"timestamp"`
	Packets     []interface{} `json:"packets"`
}

// latestData holds the accumulated latest data.
type latestData struct {
	Timestamp   int           `json:"timestamp"`
	PacketCount int           `json:"packet_count"`
	Packets     []interface{} `json:"packets"`
}

// Global variables for state management
var (
	// latest holds the accumulated latest message data.
	latest latestData
	
	// latestMu protects access to latest (RWMutex allows multiple readers)
	latestMu sync.RWMutex
	
	// clients keeps track of connected WebSocket clients.
	clients = make(map[*websocket.Conn]bool)
	
	// clientsMu protects access to clients map
	clientsMu sync.Mutex
	
	// broadcast is a channel used to deliver Redis messages to WebSocket clients.
	broadcast = make(chan string, 100)
)
