package main

import (
	"errors"
	"net/http"
)

// meResponse merges daysuntil's local identity anchor with profile-service
// data. The JSON shape matches what the frontend already expects
// (username/display_name/username_set) plus the new profile fields.
type meResponse struct {
	ID                int64  `json:"id"`
	Username          string `json:"username"`
	DisplayName       string `json:"display_name"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	AvatarURL         string `json:"avatar_url"`
	UsernameSet       bool   `json:"username_set"`
	SuggestedUsername string `json:"suggested_username,omitempty"`
}

func meResponseFromProfile(user User, profile Profile) meResponse {
	return meResponse{
		ID:                user.ID,
		Username:          profile.Username,
		DisplayName:       profile.DisplayName,
		FirstName:         profile.FirstName,
		LastName:          profile.LastName,
		AvatarURL:         profile.AvatarURL,
		UsernameSet:       profile.UsernameSet,
		SuggestedUsername: profile.SuggestedUsername,
	}
}

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

	profile, err := h.profileClient.GetBySub(r.Context(), user.OIDCSub)
	if err != nil {
		writeProfileClientError(w, err)
		return
	}

	writeJSON(w, meResponseFromProfile(user, profile))
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

func writeProfileClientError(w http.ResponseWriter, err error) {
	var invalid *ErrProfileInvalidRequest
	switch {
	case errors.Is(err, ErrProfileNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, ErrProfileUsernameUsed):
		http.Error(w, "username is already taken", http.StatusConflict)
	case errors.As(err, &invalid):
		http.Error(w, invalid.Message, http.StatusBadRequest)
	default:
		http.Error(w, "profile service unavailable", http.StatusBadGateway)
	}
}
