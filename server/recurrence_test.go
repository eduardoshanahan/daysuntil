package main

import (
	"testing"
	"time"
)

func TestNextOccurrenceRollsForwardPastRules(t *testing.T) {
	anchor := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		rule string
		want time.Time
	}{
		{"none", anchor},
		{"daily", time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)},
		{"weekly", time.Date(2026, 8, 6, 9, 0, 0, 0, time.UTC)},
		{"monthly", time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)},
		{"yearly", time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			got := nextOccurrence(anchor, tc.rule, now)
			if !got.Equal(tc.want) {
				t.Fatalf("nextOccurrence(%v, %q, %v) = %v, want %v", anchor, tc.rule, now, got, tc.want)
			}
		})
	}
}

func TestNextOccurrenceLeavesFutureAnchorUnchanged(t *testing.T) {
	anchor := time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

	got := nextOccurrence(anchor, "yearly", now)
	if !got.Equal(anchor) {
		t.Fatalf("expected anchor already in the future to be left unchanged, got %v", got)
	}
}
