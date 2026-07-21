package main

import "database/sql"

func deleteUserAccount(db *sql.DB, userID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM intervals WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM share_groups WHERE user_id=$1`, userID); err != nil {
		return err
	}

	res, err := tx.Exec(`DELETE FROM users WHERE id=$1`, userID)
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
