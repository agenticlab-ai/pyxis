package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

// check-citations resolves the typed cross-document citations reviewers verify
// by reading — the requirement↔design↔record join — to a real anchor, so a
// dangling reference is a gate error instead of a review catch
// (docs/tech-design/citation-resolution.md). Present (live OR superseded)
// resolves; absent entirely is dangling and fails. It covers the grammars
// nothing else checks (prd:/design:#anchor/brainstorm:/design-brainstorm:/
// corpus:), reuses the code-span/fence exemption, and is complementary to
// gen-index's non-fatal stale-serves flag (unchanged): this fails
// never-existed targets, gen-index flags moved-on ones (DINV-5). Base-free,
// read-only (DINV-4).
func init() {
	Register(Check{
		Verb:           "check-citations",
		Help:           "typed cross-document citations resolve (prd:/design:/brainstorm:/corpus:)",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return CheckCitations(c.Root, c.Cfg, c.Scan) },
	})
}

// citationRe matches a typed citation token. The closed grammar set (DINV-6);
// `design-brainstorm` precedes `design` for clarity though the trailing colon
// already disambiguates. The payload starts alphanumeric, so a metavariable
// like `prd:<feature>/<ID>` (leading `<`) yields no match and never fires.
var citationRe = regexp.MustCompile(`\b(prd|design-brainstorm|brainstorm|design|corpus):([A-Za-z0-9][A-Za-z0-9._/#-]*)`)

// Citation is one typed reference outside a fenced block or inline code span.
// The scanner is shared with draft readiness so both gates use the same grammar.
type Citation struct {
	Line    int
	Grammar string
	Payload string
}

func ScanCitations(body []byte) []Citation {
	var citations []Citation
	fenced := false
	line := 0
	for raw := range strings.SplitSeq(string(schema.Normalize(body)), "\n") {
		line++
		if strings.HasPrefix(strings.TrimSpace(raw), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, match := range citationRe.FindAllStringSubmatch(stripCodeSpans(raw), -1) {
			payload := strings.TrimRight(match[2], ".,);:")
			if payload != "" {
				citations = append(citations, Citation{Line: line, Grammar: match[1], Payload: payload})
			}
		}
	}
	return citations
}

// CheckCitations walks the governed corpus and reports every typed citation
// whose target does not resolve. Base-free: it reads the current tree only. It
// is the tree-wide caller of CheckCitationsInFiles over the whole scan set — the
// scoping-independent resolution core the contract gate's link check reuses over
// one feature's own documents (prd:contract-gate/NR-3), so the two callers share
// one resolver rather than forking it.
func CheckCitations(root string, cfg config.Config, scan ScanScope) []schema.Defect {
	// skipReviews=true carves docs/reviews out of the scan (design §3.2's "docs/
	// tree" refined): docs/reviews holds the unmanned reviewers' evidence
	// packages — commentary ABOUT the authored requirement↔design↔record join,
	// written in reading-shorthand the §3.1 grammar does not admit (`design:
	// <Section>` with no slug; `prd:<feature>/<NNN>` as a document-number subject
	// key). Neither has a conforming form, so scanning reviews would red the gate
	// on legitimate commentary — an A-6 violation whose only "fix" is
	// grammar-widening, which DINV-6 forbids. Templates stay in scan: their
	// example citations are backticked, exempt under §3.2's code-span rule.
	return CheckCitationsInFiles(root, cfg, citationScanFiles(root, cfg, scan))
}

// CheckCitationsInFiles resolves every typed citation appearing in the given
// files and reports each that does not resolve, naming its file+line. It is the
// shared resolution core: CheckCitations passes the whole governed corpus and
// the contract gate passes one feature's own documents. Draft-born plans use
// their pinned lock for their own PRD and design citations; all other files
// and citations keep the document-pipeline resolver. Files are read in the
// order given; a file that cannot be read is skipped. Read-only.
func CheckCitationsInFiles(root string, cfg config.Config, files []string) []schema.Defect {
	env := designEnv(root, "", "", cfg) // PRDUnits (any status), CorpusIDs — the resolution universes
	slugs := map[string]bool{}
	designs, _ := LoadDesigns(root, cfg)
	for _, d := range designs {
		slugs[d.Slug] = true
	}
	grandfathered := grandfatheredCitations(cfg.GrandfatheredCitations)

	var defects []schema.Defect
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		relSrc, _ := filepath.Rel(root, path)
		rel := filepath.ToSlash(relSrc)
		locked := lockedPlanCitations(root, rel, b, cfg)
		for _, citation := range ScanCitations(b) {
			grammar, payload := citation.Grammar, citation.Payload
			ok, tried := locked.resolve(grammar, payload)
			if !ok && tried == "" {
				ok, tried = resolveCitation(grammar, payload, env, slugs, root, cfg)
			}
			if !ok {
				if grandfathered[rel+"\x00"+grammar+":"+payload] {
					continue // a pre-existing dangler frozen into an immutable artifact (config)
				}
				defects = append(defects, vdef("check-citations", rel, "citation",
					fmt.Sprintf("line %d: citation %q does not resolve (%s)", citation.Line, grammar+":"+payload, tried),
					"cite an existing anchor — a superseded target still resolves, a never-existed one does not"))
			}
		}
	}
	return defects
}

// resolveCitation reports whether a typed citation resolves, and — when it
// does not — what universe it failed against. The prd:/design:/corpus: rules
// mirror the design-lint resolver's universes (present-at-any-status =
// resolves); brainstorm:/design-brainstorm: resolve a record file plus anchor.
func resolveCitation(grammar, payload string, env techdesigns.Env, slugs map[string]bool, root string, cfg config.Config) (bool, string) {
	switch grammar {
	case "prd":
		feature, unit, hasUnit := strings.Cut(payload, "/")
		units, ok := env.PRDUnits[feature]
		if !ok {
			return false, fmt.Sprintf("no ledger for feature %q", feature)
		}
		// `prd:<feature>` (no unit) cites the whole PRD — resolves on the
		// ledger. `prd:<feature>/<ID>` cites a unit; a range (`FR-1..11`)
		// resolves on its first endpoint.
		if hasUnit && unit != "" {
			if _, ok := units[firstOfRange(unit)]; !ok {
				return false, fmt.Sprintf("unit %s not in %s's projection", unit, feature)
			}
		}
		return true, ""
	case "corpus":
		if !env.CorpusIDs[firstOfRange(payload)] {
			return false, "ID not present in the placeholder corpus"
		}
		return true, ""
	case "design":
		slug, anchor := splitAnchor(payload)
		if !slugs[slug] {
			return false, fmt.Sprintf("no design slug %q", slug)
		}
		if anchor != "" && !anchorInFile(filepath.Join(root, filepath.FromSlash(cfg.TechDesigns), slug+".md"), firstAnchor(anchor)) {
			return false, fmt.Sprintf("anchor %s not in design %s", anchor, slug)
		}
		return true, ""
	case "brainstorm", "design-brainstorm":
		// Both classes live in the records directory as `<slug>.md`; the slug
		// carries its own suffix (a stage-3 record's ends in `-tech-design`).
		// The check verifies existence, not stage-class aptness (§3.4). The
		// brainstorm-record root resolves from config (default docs/brainstorms).
		slug, anchor := splitAnchor(payload)
		return recordResolve(filepath.Join(root, filepath.FromSlash(cfg.Brainstorms), slug+".md"), "record "+slug, anchor)
	}
	return true, "" // unreachable: the regex admits only the closed set
}

// firstOfRange returns the first endpoint of a range anchor/unit token
// (`FR-1..11` → `FR-1`, `T-1..T-3` → `T-1`); a non-range returns unchanged. A
// citation to a range resolves on its start (§3.3's existence posture).
func firstOfRange(s string) string { return strings.SplitN(s, "..", 2)[0] }

// firstAnchor reduces a compound anchor reference to its lead anchor for the
// existence check. The corpus writes anchor citations three prose-convenience
// ways the grammar's single `#<anchor>` glosses: a range (`T-1..T-3`), a
// slash-list (`DINV-3/4/5`, `F-2/#F-3`), and an embedded-`#` list
// (`D-1..#D-4`). Each resolves on its first component — §3.3's "resolves on its
// start" posture, generalized from ranges to lists; the tail is a permissive
// pass (a gate false-positive blocks merges, so existence errs toward
// resolving, per anchorInFile). Only `..` and `/`/`#` separate, never a lone
// `.`, so a dotted section anchor like `3.1` returns unchanged.
func firstAnchor(anchor string) string {
	head := strings.SplitN(anchor, "..", 2)[0] // range → lead endpoint
	for _, tok := range strings.FieldsFunc(head, func(r rune) bool { return r == '/' || r == '#' }) {
		if t := strings.TrimSpace(tok); t != "" {
			return t
		}
	}
	return strings.TrimSpace(head)
}

// recordResolve resolves a record file plus an optional anchor.
func recordResolve(path, name, anchor string) (bool, string) {
	if _, err := os.Stat(path); err != nil {
		return false, "no " + name
	}
	if anchor != "" && !anchorInFile(path, firstAnchor(anchor)) {
		return false, fmt.Sprintf("anchor %s not in %s", anchor, name)
	}
	return true, ""
}

// splitAnchor splits `slug#anchor` and strips a leading § from the anchor.
func splitAnchor(payload string) (slug, anchor string) {
	slug, anchor, _ = strings.Cut(payload, "#")
	return slug, strings.TrimPrefix(anchor, "§")
}

// anchorInFile reports whether the anchor token appears in the file as a word.
// Permissive by design: a gate reds only a clear break (file or token absent),
// never a citation whose anchor is merely formatted unexpectedly — false
// positives block merges, so existence errs toward resolving.
func anchorInFile(path, anchor string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	re, err := regexp.Compile(`\b` + regexp.QuoteMeta(anchor) + `\b`)
	if err != nil {
		return true // a pathological anchor is not this check's defect to raise
	}
	return re.Match(b)
}

// grandfatheredCitations builds the skip set from the `<file> | <token>` config
// entries, keyed `<slash-path>\x00<grammar:token>`. Each entry is a genuine
// pre-existing dangling citation frozen into an immutable artifact (a merged
// design under INV-41, or an add-only ledger/amendment entry) — a real catch
// the check surfaced that cannot be corrected in place, only when the artifact
// is next superseded/amended. Listing it keeps A-6 green without editing frozen
// history; a NEW dangling citation is never on the list, so the gate still
// bites. An entry without a ` | ` cannot match a real (file, token) key, so a
// malformed line silently exempts nothing (fail-closed toward catching).
func grandfatheredCitations(entries []string) map[string]bool {
	set := map[string]bool{}
	for _, e := range entries {
		file, token, ok := strings.Cut(e, "|")
		if !ok {
			continue
		}
		set[strings.TrimSpace(file)+"\x00"+strings.TrimSpace(token)] = true
	}
	return set
}
