// Package main contains a fake HTTP server for demonstrating the rate limiter package.
package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// FakeServer simulates an HTTP server with bot/DDoS protection.
// It assigns random delays to clients (per User-Agent) and rejects
// requests made before the assigned delay has elapsed.
type FakeServer struct {
	port     string
	minDelay time.Duration
	maxDelay time.Duration
	server   *http.Server
	mu       sync.Mutex
	clients  map[string]*clientState
}

// clientState tracks the state for each unique User-Agent.
type clientState struct {
	assignedDelay time.Duration
	lastRequest   time.Time
}

// NewFakeServer creates a new fake server instance.
// Default delays: 500ms minimum, 3s maximum.
func NewFakeServer(port string) *FakeServer {
	return &FakeServer{
		port:     port,
		minDelay: 500 * time.Millisecond,
		maxDelay: 3 * time.Second,
		clients:  make(map[string]*clientState),
	}
}

// Start begins the fake server in a goroutine.
// Returns immediately; the server runs in the background.
func (s *FakeServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.server = &http.Server{
		Addr:    s.port,
		Handler: mux,
	}

	go func() {
		fmt.Printf("🛡️  Bot-protected server started on http://localhost%s\n", s.port)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		s.server.ListenAndServe()
	}()

	return nil
}

// handleRequest handles incoming HTTP requests.
// - Assigns random delay to new clients (per User-Agent)
// - Returns 429 if request comes too soon
// - Returns 200 with delay in seconds (plain text) otherwise
func (s *FakeServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	userAgent := r.UserAgent()
	if userAgent == "" {
		userAgent = "unknown"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	client, exists := s.clients[userAgent]

	// New client - assign delay
	if !exists {
		delay := s.randomDelay()
		s.clients[userAgent] = &clientState{
			assignedDelay: delay,
			lastRequest:   now,
		}
		fmt.Printf("[server] 📥 [%s] First request\n", userAgent)
		fmt.Printf("[server] ⏱️  Assigned delay: %s\n", delay.Round(time.Millisecond))
		fmt.Printf("[server] ✅ 200 OK - delay: %.3f\n", delay.Seconds())
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%.3f", delay.Seconds())
		return
	}

	elapsed := now.Sub(client.lastRequest)
	required := client.assignedDelay

	// Too soon - reject with 429
	if elapsed < required {
		remaining := required - elapsed
		fmt.Printf("[server] 📥 [%s] Request received\n", userAgent)
		fmt.Printf("[server] ❌ Too early! (elapsed: %s, required: %s)\n", elapsed.Round(time.Millisecond), required.Round(time.Millisecond))
		fmt.Printf("[server] ⚠️  429 Too Many Requests - wait %s\n", remaining.Round(time.Millisecond))
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, "too early - wait %.3f seconds", remaining.Seconds())
		return
	}

	// Delay elapsed - assign new delay
	newDelay := s.randomDelay()
	client.assignedDelay = newDelay
	client.lastRequest = now

	fmt.Printf("[server] 📥 [%s] Request received\n", userAgent)
	fmt.Printf("[server] ⏱️  New delay assigned: %s\n", newDelay.Round(time.Millisecond))
	fmt.Printf("[server] ✅ 200 OK - delay: %.3f\n", newDelay.Seconds())
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%.3f", newDelay.Seconds())
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
