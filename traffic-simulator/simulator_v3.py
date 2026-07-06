#!/usr/bin/env python3
"""Generate one aggregated traffic record per directed edge per second."""

import argparse
import json
import logging
import random
import statistics
import threading
import time
from dataclasses import dataclass, field
from typing import Dict, List

import redis


logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s] %(levelname)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger(__name__)


def node_id_to_ip(node_id: int) -> str:
    """Map a simulator node id to a stable private IPv4 address."""
    return f"192.168.110.{node_id + 1}"


def generate_bins(samples_per_second: int) -> Dict[str, List[int]]:
    """Generate matching packet-count and byte-total samples for one edge."""
    tcp_packets = [random.randint(0, 1000) for _ in range(samples_per_second)]
    udp_packets = [random.randint(0, 1000) for _ in range(samples_per_second)]

    return {
        "tcp_packets": tcp_packets,
        "tcp_bytes": [count * random.randint(64, 1500) for count in tcp_packets],
        "udp_packets": udp_packets,
        "udp_bytes": [count * random.randint(64, 1500) for count in udp_packets],
    }


def generate_edge_record(
    source_node_id: int,
    dest_node_id: int,
    timestamp: int,
    samples_per_second: int,
) -> Dict:
    """Build one complete one-second record for a directed edge."""
    bins = generate_bins(samples_per_second)
    total_packets = sum(bins["tcp_packets"]) + sum(bins["udp_packets"])
    total_bytes = sum(bins["tcp_bytes"]) + sum(bins["udp_bytes"])

    return {
        "timestamp": timestamp,
        "node_id": source_node_id,
        "source_ip": node_id_to_ip(source_node_id),
        "dest_ip": node_id_to_ip(dest_node_id),
        "samples_per_second": samples_per_second,
        "total_packets": total_packets,
        "total_bytes": total_bytes,
        **bins,
    }


@dataclass
class SimulatorConfig:
    redis_host: str = "localhost"
    redis_port: int = 6379
    redis_db: int = 0
    num_nodes: int = 5
    samples_per_second: int = 100
    duration_seconds: int = 10
    ttl_seconds: int = 3600
    stats_interval: int = 5
    key_format: str = "packet"


@dataclass
class NodeStats:
    node_id: int
    records_generated: int = 0
    records_sent: int = 0
    bytes_sent: int = 0
    write_times_ms: List[float] = field(default_factory=list)
    errors: int = 0


class HashStorageWriter:
    """Write aggregated edge records as Redis hashes."""

    def __init__(self, redis_client: redis.Redis, key_format: str = "packet"):
        self.redis = redis_client
        self.key_format = key_format

    def write_records(self, records: List[Dict], ttl: int) -> float:
        if not records:
            return 0.0

        start = time.perf_counter()
        pipeline = self.redis.pipeline()

        for record in records:
            key = (
                f"{self.key_format}:{record['dest_ip']}:"
                f"{record['source_ip']}:{record['timestamp']}"
            )
            mapping = {
                "timestamp": record["timestamp"],
                "node_id": record["node_id"],
                "source_ip": record["source_ip"],
                "dest_ip": record["dest_ip"],
                "samples_per_second": record["samples_per_second"],
                "total_packets": record["total_packets"],
                "total_bytes": record["total_bytes"],
                "udp_packets": json.dumps(record["udp_packets"]),
                "udp_bytes": json.dumps(record["udp_bytes"]),
                "tcp_packets": json.dumps(record["tcp_packets"]),
                "tcp_bytes": json.dumps(record["tcp_bytes"]),
            }
            pipeline.hset(key, mapping=mapping)
            pipeline.expire(key, ttl)

        pipeline.execute()
        return (time.perf_counter() - start) * 1000


class HPCNode(threading.Thread):
    """Produce one outgoing record per destination each second."""

    def __init__(
        self,
        node_id: int,
        config: SimulatorConfig,
        redis_client: redis.Redis,
    ):
        super().__init__(daemon=True)
        self.node_id = node_id
        self.config = config
        self.writer = HashStorageWriter(redis_client, config.key_format)
        self.stats = NodeStats(node_id=node_id)
        self.stop_event = threading.Event()

    def stop(self) -> None:
        self.stop_event.set()

    def run(self) -> None:
        for second_index in range(self.config.duration_seconds):
            if self.stop_event.is_set():
                break

            second_started_at = time.monotonic()
            timestamp = int(time.time())
            records = [
                generate_edge_record(
                    self.node_id,
                    dest_node_id,
                    timestamp,
                    self.config.samples_per_second,
                )
                for dest_node_id in range(self.config.num_nodes)
                if dest_node_id != self.node_id
            ]
            self.stats.records_generated += len(records)

            try:
                write_time_ms = self.writer.write_records(
                    records, self.config.ttl_seconds
                )
                self.stats.write_times_ms.append(write_time_ms)
                self.stats.records_sent += len(records)
                self.stats.bytes_sent += sum(r["total_bytes"] for r in records)
            except Exception as exc:
                self.stats.errors += 1
                logger.error("Node %d write error: %s", self.node_id, exc)

            if second_index == self.config.duration_seconds - 1:
                break

            remaining = 1.0 - (time.monotonic() - second_started_at)
            if remaining > 0:
                self.stop_event.wait(remaining)
            else:
                logger.warning(
                    "Node %d took %.3fs to generate and write one second",
                    self.node_id,
                    time.monotonic() - second_started_at,
                )


class SimulationController:
    def __init__(self, config: SimulatorConfig):
        self.config = config
        self.redis_client = redis.Redis(
            host=config.redis_host,
            port=config.redis_port,
            db=config.redis_db,
            decode_responses=True,
            socket_connect_timeout=5,
        )
        self.nodes: List[HPCNode] = []

    def setup(self) -> None:
        self.redis_client.ping()
        logger.info(
            "Connected to Redis at %s:%d",
            self.config.redis_host,
            self.config.redis_port,
        )

        keys = list(self.redis_client.scan_iter(f"{self.config.key_format}:*"))
        if keys:
            self.redis_client.delete(*keys)
        logger.info("Cleaned up %d old traffic keys", len(keys))

    def run(self) -> None:
        edge_count = self.config.num_nodes * (self.config.num_nodes - 1)
        logger.info("HPC TRAFFIC SIMULATOR v3")
        logger.info("Nodes: %d", self.config.num_nodes)
        logger.info("Directed edges: %d", edge_count)
        logger.info("Samples per second: %d", self.config.samples_per_second)
        logger.info(
            "Bin width: %.3f ms", 1000 / self.config.samples_per_second
        )
        logger.info("Redis records per second: %d", edge_count)
        logger.info("Duration: %ds", self.config.duration_seconds)

        self.nodes = [
            HPCNode(node_id, self.config, self.redis_client)
            for node_id in range(self.config.num_nodes)
        ]
        for node in self.nodes:
            node.start()

        started_at = time.monotonic()
        next_stats_at = self.config.stats_interval

        try:
            while any(node.is_alive() for node in self.nodes):
                elapsed = time.monotonic() - started_at
                if self.config.stats_interval and elapsed >= next_stats_at:
                    sent = sum(node.stats.records_sent for node in self.nodes)
                    logger.info(
                        "[%.1fs] Records: %d | Average rate: %.1f records/s",
                        elapsed,
                        sent,
                        sent / elapsed if elapsed else 0,
                    )
                    next_stats_at += self.config.stats_interval
                time.sleep(0.1)
        except KeyboardInterrupt:
            logger.info("Simulation interrupted")
            for node in self.nodes:
                node.stop()
        finally:
            for node in self.nodes:
                node.join()

        self.print_final_stats()

    def print_final_stats(self) -> None:
        records_generated = sum(n.stats.records_generated for n in self.nodes)
        records_sent = sum(n.stats.records_sent for n in self.nodes)
        bytes_sent = sum(n.stats.bytes_sent for n in self.nodes)
        errors = sum(n.stats.errors for n in self.nodes)
        write_times = [
            value
            for node in self.nodes
            for value in node.stats.write_times_ms
        ]

        logger.info("Simulation complete")
        logger.info("Records generated: %d", records_generated)
        logger.info("Records sent: %d", records_sent)
        logger.info("Aggregated traffic bytes: %d", bytes_sent)
        logger.info("Errors: %d", errors)
        if write_times:
            logger.info("Average write latency: %.2f ms", statistics.mean(write_times))
            logger.info("Median write latency: %.2f ms", statistics.median(write_times))


def parse_args() -> SimulatorConfig:
    parser = argparse.ArgumentParser(
        description="Traffic Simulator v3: one aggregated record per edge per second",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument("--redis-host", default="localhost")
    parser.add_argument("--redis-port", type=int, default=6379)
    parser.add_argument("--redis-db", type=int, default=0)
    parser.add_argument("--nodes", type=int, default=5)
    parser.add_argument(
        "--samples-per-second",
        "--sps",
        dest="samples_per_second",
        type=int,
        default=100,
        help="Number of bins in each one-second traffic array",
    )
    parser.add_argument("--duration", type=int, default=10)
    parser.add_argument("--ttl", type=int, default=3600)
    parser.add_argument("--stats-interval", type=int, default=5)
    args = parser.parse_args()

    if args.nodes < 1 or args.nodes > 255:
        parser.error("--nodes must be between 1 and 255")
    if args.samples_per_second < 1:
        parser.error("--samples-per-second/--sps must be at least 1")
    if args.duration < 1:
        parser.error("--duration must be at least 1")
    if args.ttl < 1:
        parser.error("--ttl must be at least 1")
    if args.stats_interval < 0:
        parser.error("--stats-interval cannot be negative")

    return SimulatorConfig(
        redis_host=args.redis_host,
        redis_port=args.redis_port,
        redis_db=args.redis_db,
        num_nodes=args.nodes,
        samples_per_second=args.samples_per_second,
        duration_seconds=args.duration,
        ttl_seconds=args.ttl,
        stats_interval=args.stats_interval,
    )


def main() -> None:
    config = parse_args()
    try:
        controller = SimulationController(config)
        controller.setup()
        controller.run()
    except Exception as exc:
        logger.error("Fatal error: %s", exc)
        raise SystemExit(1) from exc


if __name__ == "__main__":
    main()
