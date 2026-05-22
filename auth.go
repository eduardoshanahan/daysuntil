package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "daysuntil_session"
	oauthStateCookie  = "daysuntil_oauth_state"
	sessionTTL        = 30 * 24 * time.Hour
)

var errUserNotFound = errors.New("user not found")
var errWrongPassword = errors.New("wrong password")

type contextKey string

const userContextKey contextKey = "authenticated-user"

type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	PublicSlug  string `json:"public_slug"`
}

type registerCredentials struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type githubOAuthConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
	AuthorizeURL string
	TokenURL     string
	UserURL      string
}

type authProvidersResponse struct {
	GitHubEnabled bool `json:"github_enabled"`
}

type githubOAuthUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

func initAuthDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id                    INTEGER PRIMARY KEY AUTOINCREMENT,
		email                 TEXT NOT NULL DEFAULT '',
		username              TEXT NOT NULL UNIQUE,
		public_slug           TEXT NOT NULL DEFAULT '',
		display_name          TEXT NOT NULL DEFAULT '',
		password_hash         TEXT NOT NULL,
		auth_provider         TEXT NOT NULL DEFAULT '',
		auth_provider_user_id TEXT NOT NULL DEFAULT '',
		created_at            TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}

	if err := ensureUserColumn(db, "display_name", "ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureUserColumn(db, "email", "ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureUserColumn(db, "public_slug", "ALTER TABLE users ADD COLUMN public_slug TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureUserColumn(db, "auth_provider", "ALTER TABLE users ADD COLUMN auth_provider TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureUserColumn(db, "auth_provider_user_id", "ALTER TABLE users ADD COLUMN auth_provider_user_id TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider_identity ON users(auth_provider, auth_provider_user_id) WHERE auth_provider <> '' AND auth_provider_user_id <> ''`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email <> ''`)
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmail(email string) (string, error) {
	email = normalizeEmail(email)
	if email == "" {
		return "", fmt.Errorf("email is required")
	}
	if len(email) > 254 {
		return "", fmt.Errorf("email must be at most 254 characters")
	}

	addr, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(addr.Address, email) {
		return "", fmt.Errorf("email must be a valid address")
	}
	parts := strings.SplitN(addr.Address, "@", 2)
	if len(parts) != 2 || !strings.Contains(parts[1], ".") {
		return "", fmt.Errorf("email must be a valid address")
	}
	return email, nil
}

func validateUsername(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
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
	return username, nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

func validateRegistration(creds registerCredentials) (string, string, error) {
	email, err := validateEmail(creds.Email)
	if err != nil {
		return "", "", err
	}
	username, err := validateUsername(creds.Username)
	if err != nil {
		return "", "", err
	}
	if err := validatePassword(creds.Password); err != nil {
		return "", "", err
	}
	return email, username, nil
}

func createUser(db *sql.DB, creds registerCredentials) (User, error) {
	email, username, err := validateRegistration(creds)
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
		`INSERT INTO users (email, username, public_slug, display_name, password_hash, auth_provider, auth_provider_user_id, created_at) VALUES (?, ?, '', ?, ?, '', '', ?)`,
		email, username, username, string(passwordHash), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return User{}, fmt.Errorf("account could not be created with those details")
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

	publicSlug, err := ensurePublicSlug(db, userID, "")
	if err != nil {
		return User{}, err
	}

	return User{ID: userID, Username: username, DisplayName: username, PublicSlug: publicSlug}, nil
}

func authenticateUser(db *sql.DB, creds loginCredentials) (User, error) {
	email := normalizeEmail(creds.Email)
	if email == "" || creds.Password == "" {
		return User{}, fmt.Errorf("email and password are required")
	}

	var user User
	var passwordHash string
	err := db.QueryRow(
		`SELECT id, username, public_slug, display_name, password_hash FROM users WHERE email=?`,
		email,
	).Scan(&user.ID, &user.Username, &user.PublicSlug, &user.DisplayName, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, errUserNotFound
		}
		return User{}, err
	}

	if passwordHash == "" {
		return user, errUserNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(creds.Password)); err != nil {
		return user, errWrongPassword
	}

	user.PublicSlug, err = ensurePublicSlug(db, user.ID, user.PublicSlug)
	if err != nil {
		return User{}, err
	}

	return user, nil
}

func authProviders(config githubOAuthConfig) authProvidersResponse {
	return authProvidersResponse{GitHubEnabled: config.Enabled()}
}

func githubConfigFromEnv() githubOAuthConfig {
	callbackURL := strings.TrimSpace(os.Getenv("GITHUB_CALLBACK_URL"))
	if callbackURL == "" {
		baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BASE_URL")), "/")
		if baseURL != "" {
			callbackURL = baseURL + "/api/oauth/github/callback"
		}
	}

	return githubOAuthConfig{
		ClientID:     strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")),
		CallbackURL:  callbackURL,
		AuthorizeURL: "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserURL:      "https://api.github.com/user",
	}
}

func (c githubOAuthConfig) Enabled() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.CallbackURL != ""
}

func cookieSecureFromEnv(config githubOAuthConfig) (bool, error) {
	configuredValue := strings.TrimSpace(os.Getenv("COOKIE_SECURE"))
	httpsDeployment := urlUsesHTTPS(os.Getenv("BASE_URL")) || urlUsesHTTPS(config.CallbackURL)

	switch configuredValue {
	case "":
		return httpsDeployment, nil
	case "true":
		return true, nil
	case "false":
		if httpsDeployment {
			return false, fmt.Errorf("COOKIE_SECURE=false is not allowed when BASE_URL or GITHUB_CALLBACK_URL uses https")
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

func findUserByProvider(db *sql.DB, provider, providerUserID string) (User, error) {
	var user User
	err := db.QueryRow(
		`SELECT id, username, public_slug, display_name FROM users WHERE auth_provider=? AND auth_provider_user_id=?`,
		provider, providerUserID,
	).Scan(&user.ID, &user.Username, &user.PublicSlug, &user.DisplayName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return user, nil
}

func findOrCreateOAuthUser(db *sql.DB, provider, providerUserID, providerLogin string) (User, error) {
	user, err := findUserByProvider(db, provider, providerUserID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}

	baseUsername := providerUsername(provider, providerLogin, providerUserID)
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	var userCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		return User{}, err
	}

	username := baseUsername
	for attempt := 2; ; attempt += 1 {
		res, err := tx.Exec(
			`INSERT INTO users (email, username, public_slug, display_name, password_hash, auth_provider, auth_provider_user_id, created_at) VALUES ('', ?, '', ?, '', ?, ?, ?)`,
			username, providerLogin, provider, providerUserID, now,
		)
		if err == nil {
			userID, lastErr := res.LastInsertId()
			if lastErr != nil {
				return User{}, lastErr
			}

			if userCount == 0 {
				_, lastErr = tx.Exec(`UPDATE intervals SET user_id=? WHERE user_id IS NULL`, userID)
				if lastErr != nil {
					return User{}, lastErr
				}
			}

			if lastErr = tx.Commit(); lastErr != nil {
				return User{}, lastErr
			}

			displayName := providerLogin
			if strings.TrimSpace(displayName) == "" {
				displayName = username
			}
			publicSlug, slugErr := ensurePublicSlug(db, userID, "")
			if slugErr != nil {
				return User{}, slugErr
			}
			return User{ID: userID, Username: username, DisplayName: displayName, PublicSlug: publicSlug}, nil
		}

		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "auth_provider") && strings.Contains(errText, "auth_provider_user_id") {
			return findUserByProvider(db, provider, providerUserID)
		}
		if !strings.Contains(errText, "unique") {
			return User{}, err
		}

		username = fmt.Sprintf("%s-%d", truncateUsername(baseUsername, 60), attempt)
	}
}

func providerUsername(provider, login, providerUserID string) string {
	normalized := normalizeExternalUsername(login)
	if normalized == "" {
		normalized = providerUserID
	}
	username := fmt.Sprintf("%s_%s", provider, normalized)
	if len(username) > 64 {
		return username[:64]
	}
	return username
}

func normalizeExternalUsername(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false

	for _, r := range value {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if valid {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

func truncateUsername(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
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
		`SELECT u.id, u.username, u.public_slug, u.display_name, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id=?`,
		hashToken(rawToken),
	).Scan(&user.ID, &user.Username, &user.PublicSlug, &user.DisplayName, &expiresAt)
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

func setOAuthStateCookie(w http.ResponseWriter, state string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   600,
		Expires:  time.Now().Add(10 * time.Minute),
	})
}

func clearOAuthStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func githubAuthorizeURL(config githubOAuthConfig, state string) string {
	values := url.Values{}
	values.Set("client_id", config.ClientID)
	values.Set("redirect_uri", config.CallbackURL)
	values.Set("scope", "read:user")
	values.Set("state", state)
	values.Set("allow_signup", "true")
	return config.AuthorizeURL + "?" + values.Encode()
}

func exchangeGitHubCode(client *http.Client, config githubOAuthConfig, code string) (string, error) {
	values := url.Values{}
	values.Set("client_id", config.ClientID)
	values.Set("client_secret", config.ClientSecret)
	values.Set("code", code)
	values.Set("redirect_uri", config.CallbackURL)

	req, err := http.NewRequest(http.MethodPost, config.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp githubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK || tokenResp.AccessToken == "" {
		if tokenResp.Error != "" {
			return "", fmt.Errorf("github token exchange failed: %s", tokenResp.Error)
		}
		return "", fmt.Errorf("github token exchange failed")
	}

	return tokenResp.AccessToken, nil
}

func fetchGitHubUser(client *http.Client, config githubOAuthConfig, token string) (githubOAuthUser, error) {
	req, err := http.NewRequest(http.MethodGet, config.UserURL, nil)
	if err != nil {
		return githubOAuthUser{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return githubOAuthUser{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return githubOAuthUser{}, fmt.Errorf("github user request failed: %s", strings.TrimSpace(string(body)))
	}

	var user githubOAuthUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return githubOAuthUser{}, err
	}
	if user.ID == 0 || user.Login == "" {
		return githubOAuthUser{}, fmt.Errorf("github user response was incomplete")
	}
	return user, nil
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
