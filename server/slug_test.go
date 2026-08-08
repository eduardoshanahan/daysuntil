package main

import (
	"regexp"
	"testing"
)

var opaqueSlugPattern = regexp.MustCompile(`^[a-z0-9]+$`)

func TestRandomShareGroupSlugHasSufficientEntropyAndShape(t *testing.T) {
	slug, err := randomShareGroupSlug()
	if err != nil {
		t.Fatalf("randomShareGroupSlug: %v", err)
	}

	if len(slug) != shareGroupSlugLength {
		t.Fatalf("expected length %d, got %d in %q", shareGroupSlugLength, len(slug), slug)
	}
	if !opaqueSlugPattern.MatchString(slug) {
		t.Fatalf("expected an opaque lowercase-base36 slug, got %q", slug)
	}

	// 26 base36 chars ≈ 134 bits — well over the 128-bit minimum the
	// review asked for (up from the previous ~46-bit readable-word scheme).
	const minBits = 128.0
	const bitsPerChar = 5.169925001 // log2(36)
	if float64(shareGroupSlugLength)*bitsPerChar < minBits {
		t.Fatalf("shareGroupSlugLength=%d is below the 128-bit minimum", shareGroupSlugLength)
	}
}

func TestRandomShareGroupSlugIsNotDeterministic(t *testing.T) {
	seen := make(map[string]bool, 50)
	for i := 0; i < 50; i++ {
		slug, err := randomShareGroupSlug()
		if err != nil {
			t.Fatalf("randomShareGroupSlug: %v", err)
		}
		if seen[slug] {
			t.Fatalf("got a duplicate slug %q across only 50 draws — entropy source looks broken", slug)
		}
		seen[slug] = true
	}
}
