// Package main implements a real-time traffic data server.
package main

// Packet represents a network packet with arbitrary fields.
type Packet struct {
	Key string `json:"_key"`

	Timestamp        int    `json:"timestamp"`
	Seq              int    `json:"seq"`
	NodeID           int    `json:"node_id"`
	Src              string `json:"source_ip"`
	Dest             string `json:"dest_ip"`
	SamplesPerSecond int    `json:"samples_per_second"`
	TotalBytes       int    `json:"total_bytes"`

	UDPPackets []int `json:"udp_packets"`
	UDPBytes   []int `json:"udp_bytes"`
	TCPPackets []int `json:"tcp_packets"`
	TCPBytes   []int `json:"tcp_bytes"`
}

// PacketSummary is the compact edge payload sent to the frontend.
type PacketSummary struct {
	Src              string `json:"src"`
	Dest             string `json:"dest"`
	Timestamp        int    `json:"timestamp"`
	SamplesPerSecond int    `json:"samples_per_second"`

	TCPPacketsTotal int `json:"tcp_packets_total"`
	TCPBytesTotal   int `json:"tcp_bytes_total"`

	UDPPacketsTotal int `json:"udp_packets_total"`
	UDPBytesTotal   int `json:"udp_bytes_total"`

	TotalPackets int `json:"total_packets"`
	TotalBytes   int `json:"total_bytes"`
}

// EdgeDetail is the full latest payload for one directed edge.
type EdgeDetail struct {
	Src              string `json:"src"`
	Dest             string `json:"dest"`
	Timestamp        int    `json:"timestamp"`
	SamplesPerSecond int    `json:"samples_per_second"`

	UDPPackets []int `json:"udp_packets"`
	UDPBytes   []int `json:"udp_bytes"`
	TCPPackets []int `json:"tcp_packets"`
	TCPBytes   []int `json:"tcp_bytes"`
}

// NodeMetadata describes one statically located network node.
type NodeMetadata struct {
	IP       string `json:"ip"`
	Rack     string `json:"rack"`
	Hostname string `json:"hostname,omitempty"`
}

// Topology is the complete IP-keyed node mapping for a WebSocket session.
type Topology struct {
	Nodes map[string]NodeMetadata `json:"nodes"`
}

// HistoryFrame is one complete lightweight graph snapshot at a stored timestamp.
type HistoryFrame struct {
	Timestamp int                      `json:"timestamp"`
	Data      map[string]PacketSummary `json:"data"`
}

// HistoryResponse is one timestamp-paginated chunk of graph history.
type HistoryResponse struct {
	Start     int            `json:"start"`
	End       int            `json:"end"`
	Frames    []HistoryFrame `json:"frames"`
	NextStart *int           `json:"next_start,omitempty"`
	HasMore   bool           `json:"has_more"`
}
