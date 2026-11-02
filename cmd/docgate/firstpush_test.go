package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrePushRefusesMissingMainlineDespiteEpoch(t *testing.T) {
	// This fixture models a local push. Hosted CI exports a proposal base for
	// its own checkout; it must not turn the fixture into a hosted push.
	t.Setenv("PYXIS_CI_BASE", "")
	root := docgateRoot(t)
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-b", "main")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.invalid")
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "product-vision.md"), []byte("Baseline governance.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "tech-design"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "tech-design", ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"gen-index"}) }); code != 0 {
		t.Fatalf("seed design index: code=%d stderr=%q", code, stderr)
	}
	git("add", "--", ".pyxis/config.yaml", "docs/product-vision.md", "docs/tech-design/.gitkeep", "docs/tech-design/index.md", "docs/tech-design/index.json")
	git("commit", "-m", "pyxis init — converged tree")
	epoch := git("rev-parse", "HEAD")
	configBody := "schema_version: pyxis-config/v2\nepoch:\n  commit: \"" + epoch + "\"\n"
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "--", ".pyxis/config.yaml")
	git("commit", "-m", "pyxis init — epoch")
	// Model already-landed history that must not be re-gated when creating a
	// secondary remote. The configured mainline below already includes it.
	if err := os.WriteFile(filepath.Join(root, "docs", "product-vision.md"), []byte("Baseline governance.\nHistorical governance addition.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "--", "docs/product-vision.md")
	git("commit", "-m", "historical governance addition")
	head := git("rev-parse", "HEAD")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "refs/heads/main "+head+" refs/heads/main "+strings.Repeat("0", 40)+"\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"pre-push"}) })
	if code == 0 {
		t.Fatalf("ordinary push without a remote baseline must refuse: stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "origin/main") {
		t.Fatalf("missing-mainline refusal must name the configured baseline: code=%d stderr=%q", code, stderr)
	}
	// A mirror creation with an all-zero destination uses the configured base.
	git("update-ref", "refs/remotes/origin/main", head)
	r2, w2, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r2.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := io.WriteString(w2, "refs/heads/main "+head+" refs/heads/main "+strings.Repeat("0", 40)+"\n"); err != nil {
		t.Fatal(err)
	}
	if err := w2.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = r2
	code, _, stderr = captureDocgateOutput(t, func() int { return run([]string{"pre-push"}) })
	if code != 0 {
		t.Fatalf("mirror creation must use origin/main: code=%d stderr=%q", code, stderr)
	}
	// A forged public init subject cannot authenticate a base for a fresh repo.
	git("update-ref", "-d", "refs/remotes/origin/main")
	for _, configuredEpoch := range []string{"HEAD", epoch} {
		configBody = "schema_version: pyxis-config/v2\nepoch:\n  commit: \"" + configuredEpoch + "\"\n"
		if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(configBody), 0o644); err != nil {
			t.Fatal(err)
		}
		r3, w3, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w3, "refs/heads/main "+head+" refs/heads/main "+strings.Repeat("0", 40)+"\n"); err != nil {
			t.Fatal(err)
		}
		if err := w3.Close(); err != nil {
			t.Fatal(err)
		}
		os.Stdin = r3
		code, _, stderr = captureDocgateOutput(t, func() int { return run([]string{"pre-push"}) })
		if err := r3.Close(); err != nil {
			t.Fatal(err)
		}
		if code == 0 {
			t.Fatalf("forged epoch %q bypassed the gate: %q", configuredEpoch, stderr)
		}
	}
}
