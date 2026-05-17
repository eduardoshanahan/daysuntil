package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *handler) listShareGroups(w http.ResponseWriter, r *http.Request) {
	user, err := authenticatedUser(h.db, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groups, err := listShareGroups(h.db, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if groups == nil {
		groups = []ShareGroup{}
	}
	writeJSON(w, groups)
}

func (h *handler) createShareGroup(w http.ResponseWriter, r *http.Request) {
	user, err := authenticatedUser(h.db, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload shareGroupPayload
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	group, err := createShareGroup(h.db, user.ID, payload.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, group)
}

func (h *handler) updateShareGroup(w http.ResponseWriter, r *http.Request) {
	user, err := authenticatedUser(h.db, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var payload shareGroupPayload
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	group, err := updateShareGroup(h.db, user.ID, groupID, payload.Name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, group)
}

func (h *handler) deleteShareGroup(w http.ResponseWriter, r *http.Request) {
	user, err := authenticatedUser(h.db, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := deleteShareGroup(h.db, user.ID, groupID); err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) rotateShareGroup(w http.ResponseWriter, r *http.Request) {
	user, err := authenticatedUser(h.db, r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	group, err := rotateShareGroupSlug(h.db, user.ID, groupID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, group)
}
