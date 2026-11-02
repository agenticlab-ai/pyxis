package github

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectedPublicationCredentialsCoverIdentityAndWrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n[ \"$GH_HOST\" = github.com ] && [ \"$GH_TOKEN\" = reviewer ] || exit 2\nprintf reviewer\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("TEST_SELECTED_REVIEW", "reviewer")
	t.Setenv("GH_TOKEN", "ambient")
	t.Setenv("GH_HOST", "wrong.example")
	adapter, err := NewHostAdapterWithTokenEnv("o/code", "TEST_SELECTED_REVIEW", &memoryPublicationStore{}, "github.com")
	if err != nil {
		t.Fatal(err)
	}
	c := adapter.gh.(*ghClient)
	for _, input := range [][]byte{nil, []byte("publication body")} {
		if out, err := c.run(t.Context(), input, "api", "user"); err != nil || string(out) != "reviewer" {
			t.Fatalf("selected publication transport %s %v", out, err)
		}
	}
	if _, err := NewHostAdapterWithTokenEnv("o/code", "", &memoryPublicationStore{}); err != nil {
		t.Fatalf("legacy publication adapter failed: %v", err)
	}
	if _, err := NewHostAdapterWithTokenEnv("o/code", "TEST_SELECTED_REVIEW", nil); err == nil {
		t.Fatal("publication without durable store accepted")
	}
}
