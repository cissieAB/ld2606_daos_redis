#!/usr/bin/env python3
"""
Traffic Simulator v2
- Hash-based storage design (optimized for throughput)
- MPI-style concurrent node writes
- Configurable node count, packet rates, retention
- Comprehensive statistics and monitoring

=== Usage Examples ===

# Basic: 32 nodes, 100 pps each, 10 second run, Redis on localhost
python3 hpc_traffic_simulator_v2.py --nodes 32 --pps 100 --duration 10

# Remote Redis server (cross-node test):
python3 hpc_traffic_simulator_v2.py --nodes 32 --pps 100 --duration 10 --redis-host 10.191.130.128

# High concurrency stress test:
python3 hpc_traffic_simulator_v2.py --nodes 256 --pps 100 --duration 10 --redis-host 10.191.130.128

# High frequency per node:
python3 hpc_traffic_simulator_v2.py --nodes 32 --pps 500 --duration 10 --redis-host 10.191.130.128

# Custom TTL (5 min retention instead of default 1 hour):
python3 hpc_traffic_simulator_v2.py --nodes 32 --pps 100 --duration 10 --ttl 300

# Full options:
#   --nodes N          Number of concurrent simulated HPC nodes (threads)
#   --pps N            Packets per second per node (default: 100)
#   --duration N       Test duration in seconds (default: 10)
#   --ttl N            Redis key TTL in seconds (default: 3600 = 1 hour)
#   --redis-host HOST  Redis server hostname/IP (default: localhost)
#   --redis-port PORT  Redis server port (default: 6379)
#   --redis-db N       Redis database number (default: 0)
#   --stats-interval N Seconds between periodic stats prints (default: 5)

=== Notes ===
- Requires: pip install redis
- Redis must be accessible (disable protected-mode for remote access or set a password)
- Each node runs as a separate thread with its own Redis pipeline
- Output includes per-node stats, aggregate throughput, and latency percentiles (P50/P95/P99)
"""
import redis
import json
import time
import random
import statistics
import threading
import argparse
import logging
from typing import List, Dict
from dataclasses import dataclass, field
from datetime import datetime
from queue import Queue

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='[%(asctime)s] %(levelname)s: %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger(__name__)

@dataclass
class NodeStats:
    """Per-node statistics"""
    node_id: int
    packets_generated: int = 0
    packets_sent: int = 0
    bytes_sent: int = 0
    write_times: List[float] = field(default_factory=list)
    errors: int = 0
    
    def summary(self):
        """Generate summary for this node"""
        return {
            "node_id": self.node_id,
            "packets": self.packets_generated,
            "bytes": self.bytes_sent,
            "write_latency_avg_ms": statistics.mean(self.write_times) if self.write_times else 0,
            "write_latency_p95_ms": statistics.quantiles(self.write_times, n=20)[18] if len(self.write_times) > 10 else 0,
            "errors": self.errors,
        }

@dataclass
class SimulatorConfig:
    """Simulator configuration"""
    redis_host: str = "localhost"
    redis_port: int = 6379
    redis_db: int = 0
    
    num_nodes: int = 5
    packets_per_second: int = 100
    duration_seconds: int = 10
    
    ttl_seconds: int = 3600  # 1 hour
    stats_interval: int = 5  # Print stats every N seconds
    
    # Key format: packet:{destIP}:{srcIP}:{timestamp_ms}
    key_format: str = "packet"  # namespace prefix

def generate_packet(node_id: int, timestamp_ms: int, seq: int) -> Dict:
    """Generate a simulated HPC network packet"""
    return {
        "timestamp_ms": timestamp_ms,
        "seq": seq,
        "node_id": node_id,
        "source_ip": f"192.168.{(node_id // 256) % 256}.{node_id % 256}",
        "dest_ip": f"10.0.{random.randint(1, 255)}.{random.randint(1, 255)}",
        "total_bytes": random.randint(64, 1500),
        "udp_packets": json.dumps([random.randint(64, 1500) for _ in range(10)]),
        "udp_bytes": json.dumps([random.randint(1000, 60000) for _ in range(10)]),
        "tcp_packets": json.dumps([random.randint(100, 1000) for _ in range(10)]),
        "tcp_bytes": json.dumps([random.randint(1000, 600000) for _ in range(10)])
    }

class HashStorageWriter:
    """Hash-based storage writer (production design)"""
    
    def __init__(self, redis_client: redis.Redis, key_format: str = "packet"):
        self.redis = redis_client
        self.key_format = key_format
        self.pipeline_batch_size = 100  # Batch writes for efficiency
    
    def write_packets(self, packets: List[Dict], ttl: int) -> float:
        """Write packet batch and return elapsed time in ms"""
        if not packets:
            return 0.0
        
        start = time.time()
        pipeline = self.redis.pipeline()
        
        for packet in packets:
            # Key format: packet:{destIP}:{srcIP}:{timestamp_ms}:{seq}
            key = (f"{self.key_format}:{packet['dest_ip']}:{packet['source_ip']}"
                   f":{packet['timestamp_ms']}:{packet['seq']}")
            
            # Store packet as hash
            pipeline.hset(key, mapping={
                "timestamp_ms": packet["timestamp_ms"],
                "seq": packet["seq"],
                "node_id": packet["node_id"],
                "source_ip": packet["source_ip"],
                "dest_ip": packet["dest_ip"],
                "total_bytes": packet["total_bytes"],
                "udp_packets": packet["udp_packets"],
                "udp_bytes": packet["udp_bytes"],
                "tcp_packets": packet["tcp_packets"],
                "tcp_bytes": packet["tcp_bytes"],
            })
            
            # Set TTL
            pipeline.expire(key, ttl)
        
        # Execute pipeline
        pipeline.execute()
        elapsed_ms = (time.time() - start) * 1000
        return elapsed_ms

class HPCNode(threading.Thread):
    """Simulates a single HPC compute node"""
    
    def __init__(self, node_id: int, config: SimulatorConfig, writer: HashStorageWriter,
                 stats_queue: Queue):
        super().__init__(daemon=True)
        self.node_id = node_id
        self.config = config
        self.writer = writer
        self.stats_queue = stats_queue
        
        self.stats = NodeStats(node_id=node_id)
        self.running = False
        self.seq = 0
    
    def run(self):
        """Main node loop: generate and send packets"""
        self.running = True
        start_time = time.time()
        batch_interval = 1.0  # One batch per second
        
        logger.info(f"Node {self.node_id}: Starting (target {self.config.packets_per_second} pps)")
        
        try:
            while self.running:
                batch_start = time.time()
                timestamp_ms = int(time.time() * 1000)
                
                # Generate packet batch
                packets = [
                    generate_packet(self.node_id, timestamp_ms + i, self.seq + i)
                    for i in range(self.config.packets_per_second)
                ]
                self.seq += self.config.packets_per_second
                self.stats.packets_generated += len(packets)
                
                # Write to Redis
                try:
                    write_time_ms = self.writer.write_packets(packets, self.config.ttl_seconds)
                    self.stats.write_times.append(write_time_ms)
                    self.stats.packets_sent += len(packets)
                    self.stats.bytes_sent += sum(p.get("total_bytes", 0) for p in packets)
                except Exception as e:
                    self.stats.errors += 1
                    logger.error(f"Node {self.node_id}: Write error: {e}")
                
                # Check duration
                elapsed = time.time() - start_time
                if elapsed >= self.config.duration_seconds:
                    logger.info(f"Node {self.node_id}: Duration reached ({elapsed:.1f}s)")
                    break
                
                # Sleep to maintain rate
                batch_elapsed = time.time() - batch_start
                sleep_time = max(0, batch_interval - batch_elapsed)
                if sleep_time > 0:
                    time.sleep(sleep_time)
                elif batch_elapsed > 2 * batch_interval:
                    logger.warning(
                        f"Node {self.node_id}: Behind schedule (batch took {batch_elapsed:.2f}s)"
                    )
        
        except KeyboardInterrupt:
            logger.info(f"Node {self.node_id}: Interrupted")
        finally:
            self.running = False
            self.stats_queue.put(self.stats)

class SimulationController:
    """Orchestrates the simulation"""
    
    def __init__(self, config: SimulatorConfig):
        self.config = config
        self.redis_client = None
        self.writer = None
        self.nodes = []
        self.stats_queue = Queue()
    
    def setup(self):
        """Initialize simulation"""
        # Connect to Redis
        try:
            self.redis_client = redis.Redis(
                host=self.config.redis_host,
                port=self.config.redis_port,
                db=self.config.redis_db,
                decode_responses=True,
                socket_connect_timeout=5
            )
            self.redis_client.ping()
            logger.info(f"✓ Connected to Redis at {self.config.redis_host}:{self.config.redis_port}")
        except Exception as e:
            logger.error(f"✗ Failed to connect to Redis: {e}")
            raise
        
        # Clean up old data
        try:
            for key in self.redis_client.scan_iter(f"{self.config.key_format}:*"):
                self.redis_client.delete(key)
            logger.info("✓ Cleaned up old data")
        except Exception as e:
            logger.warning(f"Warning: Failed to clean up: {e}")
        
        # Create writer
        self.writer = HashStorageWriter(self.redis_client, self.config.key_format)
    
    def run(self):
        """Run the simulation"""
        logger.info(f"\n{'='*80}")
        logger.info(f"HPC TRAFFIC SIMULATOR v2")
        logger.info(f"{'='*80}")
        logger.info(f"Configuration:")
        logger.info(f"  Nodes: {self.config.num_nodes}")
        logger.info(f"  Packets/sec/node: {self.config.packets_per_second}")
        logger.info(f"  Total throughput: {self.config.num_nodes * self.config.packets_per_second} pps")
        logger.info(f"  Duration: {self.config.duration_seconds}s")
        logger.info(f"  TTL: {self.config.ttl_seconds}s")
        logger.info(f"  Storage: Hash design")
        logger.info(f"{'='*80}\n")
        
        # Create and start nodes
        logger.info(f"Starting {self.config.num_nodes} nodes...")
        for node_id in range(self.config.num_nodes):
            node = HPCNode(node_id, self.config, self.writer, self.stats_queue)
            node.start()
            self.nodes.append(node)
        
        # Monitor progress
        start_time = time.time()
        last_stats_time = start_time
        total_packets = 0
        
        try:
            while True:
                # Check if all nodes completed
                if all(not node.is_alive() for node in self.nodes):
                    break
                
                # Print stats periodically
                elapsed = time.time() - start_time
                if elapsed - last_stats_time >= self.config.stats_interval:
                    # Collect live stats
                    total_packets = sum(node.stats.packets_sent for node in self.nodes)
                    throughput = total_packets / elapsed if elapsed > 0 else 0
                    
                    logger.info(f"[{elapsed:6.1f}s] Packets: {total_packets:,} | "
                               f"Throughput: {throughput:7.0f} pps")
                    last_stats_time = elapsed
                
                time.sleep(1)
        
        except KeyboardInterrupt:
            logger.info("\n⚠ Simulation interrupted by user")
        
        # Wait for all nodes
        for node in self.nodes:
            node.join(timeout=5)
        
        # Collect final statistics
        self._print_final_stats()
    
    def _print_final_stats(self):
        """Print final statistics"""
        logger.info(f"\n{'='*80}")
        logger.info("FINAL STATISTICS")
        logger.info(f"{'='*80}\n")
        
        # Collect all stats
        all_stats = [node.stats for node in self.nodes]
        
        # Aggregate
        total_packets = sum(s.packets_sent for s in all_stats)
        total_bytes = sum(s.bytes_sent for s in all_stats)
        total_errors = sum(s.errors for s in all_stats)
        all_write_times = []
        for s in all_stats:
            all_write_times.extend(s.write_times)
        
        # Per-node stats
        logger.info("Per-Node Statistics:")
        logger.info(f"{'Node':<6} {'Packets':<12} {'Bytes':<12} {'Avg Lat (ms)':<15} {'Errors':<8}")
        logger.info("-" * 60)
        for stats in all_stats:
            summary = stats.summary()
            logger.info(
                f"{summary['node_id']:<6} {summary['packets']:<12,} {summary['bytes']:<12,} "
                f"{summary['write_latency_avg_ms']:<15.2f} {summary['errors']:<8}"
            )
        
        # Aggregate stats
        logger.info(f"\nAggregate Statistics:")
        logger.info(f"  Total Packets: {total_packets:,}")
        logger.info(f"  Total Bytes: {total_bytes:,}")
        logger.info(f"  Total Errors: {total_errors}")
        logger.info(f"  Duration: {self.config.duration_seconds}s")
        logger.info(f"  Throughput: {total_packets / self.config.duration_seconds:.0f} pps")
        
        if all_write_times:
            logger.info(f"\nWrite Latency Statistics (across all writes):")
            logger.info(f"  Count: {len(all_write_times)}")
            logger.info(f"  Avg: {statistics.mean(all_write_times):.2f} ms")
            logger.info(f"  P50: {statistics.median(all_write_times):.2f} ms")
            logger.info(f"  P95: {statistics.quantiles(all_write_times, n=20)[18]:.2f} ms")
            logger.info(f"  P99: {statistics.quantiles(all_write_times, n=100)[98]:.2f} ms")
            logger.info(f"  Min: {min(all_write_times):.2f} ms")
            logger.info(f"  Max: {max(all_write_times):.2f} ms")
        
        # Redis stats
        try:
            info = self.redis_client.info()
            used_memory_mb = info.get('used_memory', 0) / (1024 * 1024)
            logger.info(f"\nRedis Statistics:")
            logger.info(f"  Used Memory: {used_memory_mb:.2f} MB")
            logger.info(f"  Keys: {self.redis_client.dbsize():,}")
        except Exception as e:
            logger.warning(f"Could not fetch Redis stats: {e}")
        
        logger.info(f"\n{'='*80}")
        logger.info("✓ Simulation complete!")
        logger.info(f"{'='*80}\n")

def main():
    parser = argparse.ArgumentParser(
        description=" Traffic Simulator v2 ",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter
    )
    
    # Redis options
    parser.add_argument("--redis-host", default="localhost",
                       help="Redis server hostname")
    parser.add_argument("--redis-port", type=int, default=6379,
                       help="Redis server port")
    parser.add_argument("--redis-db", type=int, default=0,
                       help="Redis database number")
    
    # Simulation options
    parser.add_argument("--nodes", type=int, default=5,
                       help="Number of concurrent HPC nodes")
    parser.add_argument("--pps", type=int, default=100,
                       help="Packets per second per node")
    parser.add_argument("--duration", type=int, default=10,
                       help="Simulation duration in seconds")
    parser.add_argument("--ttl", type=int, default=3600,
                       help="Data TTL in seconds (1 hour default)")
    parser.add_argument("--stats-interval", type=int, default=5,
                       help="Print stats every N seconds")
    
    args = parser.parse_args()
    
    # Create config
    config = SimulatorConfig(
        redis_host=args.redis_host,
        redis_port=args.redis_port,
        redis_db=args.redis_db,
        num_nodes=args.nodes,
        packets_per_second=args.pps,
        duration_seconds=args.duration,
        ttl_seconds=args.ttl,
        stats_interval=args.stats_interval,
    )
    
    # Run simulation
    try:
        controller = SimulationController(config)
        controller.setup()
        controller.run()
    except KeyboardInterrupt:
        logger.info("\n⚠ Terminated by user")
    except Exception as e:
        logger.error(f"\n✗ Fatal error: {e}", exc_info=True)
        exit(1)

if __name__ == "__main__":
    main()
