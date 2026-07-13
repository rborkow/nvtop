# nvtop-web

A web dashboard and JSON API for GPU metrics, built on [nvtop](https://github.com/Syllo/nvtop),
packaged as a Docker container for TrueNAS SCALE (or any Docker host).

nvtop's C code is untouched: a small Go sidecar supervises `nvtop --snapshot --loop`
(nvtop's built-in headless JSON mode), parses its output into clean numeric JSON,
and serves a self-contained dashboard plus an API that Home Assistant, scripts,
or anything else on your LAN can consume.

```
browser ──► GET /              live dashboard (charts + process table)
HA/scripts ► GET /api/metrics  latest snapshot, clean numeric JSON
            GET /api/history   ring buffer of recent samples (for charts)
            GET /healthz       200 ok / 503 stale|no_gpu
```

## Quick start (TrueNAS SCALE 24.10+)

1. **Check your TrueNAS version.** Apps must be Docker-based (Electric Eel 24.10
   or later). For AMD Strix Point iGPUs (e.g. Radeon 890M in the Minisforum N5
   Pro), the kernel needs gfx1150 support — TrueNAS **25.04+ (kernel 6.12)**. On
   24.10 (kernel 6.6) the GPU may not be visible at all. Verify over SSH:
   `ls -l /dev/dri` should list `card0` and `renderD128`.

2. **Install the app.** In the TrueNAS UI: *Apps → Discover Apps → ⋮ →
   Install via YAML*, paste [`deploy/truenas-app.yaml`](deploy/truenas-app.yaml),
   adjust the host port (default `8085`) if needed, and deploy.

3. **Open the dashboard** at `http://<truenas-ip>:8085/`.

4. **(Optional) Home Assistant:** add
   [`deploy/home-assistant-rest.yaml`](deploy/home-assistant-rest.yaml) to your
   HA configuration for GPU utilization / temperature / VRAM / power sensors.

### Why the container needs privileges

| Setting | Why |
|---|---|
| `devices: /dev/dri` | nvtop opens the DRM render node to query the GPU — required for any metrics. |
| `pid: host` + `cap_add: SYS_PTRACE` | Per-process GPU usage: nvtop reads `/proc/<pid>/fdinfo` of *host* processes and uses the `kcmp` syscall to deduplicate DRM handles. Omit both (and lose the process table) if you only want device-level metrics. |
| `/etc/passwd` (read-only, optional) | Resolves host UIDs to usernames in the process table; without it you see numeric UIDs. |

The container runs with `cap_drop: ALL` (+ only `SYS_PTRACE` back),
`no-new-privileges`, and a read-only root filesystem; it has no writable
mounts and exposes one read-only HTTP port.

**Exposure note:** the API intentionally reports process command lines
(that's the point of a process table), and command lines sometimes contain
paths or tokens. There is no built-in auth — keep the port LAN-only, and use
the device-only variant in `deploy/truenas-app.yaml` if you don't want host
process info exposed at all. The API sends no CORS headers, so browser pages
from other origins cannot read it.

## API

`GET /api/metrics` — latest sample:

```json
{
  "timestamp": "2026-07-13T18:04:11Z",
  "status": "ok",
  "age_seconds": 1.2,
  "interval_seconds": 2,
  "gpus": [{
    "index": 0,
    "device_name": "AMD Radeon 890M Graphics",
    "gpu_util_pct": 42, "temp_c": 62, "power_draw_w": 18,
    "gpu_clock_mhz": 2900, "mem_clock_mhz": 1000,
    "mem_total_bytes": 17179869184, "mem_used_bytes": 3221225472, "mem_free_bytes": 13958643712,
    "encode_pct": 9, "decode_pct": 9, "encode_decode_shared": true,
    "fan_note": "CPU Fan",
    "processes": [{ "pid": 2141, "user": "jellyfin", "kind": "graphic",
                    "gpu_usage_pct": 38, "gpu_mem_bytes": 1288490188,
                    "cmdline": "/usr/lib/jellyfin-ffmpeg/ffmpeg ..." }]
  }]
}
```

Every metric may be `null` — nvtop only reports what the driver exposes
(see *iGPU caveats* below). `GET /api/history?since=<unix>` returns compact
per-sample points (no process lists) for charting.

## iGPU caveats (Radeon 890M and friends)

- **VRAM numbers are carve-out + GTT semantics**, not discrete-card VRAM: the
  iGPU borrows system RAM, so "total" reflects the configured carve-out and
  usage can exceed what the BIOS reserves.
- **Power draw, memory clock, or memory utilization may be `null`** depending
  on kernel/driver version. The dashboard and the HA sensors handle nulls.
- **Fan** shows as "CPU Fan" — integrated GPUs share the package cooler.

## Building locally

```sh
# run tests
make -C web test

# run the dashboard on a machine with no GPU (replays a captured snapshot)
make -C web run-replay          # then open http://localhost:8080

# cross-build the linux/amd64 image from any machine (incl. Apple Silicon)
make -C web docker-amd64
```

The image is published by CI (`.github/workflows/nvtop-web.yml`) to
`ghcr.io/rborkow/nvtop-web` on pushes touching `web/`. After the first push,
set the ghcr package to **public** or the NAS won't be able to pull it.

## How it works

- `nvtop -s -l -d <interval>` prints one JSON array per interval to stdout
  (values are strings with unit suffixes — `"62C"`, `"2900MHz"`, `"42%"`).
- The Go sidecar frames the stream by bracket depth, strips the unit suffixes
  into typed numbers, keeps the latest snapshot plus an in-memory history ring
  (default 1 h), and restarts nvtop with backoff if it dies or wedges.
- State is memory-only by design: a container restart loses only chart history.

## Configuration

| Flag | Env | Default | |
|---|---|---|---|
| `--listen` | `NVTOP_WEB_LISTEN` | `:8080` | HTTP bind address |
| `--interval` | `NVTOP_WEB_INTERVAL` | `2s` | sampling interval |
| `--history` | `NVTOP_WEB_HISTORY` | `1h` | ring buffer span |
| `--nvtop-path` | `NVTOP_WEB_NVTOP_PATH` | `/usr/local/bin/nvtop` | nvtop binary |
| `--replay` | `NVTOP_WEB_REPLAY` | — | replay a snapshot file instead of running nvtop |
