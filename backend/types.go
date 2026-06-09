// Package main implements a real-time traffic data server.
package main

// Packet represents a network packet with arbitrary fields.
type Packet struct {
	Key string `json:"_key"`

	Timestamp  int    `json:"timestamp"`
	Seq        int    `json:"seq"`
	NodeID     int    `json:"node_id"`
	Src        string `json:"source_ip"`
	Dest       string `json:"dest_ip"`
	TotalBytes int    `json:"total_bytes"`

	UDPPackets []int `json:"udp_packets"`
	UDPBytes   []int `json:"udp_bytes"`
	TCPPackets []int `json:"tcp_packets"`
	TCPBytes   []int `json:"tcp_bytes"`
}

// PacketSummary is the compact edge payload sent to the frontend.
type PacketSummary struct {
	Src       string `json:"src"`
	Dest      string `json:"dest"`
	Timestamp int    `json:"timestamp"`

	TCPPacketsTotal int `json:"tcp_packets_total"`
	TCPBytesTotal   int `json:"tcp_bytes_total"`

	UDPPacketsTotal int `json:"udp_packets_total"`
	UDPBytesTotal   int `json:"udp_bytes_total"`

	TotalPackets int `json:"total_packets"`
	TotalBytes   int `json:"total_bytes"`
}
