package main

import (
	"os"
	"strings"
	"testing"
)

func parseIgnoreEntries(content string) []string {
	var entries []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			entries = append(entries, line)
		}
	}
	return entries
}

func TestDockerignoreExcludesEnvFile(t *testing.T) {
	data, err := os.ReadFile(".dockerignore")
	if err != nil {
		t.Fatalf("failed to read .dockerignore: %v", err)
	}

	entries := parseIgnoreEntries(string(data))
	found := false
	for _, e := range entries {
		if e == ".env" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal(".dockerignore must exclude .env to prevent leaking secrets into the container image")
	}
}
