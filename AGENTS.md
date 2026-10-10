# Project Notes

## Go Installation

Go is installed locally at `$HOME/sdk/go`. Add it to your `PATH` for each shell session:

```bash
export PATH=$HOME/sdk/go/bin:$PATH
```

## Build & Run

```bash
cd /home/tfaqir/learn/GoLang/FirstGoProject
export PATH=$HOME/sdk/go/bin:$PATH
go build -o firstgoproject .
./firstgoproject
```

The app exposes:

- `http://localhost:6060/debug/pprof/` – standard Go pprof endpoints
- `http://localhost:6060/explode` – start the metric-label cardinality explosion simulation
- `http://localhost:6060/reset` – drop the simulated metric store and force GC
- `http://localhost:6060/gc` – force a GC cycle
- `http://localhost:6060/heapdump` – write a binary heap dump to a temp file

## Profiling Recipes

Capture heap profile:

```bash
curl -s http://localhost:6060/debug/pprof/heap > heap.prof
```

Capture CPU profile (5 seconds):

```bash
curl -s "http://localhost:6060/debug/pprof/profile?seconds=5" > cpu.prof
```

Analyze profiles:

```bash
go tool pprof -top -inuse_space heap.prof
go tool pprof -top -alloc_space heap.prof
go tool pprof -top cpu.prof
go tool pprof -list main.doWork cpu.prof
```

Compare two heap profiles:

```bash
go tool pprof -top -inuse_space --diff_base=heap-baseline.prof heap-exploded.prof
```

Interactive web viewer (install `graphviz` for graph rendering):

```bash
go tool pprof -http=:8080 heap-exploded.prof
```

## OTel-Style Metric Registry Endpoints

- `http://localhost:6060/record/<metric>?label=value` – record a single metric point
- `http://localhost:6060/start-load` – start a background load generator that creates high-cardinality label sets
- `http://localhost:6060/stop-load` – stop the load generator
- `http://localhost:6060/registry-size` – show current metric and label-set counts

## Pprof Labels

The app labels normal work as `work=steady` and explosion/load work as `work=explosion`. To see CPU time split by label:

```bash
go tool pprof -tags cpu.prof
go tool pprof -top -tagfocus=work:explosion cpu.prof
go tool pprof -top -tagfocus=work:steady cpu.prof
```

## Object-Level Heap Inspection with `goref`

`goref` attaches to a running process and produces a pprof-compatible reference graph (`grf.out`).

```bash
export PATH=$HOME/go/bin:$PATH
# allow ptrace if needed
sudo sh -c 'echo 0 > /proc/sys/kernel/yama/ptrace_scope'

grf attach $(pgrep -f firstgoproject)
go tool pprof -top -inuse_space grf.out
```

## Simulating Metric Explosions

### Simple global map explosion

1. Start the app.
2. Capture a baseline heap profile.
3. `curl http://localhost:6060/explode` to create 1,000,000 unique label keys.
4. Capture another heap profile and compare.
5. `curl http://localhost:6060/reset` to release the memory and verify GC behavior.

### OTel collector-style load generator

1. Start the app and capture a baseline heap profile.
2. `curl http://localhost:6060/start-load`.
3. Watch `/registry-size` and capture heap/CPU profiles.
4. `curl http://localhost:6060/stop-load` when done.

## /record Endpoint Load Test

Drive the registry with unique labels and watch memory grow:

```bash
# 10,000 sequential requests
seq 1 10000 | xargs -P 1 -I{} curl -s \
  "http://localhost:6060/record/http_request_duration?client=client_{}&status=200" > /dev/null

# 50,000 concurrent requests (8 workers)
seq 10001 60000 | xargs -P 8 -I{} curl -s \
  "http://localhost:6060/record/http_request_duration?client=client_{}&status=200" > /dev/null
```

## Visual Graphs

`graphviz` is installed. Generate PNG/SVG call graphs and flame graphs:

```bash
go tool pprof -png heap-record-60k.prof > heap-record-60k.png
go tool pprof -svg mutex-record-60k.prof > mutex-record-60k.svg
go tool pprof -png --diff_base=heap-record-baseline.prof heap-record-60k.prof > heap-diff-60k.png

# Interactive web viewer
go tool pprof -http=:8080 heap-record-60k.prof
```

## Other Runtime Profiles

```bash
curl -s http://localhost:6060/debug/pprof/goroutine > goroutine.prof
curl -s http://localhost:6060/debug/pprof/block > block.prof
curl -s http://localhost:6060/debug/pprof/mutex > mutex.prof

go tool pprof -top goroutine.prof
go tool pprof -top block.prof
go tool pprof -top mutex.prof
go tool pprof -list main.record mutex.prof
```
