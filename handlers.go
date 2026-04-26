package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type handler struct {
	db *sql.DB
}

func (h *handler) listIntervals(w http.ResponseWriter, r *http.Request) {
	intervals, err := listIntervals(h.db)
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
	var iv Interval
	if err := json.NewDecoder(r.Body).Decode(&iv); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := validateInterval(iv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := createInterval(h.db, iv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, created)
}

func (h *handler) updateInterval(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var iv Interval
	if err := json.NewDecoder(r.Body).Decode(&iv); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := validateInterval(iv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := updateInterval(h.db, id, iv); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	iv.ID = id
	writeJSON(w, iv)
}

func (h *handler) deleteInterval(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := deleteInterval(h.db, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateInterval(iv Interval) error {
	if iv.Name == "" {
		return fmt.Errorf("name is required")
	}
	if _, err := parseDate(iv.StartDate); err != nil {
		return fmt.Errorf("invalid start_date: %w", err)
	}
	if _, err := parseDate(iv.EndDate); err != nil {
		return fmt.Errorf("invalid end_date: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
