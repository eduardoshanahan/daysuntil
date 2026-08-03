package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	sessionCookieName = "daysuntil_session"
	oidcStateCookie   = "daysuntil_oidc_state"
	oidcPKCECookie    = "daysuntil_oidc_pkce"
	sessionTTL        = 30 * 24 * time.Hour
)

type contextKey string

const userContextKey contextKey = "authenticated-user"

// User is daysuntil's local identity anchor — just enough to key
// sessions, intervals, and share_groups by. Profile data (username,
// display name, etc.) lives in profile-service and is fetched separately
// via ProfileClient, keyed by OIDCSub.
type User struct {
	ID      int64
	OIDCSub string
}

func initAuthDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id         BIGSERIAL PRIMARY KEY,
		oidc_sub   TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_oidc_sub ON users(oidc_sub) WHERE oidc_sub <> ''`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at TIMESTAMPTZ NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`)
	if err != nil {
		return err
	}

	return nil
}

// findOrCreateLocalUser looks up (or creates) daysuntil's local identity
// anchor row for an OIDC sub. It does not touch profile data — callers
// (the OIDC callback handler) are responsible for also calling
// ProfileClient.FindOrCreate so a profile-service profile exists.
func findOrCreateLocalUser(db *sql.DB, sub string) (User, error) {
	if sub == "" {
		return User{}, fmt.Errorf("oidc sub is required")
	}

	var user User
	err := db.QueryRow(`SELECT id, oidc_sub FROM users WHERE oidc_sub=$1`, sub).Scan(&user.ID, &user.OIDCSub)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}

	tx, err := db.Begin()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	var userCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		return User{}, err
	}

	var userID int64
	err = tx.QueryRow(`INSERT INTO users (oidc_sub) VALUES ($1) RETURNING id`, sub).Scan(&userID)
	if err != nil {
		return User{}, err
	}

	if userCount == 0 {
		if _, err := tx.Exec(`UPDATE intervals SET user_id=$1 WHERE user_id IS NULL`, userID); err != nil {
			return User{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return User{ID: userID, OIDCSub: sub}, nil
}

func createSession(db *sql.DB, userID int64) (string, time.Time, error) {
	rawToken, err := randomToken(32)
	if err != nil {
		return "", time.Time{}, err
	}

	sessionID := hashToken(rawToken)
	expiresAt := time.Now().UTC().Add(sessionTTL)
	_, err = db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		sessionID, userID, expiresAt,
	)
	if err != nil {
		return "", time.Time{}, err
	}

	return rawToken, expiresAt, nil
}

func deleteSession(db *sql.DB, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	_, err := db.Exec(`DELETE FROM sessions WHERE id=$1`, hashToken(rawToken))
	return err
}

func findUserBySession(db *sql.DB, rawToken string) (User, error) {
	var user User
	var expiresAt time.Time

	err := db.QueryRow(
		`SELECT u.id, u.oidc_sub, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id=$1`,
		hashToken(rawToken),
	).Scan(&user.ID, &user.OIDCSub, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	if !expiresAt.After(time.Now().UTC()) {
		_ = deleteSession(db, rawToken)
		return User{}, ErrNotFound
	}

	return user, nil
}

func authMiddleware(h *handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := authenticatedUser(h.db, r)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func authenticatedUser(db *sql.DB, r *http.Request) (User, error) {
	if raw := bearerToken(r); raw != "" {
		return findUserByAPIToken(db, raw)
	}

	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return User{}, ErrNotFound
	}
	return findUserBySession(db, cookie.Value)
}

// bearerToken extracts a token from "Authorization: Bearer <token>" so a
// native client (mobile app, script) can authenticate without a browser
// cookie jar. Checked before the session cookie in authenticatedUser, so
// both paths populate the identical userContextKey downstream.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func userFromContext(ctx context.Context) (User, error) {
	user, ok := ctx.Value(userContextKey).(User)
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure, crossOrigin bool) {
	sameSite := http.SameSiteLaxMode
	if crossOrigin {
		sameSite = http.SameSiteNoneMode
		secure = true
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: sameSite,
		Secure:   secure,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, secure, crossOrigin bool) {
	sameSite := http.SameSiteLaxMode
	if crossOrigin {
		sameSite = http.SameSiteNoneMode
		secure = true
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: sameSite,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func setOIDCStateCookie(w http.ResponseWriter, state string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   600,
		Expires:  time.Now().Add(10 * time.Minute),
	})
}

func clearOIDCStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func setOIDCPKCECookie(w http.ResponseWriter, verifier string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcPKCECookie,
		Value:    verifier,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   600,
		Expires:  time.Now().Add(10 * time.Minute),
	})
}

func clearOIDCPKCECookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcPKCECookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func cookieSecureFromEnv() (bool, error) {
	configuredValue := strings.TrimSpace(os.Getenv("COOKIE_SECURE"))
	httpsDeployment := urlUsesHTTPS(os.Getenv("BASE_URL"))

	switch configuredValue {
	case "":
		return httpsDeployment, nil
	case "true":
		return true, nil
	case "false":
		if httpsDeployment {
			return false, fmt.Errorf("COOKIE_SECURE=false is not allowed when BASE_URL uses https")
		}
		return false, nil
	default:
		return false, fmt.Errorf("COOKIE_SECURE must be true, false, or unset")
	}
}

func urlUsesHTTPS(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return false
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "https")
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
