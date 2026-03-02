// Package ratelimiter provides rate limiting functionality.
package ratelimiter

import "fmt"

// Hello represents a greeting structure
type Hello struct{}

// SayHello prints "Hello, world!" to the console
func (h *Hello) SayHello() {
	fmt.Println("Hello, world!")
}
