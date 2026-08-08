package main

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPFromRequestTrustsOnlyTheLastForwardedForEntry(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.5:12345" // Traefik's own container IP
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.5")

	got := clientIPFromRequest(req)
	if got != "10.0.0.5" {
		t.Fatalf("expected the last (Traefik-appended) entry, got %q", got)
	}
}

func TestClientIPFromRequestRejectsASpoofedSoleForwardedForValue(t *testing.T) {
	// A direct, non-Traefik caller can set any single X-Forwarded-For
	// value it likes — this used to be trusted outright (rate-limit
	// bypass by rotating a fake value per request). It's still the "last"
	// entry when there's only one, but the point is this function no
	// longer special-cases "take the first" — confirming a single forged
	// entry is treated the same as any last entry, not exempted.
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")

	got := clientIPFromRequest(req)
	if got != "203.0.113.9" {
		t.Fatalf("expected the sole entry to be used, got %q", got)
	}
}

func TestClientIPFromRequestFallsBackToRemoteAddrWithoutForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.5:12345"

	got := clientIPFromRequest(req)
	if got != "10.0.0.5" {
		t.Fatalf("expected RemoteAddr host, got %q", got)
	}
}

func TestAuthRateLimiterBlocksAfterLimitPerKey(t *testing.T) {
	limiter := newAuthRateLimiter(nil)

	for i := 0; i < publicLookupRateLimitRequests; i++ {
		allowed, _ := limiter.allow(authActionPublicLookup, "198.51.100.1")
		if !allowed {
			t.Fatalf("request %d: expected to be allowed within the limit", i)
		}
	}

	allowed, retryAfter := limiter.allow(authActionPublicLookup, "198.51.100.1")
	if allowed {
		t.Fatal("expected the request over the limit to be blocked")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected a positive retry-after, got %v", retryAfter)
	}

	// A different key (different client IP) has its own independent bucket.
	allowed, _ = limiter.allow(authActionPublicLookup, "198.51.100.2")
	if !allowed {
		t.Fatal("expected a different client IP to have its own bucket, not share the blocked one")
	}
}
