# Issue #6 Plan: Measure latency on the Redis server/receiver side

Issue: <https://github.com/cissieAB/ld2606_daos_redis/issues/6>

## Goal

Extend the current sender-side latency measurements so we also measure receiver-side latency, including Redis query time, on the backend/server side.

## Current state

- `traffic-simulator/simulator_v2.py` measures write latency on the sender side.
- `traffic-simulator/Redis_measurements.md` documents sender-side P50/P95/P99 write latency results.
- The Go backend already:
  - subscribes to Redis pub/sub,
  - serves `/latest`,
  - initializes state from Redis in `backend/redis.go`,
  - has RediSearch-based query logic for startup reconstruction.

## Proposed measurement scope

Split receiver-side latency into clear components:

1. *End-to-end ingest latency*
   - From packet timestamp produced by simulator to the time the backend receives/processes it.
   - Best for answering, "How stale is data when it reaches the receiver?"

2. *Redis query latency*
   - Time spent by backend querying Redis/RediSearch for data.
   - Best for answering the issue text: "Include the query time in the latency measurements."

3. *Optional HTTP read latency*
   - Time for `/latest` requests, if the team wants user-visible read latency too.
   - This is useful, but probably secondary to (1) and (2).

## Recommended implementation

### Phase 1, add backend query latency instrumentation

Add timing around Redis/RediSearch calls in `backend/redis.go`:

- Measure duration for the startup query in `initializeLatestData()`.
- If more query paths are added later, reuse the same timing helper.
- Record at least:
  - count
  - avg
  - min
  - max
  - P50
  - P95
  - P99

Implementation suggestion:

- Add a small in-memory latency collector in backend, for example:
  - `type LatencyStats struct { ... }`
  - protected by `sync.Mutex`
- Add helper like:
  - `measureQuery("initializeLatestData", func() error { ... })`
- Expose metrics via:
  - logs first, and/or
  - a lightweight endpoint such as `/metrics/latency` or `/stats`

Why first:
- Low risk
- Directly addresses query-time requirement
- Easy to validate locally

### Phase 2, add receiver-side ingest latency

Use packet timestamps already embedded in simulator payloads:

- On backend receipt of pub/sub message, compute:
  - `receiver_now_ms - packet.timestamp_ms`
- Aggregate this into ingest latency stats.

Important detail:
- If packets are batched, compute either:
  - per-packet latency, or
  - one latency per batch using payload timestamp
- Prefer documenting exactly which definition is used.

Recommended definition:
- Start with *batch ingest latency* using `payload.Timestamp`, because it matches current payload structure and is simpler.
- If needed later, add per-packet latency for finer analysis.

### Phase 3, update measurement workflow and docs

Update `traffic-simulator/Redis_measurements.md` or add a new doc with:

- sender-side write latency
- receiver-side ingest latency
- backend query latency
- test conditions:
  - nodes
  - duration
  - pps
  - Redis host
  - mode 1 vs mode 2
- exact meaning of each metric

Suggested result table columns:

| Metric | Mode 1 | Mode 2 |
|---|---:|---:|
| Sender write P95 (ms) | ... | ... |
| Sender write P99 (ms) | ... | ... |
| Receiver ingest P95 (ms) | ... | ... |
| Receiver ingest P99 (ms) | ... | ... |
| Redis query P95 (ms) | ... | ... |
| Redis query P99 (ms) | ... | ... |

## Concrete file changes

### Backend

- `backend/redis.go`
  - wrap Redis/RediSearch query calls with timers
  - add ingest-latency timing when processing pub/sub payloads
- `backend/types.go`
  - add stats structs/shared state
- `backend/handlers.go`
  - optionally expose stats endpoint
- `backend/README.md`
  - document new latency metrics and endpoint usage

### Simulator/docs

- `traffic-simulator/Redis_measurements.md`
  - add receiver-side/query-side sections
- optionally add a helper script for benchmark orchestration if repeated runs are expected

## Validation plan

1. Pull latest `main`
2. Run Redis locally or on target server
3. Start backend with debug logging
4. Run `simulator_v2.py` with fixed settings, for example:
   - `--nodes 32 --duration 60 --pps 100 --mode 1`
   - repeat for mode 2
5. Capture:
   - sender write latency stats
   - backend ingest latency stats
   - backend query latency stats
6. Compare results across modes
7. Document whether query time is negligible or dominant under load

## Open questions to confirm in review

1. Should "receiver-side latency" mean:
   - pub/sub receive latency,
   - query latency,
   - or both?
2. Should we measure only startup query latency, or also recurring query endpoints?
3. Does the team want:
   - log-based stats,
   - an HTTP stats endpoint,
   - or Prometheus-style metrics?

## Recommendation

Implement both *query latency* and *receiver ingest latency*, but ship them in that order:

1. query latency instrumentation first,
2. ingest latency second,
3. docs/results update last.

That gives the team a small first PR with clear value, then a second PR for broader measurement coverage if needed.
