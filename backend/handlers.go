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

// handleEdge returns the latest full sample arrays for one directed edge.
func handleEdge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	src := r.URL.Query().Get("src")
	dest := r.URL.Query().Get("dest")
	if src == "" || dest == "" {
		http.Error(w, "Both src and dest query parameters are required", http.StatusBadRequest)
		return
	}

	detail, ok := latestEdgeDetail(src, dest)
	if !ok {
		http.Error(w, "Edge not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(detail); err != nil {
		http.Error(w, "Failed to encode edge", http.StatusInternalServerError)
	}
}
