package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (h *handler) publicShareGroup(w http.ResponseWriter, r *http.Request) {
	groupSlug := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "groupSlug")))
	if groupSlug == "" {
		http.Error(w, "group slug is required", http.StatusBadRequest)
		return
	}

	profile, err := publicShareGroupBySlug(h.db, groupSlug)
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

