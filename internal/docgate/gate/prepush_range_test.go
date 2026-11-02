package gate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// TestPushRangeIgnoresSharedCloneTreeState pins #357's acceptance: a history
// check judging the PUSHED RANGE (merge-base..sha) neither fails on another
// session's uncommitted working-tree state (the false positive) nor lets a
// violating committed range hide behind a clean tree (the false negative's
// complement — tree scope would attribute it identically, but range scope is
// what makes the verdict about the push and nothing else).
func TestPushRangeIgnoresSharedCloneTreeState(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Defaults()

	run("init", "-q", "-b", "main")
	write("README.md", "seed\n")
	write(".pyxis/skills/prd-review/SKILL.md", "tracked seed content\n")
	run("add", ".")
	run("commit", "-qm", "seed")
	base := run("rev-parse", "HEAD")

	// The pushed range: ONE clean doc commit — a PRD ledger entry alone.
	run("checkout", "-qb", "docs-branch")
	write("docs/prds/demo/000-base.md", "# demo\n")
	run("add", "docs/prds/demo/000-base.md")
	run("commit", "-qm", "docs: demo ledger")
	head := run("rev-parse", "HEAD")

	// Another session's uncommitted WIP dirties the shared clone: a TRACKED
	// capability-path file modified in the tree — the exact #357 trigger.
	write(".pyxis/skills/prd-review/SKILL.md", "another session's uncommitted edit\n")

	t.Run("tree scope false-positives (the bug)", func(t *testing.T) {
		defects := gate.CheckStandaloneDocs(root, base, "", cfg)
		if len(defects) == 0 {
			t.Fatal("precondition lost: the tree-scoped read no longer sees the shared-clone WIP — the fixture is stale")
		}
	})
	t.Run("range scope passes the clean push (the fix)", func(t *testing.T) {
		defects := gate.CheckStandaloneDocs(root, base, head, cfg)
		if len(defects) != 0 {
			t.Fatalf("clean pushed range failed on tree state: %+v", defects)
		}
	})

	// A violating COMMITTED range still fails with a clean tree: the same
	// branch gains a code commit bundling with the ledger entry.
	write("cmd/demo/main.go", "package main\n")
	run("add", "cmd/demo/main.go")
	run("commit", "-qm", "code: bundled implementation")
	head2 := run("rev-parse", "HEAD")
	run("checkout", "-q", "--", ".") // tree clean

	t.Run("violating committed range fails regardless of tree", func(t *testing.T) {
		defects := gate.CheckStandaloneDocs(root, base, head2, cfg)
		if len(defects) == 0 {
			t.Fatal("a committed doc+code bundle escaped the range-scoped check")
		}
	})
}

func TestParsePushRefs(t *testing.T) {
	const z = "0000000000000000000000000000000000000000"
	const a = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const b = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	cases := []struct {
		name     string
		in       string
		heads    []string
		deletion bool
	}{
		{"single push", "refs/heads/x " + a + " refs/heads/x " + z + "\n", []string{a}, false},
		{"deletion only", "(delete) " + z + " refs/heads/x " + a + "\n", nil, true},
		{"mixed keeps the pushed sha", "(delete) " + z + " refs/heads/x " + a + "\nrefs/heads/y " + b + " refs/heads/y " + z + "\n", []string{b}, false},
		{"duplicate shas dedupe", "refs/heads/x " + a + " refs/heads/x " + z + "\nrefs/tags/v " + a + " refs/tags/v " + z + "\n", []string{a}, false},
		{"empty input fails closed", "", nil, false},
		{"malformed line fails closed", "garbage\n", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			heads, deletion := gate.ParsePushRefs(strings.NewReader(tc.in))
			if deletion != tc.deletion {
				t.Fatalf("deletionOnly = %v, want %v", deletion, tc.deletion)
			}
			if strings.Join(heads, ",") != strings.Join(tc.heads, ",") {
				t.Fatalf("heads = %v, want %v", heads, tc.heads)
			}
		})
	}
}
