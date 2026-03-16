
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
- Two write modes are supported:
    - **Mode 1 (Key-Value):** Each packet record is written as its own Redis key.
    - **Mode 2 (Key-Field-Value):** Multiple packet records are grouped under a shared timestamp bucket key where each field represents a `src_ip:dst_ip` pair.


**Mode 1: Key-Value (Current implementation)**  
Each packet is written as an individual Redis key containing the full packet metadata as a mapping.

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

**Mode 2: Key-Field-Value (Bucketed implementation)**  
Packets sharing the same timestamp are grouped into a single Redis hash key.  
Each field represents a `{src_ip}:{dst_ip}` pair, and the value contains the packet context.

```python
# For each record (bucketed by timestamp)
bucket_key = f"{self.prefix}:h:{packet['timestamp_ms']}"

mapping[f"{packet['source_ip']}:{packet['dest_ip']}"] = {
    "timestamp_ms": packet["timestamp_ms"],
    "node_id": packet["node_id"],
    "source_ip": packet["source_ip"],
    "dest_ip": packet["dest_ip"],
    "total_bytes": packet["total_bytes"],
    "udp_packets": packet["udp_packets"],
    "udp_bytes": packet["udp_bytes"],
    "tcp_packets": packet["tcp_packets"],
    "tcp_bytes": packet["tcp_bytes"],
}

if mapping:
    pipe.hset(bucket_key, mapping=mapping)
```
                
### Measurements of ejfat-5 ==> ejfat-6
[Simulator_v2](./simulator_v2.py)

```bash
# On the sender-side
(venv) bash-5.1$ python3 simulator_v2.py --nodes 32 --duration 60 --pps <pps> --mode <1|2> --redis-host 129.57.177.136    # Match the server's MANAGEMENT IP

# On the Redis server (IP: 129.57.177.136), launch a Redis instance.
podman run -d -p 6379:6379 docker.io/redis/redis-stack-server:latest
```

> Nodes = 32, duration = 60 seconds, latency in milliseconds, memory in MB. 


| Metric / Category            | PPS 1 Mode 1 | PPS 1 Mode 2 | PPS 100 Mode 1 | PPS 100 Mode 2 |
| ---------------------------- | ------------ | ------------ | -------------- | -------------- |
| **Duration (s)**             | 60           | 60           | 60             | 60             |
| **Throughput (records/sec)** | 33           | 33           | 3033           | 3073           |
| **Write Count**              | 1952         | 1952         | 1820           | 1844           |
| **Avg Write Latency (ms)**   | 0.86         | 1.37         | 651.02         | 826.24         |
| **P50 Latency (ms)**         | 0.73         | 0.74         | 657.67         | 898.90         |
| **P95 Latency (ms)**         | 1.63         | 1.53         | 1117.32        | 1096.32        |
| **P99 Latency (ms)**         | 2.36         | 2.58         | 1389.33        | 1237.49        |
| **Min Latency (ms)**         | 0.26         | 0.28         | 6.15           | 5.25           |
| **Max Latency (ms)**         | 13.45        | 1018.45      | 4195.21        | 5139.56        |
| **Redis Used Memory (MB)**   | 8.29         | 7.72         | 631.73         | 563.44         |
| **Redis Keys**               | 1,952        | 1,625        | 182,000        | 61,423         |