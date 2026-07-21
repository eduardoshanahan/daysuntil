package main

import (
	"errors"
	"log"
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

	raw, err := publicShareGroupBySlug(h.db, groupSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	owner, err := h.profileClient.GetBySub(r.Context(), raw.OwnerSub)
	if err != nil {
		log.Printf("public share group: resolve owner profile failed: %v", err)
		http.Error(w, "failed to load share group", http.StatusInternalServerError)
		return
	}

	profile := PublicShareGroup{
		Name:          raw.Name,
		PublicSlug:    raw.PublicSlug,
		OwnerName:     owner.DisplayName,
		OwnerUsername: owner.Username,
		Intervals:     raw.Intervals,
	}

	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	writeJSON(w, profile)
}
