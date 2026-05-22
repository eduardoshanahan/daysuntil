package main

import (
	"errors"
	"testing"
)

func TestGitVersionInfoBuildNumber(t *testing.T) {
	tag, dirty := gitVersionInfo(func(name string, args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "describe" {
			return "v0.3.0-3-gabcdef12\n", nil
		}
		if len(args) >= 1 && args[0] == "status" {
			return " M README.md\n", nil
		}
		return "", errors.New("unexpected command")
	})

	if tag != "v0.3.0.3" {
		t.Fatalf("expected v0.3.0.3, got %q", tag)
	}
	if !dirty {
		t.Fatal("expected dirty worktree")
	}
}

func TestGitVersionInfoOnTag(t *testing.T) {
	tag, dirty := gitVersionInfo(func(name string, args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "describe" {
			return "v0.3.0-0-gabcdef12\n", nil
		}
		if len(args) >= 1 && args[0] == "status" {
			return "", nil
		}
		return "", errors.New("unexpected command")
	})

	if tag != "v0.3.0" {
		t.Fatalf("expected v0.3.0 (on tag, count=0), got %q", tag)
	}
	if dirty {
		t.Fatal("expected clean worktree")
	}
}

func TestGitVersionInfoFallsBackWithoutTag(t *testing.T) {
	tag, dirty := gitVersionInfo(func(name string, args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "describe" {
			return "", errors.New("no tags")
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
