package main

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Interval struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	Color          string `json:"color"`
	Position       int    `json:"position"`
	ShareGroupID   *int64 `json:"share_group_id"`
	ShareGroupName string `json:"share_group_name,omitempty"`
	ShareGroupSlug string `json:"share_group_slug,omitempty"`
}

type ShareGroup struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	PublicSlug string `json:"public_slug"`
}

type PublicShareGroup struct {
	Name          string     `json:"name"`
	PublicSlug    string     `json:"public_slug"`
	OwnerName     string     `json:"owner_name"`
	OwnerUsername string     `json:"owner_username"`
	Intervals     []Interval `json:"intervals"`
}

var ErrNotFound = errors.New("interval not found")

func initDB(db *sql.DB) error {
	if err := initAuthDB(db); err != nil {
		return err
	}

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS intervals (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id        INTEGER,
		name           TEXT NOT NULL,
		start_date     TEXT NOT NULL,
		end_date       TEXT NOT NULL,
		color          TEXT NOT NULL DEFAULT '#4f8ef7',
		position       INTEGER NOT NULL DEFAULT 0,
		visibility     TEXT NOT NULL DEFAULT 'private',
		share_group_id INTEGER,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return err
	}

	if err := ensureIntervalColumn(db, "color", "ALTER TABLE intervals ADD COLUMN color TEXT NOT NULL DEFAULT '#4f8ef7'"); err != nil {
		return err
	}
	if err := ensureIntervalColumn(db, "user_id", "ALTER TABLE intervals ADD COLUMN user_id INTEGER"); err != nil {
		return err
	}
	if err := ensureIntervalColumn(db, "visibility", "ALTER TABLE intervals ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private'"); err != nil {
		return err
	}
	if err := ensureIntervalColumn(db, "position", "ALTER TABLE intervals ADD COLUMN position INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureIntervalColumn(db, "share_group_id", "ALTER TABLE intervals ADD COLUMN share_group_id INTEGER"); err != nil {
		return err
	}
	if err := backfillIntervalPositions(db); err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_intervals_user_id_start_date ON intervals(user_id, start_date)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_intervals_share_group_id ON intervals(share_group_id)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS share_groups (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     INTEGER NOT NULL,
		name        TEXT NOT NULL,
		public_slug TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_share_groups_public_slug ON share_groups(public_slug)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_share_groups_user_id ON share_groups(user_id)`)
	if err != nil {
		return err
	}

	return nil
}

func ensureIntervalColumn(db *sql.DB, column, statement string) error {
	exists, err := intervalColumnExists(db, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = db.Exec(statement)
	return err
}

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

func listShareGroups(db *sql.DB, userID int64) ([]ShareGroup, error) {
	rows, err := db.Query(`SELECT id, name, public_slug FROM share_groups WHERE user_id=? ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []ShareGroup
	for rows.Next() {
		var group ShareGroup
		if err := rows.Scan(&group.ID, &group.Name, &group.PublicSlug); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func validateShareGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("name is required")
	}
	if len(name) > 80 {
		return "", errors.New("name must be at most 80 characters")
	}
	return name, nil
}

func createShareGroup(db *sql.DB, userID int64, name string) (ShareGroup, error) {
	name, err := validateShareGroupName(name)
	if err != nil {
		return ShareGroup{}, err
	}

	publicSlug, err := createUniqueShareGroupSlug(db)
	if err != nil {
		return ShareGroup{}, err
	}

	res, err := db.Exec(
		`INSERT INTO share_groups (user_id, name, public_slug, created_at) VALUES (?, ?, ?, ?)`,
		userID, name, publicSlug, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return ShareGroup{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ShareGroup{}, err
	}
	return ShareGroup{ID: id, Name: name, PublicSlug: publicSlug}, nil
}

func updateShareGroup(db *sql.DB, userID, groupID int64, name string) (ShareGroup, error) {
	name, err := validateShareGroupName(name)
	if err != nil {
		return ShareGroup{}, err
	}

	res, err := db.Exec(`UPDATE share_groups SET name=? WHERE id=? AND user_id=?`, name, groupID, userID)
	if err != nil {
		return ShareGroup{}, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return ShareGroup{}, err
	}
	if rows == 0 {
		return ShareGroup{}, ErrNotFound
	}
	return shareGroupByID(db, userID, groupID)
}

func deleteShareGroup(db *sql.DB, userID, groupID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE intervals SET share_group_id=NULL, visibility='private' WHERE user_id=? AND share_group_id=?`, userID, groupID); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM share_groups WHERE id=? AND user_id=?`, groupID, userID)
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

func rotateShareGroupSlug(db *sql.DB, userID, groupID int64) (ShareGroup, error) {
	for attempt := 0; attempt < 20; attempt++ {
		slug, err := createUniqueShareGroupSlug(db)
		if err != nil {
			return ShareGroup{}, err
		}
		res, err := db.Exec(`UPDATE share_groups SET public_slug=? WHERE id=? AND user_id=?`, slug, groupID, userID)
		if err != nil {
			return ShareGroup{}, err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return ShareGroup{}, err
		}
		if rows == 0 {
			return ShareGroup{}, ErrNotFound
		}
		return shareGroupByID(db, userID, groupID)
	}
	return ShareGroup{}, errors.New("failed to rotate share group slug")
}

func shareGroupByID(db *sql.DB, userID, groupID int64) (ShareGroup, error) {
	var group ShareGroup
	err := db.QueryRow(`SELECT id, name, public_slug FROM share_groups WHERE id=? AND user_id=?`, groupID, userID).Scan(&group.ID, &group.Name, &group.PublicSlug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ShareGroup{}, ErrNotFound
		}
		return ShareGroup{}, err
	}
	return group, nil
}

func shareGroupOwnedByUser(db *sql.DB, userID int64, groupID *int64) (*int64, error) {
	if groupID == nil {
		return nil, nil
	}
	var existing int64
	err := db.QueryRow(`SELECT id FROM share_groups WHERE id=? AND user_id=?`, *groupID, userID).Scan(&existing)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &existing, nil
}

func publicShareGroupBySlug(db *sql.DB, groupSlug string) (PublicShareGroup, error) {
	var profile PublicShareGroup
	rows, err := db.Query(
		`SELECT sg.name, sg.public_slug, u.username, u.display_name, i.id, i.name, i.start_date, i.end_date, i.color
		FROM intervals i
		JOIN share_groups sg ON sg.id = i.share_group_id
		JOIN users u ON u.id = sg.user_id
		WHERE sg.public_slug=?
		ORDER BY i.start_date`,
		groupSlug,
	)
	if err != nil {
		return PublicShareGroup{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var iv Interval
		if err := rows.Scan(&profile.Name, &profile.PublicSlug, &profile.OwnerUsername, &profile.OwnerName, &iv.ID, &iv.Name, &iv.StartDate, &iv.EndDate, &iv.Color); err != nil {
			return PublicShareGroup{}, err
		}
		profile.Intervals = append(profile.Intervals, iv)
	}
	if err := rows.Err(); err != nil {
		return PublicShareGroup{}, err
	}
	if len(profile.Intervals) == 0 {
		var existing int64
		err := db.QueryRow(`SELECT id FROM share_groups WHERE public_slug=?`, groupSlug).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			return PublicShareGroup{}, ErrNotFound
		}
		if err != nil {
			return PublicShareGroup{}, err
		}
		return PublicShareGroup{}, ErrNotFound
	}
	return profile, nil
}

func createUniqueShareGroupSlug(db *sql.DB) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		slug, err := randomPublicSlug()
		if err != nil {
			return "", err
		}
		var existing int64
		err = db.QueryRow(`SELECT id FROM share_groups WHERE public_slug=?`, slug).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			return slug, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("failed to allocate share group slug")
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
		var (
			cid        int
			name       string
			dataType   string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultVal, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
