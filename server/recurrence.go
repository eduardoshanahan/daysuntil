package main

import "time"

// nextOccurrence advances anchor forward by whole recurrence periods until
// the result is strictly after now. It never mutates the caller's stored
// dates — callers that need a rolled-forward value use the return here
// without writing it back to the original anchor column (see the reminder
// dispatcher, which advances a reminder's own remind_at this way, and
// dates.js's resolveOccurrence, which does the equivalent for the UI without
// ever touching interval start_at/end_at).
func nextOccurrence(anchor time.Time, rule string, now time.Time) time.Time {
	if !anchor.After(now) {
		switch rule {
		case "daily":
			for !anchor.After(now) {
				anchor = anchor.AddDate(0, 0, 1)
			}
		case "weekly":
			for !anchor.After(now) {
				anchor = anchor.AddDate(0, 0, 7)
			}
		case "monthly":
			for !anchor.After(now) {
				anchor = anchor.AddDate(0, 1, 0)
			}
		case "yearly":
			for !anchor.After(now) {
				anchor = anchor.AddDate(1, 0, 0)
			}
		}
	}
	return anchor
}
