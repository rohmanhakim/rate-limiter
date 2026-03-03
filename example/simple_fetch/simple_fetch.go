// Package main demonstrates a simple rate-limited HTTP client.
package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	ratelimiter "github.com/rohmanhakim/rate-limiter"
	"github.com/rohmanhakim/rate-limiter/example/simple_fetch/simple_logger"
)

func main() {
	fmt.Println("🚀 Rate Limiter Demo - Simple Fetch")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Start fake server
	server := NewFakeServer(":8080")
	server.Start()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)
	fmt.Println()

	// Create rate limiter using default constructor (no functional options)
	limiter := ratelimiter.NewConcurrentRateLimiter()

	// Configure using setter methods
	limiter.SetBaseDelay(500 * time.Millisecond)
	limiter.SetJitter(100 * time.Millisecond)

	// Set custom debug logger
	logger := simple_logger.NewSimpleLogger()
	limiter.SetDebugLogger(logger)

	// Create client
	client := NewSimpleClient(limiter)

	// Run requests
	client.execute()

	// Cleanup
	fmt.Println("\n🛑 Shutting down server...")
	server.Stop()
	fmt.Println("✅ Demo completed!")
}

// FakeServer is a simple HTTP server with random delay.
// No per-client state tracking - just returns random delay for each request.
type FakeServer struct {
	port     string
	minDelay time.Duration
	maxDelay time.Duration
	server   *http.Server
}

// NewFakeServer creates a new fake server instance.
func NewFakeServer(port string) *FakeServer {
	return &FakeServer{
		port:     port,
		minDelay: 200 * time.Millisecond,
		maxDelay: 1 * time.Second,
	}
}

// Start begins the fake server in a goroutine.
func (s *FakeServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.server = &http.Server{
		Addr:    s.port,
		Handler: mux,
	}

	go func() {
		fmt.Printf("🌐 Simple server started on http://localhost%s\n", s.port)
		s.server.ListenAndServe()
	}()
}

// handleRequest handles incoming HTTP requests.
// Returns 200 OK with a random delay in seconds (plain text).
func (s *FakeServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	delay := s.randomDelay()
	fmt.Printf("[server] 📥 Request received - returning delay: %s\n", delay.Round(time.Millisecond))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%.3f", delay.Seconds())
}

// randomDelay generates a random delay between min and max.
func (s *FakeServer) randomDelay() time.Duration {
	range_ := s.maxDelay - s.minDelay
	return s.minDelay + time.Duration(rand.Int63n(int64(range_)))
}

// Stop gracefully shuts down the server.
func (s *FakeServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// SimpleClient demonstrates a simple rate-limited HTTP client.
type SimpleClient struct {
	ctx     context.Context
	limiter ratelimiter.RateLimiter
	client  *http.Client
}

// NewSimpleClient creates a new simple client with rate limiting.
func NewSimpleClient(limiter ratelimiter.RateLimiter) *SimpleClient {
	return &SimpleClient{
		ctx:     context.Background(),
		limiter: limiter,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// execute runs sequential requests with rate limiting.
func (c *SimpleClient) execute() {
	const requestCount = 5
	host := "localhost:8080"

	fmt.Printf("📡 Making %d sequential requests to http://%s\n\n", requestCount, host)

	for i := 1; i <= requestCount; i++ {
		c.makeRequest(host, i)
	}
}

// makeRequest performs a single HTTP request with rate limiting.
func (c *SimpleClient) makeRequest(host string, requestNum int) {
	// Resolve delay from rate limiter before making request
	delay := c.limiter.ResolveDelay(c.ctx, host)

	// Wait if we're being rate-limited
	if delay > 0 {
		fmt.Printf("[client] ⏳ Rate limit: waiting %s before request #%d\n", delay.Round(time.Millisecond), requestNum)
		time.Sleep(delay)
	}

	// Create request
	url := fmt.Sprintf("http://%s", host)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("[client] ❌ Failed to create request #%d: %v\n", requestNum, err)
		return
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("[client] ❌ Request #%d failed: %v\n", requestNum, err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Handle response
	if resp.StatusCode == http.StatusOK {
		var seconds float64
		if _, err := fmt.Sscanf(string(body), "%f", &seconds); err == nil && seconds >= 0 {
			serverDelay := time.Duration(seconds * float64(time.Second))
			c.limiter.SetResourceDelay(host, serverDelay)
			fmt.Printf("[client] ✅ 200 OK on request #%d - server delay: %s\n", requestNum, serverDelay.Round(time.Millisecond))
		}
	} else {
		fmt.Printf("[client] ⚠️  Unexpected status %d on request #%d\n", resp.StatusCode, requestNum)
	}

	// Mark last consumed time
	c.limiter.MarkLastConsumedAsNow(host)
}
