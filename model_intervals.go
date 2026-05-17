package main

import (
	"database/sql"
	"errors"
	"time"
)

func listIntervals(db *sql.DB, userID int64) ([]Interval, error) {
	rows, err := db.Query(
		`SELECT i.id, i.name, i.start_date, i.end_date, i.color, i.position, sg.id, sg.name, sg.public_slug
		FROM intervals i
		LEFT JOIN share_groups sg ON sg.id = i.share_group_id
		WHERE i.user_id=?
		ORDER BY i.position, i.id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var intervals []Interval
	for rows.Next() {
		iv, err := scanIntervalRow(rows)
		if err != nil {
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
	position, err := nextIntervalPosition(db, userID)
	if err != nil {
		return Interval{}, err
	}
	res, err := db.Exec(
		`INSERT INTO intervals (user_id, name, start_date, end_date, color, position, share_group_id, visibility) VALUES (?, ?, ?, ?, ?, ?, ?, 'private')`,
		userID, iv.Name, iv.StartDate, iv.EndDate, iv.Color, position, iv.ShareGroupID,
	)
	if err != nil {
		return Interval{}, err
	}
	iv.ID, _ = res.LastInsertId()
	return intervalByID(db, userID, iv.ID)
}

func updateInterval(db *sql.DB, userID, id int64, iv Interval) error {
	if iv.Color == "" {
		iv.Color = "#4f8ef7"
	}
	res, err := db.Exec(
		`UPDATE intervals SET name=?, start_date=?, end_date=?, color=?, share_group_id=?, visibility='private' WHERE id=? AND user_id=?`,
		iv.Name, iv.StartDate, iv.EndDate, iv.Color, iv.ShareGroupID, id, userID,
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

func intervalByID(db *sql.DB, userID, id int64) (Interval, error) {
	row := db.QueryRow(
		`SELECT i.id, i.name, i.start_date, i.end_date, i.color, i.position, sg.id, sg.name, sg.public_slug
		FROM intervals i
		LEFT JOIN share_groups sg ON sg.id = i.share_group_id
		WHERE i.id=? AND i.user_id=?`,
		id,
		userID,
	)

	var iv Interval
	var shareGroupID sql.NullInt64
	var shareGroupName sql.NullString
	var shareGroupSlug sql.NullString
	err := row.Scan(&iv.ID, &iv.Name, &iv.StartDate, &iv.EndDate, &iv.Color, &iv.Position, &shareGroupID, &shareGroupName, &shareGroupSlug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Interval{}, ErrNotFound
		}
		return Interval{}, err
	}
	if shareGroupID.Valid {
		id := shareGroupID.Int64
		iv.ShareGroupID = &id
		iv.ShareGroupName = shareGroupName.String
		iv.ShareGroupSlug = shareGroupSlug.String
	}
	return iv, nil
}

func scanIntervalRow(scanner interface {
	Scan(dest ...any) error
}) (Interval, error) {
	var iv Interval
	var shareGroupID sql.NullInt64
	var shareGroupName sql.NullString
	var shareGroupSlug sql.NullString
	if err := scanner.Scan(&iv.ID, &iv.Name, &iv.StartDate, &iv.EndDate, &iv.Color, &iv.Position, &shareGroupID, &shareGroupName, &shareGroupSlug); err != nil {
		return Interval{}, err
	}
	if shareGroupID.Valid {
		id := shareGroupID.Int64
		iv.ShareGroupID = &id
		iv.ShareGroupName = shareGroupName.String
		iv.ShareGroupSlug = shareGroupSlug.String
	}
	return iv, nil
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

func nextIntervalPosition(db *sql.DB, userID int64) (int, error) {
	var maxPosition sql.NullInt64
	if err := db.QueryRow(`SELECT MAX(position) FROM intervals WHERE user_id=?`, userID).Scan(&maxPosition); err != nil {
		return 0, err
	}
	if !maxPosition.Valid {
		return 1, nil
	}
	return int(maxPosition.Int64) + 1, nil
}

func backfillIntervalPositions(db *sql.DB) error {
	rows, err := db.Query(`SELECT DISTINCT user_id FROM intervals WHERE user_id IS NOT NULL`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, userID := range userIDs {
		if err := normalizeIntervalPositions(db, userID); err != nil {
			return err
		}
	}
	return nil
}

func normalizeIntervalPositions(db *sql.DB, userID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT id FROM intervals WHERE user_id=? ORDER BY position, id`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for index, id := range ids {
		position := index + 1
		if _, err := tx.Exec(`UPDATE intervals SET position=? WHERE id=? AND user_id=?`, position, id, userID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func moveInterval(db *sql.DB, userID, id int64, direction string) error {
	if err := normalizeIntervalPositions(db, userID); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentPosition int
	err = tx.QueryRow(`SELECT position FROM intervals WHERE id=? AND user_id=?`, id, userID).Scan(&currentPosition)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	comparator := "<"
	order := "DESC"
	if direction == "down" {
		comparator = ">"
		order = "ASC"
	}

	var neighborID int64
	var neighborPosition int
	err = tx.QueryRow(
		`SELECT id, position FROM intervals
		WHERE user_id=? AND position `+comparator+` ?
		ORDER BY position `+order+`, id `+order+`
		LIMIT 1`,
		userID,
		currentPosition,
	).Scan(&neighborID, &neighborPosition)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	if _, err := tx.Exec(`UPDATE intervals SET position=? WHERE id=? AND user_id=?`, neighborPosition, id, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE intervals SET position=? WHERE id=? AND user_id=?`, currentPosition, neighborID, userID); err != nil {
		return err
	}

	return tx.Commit()
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
