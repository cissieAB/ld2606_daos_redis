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

- Redis polling (1s default): materialized view of latest packet per `src:dest` pair
- WebSocket broadcasting to connected clients
- RediSearch integration for querying historical data
- HTTP REST API for latest traffic data
- Stale pair pruning when simulator runs replace the active Redis data
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
├── main.go                          # Application startup and route wiring
├── config.go                        # Environment configuration and logging helpers
├── handlers.go                      # HTTP handlers
├── websocket.go                     # WebSocket connection management
├── broadcast.go                     # WebSocket update/snapshot payloads
├── redis.go                         # Redis startup initialization and polling loop
├── redis_index.go                   # RediSearch index and query helpers
├── redis_document.go                # Redis document decoding
├── state.go                         # In-memory latest src:dest materialized view
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
Returns the current materialized graph state as JSON. The `data` object is keyed by `source_ip:dest_ip`.
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
Real-time traffic data updates. New connections receive a full `snapshot`; normal polls send `update` messages with changed edges. If stale pairs are pruned, the backend sends another full `snapshot`.
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received traffic data:', data);
};
```

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
- `redis.go` - Redis initialization and polling flow
- `redis_index.go` - RediSearch index and packet queries
- `redis_document.go` - Redis document decoding
- `state.go` - Materialized latest `src:dest` state and pruning
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
