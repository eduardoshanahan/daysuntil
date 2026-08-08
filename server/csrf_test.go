package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFProtectionMiddlewareAllowsSafeMethods(t *testing.T) {
	h := &handler{}
	mw := csrfProtectionMiddleware(h)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		req := httptest.NewRequest(method, "/api/intervals", nil)
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected safe method to bypass the origin check, got %d", method, rec.Code)
		}
	}
}

func TestCSRFProtectionMiddlewareBlocksMissingOrigin(t *testing.T) {
	h := &handler{}
	mw := csrfProtectionMiddleware(h)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/intervals", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a cookie-shaped request with no Origin header, got %d", rec.Code)
	}
}

func TestCSRFProtectionMiddlewareBlocksMismatchedOrigin(t *testing.T) {
	h := &handler{}
	mw := csrfProtectionMiddleware(h)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/intervals", nil)
	req.Header.Set("Origin", "https://attacker.example")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a forged cross-site Origin, got %d", rec.Code)
	}
}

func TestCSRFProtectionMiddlewareAllowsSameOriginRequest(t *testing.T) {
	h := &handler{} // no WEB_ORIGIN: same-origin deployment, expected origin is derived from Host
	mw := csrfProtectionMiddleware(h)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/intervals", nil) // req.Host defaults to example.com
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected a matching same-origin request to pass, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestCSRFProtectionMiddlewareUsesConfiguredWebOrigin(t *testing.T) {
	h := &handler{webOrigin: "https://daysuntil-web.example.com"}
	mw := csrfProtectionMiddleware(h)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// The API's own host is no longer a valid Origin once WEB_ORIGIN is set —
	// only the configured web frontend is.
	sameHost := httptest.NewRequest(http.MethodPost, "/api/intervals", nil)
	sameHost.Header.Set("Origin", "https://daysuntil-api.example.com")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, sameHost)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected the API's own origin to be rejected once WEB_ORIGIN is set, got %d", rec.Code)
	}

	webOrigin := httptest.NewRequest(http.MethodPost, "/api/intervals", nil)
	webOrigin.Header.Set("Origin", "https://daysuntil-web.example.com")
	rec = httptest.NewRecorder()
	mw.ServeHTTP(rec, webOrigin)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected the configured WEB_ORIGIN to be accepted, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestCSRFProtectionMiddlewareExemptsBearerRequests(t *testing.T) {
	h := &handler{webOrigin: "https://daysuntil-web.example.com"}
	mw := csrfProtectionMiddleware(h)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No Origin header at all — the shape of a native client, not a browser —
	// but a bearer token is present, so nothing can be forged.
	req := httptest.NewRequest(http.MethodPost, "/api/intervals", nil)
	req.Header.Set("Authorization", "Bearer some-api-token")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected a bearer-authenticated request to bypass the origin check, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestCSRFIntegrationBlocksCrossSiteCookieRequest(t *testing.T) {
	db, router, profiles := newTestServer(t)
	cookie, _ := createTestUser(t, db, profiles, "sub-csrf-001", "csrfuser")

	req := httptest.NewRequest(http.MethodPost, "/api/intervals", nil)
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected a forged cross-site request to be rejected, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestDecodeJSONBodyRejectsNonJSONContentType(t *testing.T) {
	db, router, profiles := newTestServer(t)
	cookie, _ := createTestUser(t, db, profiles, "sub-csrf-002", "csrfuser2")

	req := httptest.NewRequest(http.MethodPost, "/api/tokens", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415 for a non-JSON Content-Type, got %d (%s)", rec.Code, rec.Body.String())
	}
}
