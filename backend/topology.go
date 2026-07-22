package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// loadTopology reads and validates the backend's static topology file.
func loadTopology(path string) (Topology, error) {
	file, err := os.Open(path)
	if err != nil {
		return Topology{}, err
	}
	defer file.Close()

	var topology Topology
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&topology); err != nil {
		return Topology{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Topology{}, fmt.Errorf("topology file must contain one JSON object")
	}
	if len(topology.Nodes) == 0 {
		return Topology{}, fmt.Errorf("topology must contain at least one node")
	}

	for ip, node := range topology.Nodes {
		if net.ParseIP(ip).To4() == nil {
			return Topology{}, fmt.Errorf("invalid topology IPv4 address %q", ip)
		}
		if node.IP != ip {
			return Topology{}, fmt.Errorf("topology node key %q does not match ip %q", ip, node.IP)
		}
		if strings.TrimSpace(node.Rack) == "" {
			return Topology{}, fmt.Errorf("topology node %q has an empty rack", ip)
		}
	}

	return topology, nil
}
