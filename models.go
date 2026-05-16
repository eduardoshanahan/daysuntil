package main

import (
	"database/sql"
	"errors"
	"time"
)

type Interval struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Color     string `json:"color"`
}

var ErrNotFound = errors.New("interval not found")

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

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_intervals_user_id_start_date ON intervals(user_id, start_date)`)
	if err != nil {
		return err
	}

	return nil
}

func listIntervals(db *sql.DB, userID int64) ([]Interval, error) {
	rows, err := db.Query(
		`SELECT id, name, start_date, end_date, color
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
		if err := rows.Scan(&iv.ID, &iv.Name, &iv.StartDate, &iv.EndDate, &iv.Color); err != nil {
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
	res, err := db.Exec(
		`INSERT INTO intervals (user_id, name, start_date, end_date, color) VALUES (?, ?, ?, ?, ?)`,
		userID, iv.Name, iv.StartDate, iv.EndDate, iv.Color,
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
	res, err := db.Exec(
		`UPDATE intervals SET name=?, start_date=?, end_date=?, color=? WHERE id=? AND user_id=?`,
		iv.Name, iv.StartDate, iv.EndDate, iv.Color, id, userID,
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
