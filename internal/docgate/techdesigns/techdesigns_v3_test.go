package techdesigns_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

// FR-20's three new clauses: corpus resolution, liveness at introduction,
// and the Approved-status check.
func TestServesHardening(t *testing.T) {
	// Corpus ref to a nonexistent ID → red; to a present ID → green.
	e := env()
	e.CorpusIDs = map[string]bool{"IMPL-1": true}
	bad := mk(t, "bad-corpus", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | corpus:IMPL-99 |", 1), goodBody)
	wantDefect(t, techdesigns.Lint([]*techdesigns.Design{bad}, e), "not present in the placeholder corpus")
	good := mk(t, "good-corpus", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | corpus:IMPL-1 |", 1), goodBody)
	if d := techdesigns.Lint([]*techdesigns.Design{good}, e); len(d) != 0 {
		t.Fatalf("resolving corpus ref flagged: %v", d)
	}

	// Liveness at introduction: a NEW design serving a retired unit → red;
	// the same design pre-existing (not in NewDesigns) → green (flag-only
	// territory — DINV-2 of the serves-hardening design).
	e2 := env()
	dead := mk(t, "dead-anchor", strings.Replace(goodHeader, "| **Serves** | — |", "| **Serves** | prd:docgate/FR-2 · prd:docgate/FR-1 |", 1), goodBody)
	e2.NewDesigns = map[string]bool{"dead-anchor": true}
	wantDefect(t, techdesigns.Lint([]*techdesigns.Design{dead}, e2), "new designs serve live requirements only")
	e2.NewDesigns = map[string]bool{}
	if d := techdesigns.Lint([]*techdesigns.Design{dead}, e2); len(d) != 0 {
		t.Fatalf("pre-existing design retro-red: %v", d)
	}

	// Status must record Approved (non-grandfathered).
	draft := mk(t, "draft-status", strings.Replace(goodHeader, "| **Status** | Approved |", "| **Status** | Draft v0.1 — in review |", 1), goodBody)
	wantDefect(t, techdesigns.Lint([]*techdesigns.Design{draft}, env()), "does not record Approved")
	e3 := env()
	e3.Grandfathered = map[string]bool{"draft-status": true}
	if d := techdesigns.Lint([]*techdesigns.Design{draft}, e3); len(d) != 0 {
		t.Fatalf("grandfathered draft flagged: %v", d)
	}
}
