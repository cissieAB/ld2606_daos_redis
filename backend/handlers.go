// Package main implements a real-time traffic data server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type historyLoader func(context.Context, int, int, int) (HistoryResponse, error)

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

func handleHistory(load historyLoader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		start, err := requiredNonNegativeInt(r, "start")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		end, err := requiredNonNegativeInt(r, "end")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if start > end {
			writeJSONError(w, http.StatusBadRequest, "start must be less than or equal to end")
			return
		}

		limit := defaultHistoryLimit
		if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
			limit, err = strconv.Atoi(rawLimit)
			if err != nil || limit < 1 || limit > maxHistoryLimit {
				writeJSONError(w, http.StatusBadRequest, "limit must be between 1 and 120")
				return
			}
		}

		response, err := load(r.Context(), start, end, limit)
		if err != nil {
			errorLog("History query failed: %v", err)
			writeJSONError(w, http.StatusServiceUnavailable, "history unavailable")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			errorLog("Failed to encode history response: %v", err)
		}
	}
}

func requiredNonNegativeInt(r *http.Request, name string) (int, error) {
	raw := r.URL.Query().Get(name)
	value, err := strconv.Atoi(raw)
	if raw == "" || err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return value, nil
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
