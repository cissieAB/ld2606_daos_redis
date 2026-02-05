package main

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// upgrader converts HTTP requests to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// handleMessages sends each broadcast message to all connected clients.
func handleMessages() {
	// Read messages from the broadcast channel forever.
	for msg := range broadcast {
		// Send to every connected WebSocket client.
		clientsMu.Lock()
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, []byte(msg))
			if err != nil {
				debugLog("Error sending message to WebSocket: %v", err)
				// Remove client if sending fails.
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

	// Keep the connection open and optionally read messages from client.
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
