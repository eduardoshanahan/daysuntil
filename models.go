package main

import (
	"database/sql"
	"time"
)

type Interval struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Color     string `json:"color"`
}

func initDB(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS intervals (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL,
		start_date TEXT NOT NULL,
		end_date   TEXT NOT NULL,
		color      TEXT NOT NULL DEFAULT '#4f8ef7'
	)`)
	if err != nil {
		return err
	}
	// Migration: add color column to existing databases (ignore error if already exists).
	db.Exec(`ALTER TABLE intervals ADD COLUMN color TEXT NOT NULL DEFAULT '#4f8ef7'`)
	return nil
}

func listIntervals(db *sql.DB) ([]Interval, error) {
	rows, err := db.Query(`SELECT id, name, start_date, end_date, color FROM intervals ORDER BY start_date`)
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

func createInterval(db *sql.DB, iv Interval) (Interval, error) {
	if iv.Color == "" {
		iv.Color = "#4f8ef7"
	}
	res, err := db.Exec(
		`INSERT INTO intervals (name, start_date, end_date, color) VALUES (?, ?, ?, ?)`,
		iv.Name, iv.StartDate, iv.EndDate, iv.Color,
	)
	if err != nil {
		return Interval{}, err
	}
	iv.ID, _ = res.LastInsertId()
	return iv, nil
}

func updateInterval(db *sql.DB, id int64, iv Interval) error {
	if iv.Color == "" {
		iv.Color = "#4f8ef7"
	}
	_, err := db.Exec(
		`UPDATE intervals SET name=?, start_date=?, end_date=?, color=? WHERE id=?`,
		iv.Name, iv.StartDate, iv.EndDate, iv.Color, id,
	)
	return err
}

func deleteInterval(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM intervals WHERE id=?`, id)
	return err
}

// parseDate parses an ISO 8601 date string (YYYY-MM-DD).
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
