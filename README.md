# LD2606 DAOS Redis Project

Multi-component system for real-time network traffic monitoring with Redis pub/sub integration and DAOS storage.

> 🚀 **Quick Start**: See [GETTING_STARTED.md](GETTING_STARTED.md) for 5-minute setup guide

## Components

### 1. [Backend Server](backend/)
Real-time traffic monitoring server with WebSocket broadcasting
- **Language**: Go
- **Features**: Redis pub/sub, WebSocket, RediSearch integration
- **Purpose**: Receive and broadcast traffic data in real-time

### 2. [Traffic Simulator](traffic-simulator/) 
Mock network traffic data generator (shared component)
- **Language**: Python
- **Features**: Multi-node simulation, configurable packet rates
- **Purpose**: Generate realistic traffic data for testing
- **Used by**: Backend server, DAOS client

### 3. [DAOS Client](daos-client/)
Data persistence layer (to be implemented)
- **Purpose**: Copy data from Redis to DAOS storage
- **Used by**: Other team members


## Architecture

```
┌─────────────────┐
│  Redis Server   │
│  (RediSearch)   │
└────┬────────┬───┘
     │        │
     │        └─────────┐
     ↓                  ↓
┌────────────┐   ┌─────────────┐
│  Backend   │   │ DAOS Client │
│  (Go)      │   │             │
└─────┬──────┘   └─────────────┘
      │
      ↓ WebSocket
┌─────────────┐
│ Web Clients │
└─────────────┘
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
│   ├── simulator.py             # Python simulator
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
