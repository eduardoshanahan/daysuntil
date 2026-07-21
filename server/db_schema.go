package main

import "database/sql"

func initDB(db *sql.DB) error {
	if err := initAuthDB(db); err != nil {
		return err
	}

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS intervals (
		id         BIGSERIAL PRIMARY KEY,
		user_id    BIGINT REFERENCES users(id) ON DELETE CASCADE,
		name       TEXT NOT NULL,
		start_date TEXT NOT NULL,
		end_date   TEXT NOT NULL,
		color      TEXT NOT NULL DEFAULT '#4f8ef7',
		position   INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_intervals_user_id_start_date ON intervals(user_id, start_date)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS share_groups (
		id          BIGSERIAL PRIMARY KEY,
		user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		public_slug TEXT NOT NULL,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
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

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS interval_share_groups (
		interval_id    BIGINT NOT NULL REFERENCES intervals(id) ON DELETE CASCADE,
		share_group_id BIGINT NOT NULL REFERENCES share_groups(id) ON DELETE CASCADE,
		PRIMARY KEY(interval_id, share_group_id)
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_interval_share_groups_share_group_id ON interval_share_groups(share_group_id)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS rate_limit_buckets (
		key      TEXT PRIMARY KEY,
		count    INTEGER NOT NULL,
		reset_at TIMESTAMPTZ NOT NULL
	)`)
	if err != nil {
		return err
	}

	return nil
}
