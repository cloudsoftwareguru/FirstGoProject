package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"time"
)

var (
	explodeOnce   sync.Once
	explodeMax    = flag.Int("explode-max", 1_000_000, "max unique keys for metric explosion simulation")
	metricStore   map[string]int
	metricStoreMu sync.RWMutex
)

func main() {
	flag.Parse()

	// Enable block and mutex profiling so /debug/pprof/block and
	// /debug/pprof/mutex return useful data.
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	// Start the pprof endpoint server.
	go func() {
		log.Println("pprof listening on http://localhost:6060/debug/pprof/")
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	http.HandleFunc("/explode", handleExplode)
	http.HandleFunc("/reset", handleReset)
	http.HandleFunc("/gc", handleGC)
	http.HandleFunc("/heapdump", handleHeapDump)

	// Simulate ongoing application work.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		pprof.Do(context.Background(), pprof.Labels("work", "steady"), func(ctx context.Context) {
			doWork()
		})
	}
}

// doWork allocates some memory and burns some CPU so profiles have data to show.
func doWork() {
	n := rand.Intn(1000)
	_ = make([]byte, n*1024)

	var sum int
	for i := 0; i < 1_000_000; i++ {
		sum += i
	}
	_ = sum
}

// handleExplode starts a background goroutine that simulates metric label
// cardinality explosion. This is the kind of allocation pattern that can cause
// an OpenTelemetry collector to run out of memory.
func handleExplode(w http.ResponseWriter, r *http.Request) {
	explodeOnce.Do(func() {
		go simulateMetricExplosion(*explodeMax)
	})
	fmt.Fprintln(w, "metric explosion simulation started")
}

// handleReset drops the simulated metric store and forces GC so you can watch
// memory return to baseline.
func handleReset(w http.ResponseWriter, r *http.Request) {
	metricStoreMu.Lock()
	metricStore = nil
	metricStoreMu.Unlock()
	runtime.GC()
	fmt.Fprintln(w, "metric store reset")
}

// handleGC forces a garbage collection cycle.
func handleGC(w http.ResponseWriter, r *http.Request) {
	runtime.GC()
	fmt.Fprintln(w, "gc forced")
}

// handleHeapDump writes a binary heap dump to a temporary file.
func handleHeapDump(w http.ResponseWriter, r *http.Request) {
	f, err := os.CreateTemp("", "heap-*.dump")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	func() {
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, fmt.Sprintf("heap dump failed: %v", rec), http.StatusInternalServerError)
			}
		}()
		debug.WriteHeapDump(f.Fd())
	}()
	fmt.Fprintf(w, "heap dump written to %s\n", f.Name())
}

// simulateMetricExplosion creates a large map with many unique string keys.
// In an OTel collector, an equivalent situation is a metric with an
// unbounded label value (for example, a user ID or request ID used as a label).
func simulateMetricExplosion(maxKeys int) {
	pprof.Do(context.Background(), pprof.Labels("work", "explosion"), func(ctx context.Context) {
		store := make(map[string]int)
		for i := 0; i < maxKeys; i++ {
			label := fmt.Sprintf("label_%d", i)
			store[label] = i
			if i%100_000 == 0 {
				log.Printf("explosion: stored %d keys", len(store))
			}
		}
		metricStoreMu.Lock()
		metricStore = store
		metricStoreMu.Unlock()
		log.Printf("explosion: finished at %d keys", len(store))
	})
}
