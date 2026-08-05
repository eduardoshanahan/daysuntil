package main

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

var version = "dev"

var (
	versionOnce   sync.Once
	versionCached string
)

func currentVersion() string {
	versionOnce.Do(func() {
		versionCached = resolveVersion()
	})
	return versionCached
}

func resolveVersion() string {
	if injected := strings.TrimSpace(version); injected != "" && injected != "dev" {
		return injected
	}

	tag, dirty := gitVersionInfo(execCommandOutput)
	if tag == "" {
		if dirty {
			return "dev-dirty"
		}
		return "dev"
	}
	if dirty {
		return tag + "-dirty"
	}
	return tag
}

func gitVersionInfo(run func(string, ...string) (string, error)) (string, bool) {
	// --exclude matters: without it, a malformed 4-segment tag (e.g.
	// v0.2.0.25, from an earlier tagging mistake) also satisfies --match's
	// "[0-9]* is digit+wildcard" glob and can be selected as the nearest
	// tag ahead of a well-formed vX.Y.Z one, corrupting the parsed output
	// (v0.2.0.25.19 instead of v0.2.0.33). Confirmed live 2026-08-05.
	descOut, descErr := run("git", "describe", "--tags", "--long", "--match", "v[0-9]*.[0-9]*.[0-9]*", "--exclude", "v[0-9]*.[0-9]*.[0-9]*.[0-9]*")
	tag := ""
	if descErr == nil {
		tag = parseDescribeOutput(strings.TrimSpace(descOut))
	}

	statusOutput, statusErr := run("git", "status", "--porcelain")
	dirty := statusErr == nil && strings.TrimSpace(statusOutput) != ""

	return tag, dirty
}

// parseDescribeOutput converts "v0.1.0-5-gabcdef" → "v0.1.0.5", or "v0.1.0-0-gabcdef" → "v0.1.0".
func parseDescribeOutput(s string) string {
	if s == "" {
		return ""
	}
	// Strip the trailing g{sha} segment (after last '-').
	lastDash := strings.LastIndex(s, "-")
	if lastDash < 0 {
		return s
	}
	withoutSHA := s[:lastDash] // e.g. "v0.1.0-5"

	// Strip the commit count segment (after last remaining '-').
	prevDash := strings.LastIndex(withoutSHA, "-")
	if prevDash < 0 {
		return withoutSHA
	}
	base := withoutSHA[:prevDash]       // e.g. "v0.1.0"
	countStr := withoutSHA[prevDash+1:] // e.g. "5"

	count, err := strconv.Atoi(countStr)
	if err != nil || count == 0 {
		return base
	}
	return base + "." + countStr
}

func execCommandOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}
