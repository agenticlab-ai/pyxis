package e2e

// Leg 6 — the removal test (A-10; design §3.9 leg 2, the substrate's inherited
// invariant). Deleting the system tree (.pyxis/) and the pointers (the
// operand's .github/workflows/docs-gate.yml) leaves the operand's OWN content —
// bucket-1: the seeded governance docs, the operand manual, the registries, the
// activated hooks, and the carina work-item store — complete and readable. The
// factory owns none of that content; a user who removes pyxis keeps their repo.
//
// Bucket-1 survivors (written OUTSIDE .pyxis/ by init's seed/hooks/welcome
// stages) vs the system tree (deleted). Not asserted to survive: anything under
// .pyxis/ (config, templates, operand-hook sources, the bin cache) — those ARE
// the system tree. Rollups (docs/prd.md, docs/techspec.md) must be ABSENT
// throughout (rule 3).

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemovalTest(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	// ci_mode=both (not the shared local-mode baseline): this leg needs the
	// hosted workflow pointer to actually exist so removing it is a genuine
	// proof, not a vacuous one (§3.8 — under local it is never materialized).
	repo, _ := scratchRepo(t, r, "--ci-mode", "both")

	// Capture the bucket-1 content BEFORE removal so we can prove it survives
	// byte-for-byte (not merely "exists").
	bucket1 := []string{
		"docs/product-vision.md",
		"docs/architecture-principles.md",
		"AGENTS.md",
		"docs/observability/incidents.md",
		"docs/tech-design/index.md",
		"docs/tech-design/index.json",
		".githooks/pre-commit",
		".githooks/pre-push",
		".githooks/prepare-commit-msg",
		".gitignore",
	}
	before := map[string][]byte{}
	for _, rel := range bucket1 {
		b, err := os.ReadFile(at(repo, rel))
		if err != nil {
			t.Fatalf("pre-removal: bucket-1 content %s missing: %v", rel, err)
		}
		before[rel] = b
	}
	// The carina work-item store carries the welcome item — bucket-1 too.
	itemsDir := at(repo, ".workitems/items")
	itemsBefore, err := os.ReadDir(itemsDir)
	if err != nil || len(itemsBefore) == 0 {
		t.Fatalf("pre-removal: carina store empty at %s (err %v)", itemsDir, err)
	}

	// ---- remove the system tree + the pointers --------------------------
	if err := os.RemoveAll(at(repo, ".pyxis")); err != nil {
		t.Fatalf("remove .pyxis: %v", err)
	}
	if err := os.Remove(at(repo, ".github/workflows/docs-gate.yml")); err != nil {
		t.Fatalf("remove the workflow pointer: %v", err)
	}

	// ---- bucket-1 content survives byte-for-byte + readable -------------
	for _, rel := range bucket1 {
		got, err := os.ReadFile(at(repo, rel))
		if err != nil {
			t.Errorf("post-removal: bucket-1 content %s did not survive: %v", rel, err)
			continue
		}
		if !equalBytes(got, before[rel]) {
			t.Errorf("post-removal: bucket-1 content %s changed", rel)
		}
	}
	itemsAfter, err := os.ReadDir(itemsDir)
	if err != nil || len(itemsAfter) != len(itemsBefore) {
		t.Errorf("post-removal: carina store did not survive intact at %s (err %v, %d→%d items)", itemsDir, err, len(itemsBefore), len(itemsAfter))
	}

	// ---- the system tree is gone (sanity: removal actually happened) ----
	if _, err := os.Stat(at(repo, ".pyxis")); !os.IsNotExist(err) {
		t.Errorf(".pyxis still present after removal (err %v)", err)
	}
	if _, err := os.Stat(at(repo, ".github/workflows/docs-gate.yml")); !os.IsNotExist(err) {
		t.Errorf("the workflow pointer still present after removal (err %v)", err)
	}

	// ---- rollups stayed absent throughout (rule 3) ----------------------
	for _, rollup := range []string{"docs/prd.md", "docs/techspec.md"} {
		if _, err := os.Stat(at(repo, filepath.FromSlash(rollup))); err == nil {
			t.Errorf("rollup %s exists — it must stay absent pre-first-ship (rule 3)", rollup)
		}
	}
}
