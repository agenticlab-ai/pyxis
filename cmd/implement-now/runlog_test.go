package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRefusesUnavailableExecutionLog(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte("executor:\n  logging:\n    directory: logs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "logs"), []byte("existing file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--repo", repo, "--repo-slug", "owner/repo", "--scope", "demo"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "open execution log: create implement-now log directory") {
		t.Fatalf("run = %d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed log open announced a run: %q", stdout.String())
	}
	if data, err := os.ReadFile(filepath.Join(repo, "logs")); err != nil || string(data) != "existing file\n" {
		t.Fatalf("existing file was changed: %q, %v", data, err)
	}
}
