package main

import (
	"database/sql"
	"errors"
	"time"
)

func listIntervals(db *sql.DB, userID int64) ([]Interval, error) {
	rows, err := db.Query(
		`SELECT id, name, start_at, end_at, timezone, all_day, color, icon, background_image_url, recurrence_rule, display_unit, position
		FROM intervals WHERE user_id=$1 ORDER BY position, id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var intervals []Interval
	for rows.Next() {
		var iv Interval
		if err := rows.Scan(&iv.ID, &iv.Name, &iv.StartAt, &iv.EndAt, &iv.Timezone, &iv.AllDay, &iv.Color, &iv.Icon, &iv.BackgroundImageURL, &iv.RecurrenceRule, &iv.DisplayUnit, &iv.Position); err != nil {
			return nil, err
		}
		iv.ShareGroups = []ShareGroup{}
		intervals = append(intervals, iv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range intervals {
		groups, err := intervalGroupsByID(db, intervals[i].ID)
		if err != nil {
			return nil, err
		}
		intervals[i].ShareGroups = groups
	}

	if intervals == nil {
		return []Interval{}, nil
	}
	return intervals, nil
}

func intervalGroupsByID(db *sql.DB, intervalID int64) ([]ShareGroup, error) {
	rows, err := db.Query(
		`SELECT sg.id, sg.name, sg.public_slug
		FROM interval_share_groups isg
		JOIN share_groups sg ON sg.id = isg.share_group_id
		WHERE isg.interval_id = $1`,
		intervalID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []ShareGroup
	for rows.Next() {
		var g ShareGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.PublicSlug); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	if groups == nil {
		groups = []ShareGroup{}
	}
	return groups, rows.Err()
}

func createInterval(db *sql.DB, userID int64, input intervalInput) (Interval, error) {
	startAt, err := time.Parse(time.RFC3339, input.StartAt)
	if err != nil {
		return Interval{}, err
	}
	endAt, err := time.Parse(time.RFC3339, input.EndAt)
	if err != nil {
		return Interval{}, err
	}

	position, err := nextIntervalPosition(db, userID)
	if err != nil {
		return Interval{}, err
	}

	tx, err := db.Begin()
	if err != nil {
		return Interval{}, err
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRow(
		`INSERT INTO intervals (user_id, name, start_at, end_at, timezone, all_day, color, icon, background_image_url, recurrence_rule, display_unit, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id`,
		userID, input.Name, startAt, endAt, input.Timezone, input.AllDay, input.Color,
		input.Icon, input.BackgroundImageURL, input.RecurrenceRule, input.DisplayUnit, position,
	).Scan(&id)
	if err != nil {
		return Interval{}, err
	}

	for _, groupID := range input.ShareGroupIDs {
		if _, err := tx.Exec(`INSERT INTO interval_share_groups (interval_id, share_group_id) VALUES ($1, $2)`, id, groupID); err != nil {
			return Interval{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Interval{}, err
	}

	return intervalByID(db, userID, id)
}

func updateInterval(db *sql.DB, userID, id int64, input intervalInput) error {
	startAt, err := time.Parse(time.RFC3339, input.StartAt)
	if err != nil {
		return err
	}
	endAt, err := time.Parse(time.RFC3339, input.EndAt)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE intervals SET name=$1, start_at=$2, end_at=$3, timezone=$4, all_day=$5, color=$6,
		icon=$7, background_image_url=$8, recurrence_rule=$9, display_unit=$10
		WHERE id=$11 AND user_id=$12`,
		input.Name, startAt, endAt, input.Timezone, input.AllDay, input.Color,
		input.Icon, input.BackgroundImageURL, input.RecurrenceRule, input.DisplayUnit, id, userID,
	)
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

	if _, err := tx.Exec(`DELETE FROM interval_share_groups WHERE interval_id=$1`, id); err != nil {
		return err
	}
	for _, groupID := range input.ShareGroupIDs {
		if _, err := tx.Exec(`INSERT INTO interval_share_groups (interval_id, share_group_id) VALUES ($1, $2)`, id, groupID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func intervalByID(db *sql.DB, userID, id int64) (Interval, error) {
	var iv Interval
	err := db.QueryRow(
		`SELECT id, name, start_at, end_at, timezone, all_day, color, icon, background_image_url, recurrence_rule, display_unit, position
		FROM intervals WHERE id=$1 AND user_id=$2`,
		id, userID,
	).Scan(&iv.ID, &iv.Name, &iv.StartAt, &iv.EndAt, &iv.Timezone, &iv.AllDay, &iv.Color, &iv.Icon, &iv.BackgroundImageURL, &iv.RecurrenceRule, &iv.DisplayUnit, &iv.Position)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Interval{}, ErrNotFound
		}
		return Interval{}, err
	}
	groups, err := intervalGroupsByID(db, id)
	if err != nil {
		return Interval{}, err
	}
	iv.ShareGroups = groups
	return iv, nil
}

func deleteInterval(db *sql.DB, userID, id int64) error {
	res, err := db.Exec(`DELETE FROM intervals WHERE id=$1 AND user_id=$2`, id, userID)
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
	if err := db.QueryRow(`SELECT MAX(position) FROM intervals WHERE user_id=$1`, userID).Scan(&maxPosition); err != nil {
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

	rows, err := tx.Query(`SELECT id FROM intervals WHERE user_id=$1 ORDER BY position, id`, userID)
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
		if _, err := tx.Exec(`UPDATE intervals SET position=$1 WHERE id=$2 AND user_id=$3`, position, id, userID); err != nil {
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
	err = tx.QueryRow(`SELECT position FROM intervals WHERE id=$1 AND user_id=$2`, id, userID).Scan(&currentPosition)
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
		WHERE user_id=$1 AND position `+comparator+` $2
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

	if _, err := tx.Exec(`UPDATE intervals SET position=$1 WHERE id=$2 AND user_id=$3`, neighborPosition, id, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE intervals SET position=$1 WHERE id=$2 AND user_id=$3`, currentPosition, neighborID, userID); err != nil {
		return err
	}

	return tx.Commit()
}
