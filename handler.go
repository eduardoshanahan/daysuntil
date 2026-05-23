package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type profileUpdate struct {
	DisplayName string `json:"display_name"`
}

type shareGroupPayload struct {
	Name string `json:"name"`
}

type moveIntervalPayload struct {
	Direction string `json:"direction"`
}

type versionResponse struct {
	Version string `json:"version"`
}

type handler struct {
	db           *sql.DB
	cookieSecure bool
	githubOAuth  githubOAuthConfig
	magicLinks   magicLinkConfig
	httpClient   *http.Client
	authLimiter  *authRateLimiter
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return err
	}

	if err := dec.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "request body must contain a single JSON object", http.StatusBadRequest)
		return err
	}

	return nil
}
