package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *handler) listReminders(w http.ResponseWriter, r *http.Request) {
	user, err := userFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	intervalID, err := strconv.ParseInt(chi.URLParam(r, "intervalID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid interval id", http.StatusBadRequest)
		return
	}

	reminders, err := listRemindersForInterval(h.db, user.ID, intervalID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, reminders)
}

func (h *handler) createReminder(w http.ResponseWriter, r *http.Request) {
	user, err := userFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	intervalID, err := strconv.ParseInt(chi.URLParam(r, "intervalID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid interval id", http.StatusBadRequest)
		return
	}

	var input reminderInput
	if err := decodeJSONBody(w, r, &input); err != nil {
		return
	}
	if err := validateReminder(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	created, err := createReminder(h.db, user.ID, intervalID, input)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, created)
}

func (h *handler) updateReminder(w http.ResponseWriter, r *http.Request) {
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

	var input reminderInput
	if err := decodeJSONBody(w, r, &input); err != nil {
		return
	}
	if err := validateReminder(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated, err := updateReminder(h.db, user.ID, id, input)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, updated)
}

func (h *handler) deleteReminder(w http.ResponseWriter, r *http.Request) {
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

	if err := deleteReminder(h.db, user.ID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
