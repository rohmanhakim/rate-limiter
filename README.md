# rate-limiter

A Go package for rate limiting functionality.

## Installation

```bash
go get github.com/rohmanhakim/rate-limiter
```

## Usage

```go
package main

import "github.com/rohmanhakim/rate-limiter"

func main() {
    h := &ratelimiter.Hello{}
    h.SayHello() // Output: Hello, world!
}
```

## License

MIT License
