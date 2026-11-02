package corpus

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
)

// TestEachAbuseRefusedAtExactlyItsOwningLayer is the corpus's first acceptance
// criterion: every seeded abuse fixture is refused at exactly the layer that
// owns it — the wiped-history pointer at the structure arm, the
// malicious-self-attribution and widen-slug at the attribution gate, the
// injection at the write-time layer, and the out-of-band write at the merge
// boundary. It replays each Case against its Owner core and requires a refusal.
func TestEachAbuseRefusedAtExactlyItsOwningLayer(t *testing.T) {
	// The declared owner of each abuse — asserted so the corpus documents (and
	// the test enforces) which layer each violation belongs to.
	wantOwner := map[string]Layer{
		"wiped-history self-elevated pointer":             StructureArm,
		"fabricated non-floor birth seed":                 StructureArm,
		"first-PR malicious same-PR self-attribution":     AttributionGate,
		"widen-your-own-scope broader slug":               AttributionGate,
		"injection-induced authority-contract write":      WriteHook,
		"out-of-band bypass caught at the merge boundary": AttributionGate,
	}
	seen := map[string]bool{}
	for _, c := range AbuseCorpus() {
		want, ok := wantOwner[c.Name]
		if !ok {
			t.Errorf("abuse case %q is not in the declared-owner map — a new fixture must declare its owning layer", c.Name)
			continue
		}
		seen[c.Name] = true
		if c.Owner != want {
			t.Errorf("%q: Owner = %s, want %s", c.Name, c.Owner, want)
		}
		refused, detail := c.Refused()
		if !refused {
			t.Errorf("%q: NOT refused at its owning layer %s — the abuse got through", c.Name, c.Owner)
			continue
		}
		if strings.TrimSpace(detail) == "" {
			t.Errorf("%q: refused with no reason — a refusal must name why", c.Name)
		}
	}
	for name := range wantOwner {
		if !seen[name] {
			t.Errorf("declared abuse %q is missing from AbuseCorpus()", name)
		}
	}
}

// TestHonestSplit_StructureArmSolelyOwnsStructuralLegality proves the split is
// honest for the wiped-history case: the structure arm reds the illegal contract
// regardless of author, while the attribution gate — which owns the merge
// boundary for authorship — does NOT refuse the very same wiped-history edit
// when a human authors it (the human-author path is open by design, DINV-4). So
// structural legality is owned by exactly the structure arm; it cannot be
// delegated to the attribution gate, which lets the human edit through.
func TestHonestSplit_StructureArmSolelyOwnsStructuralLegality(t *testing.T) {
	refused, d := StructureArmRefuses(WipedHistorySelfElevation)
	if !refused {
		t.Fatalf("the structure arm did not red the wiped-history self-elevation: %v", d)
	}

	// The same structurally-illegal edit, authored by a human, reaches the
	// attribution gate — which does not refuse it (the human path is open).
	humanEdit := []attribution.Change{{Path: authorityContract, Author: attribution.Human}}
	if r, dd := AttributionRefuses(humanEdit, firstPRResolver); r {
		t.Fatalf("the attribution gate refused a human-authored authority-contract edit — the human path must stay open, so it cannot own structural legality: %v", dd)
	}
}

// TestHonestSplit_MergeBoundaryBacksUpOutOfBandWrite proves the honest split for
// ABUSE-4: the write hook WOULD deny a normal write to the authority contract
// (so the fast layer works when it runs), but an out-of-band write skips the
// hook entirely — and the merge-boundary attribution gate still refuses the
// resulting Argus-authored diff. The write-time layer cannot stop an out-of-band
// write; the merge boundary backs it up.
func TestHonestSplit_MergeBoundaryBacksUpOutOfBandWrite(t *testing.T) {
	// The fast layer works when it is on the path.
	if denied, dec := WriteHookDenies(authorityContract, widgetFirstPR()); !denied {
		t.Fatalf("the write hook did not deny a normal authority-contract write: %s", dec.Reason())
	}

	// The out-of-band write never touched the hook; the merge boundary catches
	// the Argus-authored diff it produced.
	outOfBand := []attribution.Change{{Path: authorityContract, Author: attribution.Argus, Slug: "widget"}}
	if refused, _ := AttributionRefuses(outOfBand, firstPRResolver); !refused {
		t.Fatal("the merge boundary did not back up the out-of-band write — the honest split is broken")
	}
}

// TestGitBackedHonestSplit_OutOfBandRefusedByRealGate makes the honest split
// concrete end-to-end: a real Argus-authored commit (carrying the in-tree
// Argus-Drive trailer) writes a protected governance document out-of-band, and
// the real attribution gate (git-backed derivation + author-independent
// classification) refuses it at the merge boundary — no mock, a real git read.
func TestGitBackedHonestSplit_OutOfBandRefusedByRealGate(t *testing.T) {
	root, base := gitBaseRepo(t)

	// An out-of-band Argus write to a protected governance doc — the write hook
	// never saw it (there is no hook in this raw-git path, exactly the ABUSE-4
	// bypass). It is committed with the in-tree Argus trailer.
	writeFile(t, root, governanceDocument, "# Vision — tampered out of band\n")
	gitCommit(t, root, "docs: out-of-band edit\n\nArgus-Drive: widget", governanceDocument)

	defects := attribution.Evaluate(root, base, corpusCfg)
	if len(defects) == 0 {
		t.Fatal("the real attribution gate did not refuse the out-of-band Argus write to a governance doc")
	}
	found := false
	for _, d := range defects {
		if d.File == governanceDocument && d.Op == "governance-document" {
			found = true
		}
	}
	if !found {
		t.Errorf("no governance-document refusal for %q among %v", governanceDocument, defects)
	}
}

// TestZeroFalseRefusals is the whole-corpus guardrail: every legitimate fixture
// passes with zero false refusals (M2 / R3) — the legal and born contracts, the
// legitimate first-PR footprint, the human-authored protected-path PR, the
// committed-ledger-owned write, and the in-scope write.
func TestZeroFalseRefusals(t *testing.T) {
	for _, c := range LegitCorpus() {
		if !c.Legit {
			t.Errorf("%q is in LegitCorpus() but not marked Legit", c.Name)
		}
		refused, detail := c.Refused()
		if refused {
			t.Errorf("FALSE REFUSAL — legitimate case %q was refused at %s: %s", c.Name, c.Owner, detail)
		}
	}
}

// TestInjectionIsStructuralUnsteered is ABUSE-2: the injection fixture's write
// hook deny fires on the target's classification alone. The drive is a
// legitimate widget first-PR drive, yet the authority-contract write is denied —
// the verdict never consults the drive's intent, so a planted directive that
// produced the attempt cannot steer it.
func TestInjectionIsStructuralUnsteered(t *testing.T) {
	for _, target := range []string{
		authorityContract,
		".claude/skills/argus/authority-contract.md", // the per-CLI symlink alias
	} {
		denied, dec := WriteHookDenies(target, widgetFirstPR())
		if !denied {
			t.Errorf("%q: not denied under a legitimate drive — the deny must be structural", target)
		}
		if !strings.Contains(dec.Boundary, "authority contract") {
			t.Errorf("%q: boundary = %q, want the authority-contract class", target, dec.Boundary)
		}
	}
}

// TestOwnerLayerMatchesHonestSplit asserts the corpus's owner assignment obeys
// the honest split: the write hook owns only the at-write-time injection case;
// the out-of-band case is owned by a merge-boundary layer (the write hook could
// never own it).
func TestOwnerLayerMatchesHonestSplit(t *testing.T) {
	for _, c := range AbuseCorpus() {
		if c.Name == "out-of-band bypass caught at the merge boundary" && !c.Owner.MergeBoundary() {
			t.Errorf("the out-of-band case must be owned by a merge-boundary layer, got %s", c.Owner)
		}
		if c.Owner == WriteHook && c.Name != "injection-induced authority-contract write" {
			t.Errorf("only the injection case is owned by the fast write-time layer; %q claims it", c.Name)
		}
	}
}

// --- git fixture helpers (a real temp repo + real commits, mirroring the
// shipped arms' harnesses — never a mock of the commit graph, AP-14) ---

func gitBaseRepo(t *testing.T) (root, base string) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, root, governanceDocument, "# Vision\n")
	writeFile(t, root, "docs/wiki/note.md", "an ordinary note\n")
	git(t, root, "init", "-q", "-b", "main")
	gitCommit(t, root, "base", ".")
	base = strings.TrimSpace(string(gitOut(t, root, "rev-parse", "HEAD")))
	return root, base
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCommit(t *testing.T, root, message string, paths ...string) {
	t.Helper()
	git(t, root, append([]string{"add"}, paths...)...)
	git(t, root, "commit", "-q", "-m", message)
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	if out, err := gitCmd(root, args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func gitOut(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	out, err := gitCmd(root, args...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

func gitCmd(root string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	return cmd
}

// sanity: the corpus's fixture inputs stay well-formed against the shipped
// resolvers (a guard that a data edit does not silently degrade a fixture).
func TestFixtureResolversWellFormed(t *testing.T) {
	if boundary.ResolveFirstPR(corpusCfg, "widget") == nil {
		t.Fatal("the widget first-PR scope must resolve")
	}
	if sc, ok := boundary.ResolveCommittedLedger(corpusCfg, "broad", []byte(committedBroadLedger)); !ok || sc == nil {
		t.Fatal("the broad committed ledger must parse and resolve")
	}
}
