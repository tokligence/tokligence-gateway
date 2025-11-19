package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	totalRequests   int64
	totalErrors     int64
	totalDuration   int64 // microseconds
	minLatency      int64
	maxLatency      int64
}

func main() {
	duration := flag.Int("duration", 30, "Test duration in seconds")
	concurrency := flag.Int("c", 100, "Number of concurrent workers")
	rps := flag.Int("rps", 0, "Target requests per second (0 = unlimited)")
	url := flag.String("url", "http://localhost:8081/v1/chat/completions", "Target URL")

	flag.Parse()

	fmt.Printf("Starting load test:\n")
	fmt.Printf("  URL: %s\n", *url)
	fmt.Printf("  Duration: %d seconds\n", *duration)
	fmt.Printf("  Concurrency: %d\n", *concurrency)
	fmt.Printf("  Target RPS: %d\n", *rps)
	fmt.Println()

	stats := &Stats{minLatency: 9999999999}

	var wg sync.WaitGroup
	start := time.Now()
	done := make(chan bool)

	// Rate limiter
	var ticker *time.Ticker
	var rateChan <-chan time.Time
	if *rps > 0 {
		interval := time.Second / time.Duration(*rps)
		ticker = time.NewTicker(interval)
		rateChan = ticker.C
	}

	// Workers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{
				Timeout: 10 * time.Second,
			}

			payload := map[string]interface{}{
				"model": "loopback",
				"messages": []map[string]string{
					{"role": "user", "content": "Hello"},
				},
				"max_tokens": 100,
			}

			for {
				select {
				case <-done:
					return
				default:
					if rateChan != nil {
						<-rateChan // Rate limiting
					}

					reqStart := time.Now()

					body, _ := json.Marshal(payload)
					req, _ := http.NewRequest("POST", *url, bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer test")

					resp, err := client.Do(req)
					latency := time.Since(reqStart).Microseconds()

					atomic.AddInt64(&stats.totalRequests, 1)
					atomic.AddInt64(&stats.totalDuration, latency)

					// Update min/max
					for {
						old := atomic.LoadInt64(&stats.minLatency)
						if latency >= old || atomic.CompareAndSwapInt64(&stats.minLatency, old, latency) {
							break
						}
					}
					for {
						old := atomic.LoadInt64(&stats.maxLatency)
						if latency <= old || atomic.CompareAndSwapInt64(&stats.maxLatency, old, latency) {
							break
						}
					}

					if err != nil || resp.StatusCode != 200 {
						atomic.AddInt64(&stats.totalErrors, 1)
					}
					if resp != nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}
				}
			}
		}()
	}

	// Timer
	time.AfterFunc(time.Duration(*duration)*time.Second, func() {
		close(done)
	})

	wg.Wait()
	if ticker != nil {
		ticker.Stop()
	}

	elapsed := time.Since(start).Seconds()

	// Results
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Benchmark Results")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total Requests:     %d\n", stats.totalRequests)
	fmt.Printf("Total Failures:     %d\n", stats.totalErrors)
	fmt.Printf("Duration:           %.2f seconds\n", elapsed)
	fmt.Printf("Requests/sec:       %.2f\n", float64(stats.totalRequests)/elapsed)
	fmt.Printf("Average Latency:    %.2f ms\n", float64(stats.totalDuration)/float64(stats.totalRequests)/1000)
	fmt.Printf("Min Latency:        %.2f ms\n", float64(stats.minLatency)/1000)
	fmt.Printf("Max Latency:        %.2f ms\n", float64(stats.maxLatency)/1000)
	fmt.Printf("Error Rate:         %.2f%%\n", float64(stats.totalErrors)/float64(stats.totalRequests)*100)
	fmt.Println(strings.Repeat("=", 60))
}
