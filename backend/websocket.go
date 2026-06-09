// Package main implements a real-time traffic data server.
package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	// clients is the set of connected WebSocket clients.
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex

	// broadcast is buffered so Redis polling is not blocked by slow clients.
	broadcast = make(chan string, 100)
)

// upgrader converts HTTP requests to WebSocket connections and allows all origins.
var upgrader = websocket.Upgrader{
	// CheckOrigin allows connections from any origin (for development).
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// handleMessages broadcasts updates to all connected WebSocket clients.
// It runs continuously, sending each message from the broadcast channel to all clients.
func handleMessages() {
	// Read messages from the broadcast channel forever.
	for msg := range broadcast {
		// Send to every connected WebSocket client.
		clientsMu.Lock()
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, []byte(msg))
			if err != nil {
				debugLog("Error sending message to WebSocket: %v", err)
				// Remove client if sending fails (connection broken).
				delete(clients, client)
				debugLog("Client disconnected: %s", client.RemoteAddr())
			}
		}
		clientsMu.Unlock()
	}
}

// handleWebSocket handles WebSocket connections for real-time updates.
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		errorLog("Error upgrading to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	// Register this client for broadcasts.
	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	infoLog("WebSocket connection established: %s", conn.RemoteAddr())

	// 1. SEND SNAPSHOT IMMEDIATELY
	snapshot := latestSnapshot()

	err = conn.WriteJSON(map[string]interface{}{
		"type": "snapshot",
		"data": snapshot,
	})
	if err != nil {
		errorLog("Failed to send snapshot: %v", err)
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
		return
	}

	// 2. Keep the connection alive
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			debugLog("WebSocket connection closed: %s", conn.RemoteAddr())
			// Remove client when it disconnects.
			clientsMu.Lock()
			delete(clients, conn)
			clientsMu.Unlock()
			return
		}
		// Currently we just log client messages.
		debugLog("Received message from WebSocket client: %s", string(msg))
	}
}
