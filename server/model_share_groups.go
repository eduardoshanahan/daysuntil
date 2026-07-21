package main

import (
	"database/sql"
	"errors"
	"strings"
)

func listShareGroups(db *sql.DB, userID int64) ([]ShareGroup, error) {
	rows, err := db.Query(`SELECT id, name, public_slug FROM share_groups WHERE user_id=$1 ORDER BY created_at, id`, userID)
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

	var id int64
	err = db.QueryRow(
		`INSERT INTO share_groups (user_id, name, public_slug) VALUES ($1, $2, $3) RETURNING id`,
		userID, name, publicSlug,
	).Scan(&id)
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

	res, err := db.Exec(`UPDATE share_groups SET name=$1 WHERE id=$2 AND user_id=$3`, name, groupID, userID)
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
	res, err := db.Exec(`DELETE FROM share_groups WHERE id=$1 AND user_id=$2`, groupID, userID)
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

func rotateShareGroupSlug(db *sql.DB, userID, groupID int64) (ShareGroup, error) {
	for attempt := 0; attempt < 20; attempt++ {
		slug, err := createUniqueShareGroupSlug(db)
		if err != nil {
			return ShareGroup{}, err
		}
		res, err := db.Exec(`UPDATE share_groups SET public_slug=$1 WHERE id=$2 AND user_id=$3`, slug, groupID, userID)
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
	err := db.QueryRow(`SELECT id, name, public_slug FROM share_groups WHERE id=$1 AND user_id=$2`, groupID, userID).Scan(&group.ID, &group.Name, &group.PublicSlug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ShareGroup{}, ErrNotFound
		}
		return ShareGroup{}, err
	}
	return group, nil
}

func shareGroupsOwnedByUser(db *sql.DB, userID int64, groupIDs []int64) ([]int64, error) {
	var verified []int64
	for _, groupID := range groupIDs {
		var existing int64
		err := db.QueryRow(`SELECT id FROM share_groups WHERE id=$1 AND user_id=$2`, groupID, userID).Scan(&existing)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		verified = append(verified, existing)
	}
	return verified, nil
}

// publicShareGroupRaw is the DB-only view of a public share group: the
// owner is identified by oidc_sub, not by name — the handler layer
// resolves owner username/display name via ProfileClient, keeping this
// package free of profile-service concerns.
type publicShareGroupRaw struct {
	Name       string
	PublicSlug string
	OwnerSub   string
	Intervals  []Interval
}

func publicShareGroupBySlug(db *sql.DB, groupSlug string) (publicShareGroupRaw, error) {
	var raw publicShareGroupRaw
	rows, err := db.Query(
		`SELECT sg.name, sg.public_slug, u.oidc_sub, i.id, i.name, i.start_date, i.end_date, i.color
		FROM interval_share_groups isg
		JOIN share_groups sg ON sg.id = isg.share_group_id
		JOIN users u ON u.id = sg.user_id
		JOIN intervals i ON i.id = isg.interval_id
		WHERE sg.public_slug=$1
		ORDER BY i.start_date`,
		groupSlug,
	)
	if err != nil {
		return publicShareGroupRaw{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var iv Interval
		if err := rows.Scan(&raw.Name, &raw.PublicSlug, &raw.OwnerSub, &iv.ID, &iv.Name, &iv.StartDate, &iv.EndDate, &iv.Color); err != nil {
			return publicShareGroupRaw{}, err
		}
		raw.Intervals = append(raw.Intervals, iv)
	}
	if err := rows.Err(); err != nil {
		return publicShareGroupRaw{}, err
	}
	if len(raw.Intervals) == 0 {
		var existing int64
		err := db.QueryRow(`SELECT id FROM share_groups WHERE public_slug=$1`, groupSlug).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			return publicShareGroupRaw{}, ErrNotFound
		}
		if err != nil {
			return publicShareGroupRaw{}, err
		}
		return publicShareGroupRaw{}, ErrNotFound
	}
	return raw, nil
}

func createUniqueShareGroupSlug(db *sql.DB) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		slug, err := randomShareGroupSlug()
		if err != nil {
			return "", err
		}
		var existing int64
		err = db.QueryRow(`SELECT id FROM share_groups WHERE public_slug=$1`, slug).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			return slug, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", errors.New("failed to allocate share group slug")
}
