package main

import (
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second

	// outboundHTTPTimeout bounds every outbound call this server makes
	// (OIDC provider discovery/token exchange/JWKS, profile-service) —
	// http.DefaultClient has no timeout at all, so a stalled upstream
	// would otherwise hang the request (and the goroutine handling it)
	// indefinitely.
	outboundHTTPTimeout = 10 * time.Second
)

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}
