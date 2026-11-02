package relmanifest_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/release"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// repoRoot resolves the source repo root from the package directory and
// fails loudly when the authored policy is missing — the policy file is part
// of this package's deliverable.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".pyxis", "class-policy.yaml")); err != nil {
		t.Fatalf("authored class policy not found at repo root: %v", err)
	}
	return root
}

// stageRealPayload consumes the production packer's exact tracked-file set,
// ownership exclusions, reference snapshots and installed-workflow overlay.
// A second filesystem walker would test files the release does not install.
func stageRealPayload(t *testing.T, root string) string {
	t.Helper()
	dst := t.TempDir()
	if _, err := release.StagePayload(context.Background(), root, dst); err != nil {
		t.Fatalf("stage production payload: %v", err)
	}
	return dst
}

// Acceptance 3: the walk over the repo's ACTUAL payload tree with the
// authored .pyxis/class-policy.yaml classes every file — zero unclassed
// remainder — and every file lands in the class the policy's rows contract.
func TestAuthoredPolicyClassesRealPayloadTree(t *testing.T) {
	root := repoRoot(t)
	policyBytes, err := os.ReadFile(filepath.Join(root, ".pyxis", "class-policy.yaml"))
	if err != nil {
		t.Fatalf("read authored policy: %v", err)
	}
	policy, err := relmanifest.ParseClassPolicy(policyBytes)
	if err != nil {
		t.Fatalf("the authored class policy does not parse: %v", err)
	}
	staged := stageRealPayload(t, root)
	m, err := relmanifest.Walk(staged, policy, goldenMeta())
	var unclassed *relmanifest.UnclassedError
	if errors.As(err, &unclassed) {
		t.Fatalf("the authored policy leaves %d real payload file(s) unclassed (DINV-6): %v", len(unclassed.Paths), unclassed.Paths)
	}
	if err != nil {
		t.Fatalf("walk real payload tree: %v", err)
	}
	// Floor guard: the staged tree must be the real one, not an accidental
	// near-empty stage (the tracked .pyxis tree alone carries hundreds of files).
	if len(m.Payload.Files) < 100 {
		t.Fatalf("suspiciously small staged payload: %d files", len(m.Payload.Files))
	}
	for _, must := range []string{
		".pyxis/class-policy.yaml",
		".pyxis/config.yaml",
		".pyxis/pipeline.md",
		".pyxis/security/bwrap.apparmor",
		".github/workflows/docs-gate.yml",
		"docs/tech-design/index.json",
		"docs/tech-design/index.md",
	} {
		if !slices.ContainsFunc(m.Payload.Files, func(f relmanifest.PayloadFile) bool { return f.Path == must }) {
			t.Errorf("expected real payload file %q missing from the walk", must)
		}
	}
	// Every file's class and hash posture per the authored rows: the one
	// repo-owned row (config.yaml — init writes it before converge), the
	// generated indexes AND the installed CI workflow (converge renders it
	// from config — config-driven-portability-v1 §3.1, FR-4), engine-verbatim
	// for the rest (the stamp wiring surface included — a fresh install must
	// materialize it). Generated rows carry sha null; engine-verbatim rows a hash.
	for _, f := range m.Payload.Files {
		if f.Path == ".pyxis/skills/argus/authority-contract.md" {
			t.Fatal("Source's operator-owned authority entered the release manifest")
		}
		want := relmanifest.EngineVerbatim
		switch {
		case f.Path == ".pyxis/config.yaml":
			want = relmanifest.RepoOwned
		case strings.HasPrefix(f.Path, "docs/tech-design/index."):
			want = relmanifest.Generated
		case f.Path == ".github/workflows/docs-gate.yml":
			want = relmanifest.Generated
		}
		if f.Class != want {
			t.Errorf("real file %q classed %q, want %q", f.Path, f.Class, want)
		}
		if (want == relmanifest.EngineVerbatim) != (f.SHA256 != nil) {
			t.Errorf("real file %q (class %q): sha256 nullability wrong", f.Path, f.Class)
		}
	}
}
