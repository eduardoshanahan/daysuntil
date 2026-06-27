package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func updateDisplayName(db *sql.DB, userID int64, displayName string) (User, error) {
	_, err := db.Exec(`UPDATE users SET display_name=? WHERE id=?`, displayName, userID)
	if err != nil {
		return User{}, err
	}

	var user User
	var usernameSet int
	err = db.QueryRow(`SELECT id, username, public_slug, display_name, username_set FROM users WHERE id=?`, userID).Scan(&user.ID, &user.Username, &user.PublicSlug, &user.DisplayName, &usernameSet)
	if err != nil {
		return User{}, err
	}
	user.UsernameSet = usernameSet == 1
	return user, nil
}

func setUserUsername(db *sql.DB, userID int64, username string) (User, error) {
	username, err := validateUsername(username)
	if err != nil {
		return User{}, err
	}

	var currentUsernameSet int
	err = db.QueryRow(`SELECT username_set FROM users WHERE id=?`, userID).Scan(&currentUsernameSet)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	if currentUsernameSet == 1 {
		return User{}, fmt.Errorf("username has already been set and cannot be changed")
	}

	_, err = db.Exec(`UPDATE users SET username=?, username_set=1 WHERE id=?`, username, userID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return User{}, fmt.Errorf("username is already taken")
		}
		return User{}, err
	}

	var user User
	var usernameSet int
	err = db.QueryRow(`SELECT id, username, public_slug, display_name, username_set FROM users WHERE id=?`, userID).Scan(&user.ID, &user.Username, &user.PublicSlug, &user.DisplayName, &usernameSet)
	if err != nil {
		return User{}, err
	}
	user.UsernameSet = usernameSet == 1
	return user, nil
}

func deleteUserAccount(db *sql.DB, userID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id=?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM intervals WHERE user_id=?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM share_groups WHERE user_id=?`, userID); err != nil {
		return err
	}

	res, err := tx.Exec(`DELETE FROM users WHERE id=?`, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}

	return tx.Commit()
}
