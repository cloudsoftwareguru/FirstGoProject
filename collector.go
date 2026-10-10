package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"
)

// aggregator holds a tiny cumulative aggregation, similar to what an OTel
// collector might keep for a single metric time series.
type aggregator struct {
	sum   int64
	count int64
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]map[string]*aggregator)

	loadStop    chan struct{}
	loadOnce    sync.Once
	loadRunning bool
	loadMu      sync.Mutex
)

func init() {
	http.HandleFunc("/record/", handleRecord)
	http.HandleFunc("/start-load", handleStartLoad)
	http.HandleFunc("/stop-load", handleStopLoad)
	http.HandleFunc("/registry-size", handleRegistrySize)
}

// handleRecord lets you manually record a metric point with arbitrary labels.
// Example: curl 'http://localhost:6060/record/http_request_duration?client=abc&status=200'
func handleRecord(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Path[len("/record/"):]
	labels := r.URL.Query().Encode()
	if metric == "" {
		http.Error(w, "metric name required", http.StatusBadRequest)
		return
	}
	record(metric, labels, rand.Int63n(100))
	fmt.Fprintf(w, "recorded %s{%s}\n", metric, labels)
}

// handleStartLoad starts a background goroutine that records metrics with
// monotonically increasing label values. This mimics a metric explosion in an
// OTel collector (for example, a high-cardinality label like request_id).
func handleStartLoad(w http.ResponseWriter, r *http.Request) {
	loadMu.Lock()
	defer loadMu.Unlock()
	if loadRunning {
		fmt.Fprintln(w, "load generator already running")
		return
	}
	loadStop = make(chan struct{})
	loadRunning = true
	go runLoad(loadStop)
	fmt.Fprintln(w, "load generator started")
}

// handleStopLoad stops the background load generator.
func handleStopLoad(w http.ResponseWriter, r *http.Request) {
	loadMu.Lock()
	defer loadMu.Unlock()
	if !loadRunning {
		fmt.Fprintln(w, "load generator not running")
		return
	}
	close(loadStop)
	loadRunning = false
	fmt.Fprintln(w, "load generator stopped")
}

// handleRegistrySize reports how many metrics and label sets are currently
// held in memory.
func handleRegistrySize(w http.ResponseWriter, r *http.Request) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	metrics := len(registry)
	sets := 0
	for _, m := range registry {
		sets += len(m)
	}
	fmt.Fprintf(w, "metrics: %d\nlabel sets: %d\n", metrics, sets)
}

func runLoad(stop <-chan struct{}) {
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		default:
		}

		label := "client_id=" + strconv.Itoa(i)
		pprof.Do(context.Background(), pprof.Labels("work", "explosion"), func(ctx context.Context) {
			record("http_request_duration", label, int64(i))
		})

		if i%100000 == 0 {
			log.Printf("load generator: recorded %d unique label sets", i)
		}
		time.Sleep(10 * time.Microsecond)
	}
}

func record(metric, labels string, value int64) {
	registryMu.Lock()
	defer registryMu.Unlock()
	m, ok := registry[metric]
	if !ok {
		m = make(map[string]*aggregator)
		registry[metric] = m
	}
	a, ok := m[labels]
	if !ok {
		a = &aggregator{}
		m[labels] = a
	}
	a.sum += value
	a.count++
}
