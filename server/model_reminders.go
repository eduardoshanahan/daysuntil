package main

import (
	"database/sql"
	"errors"
	"time"
)

func listRemindersForInterval(db *sql.DB, userID, intervalID int64) ([]Reminder, error) {
	if _, err := intervalByID(db, userID, intervalID); err != nil {
		return nil, err
	}

	rows, err := db.Query(
		`SELECT id, interval_id, remind_at, repeat_rule, message, sent_at
		FROM reminders WHERE interval_id=$1 ORDER BY remind_at, id`,
		intervalID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []Reminder
	for rows.Next() {
		var rem Reminder
		if err := rows.Scan(&rem.ID, &rem.IntervalID, &rem.RemindAt, &rem.RepeatRule, &rem.Message, &rem.SentAt); err != nil {
			return nil, err
		}
		reminders = append(reminders, rem)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if reminders == nil {
		return []Reminder{}, nil
	}
	return reminders, nil
}

func createReminder(db *sql.DB, userID, intervalID int64, input reminderInput) (Reminder, error) {
	if _, err := intervalByID(db, userID, intervalID); err != nil {
		return Reminder{}, err
	}

	remindAt, err := time.Parse(time.RFC3339, input.RemindAt)
	if err != nil {
		return Reminder{}, err
	}

	var id int64
	err = db.QueryRow(
		`INSERT INTO reminders (interval_id, user_id, remind_at, repeat_rule, message)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		intervalID, userID, remindAt, input.RepeatRule, input.Message,
	).Scan(&id)
	if err != nil {
		return Reminder{}, err
	}

	return reminderByID(db, userID, id)
}

func updateReminder(db *sql.DB, userID, id int64, input reminderInput) (Reminder, error) {
	remindAt, err := time.Parse(time.RFC3339, input.RemindAt)
	if err != nil {
		return Reminder{}, err
	}

	res, err := db.Exec(
		`UPDATE reminders SET remind_at=$1, repeat_rule=$2, message=$3, sent_at=NULL
		WHERE id=$4 AND user_id=$5`,
		remindAt, input.RepeatRule, input.Message, id, userID,
	)
	if err != nil {
		return Reminder{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Reminder{}, err
	}
	if n == 0 {
		return Reminder{}, ErrNotFound
	}

	return reminderByID(db, userID, id)
}

func reminderByID(db *sql.DB, userID, id int64) (Reminder, error) {
	var rem Reminder
	err := db.QueryRow(
		`SELECT r.id, r.interval_id, r.remind_at, r.repeat_rule, r.message, r.sent_at
		FROM reminders r WHERE r.id=$1 AND r.user_id=$2`,
		id, userID,
	).Scan(&rem.ID, &rem.IntervalID, &rem.RemindAt, &rem.RepeatRule, &rem.Message, &rem.SentAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Reminder{}, ErrNotFound
		}
		return Reminder{}, err
	}
	return rem, nil
}

func deleteReminder(db *sql.DB, userID, id int64) error {
	res, err := db.Exec(`DELETE FROM reminders WHERE id=$1 AND user_id=$2`, id, userID)
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
	return nil
}

// dueReminder is the dispatcher-only view of a reminder ready to send: it
// carries the interval name (for the email subject/body) and the owner's
// oidc_sub (to look up the current email from profile-service at send
// time — daysuntil's own DB never stores email, see profileclient.go).
type dueReminder struct {
	ID           int64
	IntervalName string
	OIDCSub      string
	RemindAt     time.Time
	RepeatRule   string
	Message      string
}

func dueReminders(db *sql.DB, now time.Time) ([]dueReminder, error) {
	rows, err := db.Query(
		`SELECT r.id, i.name, u.oidc_sub, r.remind_at, r.repeat_rule, r.message
		FROM reminders r
		JOIN intervals i ON i.id = r.interval_id
		JOIN users u ON u.id = r.user_id
		WHERE r.remind_at <= $1 AND (r.repeat_rule <> 'none' OR r.sent_at IS NULL)`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var due []dueReminder
	for rows.Next() {
		var d dueReminder
		if err := rows.Scan(&d.ID, &d.IntervalName, &d.OIDCSub, &d.RemindAt, &d.RepeatRule, &d.Message); err != nil {
			return nil, err
		}
		due = append(due, d)
	}
	return due, rows.Err()
}

// markReminderSentOrAdvance marks a one-time reminder as sent, or rolls a
// repeating reminder's remind_at forward to its next occurrence — never
// mutating the interval's own dates, matching the compute-on-read approach
// used for recurring intervals (see recurrence.go).
func markReminderSentOrAdvance(db *sql.DB, d dueReminder, now time.Time) error {
	if d.RepeatRule == "none" {
		_, err := db.Exec(`UPDATE reminders SET sent_at=$1 WHERE id=$2`, now, d.ID)
		return err
	}
	next := nextOccurrence(d.RemindAt, d.RepeatRule, now)
	_, err := db.Exec(`UPDATE reminders SET remind_at=$1, sent_at=$2 WHERE id=$3`, next, now, d.ID)
	return err
}
