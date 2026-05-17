package main

import (
	"net/http"
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
