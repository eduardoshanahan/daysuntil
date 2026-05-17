package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func validateInterval(iv Interval, db *sql.DB, userID int64) (*int64, error) {
	name := strings.TrimSpace(iv.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	start, err := parseDate(iv.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date: %w", err)
	}
	end, err := parseDate(iv.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date: %w", err)
	}
	if !end.After(start) {
		return nil, fmt.Errorf("end_date must be after start_date")
	}
	groupID, err := shareGroupOwnedByUser(db, userID, iv.ShareGroupID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("share group not found")
		}
		return nil, err
	}
	return groupID, nil
}

func validateDisplayName(displayName string) (string, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "", fmt.Errorf("display_name is required")
	}
	if len(displayName) > 80 {
		return "", fmt.Errorf("display_name must be at most 80 characters")
	}
	return displayName, nil
}
