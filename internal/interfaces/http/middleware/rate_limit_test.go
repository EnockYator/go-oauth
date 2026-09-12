package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  RateLimiterConfig
	}{
		{
			name: "valid configuration",
			cfg: RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             20,
				CleanupInterval:   time.Minute,
			},
		},
		{
			name: "minimum valid configuration",
			cfg: RateLimiterConfig{
				RequestsPerSecond: 0.1,
				Burst:             1,
				CleanupInterval:   time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl, err := NewRateLimiter(tt.cfg)
			if err != nil {
				t.Fatalf("NewRateLimiter() error = %v", err)
			}

			rl.Close()
		})
	}
}

func TestNewRateLimiter_InvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  RateLimiterConfig
	}{
		{
			name: "zero requests per second",
			cfg: RateLimiterConfig{
				RequestsPerSecond: 0,
				Burst:             10,
				CleanupInterval:   time.Minute,
			},
		},
		{
			name: "negative requests per second",
			cfg: RateLimiterConfig{
				RequestsPerSecond: -1,
				Burst:             10,
				CleanupInterval:   time.Minute,
			},
		},
		{
			name: "zero burst",
			cfg: RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             0,
				CleanupInterval:   time.Minute,
			},
		},
		{
			name: "negative burst",
			cfg: RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             -1,
				CleanupInterval:   time.Minute,
			},
		},
		{
			name: "zero cleanup interval",
			cfg: RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             10,
				CleanupInterval:   0,
			},
		},
		{
			name: "negative cleanup interval",
			cfg: RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             10,
				CleanupInterval:   -time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl, err := NewRateLimiter(tt.cfg)

			if err == nil {
				if rl != nil {
					rl.Close()
				}

				t.Fatal("expected configuration error, got nil")
			}

			if rl != nil {
				rl.Close()
				t.Fatal("expected nil RateLimiter on configuration error")
			}
		})
	}
}

func TestRateLimiter_AllowBurst(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             3,
		CleanupInterval:   time.Minute,
	})

	for i := 0; i < 3; i++ {
		allowed, retryAfter, remaining := rl.allow("client-1")

		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}

		if retryAfter != 0 {
			t.Fatalf("request %d retryAfter = %v, want 0", i+1, retryAfter)
		}

		expectedRemaining := 2 - i

		if remaining != expectedRemaining {
			t.Fatalf(
				"request %d remaining = %d, want %d",
				i+1,
				remaining,
				expectedRemaining,
			)
		}
	}

	allowed, retryAfter, remaining := rl.allow("client-1")

	if allowed {
		t.Fatal("request after burst should be rejected")
	}

	if retryAfter <= 0 {
		t.Fatalf("retryAfter = %v, want > 0", retryAfter)
	}

	if remaining != 0 {
		t.Fatalf("remaining = %d, want 0", remaining)
	}
}

func TestRateLimiter_RefillsTokens(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 100,
		Burst:             1,
		CleanupInterval:   time.Minute,
	})

	allowed, _, _ := rl.allow("client-1")

	if !allowed {
		t.Fatal("first request should be allowed")
	}

	allowed, retryAfter, _ := rl.allow("client-1")

	if allowed {
		t.Fatal("second immediate request should be rejected")
	}

	if retryAfter <= 0 {
		t.Fatalf("retryAfter = %v, want > 0", retryAfter)
	}

	time.Sleep(20 * time.Millisecond)

	allowed, retryAfter, _ = rl.allow("client-1")

	if !allowed {
		t.Fatalf(
			"request after token refill should be allowed; retryAfter=%v",
			retryAfter,
		)
	}
}

func TestRateLimiter_ClientsHaveIndependentBuckets(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             1,
		CleanupInterval:   time.Minute,
	})

	allowed, _, _ := rl.allow("client-1")
	if !allowed {
		t.Fatal("client-1 first request should be allowed")
	}

	allowed, _, _ = rl.allow("client-1")
	if allowed {
		t.Fatal("client-1 second request should be rejected")
	}

	allowed, _, _ = rl.allow("client-2")
	if !allowed {
		t.Fatal("client-2 should have an independent bucket")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             1,
		CleanupInterval:   time.Hour,
	})

	key := "client-1"

	allowed, _, _ := rl.allow(key)

	if !allowed {
		t.Fatal("initial request should be allowed")
	}

	shard := rl.shardFor(key)

	shard.mu.Lock()

	bucket, exists := shard.buckets[key]
	if !exists {
		shard.mu.Unlock()
		t.Fatal("expected bucket to exist")
	}

	bucket.lastSeen = time.Now().Add(-2 * time.Hour)

	shard.mu.Unlock()

	rl.cleanup()

	shard.mu.Lock()
	_, exists = shard.buckets[key]
	shard.mu.Unlock()

	if exists {
		t.Fatal("stale bucket should have been removed")
	}
}

func TestRateLimiter_CleanupKeepsActiveBuckets(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             1,
		CleanupInterval:   time.Hour,
	})

	key := "active-client"

	allowed, _, _ := rl.allow(key)

	if !allowed {
		t.Fatal("initial request should be allowed")
	}

	rl.cleanup()

	shard := rl.shardFor(key)

	shard.mu.Lock()
	_, exists := shard.buckets[key]
	shard.mu.Unlock()

	if !exists {
		t.Fatal("active bucket should not be removed")
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 1000,
		Burst:             1000,
		CleanupInterval:   time.Minute,
	})

	const (
		goroutines = 100
		requests   = 100
	)

	var allowed atomic.Int64

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < requests; j++ {
				ok, _, _ := rl.allow("same-client")

				if ok {
					allowed.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	if allowed.Load() <= 0 {
		t.Fatal("expected at least some requests to be allowed")
	}
}

func TestRateLimiter_ConcurrentDifferentClients(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 100,
		Burst:             10,
		CleanupInterval:   time.Minute,
	})

	const clients = 100

	var wg sync.WaitGroup

	wg.Add(clients)

	for i := 0; i < clients; i++ {
		go func(i int) {
			defer wg.Done()

			key := "client-" + strconv.Itoa(i)

			for j := 0; j < 10; j++ {
				rl.allow(key)
			}
		}(i)
	}

	wg.Wait()
}

func TestRateLimiter_ClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		trustProxy bool
		remoteAddr string
		xff        string
		xRealIP    string
		want       string
	}{
		{
			name:       "direct IPv4",
			trustProxy: false,
			remoteAddr: "192.168.1.10:12345",
			want:       "192.168.1.10",
		},
		{
			name:       "direct IPv6",
			trustProxy: false,
			remoteAddr: "[2001:db8::1]:12345",
			want:       "2001:db8::1",
		},
		{
			name:       "ignores X-Forwarded-For when proxy is not trusted",
			trustProxy: false,
			remoteAddr: "192.168.1.10:12345",
			xff:        "10.0.0.1",
			want:       "192.168.1.10",
		},
		{
			name:       "uses X-Forwarded-For when proxy is trusted",
			trustProxy: true,
			remoteAddr: "192.168.1.10:12345",
			xff:        "203.0.113.10",
			want:       "203.0.113.10",
		},
		{
			name:       "uses rightmost valid X-Forwarded-For address",
			trustProxy: true,
			remoteAddr: "192.168.1.10:12345",
			xff:        "203.0.113.10, 10.0.0.1, 192.168.1.20",
			want:       "192.168.1.20",
		},
		{
			name:       "falls back to X-Real-IP",
			trustProxy: true,
			remoteAddr: "192.168.1.10:12345",
			xRealIP:    "203.0.113.20",
			want:       "203.0.113.20",
		},
		{
			name:       "ignores invalid X-Forwarded-For",
			trustProxy: true,
			remoteAddr: "192.168.1.10:12345",
			xff:        "not-an-ip",
			want:       "192.168.1.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := newTestRateLimiter(t, RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             10,
				CleanupInterval:   time.Minute,
				TrustProxy:        tt.trustProxy,
			})

			req := httptest.NewRequest(
				http.MethodGet,
				"/",
				nil,
			)

			req.RemoteAddr = tt.remoteAddr

			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}

			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			got := rl.clientIP(req)

			if got != tt.want {
				t.Fatalf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimitMiddleware_AllowsRequest(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             2,
		CleanupInterval:   time.Minute,
	})

	var called atomic.Bool

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.RateLimitMiddleware(next)

	req := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	)

	req.RemoteAddr = "192.168.1.10:12345"

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called.Load() {
		t.Fatal("next handler was not called")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get(rateLimitLimitHeader); got != "2" {
		t.Fatalf(
			"%s = %q, want %q",
			rateLimitLimitHeader,
			got,
			"2",
		)
	}

	if got := rec.Header().Get(rateLimitRemainingHeader); got != "1" {
		t.Fatalf(
			"%s = %q, want %q",
			rateLimitRemainingHeader,
			got,
			"1",
		)
	}
}

func TestRateLimitMiddleware_RejectsExceededRequest(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             1,
		CleanupInterval:   time.Minute,
	})

	var called atomic.Bool

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.RateLimitMiddleware(next)

	req1 := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	)

	req1.RemoteAddr = "192.168.1.10:12345"

	rec1 := httptest.NewRecorder()

	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d", rec1.Code, http.StatusOK)
	}

	called.Store(false)

	req2 := httptest.NewRequest(
		http.MethodGet,
		"/test",
		nil,
	)

	req2.RemoteAddr = "192.168.1.10:12345"

	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	if called.Load() {
		t.Fatal("next handler should not be called after rate limit is exceeded")
	}

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf(
			"status = %d, want %d",
			rec2.Code,
			http.StatusTooManyRequests,
		)
	}

	retryAfter := rec2.Header().Get(retryAfterHeader)

	if retryAfter == "" {
		t.Fatal("Retry-After header is missing")
	}

	if _, err := strconv.Atoi(retryAfter); err != nil {
		t.Fatalf(
			"Retry-After = %q, want integer seconds",
			retryAfter,
		)
	}

	if got := rec2.Header().Get(rateLimitLimitHeader); got != "1" {
		t.Fatalf(
			"%s = %q, want %q",
			rateLimitLimitHeader,
			got,
			"1",
		)
	}

	if got := rec2.Header().Get(rateLimitRemainingHeader); got != "0" {
		t.Fatalf(
			"%s = %q, want %q",
			rateLimitRemainingHeader,
			got,
			"0",
		)
	}
}

func TestRateLimiter_CloseIsIdempotent(t *testing.T) {
	t.Parallel()

	rl, err := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             10,
		CleanupInterval:   time.Minute,
	})
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	done := make(chan struct{})

	go func() {
		rl.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close() did not return")
	}

	// Close should be safe to call repeatedly.
	rl.Close()
}

func TestRateLimiter_CleanupDoesNotRaceWithRequests(t *testing.T) {
	t.Parallel()

	rl := newTestRateLimiter(t, RateLimiterConfig{
		RequestsPerSecond: 1000,
		Burst:             100,
		CleanupInterval:   10 * time.Millisecond,
	})

	const (
		goroutines = 20
		iterations = 100
	)

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()

			key := "client-" + strconv.Itoa(i)

			for j := 0; j < iterations; j++ {
				rl.allow(key)
			}
		}(i)
	}

	wg.Wait()
}

func newTestRateLimiter(
	t *testing.T,
	cfg RateLimiterConfig,
) *RateLimiter {
	t.Helper()

	rl, err := NewRateLimiter(cfg)
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	t.Cleanup(func() {
		rl.Close()
	})

	return rl
}
