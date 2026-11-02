package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The cross-document arm of Argus ledger-lint (docs/tech-design/
// argus-ledger-lint.md §3.2): a base-free, repo-wide self-registered check
// that scans the tracked corpus for `delivery:<slug>#<anchor>` citations,
// recognizes an in-scope citation only through the two-stage FR-9 boundary
// (citation context, then target resolvability), and classifies a recognized
// citation by the region its anchor lands in — a journal E-n is legal, the
// derived-state block is the DINV-6 defect, and an anchor resolving to neither
// is the distinct unresolvable-anchor defect (UB-2, DINV-5). Report-only; the
// only subprocess is `git ls-files`, exactly as the sibling base-free
// `check-links` reads the corpus.
func init() {
	Register(Check{
		Verb:             ledgerCitationVerb,
		Help:             "downstream citations never resolve into a delivery ledger's derived-state block",
		NeedsMergeBase:   false,
		RunOnPyxisSource: true,
		Run:              func(c Ctx) Defects { return CheckLedgerCitations(c.Root, c.Cfg) },
	})
}

const (
	ledgerCitationVerb = "check-ledger-citations"
	// opDerivedCitation and opUnresolvableAnchor are the DINV-5 distinct
	// discriminators: a cited derived-state block (FR-3) and an unresolvable
	// anchor (UB-2) are never conflated on schema.Defect.Op.
	opDerivedCitation    = "derived-citation"
	opUnresolvableAnchor = "unresolvable-anchor"
)

// deliveryCiteRe matches one `delivery:<slug>#<anchor>` occurrence: a
// kebab-case slug and a single-token anchor. The literal `<slug>` metavariable
// (angle-bracketed) does not match this class, so a grammar illustration
// written with the placeholder is not even an occurrence — recognition stage 2
// (target resolvability) additionally excludes a concrete-but-absent slug.
var deliveryCiteRe = regexp.MustCompile(`delivery:([a-z0-9]+(?:-[a-z0-9]+)*)#([A-Za-z0-9][A-Za-z0-9_-]*)`)

// CheckLedgerCitations enumerates the tracked corpus once (`git ls-files`) and
// reports a defect for every recognized citation that resolves into a delivery
// ledger's derived-state region (DINV-6) or to an anchor in neither region
// (UB-2). It is silent on every non-citation — an occurrence inside a code
// span or fenced block (stage 1), or one whose slug names no ledger (stage 2)
// — so the repo's own citation-bearing prose stays green (FR-9, DINV-3). The
// problems root the delivery ledgers live under resolves from config
// (config-driven-portability-v1 §3.2, FR-1; default docs/problems).
func CheckLedgerCitations(root string, cfg config.Config) []schema.Defect {
	problems := filepath.ToSlash(filepath.Clean(cfg.Problems))
	tracked, err := gitLsFiles(root)
	if err != nil {
		return []schema.Defect{vdef(ledgerCitationVerb, "", "corpus",
			"could not enumerate the tracked corpus: "+err.Error(),
			"the check runs `git ls-files` against the local clone; ensure it is a git repository")}
	}
	trackedSet := make(map[string]bool, len(tracked))
	for _, p := range tracked {
		trackedSet[p] = true
	}

	// One parse per referenced ledger, cached — the shared per-class parse
	// (§3.4), so this arm and the history arm never diverge on how a ledger is
	// read. A cache miss reads and parses; a hit reuses.
	type parsedLedger struct {
		ledger  *argusledger.Ledger
		defects []schema.Defect
	}
	cache := map[string]parsedLedger{}
	parseReported := map[string]bool{} // a malformed ledger's parse defect is emitted once

	var defects []schema.Defect
	for _, rel := range tracked {
		if !strings.HasSuffix(rel, ".md") {
			continue // citations live in markdown documents (FR-9's "document")
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		fenced := false
		for i, line := range strings.Split(string(schema.Normalize(b)), "\n") {
			// Stage 1 — citation context: an occurrence inside a fenced code
			// block or an inline-code/backtick span is a displayed example, not
			// a citation (the `check-links` corpus convention, reused here).
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				fenced = !fenced
				continue
			}
			if fenced {
				continue
			}
			for _, m := range deliveryCiteRe.FindAllStringSubmatch(stripCodeSpans(line), -1) {
				slug, anchor := m[1], m[2]
				// Stage 2 — target resolvability: an in-scope citation only
				// when <slug> names an existing tracked delivery ledger.
				target := problems + "/" + slug + "/delivery.md"
				if !trackedSet[target] {
					continue // not a citation — no classification, no defect
				}
				pc, ok := cache[slug]
				if !ok {
					lb, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(target)))
					if rerr != nil {
						pc = parsedLedger{defects: []schema.Defect{vdef(ledgerCitationVerb, target, "read",
							rerr.Error(), "the cited delivery ledger must be readable")}}
					} else {
						l, pd := argusledger.Parse(cfg.Problems, ledgerCitationVerb, target, lb)
						pc = parsedLedger{ledger: l, defects: pd}
					}
					cache[slug] = pc
				}
				if len(pc.defects) > 0 {
					// A malformed target cannot be classified against — a named
					// parse defect (UB-3), once per ledger, never a false pass.
					if !parseReported[slug] {
						defects = append(defects, pc.defects...)
						parseReported[slug] = true
					}
					continue
				}
				cite := "delivery:" + slug + "#" + anchor
				switch pc.ledger.ResolveAnchor(anchor) {
				case argusledger.AnchorJournal:
					// legal — a journal E-n citation (FR-4)
				case argusledger.AnchorDerived:
					defects = append(defects, vdef(ledgerCitationVerb, rel, opDerivedCitation,
						fmt.Sprintf("line %d: %s resolves into %s's derived-state block, which is never citable (DINV-6)", i+1, cite, target),
						"cite a journal E-n entry; the derived-state block is regenerated and never authoritative"))
				case argusledger.AnchorUnresolved:
					defects = append(defects, vdef(ledgerCitationVerb, rel, opUnresolvableAnchor,
						fmt.Sprintf("line %d: %s resolves to neither a journal E-n entry nor a derived-state field in %s", i+1, cite, target),
						"fix the citation anchor to name an existing journal E-n entry"))
				}
			}
		}
	}
	return defects
}

// gitLsFiles returns the repo-relative paths of every tracked file — the
// corpus FR-9 names ("every tracked document in the repo"). NUL-delimited so a
// path with unusual bytes never splits wrong.
func gitLsFiles(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for p := range strings.SplitSeq(strings.TrimRight(string(out), "\x00"), "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	sort.Strings(files)
	return files, nil
}
