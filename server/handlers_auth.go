package main

import (
	"errors"
	"net/http"
)

func (h *handler) appVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, versionResponse{Version: currentVersion()})
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		_ = deleteSession(h.db, cookie.Value)
	}

	clearSessionCookie(w, h.cookieSecure, h.crossOrigin())
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

	clearSessionCookie(w, h.cookieSecure, h.crossOrigin())
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

func (h *handler) setUsername(w http.ResponseWriter, r *http.Request) {
	user, err := authenticatedUser(h.db, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := decodeJSONBody(w, r, &req); err != nil {
		return
	}

	updatedUser, err := setUserUsername(h.db, user.ID, req.Username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, updatedUser)
}
