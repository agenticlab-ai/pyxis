package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleRefusesInvalidIntegrationBindingAfterValidDiff(t *testing.T) {
	root, base, head := repo(t, map[string]string{"internal/foo/foo.go": "package foo\n"})
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("integrations:\n  schema_version: wrong\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runLane(t, []string{
		"handle", "--repo", root, "--gh-repo", "owner/repo", "--pr", "7",
		"--base", base, "--head", head,
	})
	if code != 2 || stdout != "" || !strings.Contains(stderr, "lane handle:") || !strings.Contains(stderr, "schema_version") {
		t.Fatalf("invalid binding: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
