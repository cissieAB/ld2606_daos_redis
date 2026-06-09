// Package main implements a real-time traffic data server.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// handleRoot is a basic health check endpoint.
func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

// handleLatest returns a JSON snapshot of the latest packets (latest state for each src:dest pair).
func handleLatest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	snapshot := latestSnapshot()

	response := map[string]interface{}{
		"type": "snapshot",
		"data": snapshot,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode latest", http.StatusInternalServerError)
		return
	}
}
