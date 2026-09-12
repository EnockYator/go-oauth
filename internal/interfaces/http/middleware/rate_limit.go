package middleware

import (
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EnockYator/go-oauth/internal/interfaces/http/response"
	"github.com/EnockYator/go-oauth/internal/shared/apperror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const rateLimiterShardCount = 64

const (
	rateLimitLimitHeader     = "X-RateLimit-Limit"
	rateLimitRemainingHeader = "X-RateLimit-Remaining"
	rateLimitResetHeader     = "X-RateLimit-Reset"
	retryAfterHeader         = "Retry-After"
)

// RateLimiterConfig defines an in-process token-bucket rate limiter.
// It is used to configure the RateLimiter middleware.
// - RequestsPerSecond: Determines how quickly tokens are replenished.
// - Burst: Determines the maximum number of requests that can be accepted immediately when the bucket is full.
// - CleanupInterval: Determines how frequently inactive client buckets are removed.
// - TrustProxy: Controls whether proxy headers are trusted when determining the client IP.

type RateLimiterConfig struct {
	RequestsPerSecond float64
	Burst             int
	CleanupInterval   time.Duration
	TrustProxy        bool // TrustProxy should only be enabled when the application is behind a trusted reverse proxy/load balancer
}

// RateLimiter limits requests using a sharded in-memory token bucket.
type RateLimiter struct {
	ratePerSecond   float64
	burst           float64
	trustProxy      bool
	cleanupInterval time.Duration
	shards          [rateLimiterShardCount]*rateLimitShard
	stopOnce        sync.Once
	stopCh          chan struct{}
	doneCh          chan struct{}
}

// rateLimitShard contains the buckets belonging to one shard.
type rateLimitShard struct {
	mu      sync.Mutex
	buckets map[string]*rateLimitBucket
}

// rateLimitBucket represents one client's token bucket.
//
// Bucket state is protected by its containing shard's mutex. This keeps
// bucket lifecycle and bucket mutation under one synchronization boundary.
type rateLimitBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

// NewRateLimiter creates and validates an in-process rate limiter.
func NewRateLimiter(cfg RateLimiterConfig) (*RateLimiter, error) {
	if cfg.RequestsPerSecond <= 0 {
		return nil, fmt.Errorf(
			"invalid rate limiter configuration: requests per second must be greater than zero",
		)
	}

	if cfg.Burst <= 0 {
		return nil, fmt.Errorf(
			"invalid rate limiter configuration: burst must be greater than zero",
		)
	}

	if cfg.CleanupInterval <= 0 {
		return nil, fmt.Errorf(
			"invalid rate limiter configuration: cleanup interval must be greater than zero",
		)
	}

	rl := &RateLimiter{
		ratePerSecond:   cfg.RequestsPerSecond,
		burst:           float64(cfg.Burst),
		trustProxy:      cfg.TrustProxy,
		cleanupInterval: cfg.CleanupInterval,
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
	}

	for i := range rl.shards {
		rl.shards[i] = &rateLimitShard{
			buckets: make(map[string]*rateLimitBucket),
		}
	}

	go rl.cleanupLoop()

	return rl, nil
}

// Close stops the rate limiter's cleanup goroutine.
//
// Close is safe to call multiple times.
func (rl *RateLimiter) Close() {
	rl.stopOnce.Do(func() {
		close(rl.stopCh)
	})

	<-rl.doneCh
}

// RateLimitMiddleware applies rate limiting before passing the request
// to downstream handlers.
func (rl *RateLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.clientKey(r)

		allowed, retryAfter, remaining := rl.allow(key)

		w.Header().Set(
			rateLimitLimitHeader,
			strconv.Itoa(int(rl.burst)),
		)

		w.Header().Set(
			rateLimitRemainingHeader,
			strconv.Itoa(remaining),
		)

		span := trace.SpanFromContext(r.Context())

		if !allowed {
			if span.SpanContext().IsValid() {
				span.SetAttributes(
					attribute.Bool("rate_limit.exceeded", true),
					attribute.Float64(
						"rate_limit.retry_after_seconds",
						retryAfter.Seconds(),
					),
				)
			}

			retrySeconds := int64(retryAfter / time.Second)

			if retrySeconds < 1 {
				retrySeconds = 1
			}

			w.Header().Set(
				retryAfterHeader,
				strconv.FormatInt(retrySeconds, 10),
			)

			response.WriteError(
				w,
				r,
				apperror.New(
					r.Context(),
					apperror.CodeTooManyRequests,
					"rate limit exceeded",
					nil,
				),
			)

			return
		}

		if span.SpanContext().IsValid() {
			span.SetAttributes(
				attribute.Bool("rate_limit.exceeded", false),
			)
		}

		next.ServeHTTP(w, r)
	})
}

// allow evaluates the token bucket for the supplied client key.
//
// The entire bucket operation occurs under the shard lock. This avoids
// races between bucket lookup, creation, mutation, and cleanup.
func (rl *RateLimiter) allow(key string) (
	allowed bool,
	retryAfter time.Duration,
	remaining int,
) {
	now := time.Now()

	shard := rl.shardFor(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	bucket, exists := shard.buckets[key]

	if !exists {
		bucket = &rateLimitBucket{
			tokens:     rl.burst,
			lastRefill: now,
			lastSeen:   now,
		}

		shard.buckets[key] = bucket
	}

	elapsed := now.Sub(bucket.lastRefill).Seconds()

	if elapsed > 0 {
		bucket.tokens += elapsed * rl.ratePerSecond

		if bucket.tokens > rl.burst {
			bucket.tokens = rl.burst
		}

		bucket.lastRefill = now
	}

	bucket.lastSeen = now

	if bucket.tokens < 1 {
		deficit := 1 - bucket.tokens

		retryAfter = time.Duration(
			deficit / rl.ratePerSecond * float64(time.Second),
		)

		remaining = int(bucket.tokens)

		return false, retryAfter, remaining
	}

	bucket.tokens--

	remaining = int(bucket.tokens)

	return true, 0, remaining
}

// clientKey returns the identifier used for rate limiting.
//
// Currently the limiter is IP-based. Authentication-aware limits can be
// introduced separately without changing the token-bucket implementation.
func (rl *RateLimiter) clientKey(r *http.Request) string {
	return rl.clientIP(r)
}

// clientIP determines the client IP used by the limiter.
func (rl *RateLimiter) clientIP(r *http.Request) string {
	if rl.trustProxy {
		if ip := trustedProxyClientIP(r); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}

// trustedProxyClientIP extracts the client IP from trusted proxy headers.
//
// X-Forwarded-For is interpreted using the right-most address. This assumes
// the trusted proxy overwrites the header or otherwise guarantees that the
// right-most entry is trustworthy.
func trustedProxyClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")

		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])

			if parsed := net.ParseIP(ip); parsed != nil {
				return parsed.String()
			}
		}
	}

	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if parsed := net.ParseIP(xri); parsed != nil {
			return parsed.String()
		}
	}

	return ""
}

// shardFor returns the shard associated with a client key.
func (rl *RateLimiter) shardFor(key string) *rateLimitShard {
	hash := fnv.New32a()

	_, _ = hash.Write([]byte(key))

	return rl.shards[hash.Sum32()%rateLimiterShardCount]
}

// cleanupLoop periodically removes inactive buckets.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()
	defer close(rl.doneCh)

	for {
		select {
		case <-ticker.C:
			rl.cleanup()

		case <-rl.stopCh:
			return
		}
	}
}

// cleanup removes buckets that have been inactive long enough to no longer
// need to remain in memory.
func (rl *RateLimiter) cleanup() {
	now := time.Now()
	expiration := 2 * rl.cleanupInterval

	for _, shard := range rl.shards {
		shard.mu.Lock()

		for key, bucket := range shard.buckets {
			if now.Sub(bucket.lastSeen) > expiration {
				delete(shard.buckets, key)
			}
		}

		shard.mu.Unlock()
	}
}
