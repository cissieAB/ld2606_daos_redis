package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// handleRoot is a basic test endpoint.
func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

// handleLatest returns the latest timestamp packets.
func handleLatest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Read with RLock (allows concurrent readers)
	latestMu.RLock()
	data := latest
	latestMu.RUnlock()
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode latest", http.StatusInternalServerError)
		return
	}
}
