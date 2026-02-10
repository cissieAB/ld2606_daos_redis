# DAOS Client

## Overview

This component handles copying data from Redis to DAOS (Distributed Asynchronous Object Storage) for persistent storage.

## Status

🚧 **To be implemented by other team members**

## Purpose

- Read traffic data from Redis
- Process and transform data as needed
- Store data persistently in DAOS
- Handle data lifecycle management

## Integration Points

### 1. Redis Connection
- Connect to same Redis instance as backend
- Read from Redis hashes (key format: `packet:{dest_ip}:{source_ip}:{timestamp}`)

### 2. Data Format

Each packet in Redis is stored as a hash with fields:
```
packet:{dest_ip}:{source_ip}:{timestamp}
  - timestamp: Unix timestamp
  - source_ip: Source IP address
  - dest_ip: Destination IP address
  - total_bytes: Total bytes (string)
  - udp_packets: JSON array
  - udp_bytes: JSON array
  - tcp_packets: JSON array
  - tcp_bytes: JSON array
```

### 3. Traffic Simulator

Use the shared traffic simulator for testing:
```bash
cd ../traffic-simulator
./setup.sh
source venv/bin/activate
python simulator.py --redis-host localhost
```

See [`../traffic-simulator/README.md`](../traffic-simulator/README.md) for details.

## Development Setup

### Prerequisites
- DAOS client libraries
- Redis connection (localhost:6379 or remote)
- Access to traffic simulator

### Testing Workflow
1. Start Redis: `docker run -d -p 6379:6379 redis/redis-stack-server:latest`
2. Generate test data: Run traffic simulator
3. Develop DAOS client to read and store data
4. Verify data persistence in DAOS

## Documentation

- Main project: [`../README.md`](../README.md)
- Backend API: [`../backend/README.md`](../backend/README.md)
- Traffic simulator: [`../traffic-simulator/README.md`](../traffic-simulator/README.md)
