# Issue #6 Plan: Measure end-to-end latency from client write to server query

Issue: <https://github.com/cissieAB/ld2606_daos_redis/issues/6>

## Goal

Measure true end-to-end latency from the client write path to the server query path, rather than only measuring sender-side write latency.

Target definition:

- *End-to-end latency* = time from when the client writes a packet record to Redis to when the server finishes the corresponding query and has the result available.

This should explicitly include:

- client-side write time,
- Redis/network transit time,
- server-side query time.

## Current state

- `traffic-simulator/simulator_v2.py` measures sender-side write latency only.
- `traffic-simulator/Redis_measurements.md` documents sender-side P50/P95/P99 write latency results.
- The Go backend already:
  - subscribes to Redis pub/sub,
  - serves `/latest`,
  - initializes state from Redis in `backend/redis.go`,
  - has RediSearch-based query logic for startup reconstruction.

## Recommended measurement model

Split the end-to-end path into three measured components, then report both the parts and the total:

1. *Client write latency*
   - Time spent by the simulator performing the Redis write.
   - Already partially available in `simulator_v2.py`.

2. *Write-to-query-start delay*
   - Time from successful client write completion to when the backend begins the query that is meant to observe that data.
   - Captures Redis visibility delay, transport delay, scheduling delay, and backend trigger delay.

3. *Server query latency*
   - Time spent by the backend executing the Redis or RediSearch query and decoding the result.

Then compute:

4. *End-to-end latency*
   - `query_finish_time - client_write_start_time`, or equivalently
   - `client write latency + write-to-query-start delay + server query latency`

This gives a clean answer to: "from client write to server query".

## Key design requirement

To measure end-to-end latency correctly, the server must query for a record that can be matched to a specific client write event.

That means each measured record needs:

- a stable unique identifier, or
- a timestamp plus enough metadata to identify the exact packet or batch.

Recommended identifier:

- add a `trace_id` or `measurement_id` per packet or batch in the simulator payload and Redis record.

Without a stable identifier, the query timing can still be measured, but the end-to-end path will be ambiguous.

## Recommended implementation

### Phase 1, add correlation metadata in the simulator

Update the simulator write path so each measured write includes:

- `write_start_ms`
- `write_end_ms`
- `trace_id`
- existing packet metadata like timestamp/src/dst/node

Recommended minimum:

- one `trace_id` per batch write if batch-level measurement is acceptable
- one `trace_id` per packet if packet-level measurement is required

Recommendation:
- start with *batch-level* end-to-end measurement because it is simpler and matches the current payload flow better.

### Phase 2, add backend query timing with correlation

In `backend/redis.go`:

- add a query path that looks up the record or batch by `trace_id` or deterministic key
- measure:
  - query start time
  - query finish time
  - query duration
- recover the write metadata from the queried record
- compute end-to-end latency on the backend side

Record at least:

- count
- avg
- min
- max
- P50
- P95
- P99

Recommended helper structure:

- `type LatencyStats struct { ... }`
- protected by `sync.Mutex`
- separate collectors for:
  - client write latency
  - write-to-query-start delay
  - server query latency
  - end-to-end latency

### Phase 3, expose measurement results

Expose the stats via one of:

- logs first, or
- a lightweight endpoint such as `/stats` or `/metrics/latency`

Recommended first version:
- logs plus one JSON endpoint for easy collection.

### Phase 4, update docs and benchmark workflow

Update `traffic-simulator/Redis_measurements.md` or add a dedicated end-to-end benchmark doc with:

- exact metric definitions
- whether measurement is packet-level or batch-level
- test conditions:
  - nodes
  - duration
  - pps
  - Redis host
  - mode 1 vs mode 2
- results for each latency component and total end-to-end latency

Suggested result table columns:

| Metric | Mode 1 | Mode 2 |
|---|---:|---:|
| Client write P95 (ms) | ... | ... |
| Client write P99 (ms) | ... | ... |
| Write-to-query-start P95 (ms) | ... | ... |
| Write-to-query-start P99 (ms) | ... | ... |
| Server query P95 (ms) | ... | ... |
| Server query P99 (ms) | ... | ... |
| End-to-end P95 (ms) | ... | ... |
| End-to-end P99 (ms) | ... | ... |

## Concrete file changes

### Simulator

- `traffic-simulator/simulator_v2.py`
  - add `trace_id`
  - record write start/end timestamps
  - ensure metadata is written into Redis in queryable form

### Backend

- `backend/redis.go`
  - add correlated query timing
  - compute end-to-end latency from recovered write metadata
- `backend/types.go`
  - add stats structs/shared state
- `backend/handlers.go`
  - optionally expose stats endpoint
- `backend/README.md`
  - document end-to-end measurement flow

### Docs

- `traffic-simulator/Redis_measurements.md`
  - extend to include end-to-end metrics
- optionally add a dedicated benchmark plan doc if the team wants a cleaner separation

## Validation plan

1. Pull latest `main`
2. Run Redis locally or on target server
3. Start backend with debug logging
4. Run `simulator_v2.py` with fixed settings, for example:
   - `--nodes 32 --duration 60 --pps 100 --mode 1`
   - repeat for mode 2
5. For each measured write or batch:
   - store `trace_id`
   - query by that identifier on the backend
   - record query duration and end-to-end latency
6. Capture:
   - client write latency stats
   - write-to-query-start delay stats
   - server query latency stats
   - end-to-end latency stats
7. Compare results across modes
8. Document which component dominates under load

## Open questions to confirm in review

1. Should measurement be packet-level or batch-level?
   - I recommend batch-level first.
2. Should the server actively query after each write, or should the benchmark run queries at a fixed sampling rate?
3. Should the query target be:
   - exact key lookup,
   - hash field lookup,
   - or RediSearch query?

These choices matter because end-to-end latency depends strongly on the query path.

## Recommendation

Center the work on *correlated end-to-end latency*:

1. add `trace_id` and write timestamps in the simulator,
2. add backend correlated query timing,
3. report component latencies and total end-to-end latency,
4. update docs/results last.

That will make the PR clearly match the requested metric: from client write to server query completion.
