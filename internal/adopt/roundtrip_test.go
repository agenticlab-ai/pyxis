package adopt_test

// The round-trip acceptance harness (existing-repo-onboarding/001 T11; the
// basis's M1/DOD-1): a hermetic end-to-end proof that adopting a repository
// carrying EVERY collision at once — an existing root convention file, a
// hook-manager `core.hooksPath`, a name-colliding workflow, a non-conforming
// corpus, and a non-Go stack — never touches what the repo owns (ABUSE-1,
// NFR-2, A-2), reverses byte-for-byte (A-7), and reaches gate-green over its
// grandfathered corpus. The empty-repo install and converge steady-state
// suites (NR-1/NR-2) are proven unchanged by their own suites, which the full
// test run exercises; the adopt-side inspect dispatch already asserts the
// empty tree routes to init. Reuses the apply/undo/inspect fixture scaffolding.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// danglingCorpusDoc is the non-conforming corpus: a relative link to a file
// that does not exist — a check-links defect the gate would red on if the
// pre-existing corpus were not grandfathered by the install epoch.
const danglingCorpusDoc = "# legacy design\n\nsee [the old plan](does-not-exist.md)\n"

// allCollisionsRepo git-initialises, at a pre-adoption commit, a repository
// carrying every collision the design's round-trip fixture names at once.
func allCollisionsRepo(t *testing.T) string {
	t.Helper()
	isolateGit(t)
	root := t.TempDir()
	git(t, root, "init", "-b", "main")

	// The adopter's own content (never-touch), including a non-Go stack marker
	// so adoption is proven not to assume Go.
	writeFile(t, filepath.Join(root, "README.md"), []byte("# their project\n"))
	writeFile(t, filepath.Join(root, "src", "index.js"), []byte("console.log('hi')\n"))
	writeFile(t, filepath.Join(root, "package.json"), []byte("{ \"name\": \"theirs\" }\n"))
	// An existing root convention file the engine must never rewrite (FR-6).
	writeFile(t, filepath.Join(root, "AGENTS.md"), []byte("# their agents manual\n"))
	// A hook-manager owning core.hooksPath (FR-4).
	writeFile(t, filepath.Join(root, ".husky", "pre-push"), []byte("#!/bin/sh\necho their-hook\n"))
	// A workflow colliding with the engine's fixed target (FR-5).
	writeFile(t, filepath.Join(root, ".github", "workflows", "docs-gate.yml"), []byte("name: their-ci\non: pull_request\n"))
	// A non-conforming corpus (grandfathered by the epoch, FR-8).
	writeFile(t, filepath.Join(root, "docs", "legacy", "old.md"), []byte(danglingCorpusDoc))

	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "pre-adoption state")
	git(t, root, "config", "--local", "core.hooksPath", ".husky")
	return root
}

func headOf(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
}

// ---- ABUSE-1 / A-2 / A-7: never-touch byte-identity + the round trip -------

func TestRoundTripAllCollisionsNeverTouchAndReversal(t *testing.T) {
	r := newReleaseWithArchive(t, adoptFilesWithWorkflow())
	root := allCollisionsRepo(t)

	// The pre-adoption never-touch set and commit.
	const (
		agents   = "AGENTS.md"
		husky    = ".husky/pre-push"
		theirWF  = ".github/workflows/docs-gate.yml"
		corpus   = "docs/legacy/old.md"
		theirHk  = "#!/bin/sh\necho their-hook\n"
		theirWFB = "name: their-ci\non: pull_request\n"
	)
	pre := snapshot(t, root)
	preHead := headOf(t, root)

	draftPlan(t, root, r)
	if _, err := adopt.Apply(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatalf("Apply over the all-collisions fixture: %v", err)
	}

	// ABUSE-1 / NFR-2, git-native: apply modified NO tracked file — its footprint
	// is new (untracked) paths only, so the diff against HEAD is empty.
	if d := strings.TrimSpace(git(t, root, "diff", "--name-only", "HEAD")); d != "" {
		t.Fatalf("apply modified tracked (owned) content — the never-touch guarantee is broken:\n%s", d)
	}
	// The owned collision files are byte-identical, and the engine's workflow
	// landed under its distinct name rather than over the adopter's.
	if got := readTree(t, root, agents); string(got) != "# their agents manual\n" {
		t.Errorf("root convention file mutated: %q", got)
	}
	if got := readTree(t, root, husky); string(got) != theirHk {
		t.Errorf("hook-manager hook mutated: %q", got)
	}
	if got := readTree(t, root, theirWF); string(got) != theirWFB {
		t.Errorf("colliding workflow mutated: %q", got)
	}
	if got := readTree(t, root, corpus); string(got) != danglingCorpusDoc {
		t.Errorf("pre-existing corpus mutated: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(adopt.DistinctWorkflowPath()))); err != nil {
		t.Errorf("engine workflow not landed at the distinct path %s: %v", adopt.DistinctWorkflowPath(), err)
	}
	// The hook chained ahead of the adopter's path.
	if got, _ := gitGet(t, root, "core.hooksPath"); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks (engine chained ahead)", got)
	}
	if got, _ := gitGet(t, root, "pyxis.chainedhookspath"); got != ".husky" {
		t.Errorf("chain marker = %q, want .husky", got)
	}

	// A-7: undo restores the whole tree byte-identical to its pre-adoption state,
	// with HEAD unchanged and the prior hook path restored.
	if _, err := adopt.Undo(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if post := snapshot(t, root); !mapsEqual(pre, post) {
		t.Fatalf("undo did not restore the pre-adoption tree:\npre  %v\npost %v", pre, post)
	}
	if h := headOf(t, root); h != preHead {
		t.Errorf("HEAD changed across the round trip: %s → %s", preHead, h)
	}
	if got, _ := gitGet(t, root, "core.hooksPath"); got != ".husky" {
		t.Errorf("core.hooksPath after undo = %q, want the restored prior .husky", got)
	}
	if _, set := gitGet(t, root, "pyxis.chainedhookspath"); set {
		t.Error("chain marker still set after undo")
	}
	// git-native: the working tree carries no tracked change versus HEAD.
	if d := strings.TrimSpace(git(t, root, "diff", "--name-only", "HEAD")); d != "" {
		t.Fatalf("tracked content diverges from HEAD after undo:\n%s", d)
	}
}

// ---- FR-8 / A-9: the grandfathered corpus reaches gate-green ---------------

func TestAllCollisionsAdoptsToGateGreen(t *testing.T) {
	r := newReleaseWithArchive(t, adoptFilesWithWorkflow())
	root := allCollisionsRepo(t)

	// Sanity: the corpus IS non-conforming — without an epoch the link check reds.
	if d := gate.CheckLinks(root, config.Defaults(), gate.ScanScope{}); len(d) == 0 {
		t.Fatal("fixture corpus is not actually non-conforming — the grandfather assertion would be vacuous")
	}

	draftPlan(t, root, r)
	if _, err := adopt.Apply(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// The adopted config carries the install epoch apply recorded.
	adopted, defects := config.Load(root)
	if len(defects) > 0 {
		t.Fatalf("adopted config does not parse: %v", defects)
	}
	if adopted.Epoch.Commit == "" {
		t.Fatal("apply recorded no epoch — grandfathering cannot engage")
	}

	// Adopted: the gate scopes by the epoch, so the non-conforming pre-existing
	// corpus is grandfathered and the corpus checks are green.
	for _, d := range gate.Gate(root, "", nil, adopted) {
		if d.Verb == "check-links" || d.Verb == "check-citations" {
			t.Errorf("adopted repo not gate-green — the pre-existing corpus was not grandfathered: %+v", d)
		}
	}
}
