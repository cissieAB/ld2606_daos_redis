# LDRD Traffic Monitoring

Multi-component system for generating, storing, and visualizing simulated network traffic with Redis Stack, a Go backend, and WebSocket clients.

> **Quick Start**: See [GETTING_STARTED.md](GETTING_STARTED.md) for the full multi-terminal setup.

## Components

### 1. [Backend Server](backend/)
Real-time traffic API and WebSocket server.

- **Language**: Go
- **Data source**: Polls Redis Stack / RediSearch for `packet:*` hashes
- **State model**: Maintains latest traffic per `source_ip:dest_ip` pair
- **API**: `/latest` returns a `snapshot`; `/ws` streams `snapshot` and `update` messages
- **Cleanup behavior**: Prunes stale in-memory pairs when simulator runs replace active Redis data

### 2. [Traffic Simulator](traffic-simulator/) 
High-throughput mock traffic generator.

- **Language**: Python
- **Primary script**: `simulator_v2.py`
- **Features**: Multi-node simulation, configurable packets/sec, TTL, Redis DB selection, and storage modes
- **Redis format**: Writes packet hashes using `packet:{dest_ip}:{source_ip}:{timestamp}`
- **Topology behavior**: Generates bounded node-to-node traffic based on `--nodes`
- **Used by**: Backend server, DAOS client

### 3. [DAOS Client](daos-client/)
Data persistence layer (to be implemented)
- **Purpose**: Copy data from Redis to DAOS storage
- **Used by**: Other team members


## Architecture

```
┌──────────────────────┐
│ Traffic Simulator v2 │
│ Python threads       │
└──────────┬───────────┘
           │ packet:* hashes
           ↓
┌──────────────────────┐
│ Redis Stack          │
│ Redis + RediSearch   │
└──────┬─────────┬─────┘
       │         │
       │         └──────────────┐
       ↓                        ↓
┌──────────────────────┐   ┌─────────────┐
│ Backend Server       │   │ DAOS Client │
│ Go polling + state   │   │             │
└──────────┬───────────┘   └─────────────┘
           │ WebSocket /ws
           ↓
┌──────────────────────┐
│ Frontend / Web Client│
└──────────────────────┘
```

## Repository Structure

```
ld2606_daos_redis/
├── README.md                    # This file
├── backend/                     # Go server
│   ├── *.go                     # Go source files
│   ├── setup.sh                 # Setup script
│   └── README.md                # Backend documentation
├── traffic-simulator/           # Shared mock generator
│   ├── simulator_v2.py          # Current Redis hash simulator
│   ├── simulator_bk.py          # Older simulator flow
│   ├── setup.sh                 # Setup script
│   └── README.md                # Simulator documentation
├── daos-client/                 # DAOS client (TBD)
│   └── ...
```

## Documentation

| Document | Description |
|----------|-------------|
| **[GETTING_STARTED.md](GETTING_STARTED.md)** | 5-minute setup guide |
| **[README.md](README.md)** | This file - project overview |
| **[CONTRIBUTING.md](CONTRIBUTING.md)** | Development guidelines |
| **[backend/README.md](backend/README.md)** | Go server setup and usage |
| **[backend/PROJECT_SUMMARY.md](backend/PROJECT_SUMMARY.md)** | Backend Technical architecture |
| **[traffic-simulator/README.md](traffic-simulator/README.md)** | Shared mock data generator usage |
| **[daos-client/README.md](daos-client/README.md)** | DAOS storage layer (TBD) |
