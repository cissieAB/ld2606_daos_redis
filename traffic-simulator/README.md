# Traffic Simulator

`simulator_v2.py` generates synthetic traffic records and stores them in Redis for the [Go backend](../backend/README.md). It does not send network packets through the interfaces monitored by eBPF and does not use Redis pub/sub.

See [Getting started](../GETTING_STARTED.md) for this repository's services and the [eCenter setup guide](https://github.com/cissieAB/eCenter/blob/main/docs/setup.md) for the complete frontend, backend, and traffic-source workflow.

## Run in the development container

From the `ld2606_daos_redis` repository root, start the services:

```bash
docker compose -f compose.dev.yaml --profile tools up --build
```

Keep that terminal running. In another terminal at the same repository root:

```bash
docker compose -f compose.dev.yaml exec simulator python3 simulator_v2.py --redis-host redis --duration 3600 --mode 1
```

Use `podman compose` in place of `docker compose` for Podman. The simulator service starts idle; the second command starts generation. Dependencies are already installed in the image, so no virtual environment activation is required.

To enter the container interactively instead:

```bash
docker compose -f compose.dev.yaml exec simulator /bin/bash
python3 simulator_v2.py --redis-host redis --duration 3600 --mode 1
```

The shell opens in `/app`, where the simulator source is mounted. Compose targets the service name, so there is no need to look up a container ID. `--redis-host redis` uses the Redis service on the Compose network; the default `localhost` would refer to the simulator container itself.

The example runs for one hour. Stop generation with `Ctrl+C`; stop the foreground Compose process separately when finished.

## Optional host execution

For execution outside the container, install Python 3 with virtual environment support and pip. The development image uses Python 3.13. Redis must already be running; the backend requires Redis Stack with RediSearch.

From this `traffic-simulator` directory:

```bash
python3 -m venv venv
source venv/bin/activate
python3 -m pip install -r requirements.txt
python3 simulator_v2.py --redis-host localhost --duration 3600 --mode 1
```

Use `localhost` when Redis is published on the same host. Otherwise replace it with the Redis machine's reachable hostname or IP. On later runs, activate the environment and run the script; recreate or update dependencies only when needed.

## V2 command-line options

| Option | Default | Meaning |
| --- | --- | --- |
| `--redis-host` | `localhost` | Redis hostname or IP; use `redis` inside the Compose simulator container. |
| `--redis-port` | `6379` | Redis port. |
| `--redis-db` | `0` | Redis database; match the backend's `REDIS_DB`. |
| `--nodes` | `5` | Concurrent simulated nodes; must be at least 1. |
| `--pps` | `100` | Generated records per node per one-second batch. |
| `--bin-no` | `100` | Length of each TCP/UDP sample array; stored as `samples_per_second`. |
| `--duration` | `10` | Run duration in seconds. Use a positive duration; zero is not an unlimited-run setting. |
| `--ttl` | `3600` | Redis key retention in seconds. Use a positive TTL. |
| `--stats-interval` | `5` | Seconds between progress reports. Zero does not disable reports. |
| `--mode` | `1` | Storage layout: 1 for the backend-compatible per-edge hash; 2 for timestamp buckets. |

Use positive values for `--pps` and `--bin-no`. `--pps` controls generated writes, while `--bin-no` controls sample resolution. They are independent. V2 does not accept the older `--publish`, `--storage`, `--channel`, or `--packets-per-second` options.

Display the parser's help without starting a simulation:

```bash
python3 simulator_v2.py --help
```

Run this in the prepared host environment or simulator container. In a host environment, a shorter run with 20 sample bins per second is:

```bash
python3 simulator_v2.py --redis-host localhost --nodes 5 --bin-no 20 --duration 60 --mode 1
```

## Stored records and topology

At startup, V2 attempts to delete existing `packet:*` keys from its selected Redis database before generating new records. Run it separately from real telemetry when existing records need to be retained.

Use **mode 1** with the backend. Each record is a Redis hash with key:

```text
packet:{dest_ip}:{source_ip}:{timestamp}
```

| Hash field | Contents |
| --- | --- |
| `timestamp` | Unix timestamp in seconds. |
| `samples_per_second` | The configured `--bin-no`. |
| `node_id` | Zero-based simulated node ID. |
| `source_ip`, `dest_ip` | Directed edge endpoints. |
| `total_bytes` | A separately generated synthetic byte value. |
| `udp_packets`, `tcp_packets` | JSON arrays of synthetic packet counts. |
| `udp_bytes`, `tcp_bytes` | JSON arrays of synthetic byte counts. |

Each sample array has `--bin-no` entries. Values are randomly generated; `total_bytes` is not calculated by summing the byte arrays. Repeated writes for the same edge and second overwrite the same hash, so `--pps` is not the number of distinct stored edge-second keys. Each write applies the configured TTL.

Nodes form a directed ring. With the default five nodes, source IPs are `192.168.110.0` through `192.168.110.4`, and the last node sends to the first. The IP helper wraps node IDs modulo 256, so counts above 256 reuse addresses.

V2 does not register topology. Add the simulated IPs to `backend/config/topology.json`, with rack assignments and optional display hostnames. Restart the backend and reconnect the frontend after topology changes. The backend reads the hashes and broadcasts authoritative snapshots.

Mode 2 uses `packet:h:{timestamp}` hashes, with `source_ip:dest_ip` fields containing JSON records. This layout is not the current backend's input format.

## Verification and troubleshooting

From the repository root, while the Compose services are running:

```bash
docker compose -f compose.dev.yaml exec redis redis-cli ping
curl http://localhost:8080/latest
```

For Podman, substitute `podman compose`. Expect `PONG` from Redis. Simulator logs report the connection, configuration, progress, write errors, and final statistics; `/latest` should contain current traffic while compatible records are arriving.

- **Connection refused:** verify Redis is running and use `redis` inside the simulator container, or the reachable host address outside it.
- **Empty graph:** check mode 1, matching Redis databases, topology IPs, and that the run has not ended. The default duration is only 10 seconds.
- **Records rejected:** use the current V2 source, which includes `samples_per_second`, and a positive bin count.
- **Memory usage:** reduce TTL, node count, or bin count. Increasing `--pps` increases repeated writes, rather than creating a unique key for every generated record.

## Other simulator variants

Older scripts such as `simulator_bk.py` have different command-line interfaces. Their options do not apply to V2. The following V3 behavior is separate from the V2 workflow above; the current backend uses its static topology file.

### Simulator v3 topology

Before producing traffic, `simulator_v3.py` registers each stable simulated IP once in Redis:

```text
topology:node:<ip>  # hash fields: ip, rack
topology:nodes      # set of all known node IPs
```

Registration uses idempotent `HSET` and `SADD` operations. Topology keys are persistent and separate from expiring packet hashes. `--nodes-per-rack` controls deterministic rack assignment and defaults to `4`; for example, nodes 1–4 belong to `rack-1` and nodes 5–8 belong to `rack-2`.

The `topology:nodes` set is the topology index and can be read with `SMEMBERS`; each returned IP directly identifies its `topology:node:<ip>` hash.

## Development

V2's main components are `generate_packet`, `HashStorageWriter`, `HPCNode`, and `SimulationController`. Keep stored fields compatible with the backend decoder when changing generation or storage. See [Redis measurements](Redis_measurements.md) for the existing measurement notes.
