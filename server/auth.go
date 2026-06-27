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

type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	PublicSlug  string `json:"public_slug"`
	UsernameSet bool   `json:"username_set"`
}

func initAuthDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		oidc_sub     TEXT NOT NULL DEFAULT '',
		username     TEXT NOT NULL UNIQUE,
		public_slug  TEXT NOT NULL DEFAULT '',
		display_name TEXT NOT NULL DEFAULT '',
		created_at   TEXT NOT NULL,
		username_set INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return err
	}

	if err := renameUserColumnIfExists(db, "zitadel_sub", "oidc_sub"); err != nil {
		return err
	}
	if err := ensureUserColumn(db, "oidc_sub", "ALTER TABLE users ADD COLUMN oidc_sub TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureUserColumn(db, "display_name", "ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureUserColumn(db, "public_slug", "ALTER TABLE users ADD COLUMN public_slug TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureUserColumn(db, "username_set", "ALTER TABLE users ADD COLUMN username_set INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	if err := dropUserColumnIfExists(db, "password_hash"); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS idx_users_email`); err != nil {
		return err
	}
	if err := dropUserColumnIfExists(db, "email"); err != nil {
		return err
	}

	_, err = db.Exec(`DROP INDEX IF EXISTS idx_users_zitadel_sub`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_oidc_sub ON users(oidc_sub) WHERE oidc_sub <> ''`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_public_slug ON users(public_slug) WHERE public_slug <> ''`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		user_id    INTEGER NOT NULL,
		expires_at TEXT NOT NULL,
		created_at TEXT NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
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

func ensureUserColumn(db *sql.DB, column, statement string) error {
	exists, err := tableColumnExists(db, "users", column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = db.Exec(statement)
	return err
}

func dropUserColumnIfExists(db *sql.DB, column string) error {
	exists, err := tableColumnExists(db, "users", column)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	_, err = db.Exec("ALTER TABLE users DROP COLUMN " + column)
	return err
}

func renameUserColumnIfExists(db *sql.DB, oldColumn, newColumn string) error {
	exists, err := tableColumnExists(db, "users", oldColumn)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	_, err = db.Exec("ALTER TABLE users RENAME COLUMN " + oldColumn + " TO " + newColumn)
	return err
}

func validateUsername(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return "", fmt.Errorf("username is required")
	}
	if strings.HasPrefix(username, "pending-") {
		return "", fmt.Errorf("username is not available")
	}
	if len(username) < 3 {
		return "", fmt.Errorf("username must be at least 3 characters")
	}
	if len(username) > 64 {
		return "", fmt.Errorf("username must be at most 64 characters")
	}
	for _, r := range username {
		isValid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if !isValid {
			return "", fmt.Errorf("username may only contain letters, numbers, dots, dashes, and underscores")
		}
	}
	return username, nil
}

func randomPlaceholderUsername() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("pending-%x", buf), nil
}

func findOrCreateOIDCUser(db *sql.DB, sub, displayName string) (User, error) {
	if sub == "" {
		return User{}, fmt.Errorf("oidc sub is required")
	}

	var user User
	var usernameSet int
	err := db.QueryRow(
		`SELECT id, username, public_slug, display_name, username_set FROM users WHERE oidc_sub=?`,
		sub,
	).Scan(&user.ID, &user.Username, &user.PublicSlug, &user.DisplayName, &usernameSet)
	if err == nil {
		user.UsernameSet = usernameSet == 1
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}

	placeholderUsername, err := randomPlaceholderUsername()
	if err != nil {
		return User{}, err
	}

	if strings.TrimSpace(displayName) == "" {
		displayName = "user"
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

	res, err := tx.Exec(
		`INSERT INTO users (oidc_sub, username, display_name, created_at, username_set) VALUES (?, ?, ?, ?, 0)`,
		sub, placeholderUsername, displayName, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return findOrCreateOIDCUser(db, sub, displayName)
		}
		return User{}, err
	}

	userID, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}

	if userCount == 0 {
		if _, err := tx.Exec(`UPDATE intervals SET user_id=? WHERE user_id IS NULL`, userID); err != nil {
			return User{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	publicSlug, err := ensurePublicSlug(db, userID, "")
	if err != nil {
		return User{}, err
	}

	return User{ID: userID, Username: placeholderUsername, DisplayName: displayName, PublicSlug: publicSlug, UsernameSet: false}, nil
}

func createSession(db *sql.DB, userID int64) (string, time.Time, error) {
	rawToken, err := randomToken(32)
	if err != nil {
		return "", time.Time{}, err
	}

	sessionID := hashToken(rawToken)
	expiresAt := time.Now().UTC().Add(sessionTTL)
	_, err = db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		sessionID, userID, expiresAt.Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339),
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
	_, err := db.Exec(`DELETE FROM sessions WHERE id=?`, hashToken(rawToken))
	return err
}

func findUserBySession(db *sql.DB, rawToken string) (User, error) {
	var user User
	var expiresAt string
	var usernameSet int

	err := db.QueryRow(
		`SELECT u.id, u.username, u.public_slug, u.display_name, u.username_set, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id=?`,
		hashToken(rawToken),
	).Scan(&user.ID, &user.Username, &user.PublicSlug, &user.DisplayName, &usernameSet, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	user.UsernameSet = usernameSet == 1

	expiresTime, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return User{}, err
	}
	if !expiresTime.After(time.Now().UTC()) {
		_ = deleteSession(db, rawToken)
		return User{}, ErrNotFound
	}

	user.PublicSlug, err = ensurePublicSlug(db, user.ID, user.PublicSlug)
	if err != nil {
		return User{}, err
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
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return User{}, ErrNotFound
	}
	return findUserBySession(db, cookie.Value)
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
