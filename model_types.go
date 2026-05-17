package main

import "errors"

type Interval struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	Color          string `json:"color"`
	Position       int    `json:"position"`
	ShareGroupID   *int64 `json:"share_group_id"`
	ShareGroupName string `json:"share_group_name,omitempty"`
	ShareGroupSlug string `json:"share_group_slug,omitempty"`
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
