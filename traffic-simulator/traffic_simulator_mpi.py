#!/usr/bin/env python3
"""
MPI HPC Traffic Simulator — True Parallel Multi-Client
=======================================================
Each MPI rank = one independent HPC client node, running as a real
separate OS process on its own CPU core. This mimics actual HPC
workloads (like MDtest) where many clients hit a shared service
simultaneously.

Usage:
  # 48 clients, 500 packets/sec each, 30 second run
  mpirun -np 48 python3 mpi_traffic_simulator.py --pps 500 --duration 30

  # Remote Redis
  mpirun -np 48 python3 mpi_traffic_simulator.py --pps 500 --duration 30 --redis-host 10.191.130.128

  # Oversubscribe (more ranks than cores)
  mpirun --oversubscribe -np 96 python3 mpi_traffic_simulator.py --pps 100 --duration 10

Options:
  --pps N            Packets per second per client (default: 100)
  --duration N       Test duration in seconds (default: 10)
  --ttl N            Redis key TTL in seconds (default: 3600)
  --redis-host HOST  Redis server (default: localhost)
  --redis-port PORT  Redis port (default: 6379)
  --redis-db N       Redis DB (default: 0)
  Key format:
    packet:{rank}:{destIP}:{srcIP}:{timestamp_ms}:{seq}
    rank = MPI rank = node ID (like real HPC node IDs)
    Metadata in key only (no redundancy). Hash stores telemetry only:
    total_bytes, udp_packets, udp_bytes, tcp_packets, tcp_bytes

Output:
  - Per-rank stats (packets sent, throughput, latency)
  - Aggregate stats from rank 0 (total pps, bandwidth, latency percentiles)
"""
import redis
import time
import json
import random
import struct
import argparse
import statistics
from mpi4py import MPI


def generate_packet(rank, seq):
    """Generate a simulated HPC network packet — matches simulator.py format.
    Key encodes: rank, dest_ip, source_ip, timestamp
    Source IP is random per packet (network tap model).
    Arrays have 100 elements (same as simulator.py).
    """
    timestamp = int(time.time())
    source_ip = f"192.168.{random.randint(1, 255)}.{random.randint(1, 255)}"
    dest_ip = f"10.0.{random.randint(1, 255)}.{random.randint(1, 255)}"

    key = f"packet:{rank}:{dest_ip}:{source_ip}:{timestamp}:{seq}"
    value = {
        "total_bytes": random.randint(64, 1500),
        "udp_packets": json.dumps([random.randint(64, 1500) for _ in range(100)]),
        "udp_bytes": json.dumps([random.randint(1000, 60000) for _ in range(100)]),
        "tcp_packets": json.dumps([random.randint(100, 1000) for _ in range(100)]),
        "tcp_bytes": json.dumps([random.randint(1000, 600000) for _ in range(100)]),
    }
    return key, value


def run_client(rank, args):
    """Run one client (MPI rank). Returns stats dict."""
    r = redis.Redis(host=args.redis_host, port=args.redis_port, db=args.redis_db)
    r.ping()  # verify connectivity

    pps = args.pps
    duration = args.duration
    interval = 1.0  # one batch per second

    seq = 0
    latencies = []
    errors = 0
    bytes_sent = 0

    # Burstiness: each second, actual packet count varies around target PPS
    # Models real HPC network telemetry — some seconds are quiet, some are bursty
    # Range: 10% to 200% of target PPS (clamped to at least 1)
    burst_low = max(1, int(pps * 0.1))
    burst_high = int(pps * 2.0)

    t_start = time.monotonic()
    t_end = t_start + duration

    while time.monotonic() < t_end:
        batch_t0 = time.monotonic()

        # Random packet count this second — different per rank, different per second
        batch_count = random.randint(burst_low, burst_high)

        pipe = r.pipeline(transaction=False)
        for i in range(batch_count):
            key, val = generate_packet(rank, seq + i)
            pipe.hset(key, mapping=val)
            if args.ttl > 0:
                pipe.expire(key, args.ttl)
            bytes_sent += val.get("total_bytes", 0)

        try:
            pipe.execute()
            batch_lat = (time.monotonic() - batch_t0) * 1000  # ms
            latencies.append(batch_lat)
        except Exception as e:
            errors += batch_count

        seq += batch_count

        # Pace: one batch per second
        batch_elapsed = time.monotonic() - batch_t0
        sleep_time = max(0, interval - batch_elapsed)
        if sleep_time > 0:
            time.sleep(sleep_time)

    actual_duration = time.monotonic() - t_start

    return {
        "rank": rank,
        "packets": seq,
        "bytes": bytes_sent,
        "duration_s": round(actual_duration, 2),
        "actual_pps": round(seq / actual_duration, 1),
        "errors": errors,
        "lat_avg_ms": round(statistics.mean(latencies), 2) if latencies else 0,
        "lat_p50_ms": round(statistics.median(latencies), 2) if latencies else 0,
        "lat_p95_ms": round(statistics.quantiles(latencies, n=20)[18], 2) if len(latencies) > 10 else (round(max(latencies), 2) if latencies else 0),
        "lat_p99_ms": round(statistics.quantiles(latencies, n=100)[98], 2) if len(latencies) > 100 else (round(max(latencies), 2) if latencies else 0),
    }


def main():
    parser = argparse.ArgumentParser(description="MPI HPC Traffic Simulator")
    parser.add_argument("--pps", type=int, default=100, help="Packets/sec per client")
    parser.add_argument("--duration", type=int, default=10, help="Duration in seconds")
    parser.add_argument("--ttl", type=int, default=3600, help="Redis key TTL")
    parser.add_argument("--redis-host", default="localhost")
    parser.add_argument("--redis-port", type=int, default=6379)
    parser.add_argument("--redis-db", type=int, default=0)
    # Packet format matches hpc_traffic_simulator_v2.py exactly
    args = parser.parse_args()

    comm = MPI.COMM_WORLD
    rank = comm.Get_rank()
    size = comm.Get_size()

    if rank == 0:
        print("=" * 70)
        print(f"MPI HPC Traffic Simulator")
        print(f"  Clients (MPI ranks): {size}")
        print(f"  Target PPS/client:   {args.pps}")
        print(f"  Target aggregate:    {args.pps * size} pps")
        print(f"  Duration:            {args.duration}s")
        print(f"  Redis:               {args.redis_host}:{args.redis_port}")
        print(f"  Key format:          packet:{'{rank}:{destIP}:{srcIP}:{ts}:{seq}'}")
        print("=" * 70)

    # Barrier — all ranks start simultaneously
    comm.Barrier()
    if rank == 0:
        print(f"\n[START] All {size} clients launching simultaneously...")
        wall_start = time.monotonic()

    # Each rank runs independently
    stats = run_client(rank, args)

    # Barrier — wait for all to finish
    comm.Barrier()

    if rank == 0:
        wall_elapsed = time.monotonic() - wall_start

    # Gather all stats to rank 0
    all_stats = comm.gather(stats, root=0)

    if rank == 0:
        print(f"\n[DONE] Wall time: {wall_elapsed:.2f}s\n")

        # Per-rank summary
        print(f"{'Rank':>6} {'Packets':>10} {'PPS':>10} {'Lat avg':>10} {'Lat P95':>10} {'Lat P99':>10} {'Errors':>8}")
        print("-" * 70)
        for s in sorted(all_stats, key=lambda x: x["rank"]):
            print(f"{s['rank']:>6} {s['packets']:>10,} {s['actual_pps']:>10.1f} "
                  f"{s['lat_avg_ms']:>8.3f}ms {s['lat_p95_ms']:>8.3f}ms {s['lat_p99_ms']:>8.3f}ms {s['errors']:>8}")

        # Aggregate
        total_pkts = sum(s["packets"] for s in all_stats)
        total_bytes = sum(s["bytes"] for s in all_stats)
        agg_pps = total_pkts / wall_elapsed
        avg_lat = statistics.mean(s["lat_avg_ms"] for s in all_stats)
        max_p95 = max(s["lat_p95_ms"] for s in all_stats)
        max_p99 = max(s["lat_p99_ms"] for s in all_stats)
        total_errors = sum(s["errors"] for s in all_stats)

        print("\n" + "=" * 70)
        print("AGGREGATE RESULTS")
        print("=" * 70)
        print(f"  Total clients:      {size}")
        print(f"  Total packets:      {total_pkts:,}")
        print(f"  Total data:         {total_bytes / 1024 / 1024:.1f} MB")
        print(f"  Wall time:          {wall_elapsed:.2f}s")
        print(f"  Aggregate PPS:      {agg_pps:,.0f}")
        print(f"  Aggregate BW:       {total_bytes / wall_elapsed / 1024 / 1024:.1f} MB/s")
        print(f"  Avg latency:        {avg_lat:.3f} ms/packet")
        print(f"  Worst P95 latency:  {max_p95:.3f} ms")
        print(f"  Worst P99 latency:  {max_p99:.3f} ms")
        print(f"  Total errors:       {total_errors}")
        print("=" * 70)


if __name__ == "__main__":
    main()
