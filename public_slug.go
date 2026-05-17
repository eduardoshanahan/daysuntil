package main

import (
	crand "crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
)

var publicSlugWords = []string{
	"amber", "apple", "atlas", "autumn", "basil", "beacon", "berry", "birch",
	"blossom", "breeze", "brook", "canyon", "cedar", "cloud", "clover", "copper",
	"coral", "cove", "cricket", "daisy", "dawn", "delta", "dune", "ember",
	"fern", "field", "finch", "fjord", "forest", "garden", "glade", "grove",
	"harbor", "hazel", "heather", "hollow", "honey", "island", "ivy", "juniper",
	"lagoon", "laurel", "leaf", "lilac", "linen", "lotus", "maple", "marsh",
	"meadow", "mercury", "mint", "mist", "monarch", "morning", "moss", "nectar",
	"oasis", "olive", "orchid", "otter", "pebble", "pine", "prairie", "quartz",
	"rain", "raven", "reef", "ripple", "river", "robin", "rose", "sage",
	"shadow", "shore", "silver", "sky", "solstice", "sparrow", "spring", "star",
	"stone", "stream", "summer", "sunset", "thistle", "timber", "trill", "valley",
	"velvet", "violet", "willow", "winter", "wren",
}

func ensurePublicSlug(db *sql.DB, userID int64, currentSlug string) (string, error) {
	if strings.TrimSpace(currentSlug) != "" {
		return currentSlug, nil
	}

	return assignPublicSlug(db, userID, true)
}

func rotatePublicSlug(db *sql.DB, userID int64) (string, error) {
	return assignPublicSlug(db, userID, false)
}

func assignPublicSlug(db *sql.DB, userID int64, onlyIfBlank bool) (string, error) {
	condition := ""
	if onlyIfBlank {
		condition = " AND public_slug=''"
	}

	for attempt := 0; attempt < 20; attempt++ {
		slug, err := randomPublicSlug()
		if err != nil {
			return "", err
		}

		res, err := db.Exec(`UPDATE users SET public_slug=? WHERE id=?`+condition, slug, userID)
		if err != nil {
			errText := strings.ToLower(err.Error())
			if strings.Contains(errText, "unique") {
				continue
			}
			return "", err
		}

		rows, err := res.RowsAffected()
		if err != nil {
			return "", err
		}
		if rows > 0 {
			return slug, nil
		}

		var existing string
		err = db.QueryRow(`SELECT public_slug FROM users WHERE id=?`, userID).Scan(&existing)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(existing) != "" {
			return existing, nil
		}
	}

	return "", fmt.Errorf("failed to allocate public slug")
}

func randomPublicSlug() (string, error) {
	parts := make([]string, 3)
	for i := range parts {
		word, err := randomSlugWord()
		if err != nil {
			return "", err
		}
		parts[i] = word
	}
	return strings.Join(parts, "-"), nil
}

func randomSlugWord() (string, error) {
	max := big.NewInt(int64(len(publicSlugWords)))
	n, err := crand.Int(crand.Reader, max)
	if err != nil {
		return "", err
	}
	return publicSlugWords[n.Int64()], nil
}
