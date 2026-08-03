package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

var validRecurrenceRules = map[string]bool{"none": true, "weekly": true, "monthly": true, "yearly": true}
var validDisplayUnits = map[string]bool{
	"auto": true, "seconds": true, "minutes": true, "hours": true,
	"days": true, "weeks": true, "months": true, "years": true, "sleeps": true,
}
var validRepeatRules = map[string]bool{"none": true, "daily": true, "weekly": true, "monthly": true, "yearly": true}

// validateReminder validates input in place, defaulting repeat_rule to
// "none" when omitted, mirroring validateInterval's normalize-then-persist
// pattern.
func validateReminder(input *reminderInput) error {
	if _, err := time.Parse(time.RFC3339, input.RemindAt); err != nil {
		return fmt.Errorf("invalid remind_at: %w", err)
	}
	if input.RepeatRule == "" {
		input.RepeatRule = "none"
	}
	if !validRepeatRules[input.RepeatRule] {
		return fmt.Errorf("invalid repeat_rule: %q", input.RepeatRule)
	}
	if utf8.RuneCountInString(input.Message) > 500 {
		return fmt.Errorf("message must be at most 500 characters")
	}
	return nil
}

// validateInterval validates input in place, filling in defaults for any
// omitted optional field (color, timezone, recurrence_rule, display_unit)
// so the normalized values are what actually gets persisted — the caller
// must use the same *input it passed in when calling createInterval/
// updateInterval afterward.
func validateInterval(input *intervalInput, db *sql.DB, userID int64) ([]int64, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	start, err := time.Parse(time.RFC3339, input.StartAt)
	if err != nil {
		return nil, fmt.Errorf("invalid start_at: %w", err)
	}
	end, err := time.Parse(time.RFC3339, input.EndAt)
	if err != nil {
		return nil, fmt.Errorf("invalid end_at: %w", err)
	}
	if end.Before(start) {
		return nil, fmt.Errorf("end_at must be on or after start_at")
	}

	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.Timezone == "" {
		input.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	if utf8.RuneCountInString(input.Icon) > 8 {
		return nil, fmt.Errorf("icon must be at most 8 characters")
	}

	input.BackgroundImageURL = strings.TrimSpace(input.BackgroundImageURL)
	if input.BackgroundImageURL != "" {
		parsed, err := url.Parse(input.BackgroundImageURL)
		if err != nil || !parsed.IsAbs() || parsed.Scheme != "https" {
			return nil, fmt.Errorf("background_image_url must be an absolute https URL")
		}
	}

	if input.RecurrenceRule == "" {
		input.RecurrenceRule = "none"
	}
	if !validRecurrenceRules[input.RecurrenceRule] {
		return nil, fmt.Errorf("invalid recurrence_rule: %q", input.RecurrenceRule)
	}

	if input.DisplayUnit == "" {
		input.DisplayUnit = "auto"
	}
	if !validDisplayUnits[input.DisplayUnit] {
		return nil, fmt.Errorf("invalid display_unit: %q", input.DisplayUnit)
	}

	if input.Color == "" {
		input.Color = "#4f8ef7"
	}

	groupIDs, err := shareGroupsOwnedByUser(db, userID, input.ShareGroupIDs)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("share group not found")
		}
		return nil, err
	}
	return groupIDs, nil
}
