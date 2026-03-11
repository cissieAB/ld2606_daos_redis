

### Redis `HSET` implementation
#### v1: No measurement, Pub-Sub and Write
No measurement included. 

#### v2: Write only, measure the Redis P95/P99 latency *at the sender side*

- By default, each "node" generate 100 records per second where each record has a full record of:
    - 4 fine-grained bins of length `bin_no` (100 by default); 
    - An integer indicates aggreaged_bytes;
    - Metadata including timestamp, src_ip, dst_ip, node_id.
- We can launch multiple nodes in one run.
- Using the same `HSET` key and data format with [v1](./simulator.py).

The above configurations have lon


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
                

