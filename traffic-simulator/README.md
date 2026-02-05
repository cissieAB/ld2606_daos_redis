# Traffic Simulator

Python script for generating mock network traffic data and publishing to Redis for testing and development.

> **Part of**: [ld2606_daos_redis](../README.md) project  
> **Shared by**: [Backend Server](../backend/README.md) and [DAOS Client](../daos-client/README.md)

## Table of Contents

1. [Overview & Features](#overview--features)
2. [Prerequisites](#prerequisites)
   - [Redis Server](#redis-server)
3. [Setup](#setup)
   - [Quick Setup (Recommended) ⭐](#quick-setup-recommended)
   - [Manual Setup (Alternative)](#manual-setup-alternative)
4. [Quick Start](#quick-start)

> 📚 **Other Docs**: [Main Project](../README.md) | [Backend](../backend/README.md) | [DAOS Client](../daos-client/README.md)
5. [Usage](#usage)
   - [Basic Usage](#basic-usage)
   - [Advanced Options](#advanced-options)
   - [Command-Line Arguments](#command-line-arguments)
   - [Examples](#examples)
6. [Output](#output)
7. [Data Format](#data-format)
8. [Integration with Backend](#integration-with-backend)
9. [Troubleshooting](#troubleshooting)
10. [Development](#development)

---

## Overview & Features

This simulator generates realistic network packet data with multiple configurable nodes, publishes to Redis pub/sub channels, and stores data in Redis for testing the backend server.

### Features

- **Multi-node simulation**: Simulate multiple traffic sources simultaneously
- **Configurable packet rate**: Control packets per second per node
- **Redis pub/sub**: Publish traffic data to Redis channels in real-time
- **Data persistence**: Store packets as Redis hashes with TTL
- **Statistics tracking**: Monitor packets generated, published, and stored
- **Batch processing**: Efficient batch operations for high throughput

## Prerequisites

[↑ Back to top](#table-of-contents)

### Redis Server

The simulator requires a running Redis server. You can either:
- Use an existing Redis server (production/remote)
- Run Redis locally using Docker or Podman (recommended for development)

#### Option 1: Local Redis with Docker/Podman

**Using Docker:**
```bash
docker run -d -p 6379:6379 redis/redis-stack-server:latest
```

**Using Podman:**
```bash
podman run -d -p 6379:6379 redis/redis-stack-server:latest
```

This will:
- ✅ Start Redis Stack (Redis + RediSearch)
- ✅ Expose port 6379 on localhost
- ✅ Run in detached mode (background)

**Verify Redis is running:**
```bash
# Using Docker
docker ps

# Using Podman  
podman ps
```

#### Option 2: Remote/Existing Redis Server

If you have an existing Redis server (e.g., `ejfat-6.jlab.org`), you can use it directly:
```bash
python3 simulator.py --redis-host ejfat-6.jlab.org
```

**Note:** Make sure the Redis server has RediSearch module installed for the backend to work properly.

## Setup

### Quick Setup (Recommended) ⭐

Use the provided setup script to automatically create the virtual environment and install dependencies:

```bash
cd scripts/traffic-simulator
./setup.sh
```

This will:
- ✅ Check Python 3 installation
- ✅ Create virtual environment (`venv/`)
- ✅ Install all dependencies
- ✅ Display usage instructions

### Manual Setup (Alternative)

If you prefer to set up manually:

#### 1. Create Python Virtual Environment

```bash
cd scripts/traffic-simulator

# Create virtual environment
python3 -m venv venv

# Activate virtual environment
source venv/bin/activate
```

#### 2. Install Dependencies

```bash
pip install -r requirements.txt
```

## Usage

### Basic Usage

[↑ Back to top](#table-of-contents)



Generate traffic with default settings (1 node, 100 packets/sec, publish + storage enabled):

```bash
python3 simulator.py --redis-host ejfat-6.jlab.org
```

**Note:** Publishing and storage are **enabled by default**. Use `--no-publish` or `--no-storage` to disable.

### Advanced Options

```bash
python3 simulator.py \
  --redis-host ejfat-6.jlab.org \
  --redis-port 6379 \
  --nodes 5 \
  --packets-per-second 200 \
  --duration 60 \
  --channel traffic_channel
```

### Command-Line Arguments

[↑ Back to top](#table-of-contents)

| Argument | Default | Description |
|----------|---------|-------------|
| `--redis-host` | `localhost` | Redis server hostname |
| `--redis-port` | `6379` | Redis server port |
| `--redis-db` | `0` | Redis database number |
| `--nodes` | `1` | Number of traffic nodes to simulate |
| `--packets-per-second` (or `--pps`) | `100` | Packets per second per node |
| `--duration` | `None` | Duration in seconds (None = infinite) |
| `--publish` | `True` | Enable pub/sub publishing (default: ON) |
| `--no-publish` | - | Disable pub/sub publishing |
| `--storage` | `True` | Enable data storage (default: ON) |
| `--no-storage` | - | Disable data storage |
| `--channel` | `traffic_channel` | Redis pub/sub channel name |
| `--stats-interval` | `5` | Interval in seconds for printing stats (0 to disable) |

### Examples

[↑ Back to top](#table-of-contents)



### Example 1: Quick Test (10 seconds)

```bash
python3 simulator.py \
  --redis-host ejfat-6.jlab.org \
  --duration 10
```

### Example 2: High Load Test (5 nodes, 500 pps each)

```bash
python3 simulator.py \
  --redis-host ejfat-6.jlab.org \
  --nodes 5 \
  --packets-per-second 500
```

### Example 3: Publish Only (No Storage)

```bash
# Useful for testing pub/sub without filling Redis
python3 simulator.py \
  --redis-host ejfat-6.jlab.org \
  --no-storage
```

### Example 4: Storage Only (No Publishing)

```bash
# Useful for loading data into Redis without pub/sub
python3 simulator.py \
  --redis-host ejfat-6.jlab.org \
  --no-publish \
  --duration 30
```

### Example 5: Custom Channel

```bash
python3 simulator.py \
  --redis-host ejfat-6.jlab.org \
  --channel my_custom_channel
```

## Output

[↑ Back to top](#table-of-contents)

The simulator displays real-time statistics every 5 seconds (configurable):

```
[00:05] Total: 500 packets, Published: 500, Stored: 500, Errors: 0
[00:10] Total: 1,000 packets, Published: 1,000, Stored: 1,000, Errors: 0
[00:15] Total: 1,500 packets, Published: 1,500, Stored: 1,500, Errors: 0
```

**Customize stats interval:**
```bash
# Print stats every 10 seconds
python3 simulator.py --redis-host localhost --stats-interval 10

# Disable periodic stats (only show final summary)
python3 simulator.py --redis-host localhost --stats-interval 0
```

Press `Ctrl+C` to stop the simulator gracefully and see final statistics.

## Data Format

[↑ Back to top](#table-of-contents)

### Published Message (JSON)

```json
{
  "timestamp": 1770147907,
  "packet_count": 100,
  "packets": [
    {
      "timestamp": 1770147907,
      "source_ip": "192.168.45.123",
      "dest_ip": "10.0.78.234",
      "total_bytes": 1024,
      "udp_packets": [...],
      "udp_bytes": [...],
      "tcp_packets": [...],
      "tcp_bytes": [...]
    }
  ]
}
```

### Stored in Redis

Each packet is stored as a hash with key format:
```
packet:{dest_ip}:{source_ip}:{timestamp}
```

Fields:
- `timestamp` - Unix timestamp
- `source_ip` - Source IP address
- `dest_ip` - Destination IP address
- `total_bytes` - Total bytes in packet
- `udp_packets` - JSON array of UDP packet sizes
- `udp_bytes` - JSON array of UDP byte counts
- `tcp_packets` - JSON array of TCP packet sizes
- `tcp_bytes` - JSON array of TCP byte counts

TTL: 1 hour (3600 seconds)

## Quick Start

[↑ Back to top](#table-of-contents)

The fastest way to get started:

```bash
# 1. Start Redis (if not already running)
docker run -d -p 6379:6379 redis/redis-stack-server:latest

# 2. Setup simulator (first time only)
cd scripts/traffic-simulator
./setup.sh

# 3. Activate and run
source venv/bin/activate
python3 simulator.py

# For remote Redis server:
# python3 simulator.py --redis-host ejfat-6.jlab.org
```

## Integration with Other Components

[↑ Back to top](#table-of-contents)

The simulator is a **shared component** used by multiple parts of the system.

### With Backend Server

1. **Start the backend**:
   ```bash
   cd ../backend
   ./setup.sh
   DEBUG=true go run .
   ```

2. **Start the simulator** (in another terminal):
   ```bash
   cd ../traffic-simulator  # or stay in this directory
   ./setup.sh
   source venv/bin/activate
   python3 simulator.py
   ```

3. **Verify**: Backend logs show received packets, WebSocket broadcasts data

See [Backend README](../backend/README.md) for backend setup details.

### With DAOS Client

1. **Generate data**:
   ```bash
   cd ../traffic-simulator
   ./setup.sh
   source venv/bin/activate
   python3 simulator.py
   ```

2. **Run DAOS client** to read and persist data

See [DAOS Client README](../daos-client/README.md) for DAOS setup details.

## Troubleshooting

[↑ Back to top](#table-of-contents)

### Connection Errors

**Error: "Connection refused"**
- Make sure Redis is running: `docker ps` or `podman ps`
- Start Redis if needed: `docker run -d -p 6379:6379 redis/redis-stack-server:latest`

**Error: "Connection timeout"**
```bash
# Check if Redis container is running
docker ps | grep redis              # Using Docker
podman ps | grep redis              # Using Podman

# Check if port 6379 is listening
lsof -i:6379                       
```

### High Memory Usage

Reduce packet rate or enable TTL (already enabled, 1 hour default)

### Publishing Not Working

- Publishing is enabled by default
- Verify channel name matches backend configuration: `--channel traffic_channel`
- Check if backend is subscribed to the correct channel

## Development

[↑ Back to top](#table-of-contents)

The simulator is self-contained and can be modified independently of the Go backend. Key classes:

- `PacketGenerator`: Generates packet data
- `TrafficNode`: Represents a traffic source
- `TrafficSimulator`: Coordinates multiple nodes

See the source code for detailed documentation.
