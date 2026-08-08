package main

import (
	crand "crypto/rand"
	"math/big"
	"strings"
)

// shareGroupSlugLength is chosen for >=128 bits of entropy: log2(36) per
// char * 26 chars ≈ 134.4 bits. Public share group pages are
// private-by-link (X-Robots-Tag: noindex, and can contain personal
// countdown data an owner doesn't want guessable) — the slug is the only
// access control, so it has to resist online brute-force, not just be
// unlisted. The previous scheme (3 readable words + a 5-char suffix) was
// ~46 bits, guessable at scale; opaque and high-entropy replaces it
// entirely rather than appending to it, since a long random suffix already
// defeats the readable words' only purpose (memorability).
const shareGroupSlugLength = 26

func randomShareGroupSlug() (string, error) {
	return randomBase36String(shareGroupSlugLength)
}

func randomBase36String(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var builder strings.Builder
	builder.Grow(length)

	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < length; i++ {
		n, err := crand.Int(crand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[n.Int64()])
	}

	return builder.String(), nil
}
