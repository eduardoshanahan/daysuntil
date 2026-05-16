package main

import (
	"os/exec"
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
	tagOutput, tagErr := run("git", "tag", "--sort=-version:refname")
	tag := ""
	if tagErr == nil {
		for _, line := range strings.Split(tagOutput, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				tag = line
				break
			}
		}
	}

	statusOutput, statusErr := run("git", "status", "--porcelain")
	dirty := statusErr == nil && strings.TrimSpace(statusOutput) != ""

	return tag, dirty
}

func execCommandOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}
