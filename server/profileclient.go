package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrProfileNotFound     = errors.New("profile not found")
	ErrProfileUsernameUsed = errors.New("username already taken")
)

// ErrProfileInvalidRequest wraps a 400 response from profile-service,
// preserving its message so handlers can relay a specific validation
// error instead of a generic failure.
type ErrProfileInvalidRequest struct {
	Message string
}

func (e *ErrProfileInvalidRequest) Error() string { return e.Message }

// Profile mirrors profile-service's response shape.
type Profile struct {
	ID            int64     `json:"id"`
	OIDCSub       string    `json:"oidc_sub"`
	Username      string    `json:"username"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	DisplayName   string    `json:"display_name"`
	AvatarURL     string    `json:"avatar_url"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	UsernameSet   bool      `json:"username_set"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ProfilePatch mirrors profile-service's partial-update request body.
type ProfilePatch struct {
	Username    *string `json:"username,omitempty"`
	FirstName   *string `json:"first_name,omitempty"`
	LastName    *string `json:"last_name,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}

// PublicProfile is the always-public projection profile-service's
// read:public operation returns for an arbitrary sub — never email or the
// personal-name fields. Used for resolving another user's identity for
// display (e.g. a public share group's owner), as opposed to GetBySub's
// full profile, which is only ever called with the caller's own
// authenticated sub.
type PublicProfile struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

// ProfileClient is the boundary daysuntil uses to reach profile-service.
// Handlers depend on this interface (not the HTTP implementation directly)
// so tests can substitute an in-memory fake without a live profile-service.
type ProfileClient interface {
	FindOrCreate(ctx context.Context, sub, displayNameHint, email string, emailVerified bool) (Profile, error)
	GetBySub(ctx context.Context, sub string) (Profile, error)
	GetByUsername(ctx context.Context, username string) (Profile, error)
	GetPublicBySub(ctx context.Context, sub string) (PublicProfile, error)
	Update(ctx context.Context, sub string, patch ProfilePatch) (Profile, error)
}

type httpProfileClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newHTTPProfileClient(baseURL, token string, httpClient *http.Client) *httpProfileClient {
	return &httpProfileClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: httpClient,
	}
}

func (c *httpProfileClient) FindOrCreate(ctx context.Context, sub, displayNameHint, email string, emailVerified bool) (Profile, error) {
	body, err := json.Marshal(struct {
		Sub             string `json:"sub"`
		DisplayNameHint string `json:"display_name_hint"`
		Email           string `json:"email"`
		EmailVerified   bool   `json:"email_verified"`
	}{Sub: sub, DisplayNameHint: displayNameHint, Email: email, EmailVerified: emailVerified})
	if err != nil {
		return Profile{}, err
	}
	return c.do(ctx, http.MethodPost, "/internal/profiles/find-or-create", body)
}

func (c *httpProfileClient) GetBySub(ctx context.Context, sub string) (Profile, error) {
	return c.do(ctx, http.MethodGet, "/internal/profiles/by-sub/"+url.PathEscape(sub), nil)
}

func (c *httpProfileClient) GetByUsername(ctx context.Context, username string) (Profile, error) {
	return c.do(ctx, http.MethodGet, "/internal/profiles/by-username/"+url.PathEscape(username), nil)
}

func (c *httpProfileClient) GetPublicBySub(ctx context.Context, sub string) (PublicProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/profiles/public/by-sub/"+url.PathEscape(sub), nil)
	if err != nil {
		return PublicProfile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PublicProfile{}, fmt.Errorf("profile-service request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var p PublicProfile
		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			return PublicProfile{}, fmt.Errorf("decode profile-service response: %w", err)
		}
		return p, nil
	case http.StatusNotFound:
		return PublicProfile{}, ErrProfileNotFound
	default:
		return PublicProfile{}, fmt.Errorf("profile-service returned unexpected status %d", resp.StatusCode)
	}
}

func (c *httpProfileClient) Update(ctx context.Context, sub string, patch ProfilePatch) (Profile, error) {
	body, err := json.Marshal(patch)
	if err != nil {
		return Profile{}, err
	}
	return c.do(ctx, http.MethodPatch, "/internal/profiles/by-sub/"+url.PathEscape(sub), body)
}

func (c *httpProfileClient) do(ctx context.Context, method, path string, body []byte) (Profile, error) {
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return Profile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Profile{}, fmt.Errorf("profile-service request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var p Profile
		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			return Profile{}, fmt.Errorf("decode profile-service response: %w", err)
		}
		return p, nil
	case http.StatusNotFound:
		return Profile{}, ErrProfileNotFound
	case http.StatusConflict:
		return Profile{}, ErrProfileUsernameUsed
	case http.StatusBadRequest:
		body, readErr := io.ReadAll(resp.Body)
		if readErr == nil && strings.TrimSpace(string(body)) != "" {
			return Profile{}, &ErrProfileInvalidRequest{Message: strings.TrimSpace(string(body))}
		}
		return Profile{}, &ErrProfileInvalidRequest{Message: "profile-service rejected the request"}
	default:
		return Profile{}, fmt.Errorf("profile-service returned unexpected status %d", resp.StatusCode)
	}
}
