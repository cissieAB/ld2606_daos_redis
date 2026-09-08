# Traffic Backend Server

Real-time traffic data server with Redis polling and WebSocket support.

## Table of Contents

- [Features](#features)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [File Structure](#file-structure)
- [Running the Server](#running-the-server)
- [Environment Variables](#environment-variables)
- [API Endpoints](#api-endpoints)
- [Building](#building)
- [Development](#development)

> 📚 **Other Docs**: [Main Project](../README.md) | [Architecture](PROJECT_SUMMARY.md) | [Simulator](../traffic-simulator/README.md) | [DAOS Client](../daos-client/README.md)

## Features

- Redis polling (1s default): live frame selected from the newest timestamp inside the safety window
- WebSocket broadcasting to connected clients
- RediSearch integration for querying historical data
- HTTP REST API for latest traffic data
- Authoritative live snapshots that clear when no timestamp is available in the safety window
- Configurable debug logging

## Prerequisites

- **Go 1.20+**: [Install Go](https://go.dev/doc/install)
- **Redis Server**: With RediSearch module (see [Simulator README](../traffic-simulator/README.md#prerequisites) for Docker/Podman setup)

> **Note**: This is part of the [ld2606_daos_redis](../README.md) project. See main README for overall architecture.

## Quick Start

```bash
# 1. Setup (first time only)
./setup.sh

# 2. Start Redis
# On EJFAT Arma Linux machines, try
# podman run -d -p 6379:6379 --name redis-traffic docker.io/redis/redis-stack-server:latest
docker run -d -p 6379:6379 redis/redis-stack-server:latest

# 3. Run the server
go run .

# 4. Test
curl http://localhost:8080/              # "Hello, World!"
curl http://localhost:8080/latest        # Latest traffic data
```

> 💡 **Tip**: See [GETTING_STARTED.md](../GETTING_STARTED.md) for complete multi-component setup

## File Structure

```
backend/
├── config/topology.json             # Static IP-to-rack node topology
├── main.go                          # Application startup and route wiring
├── config.go                        # Environment configuration and logging helpers
├── topology.go                      # Static topology loading and validation
├── handlers.go                      # HTTP handlers
├── websocket.go                     # WebSocket connection management
├── broadcast.go                     # WebSocket update/snapshot payloads
├── redis.go                         # Redis startup initialization and polling loop
├── redis_index.go                   # RediSearch index and query helpers
├── redis_document.go                # Redis document decoding
├── state.go                         # In-memory selected live frame
├── types.go                         # API payload shapes
├── utils.go                         # Small shared helpers
├── setup.sh                         # Setup script
├── go.mod/go.sum                    # Dependencies
├── README.md                        # This file (usage guide)
└── PROJECT_SUMMARY.md               # Architecture
```

> **Note**: Traffic simulator at [`../traffic-simulator/`](../traffic-simulator/) (shared component)

## Running the Server

```bash
# Production (minimal logs)
go run .

# Debug mode (detailed logs)
DEBUG=true go run .

# Custom configuration
REDIS_ADDR=localhost:6379 SERVER_PORT=:9090 go run .
```

**Log Levels**: INFO (always), ERROR (always), DEBUG (only with `DEBUG=true`)

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DEBUG` | `false` | Enable debug logging (`true` or `1`) |
| `REDIS_ADDR` | `localhost:6379` | Redis server address |
| `REDIS_DB` | `0` | Redis database number |
| `SERVER_PORT` | `:8080` | HTTP server port |
| `POLL_INTERVAL` | `1s` | How often to poll Redis for latest data |
| `TOPOLOGY_PATH` | `config/topology.json` | Static node topology file |

**Examples:**
```bash
REDIS_ADDR=localhost:6379 go run .                    # Local Redis
SERVER_PORT=:9090 DEBUG=1 go run .                    # Custom port + debug
REDIS_ADDR=redis.example.com:6379 SERVER_PORT=:3000 go run .
```

## API Endpoints

### GET /
Test endpoint that returns "Hello, World!"

### GET /latest
Returns the current live graph state as JSON. On each Redis poll, the backend queries the safety window, chooses the newest stored timestamp in that window, and rebuilds `/latest` from only that timestamp. If the window has no timestamp, `data` is empty. The `data` object is keyed by `source_ip:dest_ip`.
```json
{
  "type": "snapshot",
  "data": {
    "10.0.0.1:10.0.0.2": {
      "src": "10.0.0.1",
      "dest": "10.0.0.2",
      "timestamp": 1770147907,
      "tcp_packets_total": 4200,
      "tcp_bytes_total": 2340000,
      "udp_packets_total": 1300,
      "udp_bytes_total": 780000,
      "total_packets": 5500,
      "total_bytes": 3120000
    }
  }
}
```

### WebSocket /ws
Real-time traffic data updates. The backend loads and validates the static node
topology from `config/topology.json` at startup. `TOPOLOGY_PATH` can select a
different file. New connections receive a full `snapshot` containing that
topology and the current edge summaries. Each poll broadcasts an authoritative
edge `snapshot` for the newest timestamp in the safety window, or an empty snapshot
when no live timestamp is available. Clients replace their edge state so missing
edges disappear even while other edges remain active. Poll snapshots omit topology.

```json
{
  "type": "snapshot",
  "topology": {
    "nodes": {
      "192.168.110.1": {
        "ip": "192.168.110.1",
        "rack": "rack-1"
      }
    }
  },
  "data": {}
}
```

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received traffic data:', data);
};
```

### GET /history

Returns timestamp-paginated lightweight graph frames in ascending order. `start` and `end` are inclusive Unix seconds. `limit` defaults to 60 and may not exceed 120 frames. Missing seconds are omitted.

```http
GET /history?start=1783442400&end=1783442520&limit=60
```

When `has_more` is true, use `next_start` as the next request's `start`; it is one second after the final returned frame and therefore does not duplicate it. Historical responses contain no topology or TCP/UDP sample arrays.

### GET /edge

Without a timestamp, returns the latest full sample arrays for one directed edge:

```http
GET /edge?src=192.168.110.1&dest=192.168.110.2
```

Add an exact Unix timestamp to retrieve that stored edge-second without falling back to the nearest or latest record:

```http
GET /edge?src=192.168.110.1&dest=192.168.110.2&timestamp=1783442400
```

Timestamped requests return `400` for invalid IP addresses or timestamps, `404` when the exact record is unavailable or expired, and `503` when Redis is unavailable. The response schema is identical to the latest edge-detail response.

## Building

### Build binary
```bash
go build -o backend
./backend
```

### Build with debug enabled
```bash
DEBUG=1 go build -o backend
```

## Logging Levels

The server uses three logging levels:

- **INFO** (always shown): Important events (startup, connections, initialization)
- **ERROR** (always shown): Error conditions
- **DEBUG** (only with `DEBUG=true`): Detailed operational information

## Development

### Code Organization
The code is organized into focused modules:
- `config.go` - Configuration and logging
- `topology.go` - Static topology loading and validation
- `redis.go` - Redis initialization and polling flow
- `redis_index.go` - RediSearch index and packet queries
- `redis_document.go` - Redis document decoding
- `state.go` - Selected live-frame state
- `broadcast.go` - WebSocket update/snapshot payloads
- `websocket.go` - WebSocket connection handling
- `handlers.go` - HTTP endpoint handlers
- `types.go` - Data structures
- `utils.go` - Small shared helpers

### Mock Data Generation

Use the shared traffic simulator:
```bash
cd ../traffic-simulator
./setup.sh                           # First time only
source venv/bin/activate
python simulator_v2.py --redis-host localhost --mode 1  # timestamps in unix seconds
```

See [`../traffic-simulator/README.md`](../traffic-simulator/README.md) for full simulator documentation.

### Technical Details

For architecture and implementation details, see [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md).

### Optional hostnames

Add `"hostname": "compute-01"` beside a node’s `ip` and `rack` in `config/topology.json` to supply a display name. Existing files need no changes. Empty or omitted names use the IP in the frontend; names need not be unique. The backend includes nonempty names in the initial WebSocket topology. Traffic endpoints and storage keys remain IP-based. Restart the backend and reconnect clients after changing names. `go test ./...` verifies topology loading and hostname serialization.
