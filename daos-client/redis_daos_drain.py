#!/usr/bin/env python3
"""
Redis -> DAOS Drain Worker — Co-located with Redis
========================================================

DESIGN OVERVIEW
---------------
This script implements a "drain-and-delete" pattern for moving HPC telemetry
data from Redis (fast, volatile) to DAOS (durable, distributed).

It is designed to run ON THE SAME NODE as Redis (jlab-test-1) so that:
  1. Redis reads are LOCAL (fast, no network hop)
  2. DAOS writes go REMOTE via client libraries (to jlab-test-2)
  3. After a successful DAOS write, the script DELETES the keys from Redis


ARCHITECTURE
------------
  HPC Nodes ──(HSET)──> Redis (local, jlab-test-1)
                            │
                     Drain Worker (this script)
                            │
                     1. SCAN for packet:* keys
                     2. HGETALL each key (pipeline)
                     3. bput batch to DAOS (remote, jlab-test-2)
                     4. DELETE keys from Redis
                            │
                     Result: Redis stays lean, DAOS has the archive

FAILURE MODES
-------------
  - DAOS write fails:  Keys are NOT deleted from Redis. They will be
                        re-drained on the next cycle. No data loss.
  - Script crashes after DAOS write but before Redis delete:
                        Keys stay in Redis. Next cycle re-drains them
                        to DAOS (duplicate write, but no data loss).
                        This is "at-least-once" delivery — safe for
                        telemetry data.
  - Redis goes down:    Script retries every 5 seconds.

REQUIREMENTS
------------
  - Redis server running locally (or reachable via --redis-host)
  - DAOS client libraries installed (/opt/daos/)
  - DAOS agent running and connected to DAOS server
  - Environment variables:
      export LD_LIBRARY_PATH=/opt/daos/lib64:$LD_LIBRARY_PATH
      export PYTHONPATH=/opt/daos/lib64/python3.12/site-packages:$PYTHONPATH

USAGE
-----
  # Basic usage (Redis local, DAOS remote)
  python3 redis_daos_drain.py \\
      --daos-pool telemetry_pool \\
      --daos-cont telemetry

  # Custom drain interval and batch size
  python3 redis_daos_drain.py \\
      --daos-pool telemetry_pool \\
      --daos-cont telemetry \\
      --interval 5 --batch-size 2000

  # Dry-run mode (no DAOS needed — just test Redis scanning + delete)
  python3 redis_daos_drain.py --dry-run
"""

import redis
import json
import time
import signal
import argparse
import logging
from typing import Dict, List, Tuple

# ─── DAOS Import ───────────────────────────────────────────────────────────
# We import from pydaos.pydaos_core directly (not `import pydaos`) because
# the top-level pydaos __init__.py triggers daos_init() at import time,
# which can cause issues on some configurations. The direct import is more
# reliable and gives us the same DCont/DDict classes.
try:
    from pydaos.pydaos_core import DCont
    DAOS_AVAILABLE = True
except ImportError:
    DAOS_AVAILABLE = False

# ─── Logging Setup ─────────────────────────────────────────────────────────
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger('drain_v2')


# ═══════════════════════════════════════════════════════════════════════════
# WRITER CLASSES
# ═══════════════════════════════════════════════════════════════════════════

class DAOSWriter:
    """
    Writes batches of telemetry packets to a DAOS DDict (distributed
    dictionary) object.

    The DDict is stored inside a DAOS container, which lives inside a
    DAOS pool. Think of it like:
        Pool (telemetry_pool) → Container (telemetry) → DDict (telemetry_packets)

    We use `bput()` (bulk put) to write many key-value pairs in a single
    RPC call, which is much faster than writing one key at a time.
    """

    def __init__(self, pool_label: str, cont_label: str):
        if not DAOS_AVAILABLE:
            raise RuntimeError(
                "pydaos not available. Ensure DAOS client is installed and "
                "PYTHONPATH includes /opt/daos/lib64/python3.12/site-packages"
            )
        self.pool_label = pool_label
        self.cont_label = cont_label
        self._connect()

    def _connect(self):
        """
        Open the DAOS container and get a handle to the DDict.

        The get-or-create pattern makes this idempotent:
        - First run: creates a new DDict named "telemetry_packets"
        - Subsequent runs: opens the existing one
        """
        logger.info(f"Connecting to DAOS pool='{self.pool_label}', "
                    f"container='{self.cont_label}'")
        self.container = DCont(self.pool_label, self.cont_label)

        try:
            self.kv = self.container.get("telemetry_packets")
            logger.info("Opened existing DDict 'telemetry_packets'")
        except Exception:
            self.kv = self.container.dict("telemetry_packets")
            logger.info("Created new DDict 'telemetry_packets'")

    def write_batch(self, packets: List[Tuple[str, Dict]]) -> float:
        """
        Write a batch of (key, data_dict) pairs to DAOS.

        Returns elapsed time in milliseconds.
        """
        start = time.time()
        # Convert packet dicts to JSON strings for storage.
        # bput() sends all pairs in a single bulk RPC — very efficient.
        kv_data = {key: json.dumps(data) for key, data in packets}
        self.kv.bput(kv_data)
        return (time.time() - start) * 1000

    def read_key(self, key: str) -> Dict:
        """
        Read a single key back from DAOS.
        Useful for verification or when old data is needed after Redis deletion.
        """
        raw = self.kv[key]
        return json.loads(raw)

    def close(self):
        logger.info("DAOS writer closed.")


class DryRunWriter:
    """
    A fake writer for testing without DAOS.

    Reads and deletes from Redis happen for real, but the "write" is
    simulated. Useful for testing the drain logic without setting up DAOS.
    """

    def __init__(self):
        logger.info("DRY-RUN mode: packets will be read from Redis and "
                    "deleted, but NOT written to DAOS")

    def write_batch(self, packets: List[Tuple[str, Dict]]) -> float:
        start = time.time()
        time.sleep(len(packets) * 0.0001)  # ~0.1ms per packet (simulated)
        return (time.time() - start) * 1000

    def read_key(self, key: str) -> Dict:
        return {}

    def close(self):
        pass


# ═══════════════════════════════════════════════════════════════════════════
# DRAIN WORKER
# ═══════════════════════════════════════════════════════════════════════════

class RedisDAOSDrain:
    """
    The main drain worker. Runs in a loop:

    Each cycle:
      1. SCAN Redis for keys matching `packet:*`
      2. Fetch the data for those keys (pipelined HGETALL)
      3. Write the batch to DAOS (bput)
      4. DELETE the keys from Redis (only after DAOS write succeeds)

    The delete-after-write pattern means:
      - No watermark tracking needed (deleted keys won't be re-scanned)
      - Redis memory stays bounded to only un-archived packets
      - At-least-once delivery: if we crash between step 3 and 4,
        the data will be re-drained (duplicated in DAOS, but not lost)
    """

    def __init__(self, redis_client: redis.Redis, writer,
                 interval: int = 10, batch_size: int = 1000,
                 key_pattern: str = "packet:*"):
        self.redis = redis_client
        self.writer = writer
        self.interval = interval
        self.batch_size = batch_size
        self.key_pattern = key_pattern
        self.running = True

        # Track simple stats
        self.total_drained = 0
        self.total_cycles = 0
        self.total_errors = 0

        # Graceful shutdown
        signal.signal(signal.SIGINT, self._shutdown)
        signal.signal(signal.SIGTERM, self._shutdown)

    def _shutdown(self, signum, frame):
        logger.info(f"Received signal {signum}, shutting down gracefully...")
        self.running = False

    def _drain_cycle(self) -> int:
        """
        Execute one drain cycle.

        Returns the number of packets drained (0 if nothing to do).
        """
        # Step 1: SCAN for keys
        # ─────────────────────
        # SCAN is non-blocking (unlike KEYS) — it iterates incrementally
        # so Redis can still serve other requests. `count` is a hint to
        # Redis about how many keys to return per iteration.
        cursor, keys = self.redis.scan(
            cursor=0, match=self.key_pattern, count=self.batch_size
        )

        if not keys:
            return 0

        # Step 2: Fetch data (pipelined)
        # ──────────────────────────────
        # Pipeline sends all HGETALL commands in one round-trip.
        # Much faster than fetching keys one by one.
        pipe = self.redis.pipeline()
        for key in keys:
            pipe.hgetall(key)
        results = pipe.execute()

        # Pair up keys with their data, skip empty results
        packets = [(key, data) for key, data in zip(keys, results) if data]

        if not packets:
            return 0

        # Step 3: Write to DAOS
        # ─────────────────────
        # This is the critical section. If this fails, we do NOT delete
        # from Redis — the data stays safe for the next cycle.
        elapsed_ms = self.writer.write_batch(packets)
        throughput = len(packets) / (elapsed_ms / 1000) if elapsed_ms > 0 else 0
        logger.info(
            f"Wrote {len(packets)} packets to DAOS in {elapsed_ms:.1f}ms "
            f"({throughput:,.0f} pps)"
        )

        # Step 4: DELETE from Redis
        # ─────────────────────────
        # Only runs if the DAOS write succeeded (no exception above).
        # This is what keeps Redis memory bounded.
        written_keys = [key for key, _ in packets]
        self.redis.delete(*written_keys)
        logger.info(f"Deleted {len(written_keys)} keys from Redis")

        return len(packets)

    def run(self):
        """
        Main loop: drain cycles with configurable interval.
        """
        logger.info(
            f"Drain worker started — interval={self.interval}s, "
            f"batch_size={self.batch_size}, pattern='{self.key_pattern}'"
        )

        while self.running:
            self.total_cycles += 1
            cycle_start = time.time()

            try:
                count = self._drain_cycle()
                self.total_drained += count

                if count == 0:
                    logger.debug(f"Cycle {self.total_cycles}: no new packets")
                else:
                    logger.info(
                        f"Cycle {self.total_cycles}: drained {count} packets "
                        f"(total: {self.total_drained})"
                    )

            except redis.ConnectionError as e:
                self.total_errors += 1
                logger.error(f"Redis connection error: {e}. Retrying in 5s.")
                time.sleep(5)
                continue

            except Exception as e:
                self.total_errors += 1
                logger.error(f"Drain cycle error: {e}", exc_info=True)

            # Wait for next cycle
            elapsed = time.time() - cycle_start
            sleep_time = max(0, self.interval - elapsed)
            if self.running and sleep_time > 0:
                time.sleep(sleep_time)

        # Final report
        logger.info(
            f"Drain worker stopped. "
            f"Total: {self.total_drained} packets drained, "
            f"{self.total_cycles} cycles, {self.total_errors} errors"
        )
        self.writer.close()


# ═══════════════════════════════════════════════════════════════════════════
# MAIN
# ═══════════════════════════════════════════════════════════════════════════

def main():
    parser = argparse.ArgumentParser(
        description="Redis → DAOS Drain Worker (v2) — co-located with Redis",
        formatter_class=argparse.RawTextHelpFormatter,
        epilog="""
Examples:
  # Normal mode
  python3 redis_daos_drain_v2.py --daos-pool telemetry_pool --daos-cont telemetry

  # Fast drain (5s interval, 2000 keys per batch)
  python3 redis_daos_drain_v2.py --daos-pool telemetry_pool --daos-cont telemetry \\
      --interval 5 --batch-size 2000

  # Dry-run (test without DAOS)
  python3 redis_daos_drain_v2.py --dry-run
"""
    )

    # Redis options (default: localhost since we run on the Redis node)
    parser.add_argument("--redis-host", default="localhost",
                        help="Redis hostname (default: localhost)")
    parser.add_argument("--redis-port", type=int, default=6379,
                        help="Redis port (default: 6379)")

    # DAOS options
    parser.add_argument("--daos-pool", type=str, default=None,
                        help="DAOS pool label (e.g., telemetry_pool)")
    parser.add_argument("--daos-cont", type=str, default=None,
                        help="DAOS container label (e.g., telemetry)")

    # Drain options
    parser.add_argument("--interval", type=int, default=10,
                        help="Seconds between drain cycles (default: 10)")
    parser.add_argument("--batch-size", type=int, default=1000,
                        help="Max keys per drain cycle (default: 1000)")
    parser.add_argument("--key-pattern", default="packet:*",
                        help="Redis key glob pattern (default: packet:*)")

    # Modes
    parser.add_argument("--dry-run", action="store_true",
                        help="Scan & delete from Redis, but skip DAOS write")
    parser.add_argument("--verbose", action="store_true",
                        help="Enable debug logging")

    args = parser.parse_args()

    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)

    # Connect to Redis
    try:
        rc = redis.Redis(
            host=args.redis_host, port=args.redis_port,
            decode_responses=True
        )
        rc.ping()
        logger.info(f"Connected to Redis at {args.redis_host}:{args.redis_port}")
    except redis.ConnectionError as e:
        logger.error(f"Cannot connect to Redis: {e}")
        return

    # Initialize writer
    if args.dry_run:
        writer = DryRunWriter()
    else:
        if not args.daos_pool or not args.daos_cont:
            logger.error("--daos-pool and --daos-cont required (or use --dry-run)")
            return
        if not DAOS_AVAILABLE:
            logger.error(
                "pydaos not available. Set LD_LIBRARY_PATH and PYTHONPATH, "
                "or use --dry-run"
            )
            return
        try:
            writer = DAOSWriter(args.daos_pool, args.daos_cont)
        except Exception as e:
            logger.error(f"Cannot initialize DAOS writer: {e}")
            return

    # Start drain
    drain = RedisDAOSDrain(
        redis_client=rc,
        writer=writer,
        interval=args.interval,
        batch_size=args.batch_size,
        key_pattern=args.key_pattern,
    )
    drain.run()


if __name__ == "__main__":
    main()
