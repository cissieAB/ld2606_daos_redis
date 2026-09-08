package main

import "encoding/json"

// broadcastSnapshot sends the complete selected live frame.
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
