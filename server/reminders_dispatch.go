package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// mailSender is the boundary the dispatcher sends through — *mailer
// implements it at runtime; tests substitute a fake to assert dispatch
// behavior without a real SMTP relay.
type mailSender interface {
	send(to, subject, body string) error
}

// startReminderDispatcher runs an in-process ticker that polls for due
// reminders and emails them — no new service/queue, so this stays inside
// the existing single-container deploy. Reminders fire within one poll
// interval of their scheduled time, which is fine for day-level reminders.
func startReminderDispatcher(ctx context.Context, db *sql.DB, m mailSender, profileClient ProfileClient, poll time.Duration) {
	ticker := time.NewTicker(poll)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := dispatchDueReminders(ctx, db, m, profileClient); err != nil {
					log.Printf("reminder dispatch: %v", err)
				}
			}
		}
	}()
}

func dispatchDueReminders(ctx context.Context, db *sql.DB, m mailSender, profileClient ProfileClient) error {
	now := time.Now().UTC()

	due, err := dueReminders(db, now)
	if err != nil {
		return fmt.Errorf("query due reminders: %w", err)
	}

	for _, d := range due {
		profile, err := profileClient.GetBySub(ctx, d.OIDCSub)
		if err != nil {
			log.Printf("reminder %d: profile lookup failed: %v", d.ID, err)
			continue
		}
		if profile.Email == "" {
			log.Printf("reminder %d: owner has no email on file, skipping", d.ID)
			continue
		}
		if !profile.EmailVerified {
			// The IdP hasn't confirmed this address belongs to the user —
			// could be a typo, or someone else's address entered before
			// verification completes. Never email an unverified address.
			log.Printf("reminder %d: owner's email is unverified, skipping", d.ID)
			continue
		}

		subject := "Reminder: " + d.IntervalName
		body := d.Message
		if body == "" {
			body = d.IntervalName
		}

		if err := m.send(profile.Email, subject, body); err != nil {
			log.Printf("reminder %d: send failed: %v", d.ID, err)
			continue
		}

		if err := markReminderSentOrAdvance(db, d, now); err != nil {
			log.Printf("reminder %d: failed to mark sent: %v", d.ID, err)
		}
	}

	return nil
}
