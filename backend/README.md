# Traffic Backend Server

Real-time traffic data server with Redis pub/sub integration and WebSocket support.

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

- Redis pub/sub subscription for real-time traffic data
- WebSocket broadcasting to connected clients
- RediSearch integration for querying historical data
- HTTP REST API for latest traffic data
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
├── *.go files                       # Source code
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
| `REDIS_ADDR` | `ejfat-6.jlab.org:6379` | Redis server address |
| `SERVER_PORT` | `:8080` | HTTP server port |
| `REDIS_CHANNEL` | `traffic_channel` | Redis pub/sub channel |

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
Returns the latest traffic data as JSON:
```json
{
  "timestamp": 1770147907,
  "packet_count": 42,
  "packets": [...]
}
```

### WebSocket /ws
Real-time traffic data updates. Connect using WebSocket client:
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
- `redis.go` - Redis operations and pub/sub
- `websocket.go` - WebSocket handling
- `handlers.go` - HTTP endpoint handlers
- `types.go` - Data structures

### Mock Data Generation

Use the shared traffic simulator:
```bash
cd ../traffic-simulator
./setup.sh                           # First time only
source venv/bin/activate
python simulator.py --redis-host localhost
```

See [`../traffic-simulator/README.md`](../traffic-simulator/README.md) for full simulator documentation.

### Technical Details

For architecture and implementation details, see [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md).
