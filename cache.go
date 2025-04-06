// Package cache provides a lightweight, generic, and thread-safe in-memory cache implementation.
//
// This package supports any comparable key types and any value types, offering TTL (Time To Live) support
// for both default and per-item expiration. It includes background cleanup to prevent memory leaks
// and ensures all operations are thread-safe.
//
// Basic usage example:
//
//	c := cache.New[string, int](5*time.Minute, func(key string) (int, error) {
//		// This function is called when cache misses occur
//		return len(key), nil
//	})
//
//	// Get a value (will trigger the query function on first call)
//	val, err := c.Get("hello")
//
//	// Get a value with custom TTL
//	val, err := c.Get("hello", 10*time.Second)
//
//	// Set a value with default TTL
//	c.Set("count", 42)
//
//	// Set a value with custom TTL
//	c.Set("shortlived", 100, 10*time.Second)
package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	_defaultCleanupInterval = 15 * time.Minute
)

// Cache is a generic in-memory cache implementation with TTL support.
// It provides thread-safe operations for storing and retrieving values
// with automatic expiration and background cleanup.
type Cache[K comparable, V any] struct {
	mu         sync.Mutex
	items      map[K]*cacheItem[V]
	defaultTTL time.Duration
	query      func(key K) (V, error)
}

type cacheItem[V any] struct {
	expiration atomic.Int64 /* nanoseconds */
	mu         sync.RWMutex
	val        V
}

// New creates a new Cache instance with the given default TTL and query function.
// The cache will automatically clean up expired items in the background.
//
// Parameters:
//   - defaultExpiration: The default TTL for items in the cache.
//   - query: A function that takes a key and returns a value and an error.
//   - cleanupInterval: The interval at which the cache should clean up expired items.
//     If not provided, the default interval of 15 minutes will be used.
func New[K comparable, V any](defaultExpiration time.Duration, query func(key K) (V, error), cleanupInterval ...time.Duration) *Cache[K, V] {
	c := &Cache[K, V]{
		items:      make(map[K]*cacheItem[V]),
		defaultTTL: defaultExpiration,
		query:      query,
	}

	go func() {
		ticker := time.NewTicker(firstOrDefault(_defaultCleanupInterval, cleanupInterval...))
		defer ticker.Stop()

		for range ticker.C {
			c.cleanup()

		}
	}()

	return c
}

func (c *Cache[K, V]) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if item.expiration.Load() < now() {
			delete(c.items, key)
		}
	}
}
func (c *Cache[K, V]) getItem(key K) *cacheItem[V] {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if ok {
		return item
	}

	item = &cacheItem[V]{}
	c.items[key] = item
	return item
}

// Get retrieves a value from the cache.
// If the value is not in the cache, the query function will be called to get the value.
// The value will be cached and returned.
//
// Parameters:
//   - key: The key of the value to retrieve.
//   - ttl: The TTL for the value. If not provided, the default TTL will be used.
func (c *Cache[K, V]) Get(key K, ttl ...time.Duration) (V, error) {
	return getAndUpdateItemFromQuery(key, c.getItem(key), c.query, firstOrDefault(c.defaultTTL, ttl...).Nanoseconds())
}

// Set adds a value to the cache.
// If a TTL is provided, the value will be cached with that TTL.
// If no TTL is provided, the default TTL will be used.
//
// Parameters:
//   - key: The key of the value to set.
//   - value: The value to set in the cache.
//   - ttl: The TTL for the value. If not provided, the default TTL will be used.
func (c *Cache[K, V]) Set(key K, value V, ttl ...time.Duration) {
	if !isZeroTTL(ttl...) {
		updateItem(key, value, c.getItem(key), firstOrDefault(c.defaultTTL, ttl...).Nanoseconds())
	}
}

func isZeroTTL(ttl ...time.Duration) bool {
	return len(ttl) != 0 && ttl[0] == 0
}

func firstOrDefault[T any](v T, slice ...T) T {
	if len(slice) != 0 {
		return slice[0]
	}

	return v
}

func now() int64 {
	return time.Now().UnixNano()
}

func updateItem[K comparable, V any](key K, val V, item *cacheItem[V], ttl int64) {
	item.mu.Lock()
	defer item.mu.Unlock()

	item.val = val
	item.expiration.Store(now() + ttl)
}

func getAndUpdateItemFromQuery[K comparable, V any](key K, item *cacheItem[V], query func(key K) (V, error), ttl int64) (V, error) {
	nowTime := now()
	if item.expiration.Load() > nowTime {
		return item.val, nil
	}

	item.mu.Lock()
	defer item.mu.Unlock()

	if item.expiration.Load() > nowTime {
		return item.val, nil
	}

	val, err := query(key)
	if err != nil {
		return item.val, err
	}

	item.val = val
	item.expiration.Store(now() + ttl)
	return val, nil
}
