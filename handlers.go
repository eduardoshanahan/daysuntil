package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type profileUpdate struct {
	DisplayName string `json:"display_name"`
}

type versionResponse struct {
	Version string `json:"version"`
}

type handler struct {
	db           *sql.DB
	cookieSecure bool
	githubOAuth  githubOAuthConfig
	httpClient   *http.Client
	authLimiter  *authRateLimiter
}

func (h *handler) appVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, versionResponse{Version: currentVersion()})
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
	var creds registerCredentials
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
	var creds loginCredentials
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

func (h *handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	user, err := authenticatedUser(h.db, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := deleteUserAccount(h.db, user.ID); err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	clearSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	user, err := authenticatedUser(h.db, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var update profileUpdate
	if err := decodeJSONBody(w, r, &update); err != nil {
		return
	}

	displayName, err := validateDisplayName(update.DisplayName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updatedUser, err := updateDisplayName(h.db, user.ID, displayName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, updatedUser)
}

func (h *handler) authProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, authProviders(h.githubOAuth))
}

func (h *handler) publicProfile(w http.ResponseWriter, r *http.Request) {
	publicSlug := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "publicSlug")))
	if publicSlug == "" {
		http.Error(w, "public slug is required", http.StatusBadRequest)
		return
	}

	profile, err := publicProfileBySlug(h.db, publicSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	writeJSON(w, profile)
}

func (h *handler) githubOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !h.githubOAuth.Enabled() {
		http.Error(w, "github oauth is not configured", http.StatusServiceUnavailable)
		return
	}

	state, err := randomToken(32)
	if err != nil {
		http.Error(w, "failed to start github sign-in", http.StatusInternalServerError)
		return
	}

	setOAuthStateCookie(w, state, h.cookieSecure)
	http.Redirect(w, r, githubAuthorizeURL(h.githubOAuth, state), http.StatusFound)
}

func (h *handler) githubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !h.githubOAuth.Enabled() {
		http.Error(w, "github oauth is not configured", http.StatusServiceUnavailable)
		return
	}

	if oauthError := strings.TrimSpace(r.URL.Query().Get("error")); oauthError != "" {
		http.Redirect(w, r, "/?auth_error="+url.QueryEscape("GitHub sign-in was cancelled or denied."), http.StatusFound)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || code == "" || state == "" || cookie.Value != state {
		clearOAuthStateCookie(w, h.cookieSecure)
		http.Redirect(w, r, "/?auth_error="+url.QueryEscape("GitHub sign-in could not be verified."), http.StatusFound)
		return
	}
	clearOAuthStateCookie(w, h.cookieSecure)

	token, err := exchangeGitHubCode(h.httpClient, h.githubOAuth, code)
	if err != nil {
		http.Redirect(w, r, "/?auth_error="+url.QueryEscape("GitHub token exchange failed."), http.StatusFound)
		return
	}

	githubUser, err := fetchGitHubUser(h.httpClient, h.githubOAuth, token)
	if err != nil {
		http.Redirect(w, r, "/?auth_error="+url.QueryEscape("GitHub user lookup failed."), http.StatusFound)
		return
	}

	user, err := findOrCreateOAuthUser(h.db, "gh", fmt.Sprintf("%d", githubUser.ID), githubUser.Login)
	if err != nil {
		http.Redirect(w, r, "/?auth_error="+url.QueryEscape("GitHub account creation failed."), http.StatusFound)
		return
	}

	sessionToken, expiresAt, err := createSession(h.db, user.ID)
	if err != nil {
		http.Redirect(w, r, "/?auth_error="+url.QueryEscape("Session creation failed."), http.StatusFound)
		return
	}

	setSessionCookie(w, sessionToken, expiresAt, h.cookieSecure)
	http.Redirect(w, r, "/", http.StatusFound)
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
	if iv.Visibility == "" {
		iv.Visibility = "private"
	}
	if iv.Visibility != "private" && iv.Visibility != "public" {
		return fmt.Errorf("visibility must be private or public")
	}
	return nil
}

func validateDisplayName(displayName string) (string, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "", fmt.Errorf("display_name is required")
	}
	if len(displayName) > 80 {
		return "", fmt.Errorf("display_name must be at most 80 characters")
	}
	return displayName, nil
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

func servePublicProfileApp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	http.ServeFile(w, r, "static/index.html")
}
