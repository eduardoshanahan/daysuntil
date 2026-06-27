package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHTTPServerSetsTimeouts(t *testing.T) {
	handler := http.NewServeMux()
	server := newHTTPServer(":8080", handler)

	if server.Addr != ":8080" {
		t.Fatalf("expected addr :8080, got %q", server.Addr)
	}
	if server.Handler != handler {
		t.Fatal("expected handler to be assigned")
	}
	if server.ReadHeaderTimeout != readHeaderTimeout {
		t.Fatalf("expected ReadHeaderTimeout %v, got %v", readHeaderTimeout, server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != readTimeout {
		t.Fatalf("expected ReadTimeout %v, got %v", readTimeout, server.ReadTimeout)
	}
	if server.WriteTimeout != writeTimeout {
		t.Fatalf("expected WriteTimeout %v, got %v", writeTimeout, server.WriteTimeout)
	}
	if server.IdleTimeout != idleTimeout {
		t.Fatalf("expected IdleTimeout %v, got %v", idleTimeout, server.IdleTimeout)
	}
}

func TestPathOnlyLoggerOmitsQueryString(t *testing.T) {
	var buf bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	})

	handler := pathOnlyLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/github/callback?code=secret-code&state=secret-state", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logLine := buf.String()
	if !strings.Contains(logLine, "/api/oauth/github/callback") {
		t.Fatalf("expected log line to contain path, got %q", logLine)
	}
	if strings.Contains(logLine, "secret-code") || strings.Contains(logLine, "secret-state") || strings.Contains(logLine, "?code=") {
		t.Fatalf("expected query string to be omitted from log line, got %q", logLine)
	}
}
