package main

import (
	"errors"
	"testing"
)

func TestGitVersionInfoPrefersLatestTagAndDetectsDirty(t *testing.T) {
	tag, dirty := gitVersionInfo(func(name string, args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "tag" {
			return "v0.3.0\nv0.2.0\n", nil
		}
		if len(args) >= 1 && args[0] == "status" {
			return " M README.md\n", nil
		}
		return "", errors.New("unexpected command")
	})

	if tag != "v0.3.0" {
		t.Fatalf("expected latest tag v0.3.0, got %q", tag)
	}
	if !dirty {
		t.Fatal("expected dirty worktree")
	}
}

func TestGitVersionInfoFallsBackWithoutTag(t *testing.T) {
	tag, dirty := gitVersionInfo(func(name string, args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "tag" {
			return "", nil
		}
		if len(args) >= 1 && args[0] == "status" {
			return "", nil
		}
		return "", errors.New("unexpected command")
	})

	if tag != "" {
		t.Fatalf("expected no tag, got %q", tag)
	}
	if dirty {
		t.Fatal("expected clean worktree")
	}
}
