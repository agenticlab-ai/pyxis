package attribution

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestPositiveHumanAttestationReportsGitFailure(t *testing.T) {
	if _, err := positivelyHumanAttested(t.TempDir(), "missing"); err == nil {
		t.Fatal("positivelyHumanAttested accepted a non-repository")
	}
}

// dcfg is the default configuration the attribution tests classify under
// (today's doc-path roots); relocation is covered by the boundary's own
// TestClassify_RelocatedDocRoots and the gate-level relocation tests.
var dcfg = config.Defaults()

// The authority contract and a governance document — the two fixed,
// scope-independent protected classes both the Argus and the underivable
// branches must refuse (DINV-1).
const (
	authorityContract  = ".pyxis/skills/argus/authority-contract.md"
	governanceDocument = "docs/product-vision.md"
	inScopeDoc         = "docs/problems/widget/delivery.md" // widget's own subtree
	outOfScopeCode     = "internal/engine/core.go"
)

// firstPRResolver is the author-independent scope for a brand-new problem's
// first PR — the slug-derived fixed grammar (no code path, no same-PR input).
func firstPRResolver(slug string) *boundary.Scope { return boundary.ResolveFirstPR(dcfg, slug) }

// TestArgusAuthoredProtectedRefused_HumanNot is A-1: an Argus-authored diff
// touching each protected-path class is refused with a reason naming the path
// and the boundary; a human-authored diff to the same paths is not refused.
func TestArgusAuthoredProtectedRefused_HumanNot(t *testing.T) {
	protected := []string{authorityContract, governanceDocument, outOfScopeCode}

	// Argus-authored (driving problem "widget", first PR) — every protected
	// target refuses, and the refusal names the path + the boundary.
	var argusChanges []Change
	for _, p := range protected {
		argusChanges = append(argusChanges, Change{Path: p, Author: Argus, Slug: "widget"})
	}
	refusals := Refusals(dcfg, argusChanges, firstPRResolver)
	if len(refusals) != len(protected) {
		t.Fatalf("Argus-authored protected diffs: got %d refusals, want %d (%v)", len(refusals), len(protected), refusals)
	}
	for _, d := range refusals {
		if d.Verb != Verb {
			t.Errorf("refusal verb = %q, want %q", d.Verb, Verb)
		}
		if !strings.Contains(d.Reason, d.File) {
			t.Errorf("reason %q does not name the path %q", d.Reason, d.File)
		}
		if d.Op == "" || strings.Contains(d.Reason, "in scope") {
			t.Errorf("refusal for %q names no boundary (op=%q reason=%q)", d.File, d.Op, d.Reason)
		}
		if d.Remedy == "" {
			t.Errorf("refusal for %q carries no remedy", d.File)
		}
	}

	// Human-authored diffs to the exact same paths — never refused (NR-4/DINV-4).
	var humanChanges []Change
	for _, p := range protected {
		humanChanges = append(humanChanges, Change{Path: p, Author: Human})
	}
	if refusals := Refusals(dcfg, humanChanges, firstPRResolver); len(refusals) != 0 {
		t.Fatalf("human-authored diffs to protected paths were refused: %v", refusals)
	}
}

// TestArgusBoundaryClassesNamed asserts each protected class surfaces its own
// distinct boundary op token, so a refusal names which boundary it hit.
func TestArgusBoundaryClassesNamed(t *testing.T) {
	cases := []struct {
		path   string
		wantOp string
	}{
		{authorityContract, "authority-contract"},
		{governanceDocument, "governance-document"},
		{"docs/problems/other/delivery.md", "other-problem-ledger"},
		{outOfScopeCode, "out-of-scope"},
	}
	for _, c := range cases {
		refusals := Refusals(dcfg, []Change{{Path: c.path, Author: Argus, Slug: "widget"}}, firstPRResolver)
		if len(refusals) != 1 {
			t.Fatalf("%s: got %d refusals, want 1", c.path, len(refusals))
		}
		if refusals[0].Op != c.wantOp {
			t.Errorf("%s: op = %q, want %q", c.path, refusals[0].Op, c.wantOp)
		}
	}
}

// TestUnderivableFailsClosed_ProtectedSubset is A-2: an underivable diff
// touching a fixed protected class is refused (fail closed); a determinable
// human diff to the same path is not; a non-protected-path underivable diff is
// not refused.
func TestUnderivableFailsClosed_ProtectedSubset(t *testing.T) {
	// Underivable ∧ fixed protected class ⇒ refuse.
	for _, p := range []string{authorityContract, governanceDocument} {
		refusals := Refusals(dcfg, []Change{{Path: p, Author: Underivable}}, firstPRResolver)
		if len(refusals) != 1 {
			t.Fatalf("underivable protected %q: got %d refusals, want 1", p, len(refusals))
		}
		if !strings.Contains(refusals[0].Reason, "underivable") {
			t.Errorf("underivable refusal reason %q does not name the underivable-author case", refusals[0].Reason)
		}
	}

	// A determinable human diff to the same protected path — not refused.
	if r := Refusals(dcfg, []Change{{Path: authorityContract, Author: Human}}, firstPRResolver); len(r) != 0 {
		t.Fatalf("determinable human diff to a protected path was refused: %v", r)
	}

	// A non-protected-path underivable diff — not refused (the fail-closed rule
	// is scoped to the protected subset).
	if r := Refusals(dcfg, []Change{{Path: outOfScopeCode, Author: Underivable}}, firstPRResolver); len(r) != 0 {
		t.Fatalf("non-protected-path underivable diff was refused: %v", r)
	}
	// An underivable diff to an out-of-scope-*looking* doc is likewise not
	// refused: without a resolved drive the gate cannot judge scope, and it
	// refuses only the fixed classes.
	if r := Refusals(dcfg, []Change{{Path: "docs/tech-design/unrelated.md", Author: Underivable}}, firstPRResolver); len(r) != 0 {
		t.Fatalf("underivable non-fixed-class diff was refused: %v", r)
	}
}

// TestHumanPathOpen is A-5/NR-4: on a corpus of human-authored diffs to every
// protected path, zero are refused.
func TestHumanPathOpen(t *testing.T) {
	var changes []Change
	for _, p := range []string{
		authorityContract, governanceDocument,
		"docs/problems/other/delivery.md", outOfScopeCode,
		inScopeDoc, "docs/architecture-principles.md",
	} {
		changes = append(changes, Change{Path: p, Author: Human})
	}
	if r := Refusals(dcfg, changes, firstPRResolver); len(r) != 0 {
		t.Fatalf("human-author path not open — refused %d diffs: %v", len(r), r)
	}
}

// TestInScopeArgusNotRefused confirms an Argus-authored diff to its drive's own
// in-scope path is not refused (no false refusal — R3).
func TestInScopeArgusNotRefused(t *testing.T) {
	changes := []Change{
		{Path: inScopeDoc, Author: Argus, Slug: "widget"},
		{Path: "docs/prds/widget/000-base.md", Author: Argus, Slug: "widget"},
	}
	if r := Refusals(dcfg, changes, firstPRResolver); len(r) != 0 {
		t.Fatalf("in-scope Argus diffs were refused: %v", r)
	}
}

// committedLedger is a minimal parseable committed delivery ledger for problem
// "broad" whose merged journal owns one doc but NOT internal/engine/core.go.
const committedLedger = `# Broad — delivery ledger

| Field | Value |
|---|---|
| **Slug** | broad |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | tech-design |
| **Open items** | — |
| **Park state** | — |
| **Cumulative spend** | unavailable |

## Journal

### E-1 — ruling-recorded — authored the design

- **Date**: 2026-10-06
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)
- **External effect**: docs/tech-design/broad.md
- **Body**: Authored the design.
`

// TestWidenYourOwnScopeDoesNotReclassify is A-4/DINV-6: an Argus-authored
// out-of-scope write whose trailer names a broader problem slug does not flip
// to in-scope — the width comes from that problem's committed ledger, which the
// current drive cannot retroactively widen.
func TestWidenYourOwnScopeDoesNotReclassify(t *testing.T) {
	resolve := func(slug string) *boundary.Scope {
		if slug == "broad" {
			sc, _ := boundary.ResolveCommittedLedger(dcfg, "broad", []byte(committedLedger))
			return sc
		}
		return boundary.ResolveFirstPR(dcfg, slug)
	}

	// The drive names the broad problem, but its committed ledger owns only
	// docs/tech-design/broad.md — a code target reds as out-of-scope.
	refusals := Refusals(dcfg, []Change{{Path: outOfScopeCode, Author: Argus, Slug: "broad"}}, resolve)
	if len(refusals) != 1 || refusals[0].Op != "out-of-scope" {
		t.Fatalf("widen-your-own-scope was not refused as out-of-scope: %v", refusals)
	}

	// The one path the broad problem's committed ledger genuinely owns is in
	// scope — a legitimate drive of that problem authored it.
	if r := Refusals(dcfg, []Change{{Path: "docs/tech-design/broad.md", Author: Argus, Slug: "broad"}}, resolve); len(r) != 0 {
		t.Fatalf("a committed-ledger-owned path was refused: %v", r)
	}
}

// TestUnresolvableScopeArgusFailsClosed is the AP-17 fail-closed rule for an
// Argus diff whose scope cannot be resolved (a malformed committed ledger):
// nothing is provably in-scope, so it refuses.
func TestUnresolvableScopeArgusFailsClosed(t *testing.T) {
	resolve := func(string) *boundary.Scope { return nil } // unresolvable
	refusals := Refusals(dcfg, []Change{{Path: outOfScopeCode, Author: Argus, Slug: "widget"}}, resolve)
	if len(refusals) != 1 || refusals[0].Op != "unresolvable-scope" {
		t.Fatalf("unresolvable-scope Argus diff not failed closed: %v", refusals)
	}
}

// TestFirstPRMaliciousSelfAttributionReds is A-7(i)/(ii): on a first PR, an
// Argus-authored out-of-scope code target reds even when the drive names a
// broader slug — the slug-derived first-PR grammar admits only that slug's own
// doc subtrees and no code path (a same-PR self-attribution the trailer/ledger
// might carry is never consulted by the resolver). A-7(iii): the legitimate
// slug-subtree writes pass.
func TestFirstPRMaliciousSelfAttributionReds(t *testing.T) {
	// (i) own slug, out-of-scope code target → refused.
	if r := Refusals(dcfg, []Change{{Path: outOfScopeCode, Author: Argus, Slug: "newthing"}}, firstPRResolver); len(r) != 1 || r[0].Op != "out-of-scope" {
		t.Fatalf("first-PR out-of-scope code target not refused: %v", r)
	}
	// (ii) a broader slug still buys only that slug's own doc subtree — a code
	// target reds regardless of the slug named.
	if r := Refusals(dcfg, []Change{{Path: outOfScopeCode, Author: Argus, Slug: "argus-boundary-enforcement"}}, firstPRResolver); len(r) != 1 {
		t.Fatalf("broader-slug first-PR code target not refused: %v", r)
	}
	// (iii) the legitimate first-PR footprint — the slug's own doc subtrees and
	// the named allowlist — passes.
	legit := []Change{
		{Path: "docs/problems/newthing/problem.md", Author: Argus, Slug: "newthing"},
		{Path: "docs/prds/newthing/000-base.md", Author: Argus, Slug: "newthing"},
		{Path: "docs/brainstorms/newthing.md", Author: Argus, Slug: "newthing"},
		{Path: "docs/reviews/newthing-000.md", Author: Argus, Slug: "newthing"},
		{Path: "docs/wiki/glossary.md", Author: Argus, Slug: "newthing"},
	}
	if r := Refusals(dcfg, legit, firstPRResolver); len(r) != 0 {
		t.Fatalf("legitimate first-PR slug-subtree writes were refused: %v", r)
	}
	// The authority-contract and governance classes still refuse on a first PR
	// (fixed path sets, DINV-1).
	if r := Refusals(dcfg, []Change{{Path: authorityContract, Author: Argus, Slug: "newthing"}}, firstPRResolver); len(r) != 1 {
		t.Fatalf("first-PR authority-contract write not refused: %v", r)
	}
}

// TestPlantedDirectiveDoesNotSteer is ABUSE-2: the verdict is structural
// (authorship + path classification), so a planted directive carried in the
// Change's fields cannot steer it — an Argus out-of-scope write is refused
// whatever prose rides along, and a human diff is never refused.
func TestPlantedDirectiveDoesNotSteer(t *testing.T) {
	// The core reads only Path/Author/Slug; there is no prose channel to steer
	// it. A malicious slug is just another problem name, bounded by its resolver.
	if r := Refusals(dcfg, []Change{{Path: outOfScopeCode, Author: Argus, Slug: "../../etc"}}, firstPRResolver); len(r) != 1 {
		t.Fatalf("a malformed/hostile slug did not still refuse the out-of-scope write: %v", r)
	}
}
