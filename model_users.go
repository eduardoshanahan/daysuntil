package main

import "database/sql"

func updateDisplayName(db *sql.DB, userID int64, displayName string) (User, error) {
	_, err := db.Exec(`UPDATE users SET display_name=? WHERE id=?`, displayName, userID)
	if err != nil {
		return User{}, err
	}

	var user User
	err = db.QueryRow(`SELECT id, username, public_slug, display_name FROM users WHERE id=?`, userID).Scan(&user.ID, &user.Username, &user.PublicSlug, &user.DisplayName)
	if err != nil {
		return User{}, err
	}
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
