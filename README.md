# FirstGoProject — Go Profiling & OTel Metric Explosion Sandbox

A small Go application for learning how to profile Go programs with `pprof`, inspect heap objects, and simulate the kind of metric-label cardinality explosion that can overload an OpenTelemetry collector.

## What it does

- Runs an HTTP server on `localhost:6060` with the standard Go `pprof` endpoints.
- Performs a steady background workload (`main.doWork`) so CPU profiles have data to show.
- Simulates a **metric explosion** in two ways:
  - `/explode` — creates a large global `map[string]int`.
  - `/record/<metric>?label=value` and `/start-load` — an OTel-style metric registry where each unique label set creates a new aggregator.
- Exposes extra helpers: `/reset`, `/gc`, `/heapdump`, `/registry-size`, `/stop-load`.
- Uses `runtime/pprof` labels to separate normal work (`work=steady`) from explosion/load work (`work=explosion`).

## Requirements

- Go 1.27+ (installed locally in this environment at `$HOME/sdk/go`).
- `graphviz` (for `go tool pprof` graphs/SVGs).
- `goref` (optional, for object-reference analysis of a running process).

## Quick start

```bash
export PATH=$HOME/sdk/go/bin:$PATH
cd /home/tfaqir/learn/GoLang/FirstGoProject
go build -o firstgoproject .
./firstgoproject
```

The app prints the pprof URL:

```text
pprof listening on http://localhost:6060/debug/pprof/
```

## Endpoints

| Path | Description |
|------|-------------|
| `/debug/pprof/` | Standard Go pprof index |
| `/debug/pprof/heap` | Heap profile |
| `/debug/pprof/profile?seconds=5` | CPU profile |
| `/debug/pprof/goroutine` | Goroutine profile |
| `/debug/pprof/block` | Block profile (enabled at startup) |
| `/debug/pprof/mutex` | Mutex profile (enabled at startup) |
| `/explode` | Start the simple map-based explosion |
| `/reset` | Drop the simulated metric store and force GC |
| `/gc` | Force a GC cycle |
| `/heapdump` | Write a binary heap dump to a temp file |
| `/record/<metric>?label=value` | Record one metric point |
| `/start-load` | Start the background load generator |
| `/stop-load` | Stop the background load generator |
| `/registry-size` | Show metric and label-set counts |

## Profiling recipes

### Capture profiles

```bash
# Heap
curl -s http://localhost:6060/debug/pprof/heap > heap.prof

# CPU (5 seconds)
curl -s "http://localhost:6060/debug/pprof/profile?seconds=5" > cpu.prof

# Goroutine / block / mutex
curl -s http://localhost:6060/debug/pprof/goroutine > goroutine.prof
curl -s http://localhost:6060/debug/pprof/block > block.prof
curl -s http://localhost:6060/debug/pprof/mutex > mutex.prof
```

### Analyze profiles

```bash
# Tables
go tool pprof -top -inuse_space heap.prof
go tool pprof -top -inuse_objects heap.prof
go tool pprof -top cpu.prof

# Annotated source
go tool pprof -list main.record heap.prof

# Compare two heap profiles
go tool pprof -top -inuse_space --diff_base=heap-baseline.prof heap-after.prof
```

### Visual graphs

```bash
# Call graph PNG/SVG
go tool pprof -png heap.prof > heap.png
go tool pprof -svg mutex.prof > mutex.svg

# Interactive web viewer (opens browser at http://localhost:8080)
go tool pprof -http=:8080 heap.prof
```

### Filter by pprof labels

```bash
go tool pprof -tags cpu.prof
go tool pprof -top -tagfocus=work:explosion cpu.prof
go tool pprof -top -tagfocus=work:steady cpu.prof
```

## Simulating metric explosions

### Simple global map

```bash
curl http://localhost:6060/explode
```

This creates 1,000,000 unique string keys in a global map. Watch `inuse_space` climb in the heap profile, then release it with:

```bash
curl http://localhost:6060/reset
```

### OTel-style registry via `/record`

```bash
# 10,000 sequential unique labels
seq 1 10000 | xargs -P 1 -I{} \
  curl -s "http://localhost:6060/record/http_request_duration?client=client_{}&status=200" > /dev/null

# 50,000 concurrent unique labels (8 workers)
seq 10001 60000 | xargs -P 8 -I{} \
  curl -s "http://localhost:6060/record/http_request_duration?client=client_{}&status=200" > /dev/null

curl -s http://localhost:6060/registry-size
```

Each unique `client` value creates a new aggregator entry, just like a metric with an unbounded label value in an OTel pipeline.

## Object-level heap inspection with `goref`

`goref` attaches to the running process and produces a pprof-compatible reference graph (`grf.out`) so you can see which variables are retaining objects.

```bash
export PATH=$HOME/go/bin:$PATH
sudo sh -c 'echo 0 > /proc/sys/kernel/yama/ptrace_scope'

grf attach $(pgrep -f firstgoproject)
go tool pprof -top -inuse_space grf.out
```

## Project files

- `main.go` — server, pprof endpoints, simple workload, explosion simulation, heap-dump handler.
- `collector.go` — OTel-style metric registry, `/record`, load generator.
- `AGENTS.md` — environment-specific commands and notes.
- `README.md` — this file.

## License

This is a personal learning project; no license is set.
