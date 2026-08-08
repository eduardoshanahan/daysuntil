package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

type profileUpdate struct {
	DisplayName string `json:"display_name"`
}

type shareGroupPayload struct {
	Name string `json:"name"`
}

type moveIntervalPayload struct {
	Direction string `json:"direction"`
}

type reminderInput struct {
	RemindAt   string `json:"remind_at"`
	RepeatRule string `json:"repeat_rule"`
	Message    string `json:"message"`
}

type tokenInput struct {
	Name string `json:"name"`
}

type intervalInput struct {
	Name               string  `json:"name"`
	StartAt            string  `json:"start_at"`
	EndAt              string  `json:"end_at"`
	Timezone           string  `json:"timezone"`
	AllDay             bool    `json:"all_day"`
	Color              string  `json:"color"`
	Icon               string  `json:"icon"`
	BackgroundImageURL string  `json:"background_image_url"`
	RecurrenceRule     string  `json:"recurrence_rule"`
	DisplayUnit        string  `json:"display_unit"`
	ShareGroupIDs      []int64 `json:"share_group_ids"`
}

type versionResponse struct {
	Version string `json:"version"`
}

type handler struct {
	db            *sql.DB
	cookieSecure  bool
	webOrigin     string
	oidc          oidcConfig
	oidcRT        *oidcRuntime
	httpClient    *http.Client
	authLimiter   *authRateLimiter
	profileClient ProfileClient
}

func (h *handler) homeURL() string {
	if h.webOrigin != "" {
		return h.webOrigin + "/"
	}
	return "/"
}

func (h *handler) crossOrigin() bool { return h.webOrigin != "" }

// expectedOrigin is the Origin header value a legitimate, cookie-authenticated
// unsafe request must carry. In the split web/API deployment (WEB_ORIGIN
// set), that's the configured web frontend's origin, since the API's own
// host is never where requests originate. Otherwise the API is same-origin
// with its own frontend, so the request's own Host (with the scheme implied
// by cookieSecure) is the expected origin.
func (h *handler) expectedOrigin(r *http.Request) string {
	if h.webOrigin != "" {
		return h.webOrigin
	}
	scheme := "http"
	if h.cookieSecure {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return errors.New("unexpected content type")
	}

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
