package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	ratelimiter "github.com/rohmanhakim/rate-limiter"
)

func main() {
	fmt.Println(" Rate Limiter Demo - Concurrent Multi-Service Client")
	fmt.Println("========================================================")

	// Start fake servers
	servers := []*FakeServer{
		NewFakeServer(":8080"),
		NewFakeServer(":8081"),
	}

	for _, server := range servers {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}

	// Wait for servers to be ready
	time.Sleep(100 * time.Millisecond)
	fmt.Println()

	// Create client with rate limiter
	client := NewMultiServiceClient()

	// Run concurrent requests
	client.execute()

	// Cleanup
	fmt.Println("\n Shutting down servers...")
	for _, server := range servers {
		server.Stop()
	}
	fmt.Println(" Demo completed!")
}

// Stats tracks request statistics
type Stats struct {
	SuccessCount  int64
	RateLimited   int64
	BackoffCount  int64
	BackoffReset  int64
	ErrorCount    int64
	TotalRequests int64
}

// hostState tracks per-host backoff state for logging
type hostState struct {
	mu           sync.RWMutex
	backoffCount map[string]int
}

func newHostState() *hostState {
	return &hostState{
		backoffCount: make(map[string]int),
	}
}

func (h *hostState) getBackoffCount(host string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.backoffCount[host]
}

func (h *hostState) incrementBackoffCount(host string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.backoffCount[host]++
	return h.backoffCount[host]
}

func (h *hostState) resetBackoffCount(host string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.backoffCount[host] = 0
}

// MultiServiceClient demonstrates rate-limited concurrent requests to multiple services
type MultiServiceClient struct {
	ctx       context.Context
	limiter   ratelimiter.RateLimiter
	client    *http.Client
	stats     Stats
	hostState *hostState
}

// NewMultiServiceClient creates a new multi-service client with rate limiting
func NewMultiServiceClient() *MultiServiceClient {
	return &MultiServiceClient{
		limiter: ratelimiter.NewConcurrentRateLimiter(
			ratelimiter.WithJitter(100*time.Millisecond),
			ratelimiter.WithInitialDuration(1*time.Second),
			ratelimiter.WithMultiplier(2.0),
			ratelimiter.WithMaxDuration(30*time.Second),
		),
		ctx:       context.Background(),
		client:    &http.Client{Timeout: 10 * time.Second},
		hostState: newHostState(),
	}
}

// execute runs concurrent requests to multiple services
func (m *MultiServiceClient) execute() {
	serviceHosts := []string{
		"localhost:8080",
		"localhost:8081",
	}

	const requestsPerHost = 5
	const concurrentWorkers = 2 // concurrent requests per host

	var wg sync.WaitGroup

	// Phase 1: Respecting rate limits (each worker has unique User-Agent)
	fmt.Println("=============================================================")
	fmt.Println("|  PHASE 1: Rate Limiting with Concurrent Workers           |")
	fmt.Println("|  Each worker has unique User-Agent - no 429s expected     |")
	fmt.Println("=============================================================")
	fmt.Println()

	fmt.Printf(" Starting %d concurrent workers per host (%d hosts)\n", concurrentWorkers, len(serviceHosts))
	fmt.Printf(" %d requests per worker\n\n", requestsPerHost)

	startTime := time.Now()

	for _, host := range serviceHosts {
		for i := 0; i < concurrentWorkers; i++ {
			wg.Add(1)
			workerID := i + 1
			go m.runWorker(host, workerID, requestsPerHost, &wg)
		}
	}

	wg.Wait()

	// Print phase 1 summary
	m.printStats("Phase 1")

	// Phase 2: Demonstrate backoff (workers share User-Agent, causing 429s)
	fmt.Println("\n=============================================================")
	fmt.Println("|  PHASE 2: Backoff Demonstration                           |")
	fmt.Println("|  Workers share User-Agent - expect 429s and backoffs      |")
	fmt.Println("=============================================================")
	fmt.Println()

	// Reset stats for phase 2
	m.stats = Stats{}

	fmt.Printf(" Starting %d concurrent workers per host (SHARED User-Agent)\n", concurrentWorkers)
	fmt.Printf(" %d requests per worker\n\n", 3)

	// Reset host state for phase 2
	m.hostState = newHostState()

	for _, host := range serviceHosts {
		for i := 0; i < concurrentWorkers; i++ {
			wg.Add(1)
			workerID := i + 1
			go m.runWorkerSharedAgent(host, workerID, 3, &wg)
		}
	}

	wg.Wait()

	// Print final summary
	m.printStats("Phase 2 (Backoff Summary)")

	// Total duration
	fmt.Printf("\n  Total duration: %s\n", time.Since(startTime).Round(time.Millisecond))
}

// runWorkerSharedAgent executes requests with a SHARED User-Agent per host
// This will cause 429 responses when concurrent workers hit the same server
func (m *MultiServiceClient) runWorkerSharedAgent(host string, workerID, requestCount int, wg *sync.WaitGroup) {
	defer wg.Done()

	// All workers for the same host share the same User-Agent
	agent := fmt.Sprintf("%s-shared", host)

	for i := 1; i <= requestCount; i++ {
		m.makeRequest(host, agent, i)
	}
}

// printStats displays statistics for a phase
func (m *MultiServiceClient) printStats(phase string) {
	fmt.Println("\n-------------------------------------------------------")
	fmt.Printf("%s SUMMARY\n", phase)
	fmt.Println("-------------------------------------------------------")
	fmt.Printf("   Total requests:  %d\n", m.stats.TotalRequests)
	fmt.Printf("   Successful:      %d\n", m.stats.SuccessCount)
	fmt.Printf("   Rate limited:    %d\n", m.stats.RateLimited)
	fmt.Printf("   Backoff events:  %d\n", m.stats.BackoffCount)
	fmt.Printf("   Backoff resets:  %d\n", m.stats.BackoffReset)
	fmt.Printf("   Errors:          %d\n", m.stats.ErrorCount)
}

// runWorker executes requests for a single host
func (m *MultiServiceClient) runWorker(host string, workerID, requestCount int, wg *sync.WaitGroup) {
	defer wg.Done()

	agent := fmt.Sprintf("%s-worker-%d", host, workerID)

	for i := 1; i <= requestCount; i++ {
		m.makeRequest(host, agent, i)
	}
}

// makeRequest performs a single HTTP request with rate limiting
func (m *MultiServiceClient) makeRequest(host, agent string, requestNum int) {
	atomic.AddInt64(&m.stats.TotalRequests, 1)

	// Wait for rate limiter to allow the request
	if err := m.limiter.Wait(m.ctx, host); err != nil {
		fmt.Printf("[client] [%s] Wait cancelled for request #%d: %v\n", agent, requestNum, err)
		atomic.AddInt64(&m.stats.ErrorCount, 1)
		return
	}

	// Create new request for each call (don't reuse)
	url := fmt.Sprintf("http://%s", host)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("[client] [%s] Failed to create request #%d: %v\n", agent, requestNum, err)
		atomic.AddInt64(&m.stats.ErrorCount, 1)
		return
	}
	req.Header.Set("User-Agent", agent)

	// Execute request
	resp, err := m.client.Do(req)
	if err != nil {
		fmt.Printf("[client] [%s] Request #%d failed: %v\n", agent, requestNum, err)
		atomic.AddInt64(&m.stats.ErrorCount, 1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		m.handleSuccess(host, agent, requestNum, body)
	case http.StatusTooManyRequests:
		m.handleRateLimited(host, agent, requestNum, body)
	default:
		fmt.Printf("[client]  [%s] Unexpected status %d on request #%d\n", agent, resp.StatusCode, requestNum)
		atomic.AddInt64(&m.stats.ErrorCount, 1)
	}
}

// handleSuccess processes a successful response
func (m *MultiServiceClient) handleSuccess(host, agent string, requestNum int, body []byte) {
	atomic.AddInt64(&m.stats.SuccessCount, 1)

	// Parse server-assigned delay from response
	var seconds float64
	if _, err := fmt.Sscanf(string(body), "%f", &seconds); err == nil && seconds >= 0 {
		delay := time.Duration(seconds * float64(time.Second))
		m.limiter.SetResourceDelay(host, delay)
		fmt.Printf("[client] [%s] 200 OK on request #%d - server delay: %s\n", agent, requestNum, delay.Round(time.Millisecond))
	}

	// Reset backoff on success (if there was an active backoff)
	if m.hostState.getBackoffCount(host) > 0 {
		m.limiter.ResetBackoff(host)
		m.hostState.resetBackoffCount(host)
		atomic.AddInt64(&m.stats.BackoffReset, 1)
		fmt.Printf("[client] [%s] Backoff reset after successful request #%d\n", agent, requestNum)
	}
}

// handleRateLimited processes a 429 response
func (m *MultiServiceClient) handleRateLimited(host, agent string, requestNum int, body []byte) {
	atomic.AddInt64(&m.stats.RateLimited, 1)

	// Parse server-suggested delay from response body
	// Format: "too early - wait 0.500 seconds"
	serverDelay := parseServerDelay(string(body))

	// Trigger backoff and get the new count
	backoffCount := m.hostState.incrementBackoffCount(host)
	m.limiter.Backoff(m.ctx, host, ratelimiter.BackoffOptions{
		ServerDelay: serverDelay,
	})
	atomic.AddInt64(&m.stats.BackoffCount, 1)

	// Resolve the delay to show what backoff delay was computed
	backoffDelay := m.limiter.ResolveDelay(m.ctx, host)

	fmt.Printf("[client] [%s] 429 on request #%d - backing off (count: %d, delay: %s)\n",
		agent, requestNum, backoffCount, backoffDelay.Round(time.Millisecond))
	fmt.Printf("[client]    Server message: %s\n", string(body))
}

// parseServerDelay extracts the delay duration from the server response.
// Format: "too early - wait 0.500 seconds"
// Returns 0 if parsing fails.
func parseServerDelay(body string) time.Duration {
	var seconds float64
	n, err := fmt.Sscanf(body, "too early - wait %f seconds", &seconds)
	if err != nil || n != 1 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}
