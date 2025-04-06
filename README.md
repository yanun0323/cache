# Go Cache

A lightweight, generic, and thread-safe in-memory cache implementation for Go.

## Features

- Generic implementation supporting any comparable key types and any value types
- TTL (Time To Live) support for both default and per-item expiration
- Background cleanup to prevent memory leaks
- Thread-safe operations
- Automatic value retrieval via customizable query functions

## Requirements

- Go 1.23+

## Installation

```bash
go get github.com/yanun0323/cache
```

## Usage

### Basic Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/yanun0323/cache"
)

func main() {
	// Create a new cache with:
	// - Default TTL of 5 minutes
	// - A query function that handles cache misses
	// - Default cleanup interval (15 minutes)
	c := cache.New[string, int](5*time.Minute, func(key string) (int, error) {
		// This function is called when cache misses occur
		// Perform database lookup, API call, or computation
		return len(key), nil
	})

	// Get a value (will trigger the query function on first call)
	val, err := c.Get("hello")
	if err != nil {
		// Handle error
	}
	fmt.Println(val) // Output: 5

	// Set a value with default TTL
	c.Set("count", 42)

	// Get the value
	val, _ = c.Get("count")
	fmt.Println(val) // Output: 42

	// Set a value with custom TTL
	c.Set("shortlived", 100, 10*time.Second)
}
```

### Customizing Cache Behavior

```go
// Create a cache with custom cleanup interval
c := cache.New[string, int](
	5*time.Minute,               // Default TTL
	func(key string) (int, error) {
		return len(key), nil
	},
	1*time.Minute,               // Cleanup interval
)

// Get with custom TTL for this specific request
val, err := c.Get("key", 30*time.Second)
```

## Thread Safety

All cache operations are thread-safe. Multiple goroutines can safely call methods on a single cache instance.

## License

This project is open source and available under the [MIT License](LICENSE).
