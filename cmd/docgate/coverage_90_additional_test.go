package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func TestCoverage90FeedbackAndPrePushFailureBoundaries(t *testing.T) {
	t.Run("feedback projection write", func(t *testing.T) {
		root := docgateRoot(t)
		dir := filepath.Join(root, "docs", "feedback", "subject-alpha")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"001-conversation.md", "002-code-host.md"} {
			wire, err := os.ReadFile(filepath.Join("..", "..", "internal", "docgate", "feedback", "testdata", "subject-alpha", name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), wire, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Mkdir(filepath.Join(dir, feedback.ProjectionFile), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"fold", dir}) })
		if code != 2 || stderr == "" {
			t.Fatalf("feedback fold = %d, stderr=%q", code, stderr)
		}
	})

	t.Run("pushed head merge base", func(t *testing.T) {
		root := docgateRoot(t)
		head := initializeDocgateGit(t, root)
		t.Chdir(root)
		line := "refs/heads/main " + strings.Repeat("f", 40) + " refs/heads/main " + head + "\n"
		code, _, stderr := captureDocgateOutput(t, func() int {
			return withDocgateStdin(t, line, func() int { return run([]string{"pre-push", "--merge-base", "HEAD"}) })
		})
		if code != 1 || stderr == "" {
			t.Fatalf("pre-push = %d, stderr=%q", code, stderr)
		}
	})

	t.Run("missing feedback projection", func(t *testing.T) {
		root := docgateRoot(t)
		t.Chdir(root)
		missing := filepath.Join(root, "docs", "feedback", "missing")
		code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"verify-generated", missing}) })
		if code != 1 || stderr == "" {
			t.Fatalf("verify-generated = %d, stderr=%q", code, stderr)
		}
	})
}

func TestCoverage90SpecialDispatchAndReportEncoding(t *testing.T) {
	root := docgateRoot(t)
	t.Chdir(root)
	code, _, stderr := captureDocgateOutput(t, func() int {
		return run([]string{"context-pack", "--stage", "retired-stage", "--subject", "missing"})
	})
	if code != 1 || stderr == "" {
		t.Fatalf("unknown context pack = %d, stderr=%q", code, stderr)
	}
	code, _, stderr = captureDocgateOutput(t, func() int {
		return run([]string{"add-incident", "--date", "bad", "--tags", "one, ,two"})
	})
	if code != 1 || stderr == "" {
		t.Fatalf("add-incident = %d, stderr=%q", code, stderr)
	}

	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeEnd.Close(); err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = writeEnd
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = readEnd.Close()
	})
	defect := schema.Defect{Verb: "docgate", File: "doc.md", Op: "lint", Reason: "broken", Remedy: "repair"}
	if code := report([]schema.Defect{defect}, "-"); code != 2 {
		t.Fatalf("report to closed stdout = %d, want 2", code)
	}
	os.Stdout = oldStdout
}
