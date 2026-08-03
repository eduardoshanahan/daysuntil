package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func performBearerRequest(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.1:1234"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTokenLifecycleAndBearerAuth(t *testing.T) {
	db, router, profiles := newTestServer(t)
	cookie, _ := createTestUser(t, db, profiles, "sub-alice-001", "alice")

	create := performRequest(t, router, http.MethodPost, "/api/tokens", `{"name":"cli"}`, cookie)
	if create.Code != http.StatusCreated {
		t.Fatalf("create token: expected 201, got %d (%s)", create.Code, create.Body.String())
	}
	var created struct {
		APIToken
		Token string `json:"token"`
	}
	if err := json.NewDecoder(create.Body).Decode(&created); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if created.Token == "" {
		t.Fatal("expected a raw token in the create response")
	}
	if created.Name != "cli" {
		t.Fatalf("expected name to round-trip, got %q", created.Name)
	}

	// The bearer token must authenticate exactly like the cookie does.
	me := performBearerRequest(t, router, http.MethodGet, "/api/me", "", created.Token)
	if me.Code != http.StatusOK {
		t.Fatalf("bearer /api/me: expected 200, got %d (%s)", me.Code, me.Body.String())
	}
	user := decodeUser(t, me)
	if user.Username != "alice" {
		t.Fatalf("expected bearer auth to resolve to alice, got %q", user.Username)
	}

	list := performRequest(t, router, http.MethodGet, "/api/tokens", "", cookie)
	if list.Code != http.StatusOK {
		t.Fatalf("list tokens: expected 200, got %d (%s)", list.Code, list.Body.String())
	}
	var tokens []APIToken
	json.NewDecoder(list.Body).Decode(&tokens)
	if len(tokens) != 1 || tokens[0].ID != created.ID {
		t.Fatalf("expected the created token in the list, got %#v", tokens)
	}

	del := performRequest(t, router, http.MethodDelete, "/api/tokens/"+strconv.FormatInt(created.ID, 10), "", cookie)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete token: expected 204, got %d (%s)", del.Code, del.Body.String())
	}

	revoked := performBearerRequest(t, router, http.MethodGet, "/api/me", "", created.Token)
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a revoked token, got %d (%s)", revoked.Code, revoked.Body.String())
	}
}

func TestBearerAuthRejectsUnknownToken(t *testing.T) {
	_, router, _ := newTestServer(t)

	rec := performBearerRequest(t, router, http.MethodGet, "/api/me", "", "not-a-real-token")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unknown bearer token, got %d (%s)", rec.Code, rec.Body.String())
	}
}
