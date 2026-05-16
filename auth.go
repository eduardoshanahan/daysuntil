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
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "daysuntil_session"
	sessionTTL        = 30 * 24 * time.Hour
)

type contextKey string

const userContextKey contextKey = "authenticated-user"

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type userCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func initAuthDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at    TEXT NOT NULL
	)`)
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

func validateCredentials(creds userCredentials) (string, error) {
	username := strings.ToLower(strings.TrimSpace(creds.Username))
	if username == "" {
		return "", fmt.Errorf("username is required")
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
	if len(creds.Password) < 8 {
		return "", fmt.Errorf("password must be at least 8 characters")
	}
	return username, nil
}

func createUser(db *sql.DB, creds userCredentials) (User, error) {
	username, err := validateCredentials(creds)
	if err != nil {
		return User{}, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
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

	res, err := tx.Exec(
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, string(passwordHash), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return User{}, fmt.Errorf("username already exists")
		}
		return User{}, err
	}

	userID, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}

	if userCount == 0 {
		_, err = tx.Exec(`UPDATE intervals SET user_id=? WHERE user_id IS NULL`, userID)
		if err != nil {
			return User{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return User{ID: userID, Username: username}, nil
}

func authenticateUser(db *sql.DB, creds userCredentials) (User, error) {
	username := strings.ToLower(strings.TrimSpace(creds.Username))
	if username == "" || creds.Password == "" {
		return User{}, fmt.Errorf("username and password are required")
	}

	var user User
	var passwordHash string
	err := db.QueryRow(
		`SELECT id, username, password_hash FROM users WHERE username=?`,
		username,
	).Scan(&user.ID, &user.Username, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, fmt.Errorf("invalid username or password")
		}
		return User{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(creds.Password)); err != nil {
		return User{}, fmt.Errorf("invalid username or password")
	}

	return user, nil
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

	err := db.QueryRow(
		`SELECT u.id, u.username, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id=?`,
		hashToken(rawToken),
	).Scan(&user.ID, &user.Username, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	expiresTime, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return User{}, err
	}
	if !expiresTime.After(time.Now().UTC()) {
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

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
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
