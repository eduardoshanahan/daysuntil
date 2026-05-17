package main

import (
	"database/sql"
	"errors"
	"time"
)

type Interval struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	Color      string `json:"color"`
	Visibility string `json:"visibility"`
}

var ErrNotFound = errors.New("interval not found")

type PublicProfile struct {
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	Intervals   []Interval `json:"intervals"`
}

func initDB(db *sql.DB) error {
	if err := initAuthDB(db); err != nil {
		return err
	}

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS intervals (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    INTEGER,
		name       TEXT NOT NULL,
		start_date TEXT NOT NULL,
		end_date   TEXT NOT NULL,
		color      TEXT NOT NULL DEFAULT '#4f8ef7',
		visibility TEXT NOT NULL DEFAULT 'private',
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return err
	}

	hasColorColumn, err := intervalColumnExists(db, "color")
	if err != nil {
		return err
	}
	if !hasColorColumn {
		_, err = db.Exec(`ALTER TABLE intervals ADD COLUMN color TEXT NOT NULL DEFAULT '#4f8ef7'`)
		if err != nil {
			return err
		}
	}

	hasUserColumn, err := intervalColumnExists(db, "user_id")
	if err != nil {
		return err
	}
	if !hasUserColumn {
		_, err = db.Exec(`ALTER TABLE intervals ADD COLUMN user_id INTEGER`)
		if err != nil {
			return err
		}
	}
	hasVisibilityColumn, err := intervalColumnExists(db, "visibility")
	if err != nil {
		return err
	}
	if !hasVisibilityColumn {
		_, err = db.Exec(`ALTER TABLE intervals ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private'`)
		if err != nil {
			return err
		}
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_intervals_user_id_start_date ON intervals(user_id, start_date)`)
	if err != nil {
		return err
	}

	return nil
}

func listIntervals(db *sql.DB, userID int64) ([]Interval, error) {
	rows, err := db.Query(
		`SELECT id, name, start_date, end_date, color, visibility
		FROM intervals
		WHERE user_id=?
		ORDER BY start_date`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var intervals []Interval
	for rows.Next() {
		var iv Interval
		if err := rows.Scan(&iv.ID, &iv.Name, &iv.StartDate, &iv.EndDate, &iv.Color, &iv.Visibility); err != nil {
			return nil, err
		}
		intervals = append(intervals, iv)
	}
	return intervals, rows.Err()
}

func createInterval(db *sql.DB, userID int64, iv Interval) (Interval, error) {
	if iv.Color == "" {
		iv.Color = "#4f8ef7"
	}
	if iv.Visibility == "" {
		iv.Visibility = "private"
	}
	res, err := db.Exec(
		`INSERT INTO intervals (user_id, name, start_date, end_date, color, visibility) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, iv.Name, iv.StartDate, iv.EndDate, iv.Color, iv.Visibility,
	)
	if err != nil {
		return Interval{}, err
	}
	iv.ID, _ = res.LastInsertId()
	return iv, nil
}

func updateInterval(db *sql.DB, userID, id int64, iv Interval) error {
	if iv.Color == "" {
		iv.Color = "#4f8ef7"
	}
	if iv.Visibility == "" {
		iv.Visibility = "private"
	}
	res, err := db.Exec(
		`UPDATE intervals SET name=?, start_date=?, end_date=?, color=?, visibility=? WHERE id=? AND user_id=?`,
		iv.Name, iv.StartDate, iv.EndDate, iv.Color, iv.Visibility, id, userID,
	)
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
	return nil
}

func deleteInterval(db *sql.DB, userID, id int64) error {
	res, err := db.Exec(`DELETE FROM intervals WHERE id=? AND user_id=?`, id, userID)
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
	return nil
}

func updateDisplayName(db *sql.DB, userID int64, displayName string) (User, error) {
	_, err := db.Exec(`UPDATE users SET display_name=? WHERE id=?`, displayName, userID)
	if err != nil {
		return User{}, err
	}

	var user User
	err = db.QueryRow(`SELECT id, username, display_name FROM users WHERE id=?`, userID).Scan(&user.ID, &user.Username, &user.DisplayName)
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

	// Delete child rows explicitly so account deletion does not depend on SQLite
	// foreign key enforcement settings at runtime.
	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id=?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM intervals WHERE user_id=?`, userID); err != nil {
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

func publicProfileByUsername(db *sql.DB, username string) (PublicProfile, error) {
	var profile PublicProfile
	err := db.QueryRow(`SELECT username, display_name FROM users WHERE username=?`, username).Scan(&profile.Username, &profile.DisplayName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PublicProfile{}, ErrNotFound
		}
		return PublicProfile{}, err
	}

	rows, err := db.Query(
		`SELECT i.id, i.name, i.start_date, i.end_date, i.color, i.visibility
		FROM intervals i
		JOIN users u ON u.id = i.user_id
		WHERE u.username=? AND i.visibility='public'
		ORDER BY i.start_date`,
		username,
	)
	if err != nil {
		return PublicProfile{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var iv Interval
		if err := rows.Scan(&iv.ID, &iv.Name, &iv.StartDate, &iv.EndDate, &iv.Color, &iv.Visibility); err != nil {
			return PublicProfile{}, err
		}
		profile.Intervals = append(profile.Intervals, iv)
	}

	return profile, rows.Err()
}

// parseDate parses an ISO 8601 date string (YYYY-MM-DD).
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func intervalColumnExists(db *sql.DB, column string) (bool, error) {
	return tableColumnExists(db, "intervals", column)
}

func tableColumnExists(db *sql.DB, tableName, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}

	return false, rows.Err()
}
