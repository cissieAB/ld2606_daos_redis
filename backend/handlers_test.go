package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleEdge(t *testing.T) {
	latestMu.Lock()
	previousLatest := latest
	latest = map[string]Packet{
		pairKey("192.168.110.1", "192.168.110.2"): {
			Src:              "192.168.110.1",
			Dest:             "192.168.110.2",
			Timestamp:        123,
			SamplesPerSecond: 2,
			TCPPackets:       []int{1, 2},
			TCPBytes:         []int{100, 200},
			UDPPackets:       []int{3, 4},
			UDPBytes:         []int{300, 400},
		},
	}
	latestMu.Unlock()
	t.Cleanup(func() {
		latestMu.Lock()
		latest = previousLatest
		latestMu.Unlock()
	})

	t.Run("returns latest directed edge detail", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodGet,
			"/edge?src=192.168.110.1&dest=192.168.110.2",
			nil,
		)
		response := httptest.NewRecorder()

		handleEdge(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}

		var detail EdgeDetail
		if err := json.NewDecoder(response.Body).Decode(&detail); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if detail.SamplesPerSecond != 2 {
			t.Fatalf("samples_per_second = %d, want 2", detail.SamplesPerSecond)
		}
		if len(detail.TCPPackets) != 2 || len(detail.UDPBytes) != 2 {
			t.Fatalf("expected all sample arrays in response: %+v", detail)
		}
	})

	t.Run("requires both endpoints", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/edge?src=192.168.110.1", nil)
		response := httptest.NewRecorder()

		handleEdge(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns not found for unknown direction", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodGet,
			"/edge?src=192.168.110.2&dest=192.168.110.1",
			nil,
		)
		response := httptest.NewRecorder()

		handleEdge(response, request)

		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
		}
	})
}
