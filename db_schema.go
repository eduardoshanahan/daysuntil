package main

import "database/sql"

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
