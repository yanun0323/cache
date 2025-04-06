package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheGetGood(t *testing.T) {
	testCases := []struct {
		desc               string
		defaultTTL         time.Duration
		getterTTL          []time.Duration
		waitBetweenGet     time.Duration
		expectedQueryCount int64
		expectedError      bool
	}{
		{
			desc:               "one query with default ttl",
			defaultTTL:         5 * time.Second,
			getterTTL:          []time.Duration{},
			waitBetweenGet:     3 * time.Second,
			expectedQueryCount: 1,
			expectedError:      false,
		},
		{
			desc:               "one query with getter ttl",
			defaultTTL:         time.Second,
			getterTTL:          []time.Duration{5 * time.Second},
			waitBetweenGet:     3 * time.Second,
			expectedQueryCount: 1,
			expectedError:      false,
		},
		{
			desc:               "two queries with default ttl",
			defaultTTL:         time.Second,
			getterTTL:          []time.Duration{},
			waitBetweenGet:     3 * time.Second,
			expectedQueryCount: 2,
			expectedError:      false,
		},
		{
			desc:               "two queries with getter ttl",
			defaultTTL:         5 * time.Second,
			getterTTL:          []time.Duration{time.Second},
			waitBetweenGet:     3 * time.Second,
			expectedQueryCount: 2,
			expectedError:      false,
		},
		{
			desc:               "two queries with zero ttl",
			defaultTTL:         3 * time.Second,
			getterTTL:          []time.Duration{0},
			waitBetweenGet:     time.Second,
			expectedQueryCount: 2,
			expectedError:      false,
		},
		{
			desc:               "error with query",
			defaultTTL:         time.Second,
			getterTTL:          []time.Duration{},
			waitBetweenGet:     3 * time.Second,
			expectedQueryCount: 2,
			expectedError:      true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			count := atomic.Int64{}
			cache := New(tc.defaultTTL, func(key string) (int, error) {
				<-time.After(time.Second)
				count.Add(1)
				if tc.expectedError {
					return 0, errors.New("error")
				}

				return len(key), nil
			})

			result, err := cache.Get("test", tc.getterTTL...)
			if tc.expectedError {
				requireError(t, err)
				requireEqual(t, result, 0)
			} else {
				requireNoError(t, err)
				requireEqual(t, result, 4)
			}

			<-time.After(tc.waitBetweenGet)

			result, err = cache.Get("test", tc.getterTTL...)
			if tc.expectedError {
				requireError(t, err)
				requireEqual(t, result, 0)
			} else {
				requireNoError(t, err)
				requireEqual(t, result, 4)
			}

			requireEqual(t, count.Load(), tc.expectedQueryCount)
		})
	}
}

func TestCacheCleanup(t *testing.T) {
	cache := New(time.Second, func(key string) (int, error) {
		return len(key), nil
	}, 3*time.Second).(*cache[string, int])

	cache.Set("test", 10, time.Second)
	cache.Set("test2", 12, time.Second)

	requireEqual(t, len(cache.items), 2)

	<-time.After(5 * time.Second)

	requireEqual(t, len(cache.items), 0)

}

func TestCacheGetParallelGood(t *testing.T) {
	count := atomic.Int64{}
	cache := New(time.Second, func(key string) (int, error) {
		<-time.After(time.Second)
		count.Add(1)
		return len(key), nil
	})

	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			result, err := cache.Get("test")
			requireNoError(t, err)
			requireEqual(t, result, 4)
		}()
	}

	wg.Wait()
	requireEqual(t, count.Load(), 1)
}

func TestCacheSet(t *testing.T) {
	cache := New(time.Second, func(key string) (int, error) {
		return len(key), nil
	})

	cache.Set("test", 10)
	result, err := cache.Get("test")
	requireNoError(t, err)
	requireEqual(t, result, 10)

	result, err = cache.Get("test")
	requireNoError(t, err)
	requireEqual(t, result, 10)

	<-time.After(3 * time.Second)

	result, err = cache.Get("test")
	requireNoError(t, err)
	requireEqual(t, result, 4)
}

func requireError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func requireEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}
