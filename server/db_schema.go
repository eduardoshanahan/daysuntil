package main

import "database/sql"

func initDB(db *sql.DB) error {
	if err := initAuthDB(db); err != nil {
		return err
	}

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS intervals (
		id          BIGSERIAL PRIMARY KEY,
		user_id     BIGINT REFERENCES users(id) ON DELETE CASCADE,
		name        TEXT NOT NULL,
		start_at    TIMESTAMPTZ NOT NULL,
		end_at      TIMESTAMPTZ NOT NULL,
		timezone    TEXT NOT NULL DEFAULT 'UTC',
		all_day     BOOLEAN NOT NULL DEFAULT true,
		color       TEXT NOT NULL DEFAULT '#4f8ef7',
		icon        TEXT NOT NULL DEFAULT '',
		background_image_url TEXT NOT NULL DEFAULT '',
		recurrence_rule TEXT NOT NULL DEFAULT 'none'
			CHECK (recurrence_rule IN ('none','weekly','monthly','yearly')),
		display_unit    TEXT NOT NULL DEFAULT 'auto'
			CHECK (display_unit IN ('auto','seconds','minutes','hours','days','weeks','months','years','sleeps')),
		position    INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_intervals_user_id_start_at ON intervals(user_id, start_at)`)
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

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS reminders (
		id          BIGSERIAL PRIMARY KEY,
		interval_id BIGINT NOT NULL REFERENCES intervals(id) ON DELETE CASCADE,
		user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		remind_at   TIMESTAMPTZ NOT NULL,
		repeat_rule TEXT NOT NULL DEFAULT 'none'
			CHECK (repeat_rule IN ('none','daily','weekly','monthly','yearly')),
		message     TEXT NOT NULL DEFAULT '',
		sent_at     TIMESTAMPTZ,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_reminders_interval_id ON reminders(interval_id)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_reminders_due ON reminders(remind_at)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS api_tokens (
		id           BIGSERIAL PRIMARY KEY,
		user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash   TEXT NOT NULL,
		name         TEXT NOT NULL DEFAULT '',
		created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
		last_used_at TIMESTAMPTZ,
		expires_at   TIMESTAMPTZ
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_api_tokens_token_hash ON api_tokens(token_hash)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id ON api_tokens(user_id)`)
	if err != nil {
		return err
	}

	return nil
}
