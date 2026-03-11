
# Traffic Simulator Measurements

### Redis `HSET` implementation
#### v1: No measurement, Pub-Sub and Write
No measurements are included.

#### v2: Write only, measure Redis P95/P99 latency *at the sender-side*

- By default, each "node" generates 100 records per second, and each record contains a full record with:
    - 4 fine-grained bins of length `bin_no` (100 by default);
    - An integer indicating `aggregated_bytes`;
    - Metadata including `timestamp`, `src_ip`, `dst_ip`, and `node_id`.
- We can launch multiple nodes per run.
- Using the same `HSET` key and data format as [v1](./simulator.py).



```python
# For each record
    # Key format: packet:{destIP}:{srcIP}{timestamp_ms}
    key = (f"{self.key_format}:{packet['dest_ip']}:{packet['source_ip']}"
            f":{packet['timestamp_ms']}")
    
    pipeline.hset(key, mapping={
        "timestamp_ms": packet["timestamp_ms"],
        "node_id": packet["node_id"],
        "source_ip": packet["source_ip"],
        "dest_ip": packet["dest_ip"],
        "total_bytes": packet["total_bytes"],
        "udp_packets": packet["udp_packets"],
        "udp_bytes": packet["udp_bytes"],
        "tcp_packets": packet["tcp_packets"],
        "tcp_bytes": packet["tcp_bytes"],
    })
    pipeline.expire(key, ttl)
```
                
### Measurements of ejfat-5 ==> ejfat-6
[Simulator_v2](./simulator_v2.py)

```bash
# On the sender-side
(venv) bash-5.1$ python3 simulator_v2.py --nodes 32 --duration 60 --pps <pps> --redis-host 129.57.177.136    # Match the server's MANAGEMENT IP

# On the Redis server (IP: 129.57.177.136), launch a Redis instance.
podman run -d -p 6379:6379 docker.io/redis/redis-stack-server:latest
```

> Nodes = 32, duration = 60 seconds, latency in milliseconds, memory in MB. Only one run for magnitude estimation.

| PPS | Min Latency (ms) | P95 Latency (ms) | P99 Latency (ms) | Max Latency (ms) | Redis Mem (MB) | Redis Records/s |
|------|------|------|------|------|------|------|
| 1 | 0.33 | 1.68 | 2.41 | 4.43 | 8.31 | 33 |
| 100 | 6.13 | 937.61 | 1080.52 | 1581.99 | 650.69 | 3127 |
