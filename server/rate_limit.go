package main

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	authRateLimitWindow           = time.Minute
	publicLookupRateLimitRequests = 60
	authActionPublicLookup        = "public_lookup"
)

type authRatePolicy struct {
	limit  int
	window time.Duration
}

type authRateBucket struct {
	count   int
	resetAt time.Time
}

type authRateLimiter struct {
	mu       sync.Mutex
	db       *sql.DB
	now      func() time.Time
	policies map[string]authRatePolicy
	buckets  map[string]authRateBucket
}

func newAuthRateLimiter(db *sql.DB) *authRateLimiter {
	l := &authRateLimiter{
		db:  db,
		now: time.Now,
		policies: map[string]authRatePolicy{
			authActionPublicLookup: {
				limit:  publicLookupRateLimitRequests,
				window: authRateLimitWindow,
			},
		},
		buckets: make(map[string]authRateBucket),
	}
	if db != nil {
		l.loadFromDB()
	}
	return l
}

func (l *authRateLimiter) loadFromDB() {
	rows, err := l.db.Query(`SELECT key, count, reset_at FROM rate_limit_buckets WHERE reset_at > $1`, l.now().UTC())
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var count int
		var resetAt time.Time
		if err := rows.Scan(&key, &count, &resetAt); err != nil {
			continue
		}
		l.buckets[key] = authRateBucket{count: count, resetAt: resetAt}
	}
}

func (l *authRateLimiter) persistBucket(key string, bucket authRateBucket) {
	if l.db == nil {
		return
	}
	_, _ = l.db.Exec(
		`INSERT INTO rate_limit_buckets (key, count, reset_at) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET count = EXCLUDED.count, reset_at = EXCLUDED.reset_at`,
		key, bucket.count, bucket.resetAt.UTC(),
	)
}

func (l *authRateLimiter) allow(action, clientIP string) (bool, time.Duration) {
	policy, ok := l.policies[action]
	if !ok {
		return true, 0
	}

	now := l.now()
	key := action + ":" + clientIP

	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanupExpired(now)

	bucket, ok := l.buckets[key]
	if !ok || !bucket.resetAt.After(now) {
		b := authRateBucket{count: 1, resetAt: now.Add(policy.window)}
		l.buckets[key] = b
		l.persistBucket(key, b)
		return true, 0
	}

	if bucket.count >= policy.limit {
		return false, bucket.resetAt.Sub(now).Round(time.Second)
	}

	bucket.count++
	l.buckets[key] = bucket
	l.persistBucket(key, bucket)
	return true, 0
}

func (l *authRateLimiter) cleanupExpired(now time.Time) {
	for key, bucket := range l.buckets {
		if !bucket.resetAt.After(now) {
			delete(l.buckets, key)
		}
	}
	if l.db != nil {
		_, _ = l.db.Exec(`DELETE FROM rate_limit_buckets WHERE reset_at <= $1`, now.UTC())
	}
}

func authRateLimitMiddleware(limiter *authRateLimiter, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := clientIPFromRequest(r)
			allowed, retryAfter := limiter.allow(action, clientIP)
			if !allowed {
				seconds := max(int(retryAfter.Seconds()), 1)
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				http.Error(w, fmt.Sprintf("too many %s attempts, try again later", action), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIPFromRequest(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		ip := xff
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			ip = strings.TrimSpace(xff[:idx])
		}
		if ip != "" {
			return ip
		}
	}

	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if remoteAddr == "" {
		return "unknown"
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}
	return remoteAddr
}
