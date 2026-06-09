package main

import (
	"sync"

	"github.com/redis/go-redis/v9"
)

const (
	// safetyWindow is the lookback duration, in seconds, used to tolerate clock skew.
	safetyWindow = 2
)

var (
	// latest is the materialized view: latest packet per "source_ip:dest_ip" pair.
	latest   = make(map[string]Packet)
	latestMu sync.RWMutex

	// startingTimestamp tracks the Redis poll watermark.
	startingTimestamp int
)

func pollSinceTimestamp() int {
	since := startingTimestamp - safetyWindow
	if since < 0 {
		return 0
	}
	return since
}

func setStartingTimestamp(ts int) {
	startingTimestamp = ts
}

func getStartingTimestamp() int {
	return startingTimestamp
}

func upsertPacket(packet Packet) bool {
	key := pairKey(packet.Src, packet.Dest)

	incomingTs := packet.Timestamp
	if incomingTs == 0 {
		return false
	}

	latestMu.Lock()
	defer latestMu.Unlock()

	if existing, exists := latest[key]; exists {
		existingTs := existing.Timestamp

		if incomingTs < existingTs {
			return false
		}

		if packet.Key != "" && packet.Key == existing.Key {
			return false
		}
	}

	latest[key] = packet
	return true
}

func generateEdgeSummary(packet Packet) PacketSummary {
	tcpPacketsTotal := Sum(packet.TCPPackets)
	tcpBytesTotal := Sum(packet.TCPBytes)
	udpPacketsTotal := Sum(packet.UDPPackets)
	udpBytesTotal := Sum(packet.UDPBytes)

	return PacketSummary{
		Src:       packet.Src,
		Dest:      packet.Dest,
		Timestamp: packet.Timestamp,

		TCPPacketsTotal: tcpPacketsTotal,
		TCPBytesTotal:   tcpBytesTotal,

		UDPPacketsTotal: udpPacketsTotal,
		UDPBytesTotal:   udpBytesTotal,

		TotalPackets: tcpPacketsTotal + udpPacketsTotal,
		TotalBytes:   tcpBytesTotal + udpBytesTotal,
	}
}

func pruneStalePackets(cutoff int) int {
	latestMu.Lock()
	defer latestMu.Unlock()

	pruned := 0
	for key, packet := range latest {
		if packet.Timestamp < cutoff {
			delete(latest, key)
			pruned++
		}
	}
	return pruned
}

// applyDocuments updates the materialized view and reports incremental updates plus prune status.
func applyDocuments(docs []redis.Document) (map[string]PacketSummary, bool) {
	updates := make(map[string]PacketSummary, len(docs))
	maxTs := getStartingTimestamp()

	for _, doc := range docs {
		packet, err := docToPacket(doc)
		if err != nil {
			debugLog("Skipping document: %v", err)
			continue
		}

		if packet.Src == "" || packet.Dest == "" {
			continue
		}

		if packet.Timestamp > maxTs {
			maxTs = packet.Timestamp
		}

		if upsertPacket(packet) {
			key := pairKey(packet.Src, packet.Dest)
			updates[key] = generateEdgeSummary(packet)
		}
	}

	setStartingTimestamp(maxTs)
	pruned := pruneStalePackets(pollSinceTimestamp())
	return updates, pruned > 0
}

func latestSnapshot() map[string]PacketSummary {
	latestMu.RLock()
	defer latestMu.RUnlock()

	snapshot := make(map[string]PacketSummary, len(latest))
	for key, packet := range latest {
		snapshot[key] = generateEdgeSummary(packet)
	}
	return snapshot
}

func hasLatestPackets() bool {
	latestMu.RLock()
	defer latestMu.RUnlock()
	return len(latest) > 0
}

func initializeEmptyLatest() {
	latestMu.Lock()
	latest = make(map[string]Packet)
	latestMu.Unlock()

	setStartingTimestamp(0)
	debugLog("Initialized with empty materialized view")
}
