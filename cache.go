package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	_defaultCleanupInterval = 15 * time.Minute
)

type querier[K comparable, V any] func(key K) (V, error)

type Cache[K comparable, V any] interface {
	Get(key K, ttl ...time.Duration) (V, error)
	Set(key K, value V, ttl ...time.Duration)
}

type cache[K comparable, V any] struct {
	mu         sync.Mutex
	items      map[K]*cacheItem[V]
	defaultTTL time.Duration
	query      querier[K, V]
}

type cacheItem[V any] struct {
	expiration atomic.Int64 /* nanoseconds */
	mu         sync.RWMutex
	val        V
}

func New[K comparable, V any](defaultExpiration time.Duration, query querier[K, V], cleanupInterval ...time.Duration) Cache[K, V] {
	c := &cache[K, V]{
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

func (c *cache[K, V]) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if item.expiration.Load() < now() {
			delete(c.items, key)
		}
	}
}
func (c *cache[K, V]) getItem(key K) *cacheItem[V] {
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

func (c *cache[K, V]) Get(key K, ttl ...time.Duration) (V, error) {
	return getAndUpdateItemFromQuery(key, c.getItem(key), c.query, firstOrDefault(c.defaultTTL, ttl...).Nanoseconds())
}

func (c *cache[K, V]) Set(key K, value V, ttl ...time.Duration) {
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

func getAndUpdateItemFromQuery[K comparable, V any](key K, item *cacheItem[V], query querier[K, V], ttl int64) (V, error) {
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
