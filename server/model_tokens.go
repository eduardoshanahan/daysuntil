package main

import (
	"database/sql"
	"errors"
	"time"
)

// apiTokenTTL bounds a personal access token's blast radius if it leaks —
// long enough not to be annoying for genuine automation use, short enough
// to force periodic rotation. Tokens have no renewal path; a caller that
// still needs access after expiry creates a new one.
const apiTokenTTL = 365 * 24 * time.Hour

// APIToken is the client-facing view of a personal access token — never
// includes the hash, and the raw token itself is only ever returned once,
// at creation, by createAPIToken.
type APIToken struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

func listAPITokens(db *sql.DB, userID int64) ([]APIToken, error) {
	rows, err := db.Query(
		`SELECT id, name, created_at, last_used_at, expires_at FROM api_tokens WHERE user_id=$1 ORDER BY created_at, id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []APIToken
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.LastUsedAt, &t.ExpiresAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if tokens == nil {
		return []APIToken{}, nil
	}
	return tokens, nil
}

// createAPIToken returns the raw token (shown to the caller exactly once)
// alongside its metadata — reuses randomToken/hashToken from auth.go,
// the same shown-once/hash-stored pattern already used for sessions.
func createAPIToken(db *sql.DB, userID int64, name string) (string, APIToken, error) {
	rawToken, err := randomToken(32)
	if err != nil {
		return "", APIToken{}, err
	}
	tokenHash := hashToken(rawToken)

	expiresAt := time.Now().UTC().Add(apiTokenTTL)

	var t APIToken
	err = db.QueryRow(
		`INSERT INTO api_tokens (user_id, token_hash, name, expires_at) VALUES ($1, $2, $3, $4)
		RETURNING id, name, created_at, last_used_at, expires_at`,
		userID, tokenHash, name, expiresAt,
	).Scan(&t.ID, &t.Name, &t.CreatedAt, &t.LastUsedAt, &t.ExpiresAt)
	if err != nil {
		return "", APIToken{}, err
	}
	return rawToken, t, nil
}

func deleteAPIToken(db *sql.DB, userID, id int64) error {
	res, err := db.Exec(`DELETE FROM api_tokens WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// findUserByAPIToken authenticates a bearer token the same way
// findUserBySession authenticates a cookie: hash, look up, reject if
// expired. Successful lookups opportunistically bump last_used_at.
func findUserByAPIToken(db *sql.DB, rawToken string) (User, error) {
	var user User
	var tokenID int64
	var expiresAt sql.NullTime

	err := db.QueryRow(
		`SELECT t.id, u.id, u.oidc_sub, t.expires_at
		FROM api_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash=$1`,
		hashToken(rawToken),
	).Scan(&tokenID, &user.ID, &user.OIDCSub, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	if expiresAt.Valid && !expiresAt.Time.After(time.Now().UTC()) {
		return User{}, ErrNotFound
	}

	// Best-effort — a failed bump shouldn't fail the request that's
	// already been authenticated.
	_, _ = db.Exec(`UPDATE api_tokens SET last_used_at=now() WHERE id=$1`, tokenID)

	return user, nil
}
