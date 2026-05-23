package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func (h *handler) appVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, versionResponse{Version: currentVersion()})
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	var creds registerCredentials
	if err := decodeJSONBody(w, r, &creds); err != nil {
		return
	}

	user, err := createUser(h.db, creds)
	if err != nil {
		log.Printf("auth: register failed: %v", err)
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
		switch {
		case errors.Is(err, errUserNotFound):
			log.Printf("auth: login failed: no account found") // TODO: remove once login bug is resolved
		case errors.Is(err, errWrongPassword):
			log.Printf("auth: login failed: wrong password for user_id=%d", user.ID) // TODO: remove once login bug is resolved
		default:
			log.Printf("auth: login error: %v", err)
		}
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
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

func (h *handler) requestMagicLink(w http.ResponseWriter, r *http.Request) {
	if !h.magicLinks.Enabled() {
		http.Error(w, "magic link sign-in is not configured", http.StatusServiceUnavailable)
		return
	}

	var req magicLinkRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		return
	}

	email, err := validateEmail(req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := findUserByEmail(h.db, email)
	if err != nil {
		if errors.Is(err, errUserNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		log.Printf("auth: magic link lookup failed: %v", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	token, _, err := createMagicLinkToken(h.db, user.ID)
	if err != nil {
		log.Printf("auth: magic link token creation failed for user_id=%d: %v", user.ID, err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.magicLinks.sendLoginLink(email, token); err != nil {
		log.Printf("auth: magic link send failed for user_id=%d: %v", user.ID, err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) consumeMagicLink(w http.ResponseWriter, r *http.Request) {
	if !h.magicLinks.Enabled() {
		http.Error(w, "magic link sign-in is not configured", http.StatusServiceUnavailable)
		return
	}

	var req magicLinkConsumeRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		return
	}

	user, err := consumeMagicLinkToken(h.db, req.Token)
	if err != nil {
		http.Error(w, "invalid or expired sign-in link", http.StatusUnauthorized)
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
		if err == ErrNotFound {
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
	writeJSON(w, authProviders(h.githubOAuth, h.magicLinks))
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
