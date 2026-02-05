# Getting Started with ld2606_daos_redis

Quick guide to get the entire system running in 5 minutes.

## Prerequisites

Install these first:
- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/getting-started/installation)
- [Go 1.20+](https://go.dev/doc/install)
- [Python 3.9+](https://www.python.org/downloads/)

## 5-Minute Setup

### Step 1: Clone Repository
```bash
git clone https://github.com/cissieAB/ld2606_daos_redis.git
cd ld2606_daos_redis
```

### Step 2: Start Redis
```bash
docker run -d -p 6379:6379 --name redis-traffic redis/redis-stack-server:latest
```

### Step 3: Setup Backend
```bash
cd backend
./setup.sh
```

### Step 4: Setup Traffic Simulator
```bash
cd ../traffic-simulator
./setup.sh
```

### Step 5: Run Everything

**Terminal 1 - Backend Server:**
```bash
cd backend
go run .
```

**Terminal 2 - Traffic Simulator:**
```bash
cd traffic-simulator
source venv/bin/activate
python simulator.py
```

**Terminal 3 - Test:**
```bash
# Test REST API
curl http://localhost:8080/
curl http://localhost:8080/latest

# Test WebSocket (requires websocat or browser console)
websocat ws://localhost:8080/ws
```

## What You Should See

**Backend (Terminal 1):**
```
[INFO] Index 'idx:packets' created successfully
[INFO] Initialized with data: timestamp=1770147907 packet_count=42 packets=42
[INFO] Subscribed to traffic_channel
[INFO] Starting server on :8080 (Debug: false)
```

**Simulator (Terminal 2):**
```
✓ Connected to Redis at localhost:6379
Redis Traffic Simulator
Nodes: 1, Packets/sec: 100
[00:05] Total: 500 packets, Published: 500, Stored: 500, Errors: 0
[00:10] Total: 1,000 packets, Published: 1,000, Stored: 1,000, Errors: 0
```

## Next Steps

### For Backend Development
- Read [backend/README.md](backend/README.md)
- Check [backend/PROJECT_SUMMARY.md](backend/PROJECT_SUMMARY.md) for architecture
- Enable debug mode: `DEBUG=true go run .`

### For Traffic Simulator
- Read [traffic-simulator/README.md](traffic-simulator/README.md)
- Try different configurations
- Adjust packet rates: `--packets-per-second 500`

### For DAOS Client Development
- Read [daos-client/README.md](daos-client/README.md)
- Use traffic simulator for test data

## Common Issues

### "Cannot connect to Redis"
```bash
# Check if Redis is running
docker ps | grep redis

# Restart Redis
docker restart redis-traffic

# Or start fresh
docker run -d -p 6379:6379 --name redis-traffic redis/redis-stack-server:latest
```

### "Port 8080 already in use"
```bash
# Use different port
SERVER_PORT=:9090 go run .
```

### "Go dependencies fail"
```bash
cd backend
go mod download
go mod tidy
```

## Stopping Everything

```bash
# Stop simulator: Ctrl+C in Terminal 2
# Stop backend: Ctrl+C in Terminal 1
# Stop Redis:
docker stop redis-traffic
docker rm redis-traffic
```

## Documentation

- **[Main README](README.md)** - Project overview
- **[Backend README](backend/README.md)** - Backend server
- **[Simulator README](traffic-simulator/README.md)** - Traffic generator
- **[DAOS README](daos-client/README.md)** - DAOS client
- **[Contributing](CONTRIBUTING.md)** - Development guidelines
