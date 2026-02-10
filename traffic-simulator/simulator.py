#!/usr/bin/env python3
"""
Redis Traffic Simulator
Simulates network traffic with multiple nodes generating packets,
publishing to Redis pub/sub, and storing data for pressure testing.
"""

import redis
import json
import time
import threading
import random
import signal
import sys
from typing import List, Dict
import argparse


class PacketGenerator:
    """Generates simulated network packets with various attributes."""
    
    @staticmethod
    def generate_packet(node_id: int, packet_id: int) -> Dict:
        """Generate a single packet with random data."""
        return {
            "timestamp": int(time.time()),
            "source_ip": f"192.168.{random.randint(1, 255)}.{random.randint(1, 255)}",
            "dest_ip": f"10.0.{random.randint(1, 255)}.{random.randint(1, 255)}",
            "total_bytes": random.randint(64, 1500),
            "udp_packets": [random.randint(64, 1500) for _ in range(100)],
            "udp_bytes": [random.randint(1000, 60000) for _ in range(100)],
            "tcp_packets": [random.randint(100, 1000) for _ in range(100)],
            "tcp_bytes": [random.randint(1000, 600000) for _ in range(100)]
       }
    
    @staticmethod
    def generate_batch(node_id: int, batch_size: int, start_id: int) -> List[Dict]:
        """Generate a batch of packets."""
        return [
            PacketGenerator.generate_packet(node_id, start_id + i)
            for i in range(batch_size)
        ]


class TrafficNode:
    """Represents a single node generating traffic."""
    
    def __init__(self, node_id: int, redis_client: redis.Redis, 
                 packets_per_second: int = 100, 
                 publish_enabled: bool = False,
                 storage_enabled: bool = False,
                 channel_name: str = "traffic_channel"):
        self.node_id = node_id
        self.redis_client = redis_client
        self.packets_per_second = packets_per_second
        self.publish_enabled = publish_enabled
        self.storage_enabled = storage_enabled
        self.channel_name = channel_name
        self.running = False
        self.packet_counter = 0
        self.stats = {
            "packets_generated": 0,
            "packets_published": 0,
            "packets_stored": 0,
            "publish_errors": 0,
            "storage_errors": 0,
        }
    
    def run(self, duration_seconds: int = None):
        """Run the traffic generator."""
        self.running = True
        start_time = time.time()
        batch_interval = 1.0  # Generate batches every second
        
        print(f"[Node {self.node_id}] Starting traffic generation...")
        print(f"[Node {self.node_id}] Publish: {self.publish_enabled}, Storage: {self.storage_enabled}")
        
        try:
            while self.running:
                batch_start = time.time()
                
                # Generate packets
                packets = PacketGenerator.generate_batch(
                    self.node_id, 
                    self.packets_per_second, 
                    self.packet_counter
                )
                self.packet_counter += self.packets_per_second
                self.stats["packets_generated"] += len(packets)
                
                # Publish to Redis channel
                if self.publish_enabled:
                    self._publish_packets(packets)
                
                # Store in Redis
                if self.storage_enabled:
                    self._store_packets(packets)
                
                # Check if duration exceeded
                if duration_seconds and (time.time() - start_time) >= duration_seconds:
                    print(f"[Node {self.node_id}] Duration reached, stopping...")
                    break
                
                # Sleep to maintain rate
                elapsed = time.time() - batch_start
                sleep_time = max(0, batch_interval - elapsed)
                if sleep_time > 0:
                    time.sleep(sleep_time)
                else:
                    print(f"[Node {self.node_id}] WARNING: Cannot maintain {self.packets_per_second} pps (took {elapsed:.3f}s)")
                
        except KeyboardInterrupt:
            print(f"\n[Node {self.node_id}] Interrupted by user")
        finally:
            self.running = False
            self._print_stats()
    
    def _publish_packets(self, packets: List[Dict]):
        """Publish packets to Redis pub/sub channel."""
        try:
            # Publish the entire batch as a JSON array
            message = json.dumps({
                "timestamp": packets[0]['timestamp'],
                "packet_count": len(packets),
                "packets": packets
            })
            self.redis_client.publish(self.channel_name, message)
            self.stats["packets_published"] += len(packets)
        except Exception as e:
            self.stats["publish_errors"] += 1
            print(f"[Node {self.node_id}] Publish error: {e}")
    
    def _store_packets(self, packets: List[Dict]):
        """Store packets in Redis using various data structures."""
        try:
            pipeline = self.redis_client.pipeline()
            
            for packet in packets:
                # Store each packet as a hash
                # Convert list fields to JSON strings since Redis hashes don't support nested structures
                key = f"packet:{packet['dest_ip']}:{packet['source_ip']}:{packet['timestamp']}"
                
                # Prepare packet data with serialized lists (required for Redis hashes)
                packet_data = {
                    "timestamp": packet["timestamp"],
                    "source_ip": packet["source_ip"],
                    "dest_ip": packet["dest_ip"],
                    "total_bytes": str(packet["total_bytes"]),
                    "udp_packets": json.dumps(packet["udp_packets"]),
                    "udp_bytes": json.dumps(packet["udp_bytes"]),
                    "tcp_packets": json.dumps(packet["tcp_packets"]),
                    "tcp_bytes": json.dumps(packet["tcp_bytes"])
                }
                
                # Batch insert using pipeline
                pipeline.hset(key, mapping=packet_data)
                pipeline.expire(key, 3600)  # Expire after 1 hour
            
            pipeline.execute()
            self.stats["packets_stored"] += len(packets)
        except Exception as e:
            self.stats["storage_errors"] += 1
            print(f"[Node {self.node_id}] Storage error: {e}")
    
    def _print_stats(self):
        """Print statistics for this node."""
        print(f"\n[Node {self.node_id}] Final Statistics:")
        print(f"  Packets Generated: {self.stats['packets_generated']}")
        print(f"  Packets Published: {self.stats['packets_published']}")
        print(f"  Packets Stored:    {self.stats['packets_stored']}")
        print(f"  Publish Errors:    {self.stats['publish_errors']}")
        print(f"  Storage Errors:    {self.stats['storage_errors']}")


class TrafficSimulator:
    """Main simulator coordinating multiple traffic nodes."""
    
    def __init__(self, num_nodes: int = 1, 
                 packets_per_second: int = 100,
                 redis_host: str = "localhost",
                 redis_port: int = 6379,
                 redis_db: int = 0,
                 publish_enabled: bool = True,
                 storage_enabled: bool = True,
                 channel_name: str = "traffic_channel",
                 stats_interval: int = 5):
        
        self.num_nodes = num_nodes
        self.packets_per_second = packets_per_second
        self.publish_enabled = publish_enabled
        self.storage_enabled = storage_enabled
        self.channel_name = channel_name
        self.stats_interval = stats_interval
        self.start_time = None
        self.stats_running = False
        
        # Connect to Redis
        try:
            self.redis_client = redis.Redis(
                host=redis_host,
                port=redis_port,
                db=redis_db,
                decode_responses=True
            )
            self.redis_client.ping()
            print(f"✓ Connected to Redis at {redis_host}:{redis_port}")
        except Exception as e:
            print(f"✗ Failed to connect to Redis: {e}")
            raise
        
        self.nodes = []
        self.threads = []
    
    def _print_periodic_stats(self):
        """Print statistics periodically in a background thread."""
        while self.stats_running:
            time.sleep(self.stats_interval)
            if not self.stats_running:
                break
            
            elapsed = int(time.time() - self.start_time)
            total_generated = sum(node.stats["packets_generated"] for node in self.nodes)
            total_published = sum(node.stats["packets_published"] for node in self.nodes)
            total_stored = sum(node.stats["packets_stored"] for node in self.nodes)
            total_errors = sum(node.stats["publish_errors"] + node.stats["storage_errors"] for node in self.nodes)
            
            # Format time as MM:SS
            minutes, seconds = divmod(elapsed, 60)
            time_str = f"{minutes:02d}:{seconds:02d}"
            
            print(f"[{time_str}] Total: {total_generated:,} packets, "
                  f"Published: {total_published:,}, Stored: {total_stored:,}, "
                  f"Errors: {total_errors}")
    
    def start(self, duration_seconds: int = None):
        """Start all traffic nodes."""
        print(f"\n{'='*60}")
        print(f"Redis Traffic Simulator")
        print(f"{'='*60}")
        print(f"Nodes:              {self.num_nodes}")
        print(f"Packets/sec/node:   {self.packets_per_second}")
        print(f"Total packets/sec:  {self.num_nodes * self.packets_per_second}")
        print(f"Publish enabled:    {self.publish_enabled}")
        print(f"Storage enabled:    {self.storage_enabled}")
        if self.publish_enabled:
            print(f"Channel name:       {self.channel_name}")
        if duration_seconds:
            print(f"Duration:           {duration_seconds} seconds")
        print(f"Stats interval:     {self.stats_interval} seconds")
        print(f"{'='*60}\n")
        
        # Record start time
        self.start_time = time.time()
        
        # Create and start nodes
        for i in range(self.num_nodes):
            node = TrafficNode(
                node_id=i + 1,
                redis_client=self.redis_client,
                packets_per_second=self.packets_per_second,
                publish_enabled=self.publish_enabled,
                storage_enabled=self.storage_enabled,
                channel_name=self.channel_name
            )
            self.nodes.append(node)
            
            thread = threading.Thread(
                target=node.run,
                args=(duration_seconds,),
                name=f"Node-{i+1}"
            )
            thread.start()
            self.threads.append(thread)
        
        # Start periodic stats printer
        self.stats_running = True
        stats_thread = threading.Thread(
            target=self._print_periodic_stats,
            name="StatsReporter",
            daemon=True
        )
        stats_thread.start()
        
        # Wait for all threads
        try:
            for thread in self.threads:
                thread.join()
        except KeyboardInterrupt:
            print("\n\nShutting down all nodes...")
            self.stats_running = False
            for node in self.nodes:
                node.running = False
            for thread in self.threads:
                thread.join(timeout=2)
        finally:
            self.stats_running = False
        
        print(f"\n{'='*60}")
        print("Simulation Complete")
        print(f"{'='*60}")
        self._print_aggregate_stats()
    
    def _print_aggregate_stats(self):
        """Print aggregate statistics across all nodes."""
        total_generated = sum(node.stats["packets_generated"] for node in self.nodes)
        total_published = sum(node.stats["packets_published"] for node in self.nodes)
        total_stored = sum(node.stats["packets_stored"] for node in self.nodes)
        total_pub_errors = sum(node.stats["publish_errors"] for node in self.nodes)
        total_store_errors = sum(node.stats["storage_errors"] for node in self.nodes)
        
        print(f"\nAggregate Statistics:")
        print(f"  Total Packets Generated: {total_generated:,}")
        print(f"  Total Packets Published: {total_published:,}")
        print(f"  Total Packets Stored:    {total_stored:,}")
        print(f"  Total Publish Errors:    {total_pub_errors}")
        print(f"  Total Storage Errors:    {total_store_errors}")


def main():
    """Main entry point for the simulator."""
    parser = argparse.ArgumentParser(
        description="Redis Traffic Simulator for pressure testing",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter
    )
    parser.add_argument(
        "--nodes", type=int, default=1,
        help="Number of traffic nodes"
    )
    parser.add_argument(
        "--packets-per-second", "--pps", type=int, default=100, dest="pps",
        help="Packets per second per node"
    )
    parser.add_argument(
        "--duration", type=int, default=None,
        help="Duration in seconds (0 or None = runs until stopped)"
    )
    parser.add_argument(
        "--redis-host", type=str, default="localhost",
        help="Redis server hostname or IP address"
    )
    parser.add_argument(
        "--redis-port", type=int, default=6379,
        help="Redis server port"
    )
    parser.add_argument(
        "--redis-db", type=int, default=0,
        help="Redis database number"
    )
    parser.add_argument(
        "--publish", action="store_true", default=True,
        help="Enable Redis pub/sub publishing (default: enabled)"
    )
    parser.add_argument(
        "--no-publish", action="store_false", dest="publish",
        help="Disable Redis pub/sub publishing"
    )
    parser.add_argument(
        "--storage", action="store_true", default=True,
        help="Enable Redis data storage (default: enabled)"
    )
    parser.add_argument(
        "--no-storage", action="store_false", dest="storage",
        help="Disable Redis data storage"
    )
    parser.add_argument(
        "--channel", type=str, default="traffic_channel",
        help="Redis pub/sub channel name (default: traffic_channel)"
    )
    parser.add_argument(
        "--stats-interval", type=int, default=5,
        help="Interval in seconds for printing statistics (0 to disable)"
    )
    
    args = parser.parse_args()
    
    # Setup signal handlers for graceful shutdown
    def signal_handler(sig, frame):
        print("\n\n⚠ Received interrupt signal, shutting down gracefully...")
        sys.exit(0)
    
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    try:
        simulator = TrafficSimulator(
            num_nodes=args.nodes,
            packets_per_second=args.pps,
            redis_host=args.redis_host,
            redis_port=args.redis_port,
            redis_db=args.redis_db,
            publish_enabled=args.publish,
            storage_enabled=args.storage,
            channel_name=args.channel,
            stats_interval=args.stats_interval
        )
        
        simulator.start(duration_seconds=args.duration)
    except KeyboardInterrupt:
        print("\n\n⚠ Interrupted by user")
        sys.exit(0)
    except Exception as e:
        print(f"\n✗ Error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()