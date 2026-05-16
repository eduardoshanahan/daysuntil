package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type handler struct {
	db           *sql.DB
	cookieSecure bool
}

func (h *handler) listIntervals(w http.ResponseWriter, r *http.Request) {
	user, err := userFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	intervals, err := listIntervals(h.db, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if intervals == nil {
		intervals = []Interval{}
	}
	writeJSON(w, intervals)
}

func (h *handler) createInterval(w http.ResponseWriter, r *http.Request) {
	user, err := userFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var iv Interval
	if err := decodeJSONBody(w, r, &iv); err != nil {
		return
	}
	if err := validateInterval(iv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := createInterval(h.db, user.ID, iv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, created)
}

func (h *handler) updateInterval(w http.ResponseWriter, r *http.Request) {
	user, err := userFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var iv Interval
	if err := decodeJSONBody(w, r, &iv); err != nil {
		return
	}
	if err := validateInterval(iv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := updateInterval(h.db, user.ID, id, iv); err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	iv.ID = id
	writeJSON(w, iv)
}

func (h *handler) deleteInterval(w http.ResponseWriter, r *http.Request) {
	user, err := userFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := deleteInterval(h.db, user.ID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	var creds userCredentials
	if err := decodeJSONBody(w, r, &creds); err != nil {
		return
	}

	user, err := createUser(h.db, creds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	token, expiresAt, err := createSession(h.db, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, token, expiresAt, h.cookieSecure)
	writeJSON(w, user)
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var creds userCredentials
	if err := decodeJSONBody(w, r, &creds); err != nil {
		return
	}

	user, err := authenticateUser(h.db, creds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	token, expiresAt, err := createSession(h.db, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, token, expiresAt, h.cookieSecure)
	writeJSON(w, user)
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		_ = deleteSession(h.db, cookie.Value)
	}

	clearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) currentUser(w http.ResponseWriter, r *http.Request) {
	user, err := authenticatedUser(h.db, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, user)
}

func validateInterval(iv Interval) error {
	name := strings.TrimSpace(iv.Name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	start, err := parseDate(iv.StartDate)
	if err != nil {
		return fmt.Errorf("invalid start_date: %w", err)
	}
	end, err := parseDate(iv.EndDate)
	if err != nil {
		return fmt.Errorf("invalid end_date: %w", err)
	}
	if !end.After(start) {
		return fmt.Errorf("end_date must be after start_date")
	}
	return nil
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
