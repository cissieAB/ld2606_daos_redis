package main

import "encoding/json"

// broadcastUpdates sends incremental edge updates to all WebSocket clients.
func broadcastUpdates(updates map[string]PacketSummary) {
	payload, err := json.Marshal(map[string]interface{}{
		"type": "update",
		"data": updates,
	})
	if err != nil {
		errorLog("Error encoding broadcast payload: %v", err)
		return
	}

	select {
	case broadcast <- string(payload):
	default:
		errorLog("Broadcast channel full, dropping update")
	}
}

// broadcastSnapshot sends the complete materialized view when incremental updates are not enough.
func broadcastSnapshot() {
	payload, err := json.Marshal(map[string]interface{}{
		"type": "snapshot",
		"data": latestSnapshot(),
	})
	if err != nil {
		errorLog("Error encoding snapshot payload: %v", err)
		return
	}

	select {
	case broadcast <- string(payload):
	default:
		errorLog("Broadcast channel full, dropping snapshot")
	}
}
