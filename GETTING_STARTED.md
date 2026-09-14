# Getting started

For the complete simulator or real-telemetry workflow, including frontend startup and per-node collector setup, see the [eCenter setup guide](https://github.com/cissieAB/eCenter/blob/main/docs/setup.md).

## Start this repository's services

Install Docker with Compose, or Podman with a Compose provider. Go and Python dependencies are supplied by the development images; they are not required on the host for this workflow. Run these commands from the `ld2606_daos_redis` repository root.

For simulated traffic:

```bash
docker compose -f compose.dev.yaml --profile tools up --build
```

Keep that terminal running. In another terminal at the same repository root, start the simulator:

```bash
docker compose -f compose.dev.yaml exec simulator python3 simulator_v2.py --redis-host redis --duration 3600 --mode 1
```

The simulator container starts idle. Its image already installs Python dependencies, so virtual environment activation is unnecessary. `redis` is the Compose service hostname; `localhost` inside the simulator would refer to that container. Mode 1 writes backend-compatible hashes. The explicit duration is one hour; V2 defaults to 10 seconds.

The simulator clears existing `packet:*` records in its selected Redis database at startup. Run it separately from real telemetry when those records need to be retained.

For real telemetry, start only Redis and the backend:

```bash
podman compose -f compose.dev.yaml up
```

Use `docker compose` or `podman compose` consistently with your chosen engine. The `tools` profile is only needed for the simulator. Follow the eCenter guide to attach TC ingress and start collectors on the monitored nodes.

## Redis starts empty on every launch

Each time the `redis` container starts (`compose up`, `restart`, or a crash restart), `redis/start-fresh.sh` moves the previous run's data out of the volume into `redis-archive/YYYYMMDD-HHMMSS/` at the repository root, then starts Redis with an empty dataset. The timestamp is the launch time in UTC. Redis runs with `--appendonly yes`, so each archive holds that run's append-only write log (`appendonlydir/`) and, if present, an RDB snapshot (`dump.rdb`).

The backend recreates the `idx:packets` search index if Redis restarts underneath it. `redis-archive/` is git-ignored and is never pruned automatically; delete old runs by hand.

To inspect an archived run, start a throwaway Redis Stack on another port with that directory as its data directory:

```bash
docker run --rm -p 6380:6379 -v "$PWD/redis-archive/<YYYYMMDD-HHMMSS>:/data:Z" \
  -e REDIS_ARGS="--appendonly yes" redis/redis-stack-server:latest
```

## Verify and stop

From another terminal at the repository root:

```bash
docker compose -f compose.dev.yaml ps
docker compose -f compose.dev.yaml exec redis redis-cli ping
curl http://localhost:8080/latest
```

Substitute `podman compose` if using Podman. Redis should return `PONG`; `/latest` returns the current snapshot and can be empty before traffic arrives. Compose publishes Redis on port `6379` and the backend on `8080`.

Stop the simulator with `Ctrl+C`, then stop the foreground Compose process with `Ctrl+C`.

## Component details

- [Backend](backend/README.md): configuration, topology, APIs, host development, and tests.
- [Simulator](traffic-simulator/README.md): simulator documentation.
- [DAOS client](daos-client/README.md): storage component documentation.
