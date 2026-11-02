package ghcodehost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
)

func TestSelectedCredentialsReachEveryCodeHostTransport(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n[ \"$GH_HOST\" = github.com ] && [ \"$GH_TOKEN\" = code-actor ] || exit 2\nprintf code-actor\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TEST_SELECTED_CODE", "code-actor")
	t.Setenv("GH_TOKEN", "ambient")
	t.Setenv("GH_HOST", "wrong.example")
	for _, b := range []*budget.Budget{nil, {}} {
		h := NewGHWithTokenEnv("o/code", "main", "TEST_SELECTED_CODE", b, "github.com")
		c := h.gh.(*ghClient)
		if c.runLog == nil {
			t.Fatal("selected credential transport omitted bounded log retrieval")
		}
		if c.budget == nil || c.budgetErr != nil || (b != nil && c.budget != b) {
			t.Fatalf("selected budget was not retained: %v", c.budgetErr)
		}
		if out, err := c.run(t.Context(), "api", "user"); err != nil || string(out) != "code-actor" {
			t.Fatalf("selected read %s %v", out, err)
		}
		if out, err := c.runInput(t.Context(), []byte("payload"), "api", "write"); err != nil || string(out) != "code-actor" {
			t.Fatalf("selected write %s %v", out, err)
		}
		if out, err := c.runLog(t.Context(), 20, "run", "view"); err != nil || string(out) != "code-actor" {
			t.Fatalf("selected bounded log %s %v", out, err)
		}
	}
	reader := NewStateReaderWithTokenEnv("o/code", "TEST_SELECTED_CODE", "github.com")
	if out, err := reader.client.Query(t.Context(), "query", nil); err != nil || string(out) != "code-actor" {
		t.Fatalf("selected GraphQL read %s %v", out, err)
	}
	t.Setenv("TEST_SELECTED_CODE", "")
	if _, err := NewGHWithTokenEnv("o/code", "main", "TEST_SELECTED_CODE", &budget.Budget{}, "github.com").gh.(*ghClient).runLog(t.Context(), 20, "run", "view"); err == nil || !strings.Contains(err.Error(), "TEST_SELECTED_CODE") {
		t.Fatal("bounded log retrieval fell back from a missing selected credential")
	}
	if _, err := reader.client.Query(t.Context(), "query", nil); err == nil || !strings.Contains(err.Error(), "TEST_SELECTED_CODE") {
		t.Fatal("missing selected credential fell back to ambient")
	}
	for _, b := range []*budget.Budget{nil, {}} {
		legacy := NewGHWithTokenEnv("o/code", "main", "", b)
		if c := legacy.gh.(*ghClient); b != nil && c.budget != b {
			t.Fatal("legacy budget changed")
		}
	}
	if r := NewStateReaderWithTokenEnv("o/code", ""); r.client.(graphqlShell).host != "" {
		t.Fatal("legacy host was pinned")
	}
}
