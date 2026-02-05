# Traffic Backend Server - Technical Documentation

> **Part of**: [ld2606_daos_redis](../README.md) project  
> **Component**: Backend Go Server  
> **Related**: [User Guide](README.md) | [Simulator](../traffic-simulator/README.md) | [DAOS Client](../daos-client/README.md)

## Table of Contents

- [Overview](#overview)
- [Architecture at a Glance](#architecture-at-a-glance)
- [Data Flow](#data-flow)
  - [Startup](#startup)
  - [Runtime (Per Pub/Sub Message)](#runtime-per-pubsub-message)
  - [API Surface](#api-surface)
- [Concurrency & Thread Safety](#concurrency--thread-safety)
  - [Shared State and Locks](#shared-state-and-locks)
  - [Broadcasting via Channel (Producer → Consumer)](#broadcasting-via-channel-producer--consumer)
- [Configuration Architecture](#configuration-architecture)
- [Development Workflow](#development-workflow)
  - [Typical Development Cycle](#typical-development-cycle)
- [Testing Strategy](#testing-strategy)
  - [Manual Testing](#manual-testing)
  - [Load Testing](#load-testing)
  - [Concurrency Testing](#concurrency-testing)
- [Technology Stack](#technology-stack)
- [Dependencies](#dependencies)
- [Related Documentation](#related-documentation)

## Overview

This document provides **technical details** for developers working on or extending the backend.  
For usage instructions, see [README.md](README.md).

## Architecture at a Glance

```
                           ┌─────────────────┐
                           │  Redis Server   │
                           │  (RediSearch)   │
                           └────────┬────────┘
                                    │ Pub/Sub + Search
                                    ↓
                           ┌─────────────────┐
                           │  Go Backend     │
                           │  - Redis Sub    │
                           │  - Aggregation  │
                           │  - WebSocket    │
                           └────────┬────────┘
                                    │ WebSocket
                                    ↓
                           ┌─────────────────┐
                           │  Web Clients    │
                           │  (Real-time)    │
                           └─────────────────┘
```

## Data Flow

### Startup

- Load configuration (`config.go`)
- Connect to Redis
- Create/verify RediSearch index (`redis.go`)
- Read the latest timestamp and build the initial in-memory snapshot (`latest`)
- Start background goroutines:
  - `startRedisSubscriber()` (`redis.go`) — consumes Redis pub/sub and updates state
  - `handleMessages()` (`websocket.go`) — broadcasts messages to WebSocket clients
- Start HTTP server (`main.go`)

### Runtime (Per Pub/Sub Message)

For every message on the Redis pub/sub channel, the subscriber does two things in parallel paths:

- **Real-time path**: enqueue the raw payload into `broadcast` (consumed by `handleMessages()` for fan-out)
- **Aggregated state path**: unmarshal payload and update `latest` (accumulate/replace based on timestamp)

### API Surface

- `GET /latest`: returns the current in-memory `latest` snapshot (read-locked)
- `GET /`: basic test endpoint
- `GET /ws` (WebSocket): streams pub/sub payloads to connected clients

## Concurrency & Thread Safety

The server runs concurrently using goroutines and a small shared state surface.

### Shared State and Locks

| Shared item | Type | Protection | Why |
|-----------|------|------------|-----|
| `latest` | `latestData` | `latestMu sync.RWMutex` | read-heavy (`/latest`) with periodic writes (subscriber) |
| `clients` | `map[*websocket.Conn]bool` | `clientsMu sync.Mutex` | iteration + deletes on write errors; not a pure-read workload |
| `broadcast` | `chan string` | channel semantics | safe for concurrent send/receive |

**RWMutex usage (`latest`)**
- **Writer**: subscriber + initialization use `latestMu.Lock()` when updating `latest`
- **Readers**: `/latest` handler uses `latestMu.RLock()` and copies a snapshot

```go
// Reader pattern (handlers.go)
latestMu.RLock()
snapshot := latest
latestMu.RUnlock()
json.NewEncoder(w).Encode(snapshot)
```

### Broadcasting via Channel (Producer → Consumer)

Broadcasting is implemented as a producer/consumer pipeline using a **buffered channel**.

**Channel** (`types.go`)
```go
var broadcast = make(chan string, 100)
```

**Producer** (`startRedisSubscriber()` in `redis.go`)
```go
broadcast <- msg.Payload // blocks only if the buffer is full (backpressure)
```

**Consumer** (`handleMessages()` in `websocket.go`)
- reads `broadcast`
- fans out to all `clients`
- removes dead clients on write error

This design keeps the Redis subscriber independent from WebSocket connection management, while still providing backpressure when broadcasts can’t keep up.

## Configuration Architecture

All configuration is environment-based:
- Loaded in `config.go` via `initConfig()`
- Defaults provided for development
- Override via environment variables

See [README.md](README.md#configuration) for complete configuration reference.

## Development Workflow

See [README.md](README.md) for setup and usage instructions.

### Typical Development Cycle
1. Make code changes
2. Run with debug: `DEBUG=true go run .`
3. Test with simulator: `cd ../traffic-simulator && python simulator.py --redis-host localhost`
4. Verify WebSocket broadcasts
5. Check `/latest` endpoint

## Testing Strategy

### Manual Testing

1. Use traffic simulator (`../traffic-simulator/`) for data generation
2. Enable debug logging: `DEBUG=true go run .`
3. Monitor logs and WebSocket broadcasts
4. Verify API endpoints with concurrent requests

### Load Testing
- Multiple concurrent clients: Simulate many `/latest` requests
- Traffic simulator with multiple nodes: `--nodes 5`
- High packet rates: `--packets-per-second 1000`
- Monitor system resources with `top` or `htop`

### Concurrency Testing

Use the race detector to validate thread-safety (locks + channel usage):

```bash
go run -race .
go test -race ./...
```

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Language** | Go 1.20+ | Performance, concurrency |
| **Database** | Redis + RediSearch | Pub/sub, indexing, queries |
| **WebSocket** | gorilla/websocket | Real-time broadcasting |
| **Redis Client** | go-redis/v9 | Redis operations |


## Dependencies

```go
require (
    github.com/gorilla/websocket v1.5.3
    github.com/redis/go-redis/v9 v9.17.3
)
```

## Related Documentation

- [README.md](README.md) - Usage guide (start here)
- [../traffic-simulator/README.md](../traffic-simulator/README.md) - Mock data generator
- [../README.md](../README.md) - Main project overview
- This file - Technical architecture details

