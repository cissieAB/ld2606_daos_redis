package main

import (
	"sync"
)

const (
	// safetyWindow is the lookback duration, in seconds, used to tolerate clock skew.
	safetyWindow = 2
)

var (
	// latest is the selected live frame keyed by "source_ip:dest_ip".
	latest   = make(map[string]Packet)
	latestMu sync.RWMutex
)

func replaceLatest(packets []Packet) {
	next := make(map[string]Packet, len(packets))
	for _, packet := range packets {
		if !validHistoryPacket(packet) {
			continue
		}
		next[pairKey(packet.Src, packet.Dest)] = packet
	}

	latestMu.Lock()
	latest = next
	latestMu.Unlock()
}

func generateEdgeSummary(packet Packet) PacketSummary {
	tcpPacketsTotal := Sum(packet.TCPPackets)
	tcpBytesTotal := Sum(packet.TCPBytes)
	udpPacketsTotal := Sum(packet.UDPPackets)
	udpBytesTotal := Sum(packet.UDPBytes)

	return PacketSummary{
		Src:              packet.Src,
		Dest:             packet.Dest,
		Timestamp:        packet.Timestamp,
		SamplesPerSecond: packet.SamplesPerSecond,

		TCPPacketsTotal: tcpPacketsTotal,
		TCPBytesTotal:   tcpBytesTotal,

		UDPPacketsTotal: udpPacketsTotal,
		UDPBytesTotal:   udpBytesTotal,

		TotalPackets: tcpPacketsTotal + udpPacketsTotal,
		TotalBytes:   tcpBytesTotal + udpBytesTotal,
	}
}

func latestEdgeDetail(src, dest string) (EdgeDetail, bool) {
	latestMu.RLock()
	defer latestMu.RUnlock()

	packet, ok := latest[pairKey(src, dest)]
	if !ok {
		return EdgeDetail{}, false
	}

	return generateEdgeDetail(packet), true
}

func generateEdgeDetail(packet Packet) EdgeDetail {
	return EdgeDetail{
		Src:              packet.Src,
		Dest:             packet.Dest,
		Timestamp:        packet.Timestamp,
		SamplesPerSecond: packet.SamplesPerSecond,
		UDPPackets:       append([]int(nil), packet.UDPPackets...),
		UDPBytes:         append([]int(nil), packet.UDPBytes...),
		TCPPackets:       append([]int(nil), packet.TCPPackets...),
		TCPBytes:         append([]int(nil), packet.TCPBytes...),
	}
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

	debugLog("Initialized with empty live frame")
}
