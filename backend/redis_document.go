package main

import (
	"encoding/json"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// docToPacket converts a RediSearch document into the packet shape used by the server.
func docToPacket(doc redis.Document) (Packet, error) {
	var p Packet
	p.Key = doc.ID

	fields := doc.Fields

	mustInt := func(k string) int {
		v, ok := fields[k]
		if !ok {
			return 0
		}
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0
		}
		return i
	}

	mustStr := func(k string) string {
		v, ok := fields[k]
		if !ok {
			return ""
		}
		return v
	}

	decode := func(k string) []int {
		raw, ok := fields[k]
		if !ok {
			return nil
		}
		var out []int
		_ = json.Unmarshal([]byte(raw), &out)
		return out
	}

	p.Timestamp = mustInt("timestamp")
	p.Seq = mustInt("seq")
	p.NodeID = mustInt("node_id")
	p.Src = mustStr("source_ip")
	p.Dest = mustStr("dest_ip")
	p.TotalBytes = mustInt("total_bytes")
	p.UDPPackets = decode("udp_packets")
	p.UDPBytes = decode("udp_bytes")
	p.TCPPackets = decode("tcp_packets")
	p.TCPBytes = decode("tcp_bytes")

	return p, nil
}
