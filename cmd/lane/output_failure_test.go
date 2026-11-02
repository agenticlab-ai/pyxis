package main

import (
	"os"
	"strings"
	"testing"
)

func TestStructuredResultWriteFailuresReturnInfrastructureFault(t *testing.T) {
	root, base, head := repo(t, map[string]string{"internal/feature.go": "package feature\n"})
	fakeGH(t, "#!/bin/sh\nprintf '[]'\n")
	for _, args := range [][]string{
		{"classify", "--repo", root, "--base", base, "--head", head},
		{"handle", "--repo", root, "--base", base, "--head", head, "--gh-repo", "owner/repo", "--pr", "7"},
		{"sweep", "--repo", root, "--gh-repo", "owner/repo"},
		{"state", "--repo", root},
	} {
		t.Run(args[0], func(t *testing.T) {
			closed, err := os.CreateTemp(t.TempDir(), "closed-output")
			if err != nil {
				t.Fatal(err)
			}
			if err := closed.Close(); err != nil {
				t.Fatal(err)
			}
			stderr, err := os.CreateTemp(t.TempDir(), "diagnostic")
			if err != nil {
				t.Fatal(err)
			}
			code := run(args, closed, stderr)
			if err := stderr.Close(); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(stderr.Name())
			if err != nil {
				t.Fatal(err)
			}
			if code != 1 || !strings.Contains(string(body), "lane: write result:") {
				t.Fatalf("lost structured result reported success: code=%d, stderr=%s", code, body)
			}
		})
	}
}
