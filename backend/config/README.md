# Topology configuration

The topology file tells the dashboard which hosts exist, which rack each one
belongs to, and what to call it. It is the **only** source of topology for the
system: the frontend has no topology file of its own, and the
`topology:node:*` keys that `simulator_v3.py` writes to Redis are not read by
anything.

```
config/topology.json ──(read once at startup)──▶ backend ──(first WebSocket message)──▶ frontend
```

## Files in this directory

| File | Use |
| --- | --- |
| `topology.json` | Default. Loaded unless `TOPOLOGY_PATH` says otherwise. |
| `topology.ebpf.json` | The `ebpf` testbed's real-traffic nodes: `ebpf2203` (`129.57.178.86`) and `ebpf2201` (`129.57.178.85`), rack `ebpf`. `ebpf2202` is not listed until it is working. Select it with `TOPOLOGY_PATH=config/topology.ebpf.json`; see `docs/guide_real-traffic.md` in eCenter. |

To keep several topologies side by side, add another file here and select it
with `TOPOLOGY_PATH`. The path is relative to the backend's working directory
(`backend/`, or `/app` in the Compose container):

```yaml
# compose.dev.yaml
  backend:
    environment:
      TOPOLOGY_PATH: config/topology.mytestbed.json
```

When running the backend on the host: `TOPOLOGY_PATH=config/topology.mytestbed.json go run .`

## Format

```json
{
  "nodes": {
    "192.168.110.1": { "ip": "192.168.110.1", "rack": "rack-1", "hostname": "compute-01" },
    "192.168.110.2": { "ip": "192.168.110.2", "rack": "rack-1", "hostname": "compute-02" },
    "192.168.110.5": { "ip": "192.168.110.5", "rack": "rack-2" }
  }
}
```

| Field | Required | Meaning |
| --- | --- | --- |
| key under `nodes` | yes | The node's IPv4 address. This is the node's identity. |
| `ip` | yes | Must be identical to the key. |
| `rack` | yes | Any non-blank string. Nodes with the same value are grouped in Rack view. Avoid `external`, which is reserved for unknown endpoints. |
| `hostname` | no | Display label only. Blank or omitted shows the IP. Need not be unique. |

## Validation

The backend checks the file at startup and **exits** if any check fails,
logging `Failed to load topology from <path>: <reason>`:

- The file is exactly one JSON object with a `nodes` map.
- There is at least one node.
- Every key is a valid IPv4 address (IPv6 is not supported).
- Every node's `ip` equals its key.
- Every `rack` is non-blank.
- No unknown fields, anywhere. A typo such as `"host_name"` or `"Rack"` is rejected, not ignored.

On success it logs `Loaded N topology nodes from <path>`.

## Which IPs to list

List the addresses that appear **in the traffic records**, not the hosts'
management or SSH addresses, which may differ.

| Traffic source | IPs to list |
| --- | --- |
| `simulator_v3.py --nodes N` | `192.168.110.1` through `192.168.110.N`. Rack assignment is up to you; the simulator's own `--nodes-per-rack` is not used by the backend. |
| `simulator_v2.py` (default 5 nodes) | `192.168.110.0` through `192.168.110.4` |
| `tc_collector` on real hosts | The source and destination IPs observed on the monitored interfaces. On each host, `ip -4 -o addr show <iface>` shows its own address. |

To see which IPs are actually arriving, list the keys in Redis. Keys are
`packet:{dest_ip}:{source_ip}:{timestamp}`, with the destination first:

```bash
docker compose -f compose.dev.yaml exec redis redis-cli --scan --pattern 'packet:*' | head
```

## How the frontend uses it

- **Host view:** one node per topology entry. Hosts appear even with no current traffic.
- **Unknown endpoints:** any traffic IP not in the file is folded into a single `external` node, in both Host and Rack view. If a host you expect shows up as `external`, its IP is missing or mistyped here.
- **Rack view:** edges are aggregated by `rack`; traffic between two nodes in the same rack is not shown.
- **History mode** uses the current topology, not the topology in effect when the data was recorded.

## Applying a change

The topology is read once at backend startup and sent to each browser only
when it connects. After editing:

1. Restart the backend. With Compose, `./backend` is mounted into the container, so no rebuild is needed:
   ```bash
   docker compose -f compose.dev.yaml restart backend
   docker compose -f compose.dev.yaml logs backend --tail 5   # expect "Loaded N topology nodes"
   ```
2. Reload the dashboard in the browser.

If the backend exits right after the restart, the file failed validation; the
log line gives the reason.
