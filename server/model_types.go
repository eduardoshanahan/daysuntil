package main

import (
	"errors"
	"time"
)

type Interval struct {
	ID                 int64        `json:"id"`
	Name               string       `json:"name"`
	StartAt            time.Time    `json:"start_at"`
	EndAt              time.Time    `json:"end_at"`
	Timezone           string       `json:"timezone"`
	AllDay             bool         `json:"all_day"`
	Color              string       `json:"color"`
	Icon               string       `json:"icon"`
	BackgroundImageURL string       `json:"background_image_url"`
	RecurrenceRule     string       `json:"recurrence_rule"`
	DisplayUnit        string       `json:"display_unit"`
	Position           int          `json:"position"`
	ShareGroups        []ShareGroup `json:"share_groups"`
}

type Reminder struct {
	ID         int64      `json:"id"`
	IntervalID int64      `json:"interval_id"`
	RemindAt   time.Time  `json:"remind_at"`
	RepeatRule string     `json:"repeat_rule"`
	Message    string     `json:"message"`
	SentAt     *time.Time `json:"sent_at,omitempty"`
}

type ShareGroup struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	PublicSlug string `json:"public_slug"`
}

type PublicShareGroup struct {
	Name          string     `json:"name"`
	PublicSlug    string     `json:"public_slug"`
	OwnerName     string     `json:"owner_name"`
	OwnerUsername string     `json:"owner_username"`
	Intervals     []Interval `json:"intervals"`
}

var ErrNotFound = errors.New("interval not found")
