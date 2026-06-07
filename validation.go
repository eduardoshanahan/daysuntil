package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func validateInterval(input intervalInput, db *sql.DB, userID int64) ([]int64, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	start, err := parseDate(input.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date: %w", err)
	}
	end, err := parseDate(input.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date: %w", err)
	}
	if end.Before(start) {
		return nil, fmt.Errorf("end_date must be on or after start_date")
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
