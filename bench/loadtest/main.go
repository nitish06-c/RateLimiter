package main

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	url := flag.String("url", "http://localhost:8081/", "target URL")
	total := flag.Int("n", 1000, "total number of requests")
	concurrency := flag.Int("c", 50, "concurrent workers")
	flag.Parse()

	fmt.Printf("Load test: url=%s n=%d concurrency=%d\n\n", *url, *total, *concurrency)

	latencies := make([]float64, 0, *total)
	var mu sync.Mutex
	var allowed, denied, errors atomic.Int64

	work := make(chan struct{}, *total)
	for i := 0; i < *total; i++ {
		work <- struct{}{}
	}
	close(work)

	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}
			for range work {
				t := time.Now()
				resp, err := client.Get(*url)
				latency := time.Since(t).Seconds() * 1000 // ms

				if err != nil {
					errors.Add(1)
					continue
				}
				resp.Body.Close()

				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()

				switch resp.StatusCode {
				case http.StatusOK:
					allowed.Add(1)
				case http.StatusTooManyRequests:
					denied.Add(1)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	sort.Float64s(latencies)

	fmt.Printf("Results\n")
	fmt.Printf("-------\n")
	fmt.Printf("Duration:      %.2fs\n", elapsed.Seconds())
	fmt.Printf("Requests:      %d\n", *total)
	fmt.Printf("Throughput:    %.0f req/s\n", float64(*total)/elapsed.Seconds())
	fmt.Printf("Allowed:       %d\n", allowed.Load())
	fmt.Printf("Denied:        %d\n", denied.Load())
	fmt.Printf("Errors:        %d\n", errors.Load())
	fmt.Println()
	fmt.Printf("Latency (ms)\n")
	fmt.Printf("------------\n")
	fmt.Printf("P50:  %.2fms\n", percentile(latencies, 50))
	fmt.Printf("P95:  %.2fms\n", percentile(latencies, 95))
	fmt.Printf("P99:  %.2fms\n", percentile(latencies, 99))
	fmt.Printf("Max:  %.2fms\n", percentile(latencies, 100))

	if errors.Load() > 0 {
		fmt.Fprintf(os.Stderr, "\nWARN: %d requests failed with errors\n", errors.Load())
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(len(sorted))*p/100)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
