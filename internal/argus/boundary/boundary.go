// Package boundary is the one reusable scope-boundary predicate the write
// hook (docs/tech-design/argus-write-hook.md §3.1) and the attribution gate
// (docs/tech-design/argus-attribution-gate.md §3.2, §3.2.1) both consume:
// classify a target into exactly one protected-path class or report it
// in-scope, and resolve a drive's declared scope from an author-independent
// source so neither consumer derives that boundary from an author-writable
// same-PR input.
//
// Two honest sources bound a drive's scope width, never the author-writable
// trailer field alone:
//   - for a drive whose problem already has a committed (merged) delivery
//     ledger, the width is the ledger's own recorded reality — the paths its
//     append-only journal attributes to the drive — which a current drive
//     cannot retroactively widen (§3.2.1, DINV-6), plus its bounded numbered
//     plan documents and native problem captures;
//   - for a brand-new problem's single first documentation PR, whose ledger
//     is not yet committed, the width is a fixed function of the drive's slug
//     united with a fixed named allowlist — taking no author-controlled
//     same-PR input and admitting no code path (§3.2.1(b), the operator
//     ruling problem:argus-boundary-enforcement#A-1).
//   - for a Carina feature item already present at the merge base, the git
//     consumer may add exactly that item path when the base and current item
//     both identify the drive. An item first added in the proposal grants no
//     scope; neither does an item assigned to another drive.
//
// When scope cannot be resolved the predicate reports Unresolvable so a
// consumer fails closed (deny/refuse), never in-scope (AP-17). The package is
// pure and hermetic: it reads its inputs structurally — a planted directive
// inside any read input is data, never an instruction that steers the verdict
// (it reads git nothing; a consumer supplies the committed ledger bytes).
package boundary

import (
	"path"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// DataVersion pins the protected-path set and positive-scope rules this build
// classifies against. Bump it on any change to that data so a
// consumer can record which policy data produced a verdict (AP-6: the sets
// are contractual, version-pinned inputs, not free-form guesses).
const DataVersion = "argus-boundary/v6"

// Class is the single classification a target falls into — one protected-path
// class, the in-scope result, or the fail-closed unresolvable signal. Exactly
// one applies to any (target, drive) pair.
type Class int

const (
	// InScope — the drive's own paths: it writes them freely (the only allow).
	InScope Class = iota
	// AuthorityContract — the fixed factory-control policy file (the never-list).
	AuthorityContract
	// GovernanceDocument — a frozen/merged document of the append-only governing
	// layer the drive guide's never-list forbids a drive authoring.
	GovernanceDocument
	// OtherProblemLedger — a ledger under another problem's subtree.
	OtherProblemLedger
	// OutOfScope — a file outside the drive's resolved declared scope.
	OutOfScope
	// Unresolvable — the declared scope could not be resolved; a consumer maps
	// this to a fail-closed deny/refusal, never an allow (AP-17).
	Unresolvable
)

// String names the class a target matched, for a consumer's {path, boundary,
// remedy} reason (the write hook's NFR-4 reason shape; the gate's refusal).
func (c Class) String() string {
	switch c {
	case InScope:
		return "in scope"
	case AuthorityContract:
		return "authority contract"
	case GovernanceDocument:
		return "frozen/merged governance document"
	case OtherProblemLedger:
		return "another problem's ledger"
	case OutOfScope:
		return "out of declared scope"
	case Unresolvable:
		return "declared scope unresolvable"
	}
	return "unknown"
}

// Protected reports whether the class is one of the four protected-path
// classes (a target a drive may not write). InScope and Unresolvable are not
// protected classes — Unresolvable is the fail-closed signal, distinct from a
// classified protected hit. A consumer allows a write iff the class is
// InScope; every other class denies.
func (c Class) Protected() bool {
	switch c {
	case AuthorityContract, GovernanceDocument, OtherProblemLedger, OutOfScope:
		return true
	}
	return false
}

// authorityContractPaths is the fixed factory-control policy file, matched on
// both the canonical vendor-neutral surface and its per-CLI symlink alias so a
// write routed through either name is caught (AP-25 — the surface is the same
// object under either path).
var authorityContractPaths = map[string]bool{
	".pyxis/skills/argus/authority-contract.md":  true,
	".claude/skills/argus/authority-contract.md": true,
}

// isGovernanceDocument reports whether the normalized target t is a member of
// the append-only governing layer a drive never authors or modifies — vision
// and principles (the never-list's "governance-tier content"), the
// design-principles governing pointer, and the incident registry (append-only
// observations). A frozen/merged document of this set is a protected path
// regardless of a drive's declared scope.
//
// The set is a cohesive governance POLICY resolved from the same governing-doc
// path config the skeleton-target map derives from (config-driven-portability-v1
// §3.2, FR-1/FR-3; DINV-1) — never a fixed literal list. Each member is the
// configured location of its governing document, normalized so a config spelling
// variant still matches; on a default config the four members are byte-identical
// to the former literals (DINV-2). Version-pinned policy (DataVersion) — a config
// relocation MOVES a member, never adds one; the membership stays four.
func isGovernanceDocument(cfg config.Config, t string) bool {
	// cleanRoot is the file's config-path normalizer (firstPRAllowlist,
	// ResolveFirstPR, problemSlugRe all canonicalize config values through it);
	// keep every config-derived path on the one lever, matched against the
	// already-normalized target t exactly as problemSlugOf does.
	switch t {
	case cleanRoot(cfg.ProductVision),
		cleanRoot(cfg.ArchitecturePrinciples),
		cleanRoot(cfg.DesignPrinciples),
		cleanRoot(cfg.Incidents):
		return true
	}
	return false
}

// firstPRAllowlist is the fixed named set of shared targets a brand-new
// problem's first documentation PR legitimately touches beyond its own slug-
// derived doc subtrees (docs/tech-design/argus-attribution-gate.md §3.2.1(b)).
// Every member is author-independent — a static named entry, never derived
// from any PR input — so a mis-behaving first-PR drive cannot add to it. Its
// members are anchored at target-facing doc roots (the wiki glossary, the
// tech-design index), so each is resolved from config rather than a fixed
// literal (config-driven-portability-v1 §3.2, FR-1). Version-pinned data
// (DataVersion) — a config relocation moves the anchors, never adds members.
func firstPRAllowlist(cfg config.Config) []string {
	wiki := cleanRoot(cfg.Wiki)
	td := cleanRoot(cfg.TechDesigns)
	return []string{
		wiki + "/glossary.md",
		td + "/index.md",
		td + "/index.json",
	}
}

// cleanRoot canonicalizes a configured doc-path root to a forward-slash,
// cleaned form so joins against it are stable regardless of the config's
// spelling ("docs/wiki", "docs/wiki/", "./docs/wiki").
func cleanRoot(root string) string {
	return strings.TrimPrefix(path.Clean(strings.ReplaceAll(root, "\\", "/")), "./")
}

// Scope is a drive's resolved declared scope — the author-independent set of
// repo-relative paths the drive may write. It is built only by
// ResolveCommittedLedger or ResolveFirstPR; a nil *Scope means the scope is
// unresolvable.
type Scope struct {
	exact    map[string]bool // exact owned paths
	prefixes []string        // owned subtree prefixes, each ending in "/"
	globs    []string        // owned path.Match globs (no separator crossing)
}

// Owns reports whether the drive's declared scope permits writing target.
func (s *Scope) Owns(target string) bool {
	t := norm(target)
	if s.exact[t] {
		return true
	}
	for _, p := range s.prefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	for _, g := range s.globs {
		if ok, _ := path.Match(g, t); ok {
			return true
		}
	}
	return false
}

// WithVerifiedFeatureItem adds one Carina feature item after a caller has
// verified its owner in the merge-base tree and its current identity. The
// extension is limited to the configured numeric item path; it cannot admit an
// arbitrary path or another problem's ledger. The returned scope is a copy so
// one drive's item cannot leak into another drive's cached scope.
func WithVerifiedFeatureItem(cfg config.Config, scope *Scope, target string) *Scope {
	if scope == nil || !IsFeatureItemPath(cfg, target) {
		return scope
	}
	clone := &Scope{exact: make(map[string]bool, len(scope.exact)+1), prefixes: append([]string(nil), scope.prefixes...), globs: append([]string(nil), scope.globs...)}
	for p := range scope.exact {
		clone.exact[p] = true
	}
	clone.exact[norm(target)] = true
	return clone
}

// IsFeatureItemPath recognizes only a canonical numeric Carina item under the
// configured work-item root. The caller still has to verify that item's owner;
// this function alone never grants scope.
func IsFeatureItemPath(cfg config.Config, target string) bool {
	if target != norm(target) {
		return false
	}
	root := cleanRoot(cfg.WorkItems) + "/items/"
	t, ok := strings.CutPrefix(target, root)
	if !ok || !strings.HasSuffix(t, ".md") {
		return false
	}
	id := strings.TrimSuffix(t, ".md")
	if id == "" || id[0] < '1' || id[0] > '9' {
		return false
	}
	for _, r := range id[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Drive is the classifying context: the problem slug the drive names and its
// resolved declared scope (nil when scope is unresolvable).
type Drive struct {
	Slug  string
	Scope *Scope
}

// Classify places target into exactly one protected-path class or reports it
// in-scope, against the drive. The path-based protected classes (authority
// contract, governance document, another problem's ledger) are resolved from
// the target against the configured governing-doc / problems paths — never the
// author-writable scope — and fire even when the declared scope is unresolvable,
// so they are scope-independent. Only the in-scope-vs-out-of-scope decision
// needs a resolved scope; with none, a target that reaches that decision is
// Unresolvable (fail closed), never in-scope (AP-17, criterion 4). The
// governance-document set and the other-problem-ledger root both resolve from
// config (config-driven-portability-v1 §3.2, FR-1/FR-3; the governance policy is
// the same governing-doc keys the skeleton-target map derives from; default
// docs/product-vision.md · docs/architecture-principles.md ·
// docs/design-principles.md · docs/observability/incidents.md · docs/problems).
func Classify(cfg config.Config, target string, d Drive) Class {
	t := norm(target)

	if authorityContractPaths[t] {
		return AuthorityContract
	}
	if isGovernanceDocument(cfg, t) {
		return GovernanceDocument
	}
	if d.Slug != "" {
		if owner, ok := problemSlugOf(cfg.Problems, t); ok && owner != d.Slug {
			return OtherProblemLedger
		}
	}
	if d.Scope == nil {
		return Unresolvable
	}
	if d.Scope.Owns(t) {
		return InScope
	}
	return OutOfScope
}

// ResolveCommittedLedger derives a drive's declared scope from its problem's
// committed (merged) delivery ledger — the gate-protected, append-only journal
// whose recorded reality a current drive cannot retroactively widen (§3.2.1,
// DINV-6). The committed bytes MUST be read by the consumer from the merge
// base / committed tree; this function never reads git. The permitted scope is
// the own problem subtree, native problem captures, numbered plan Markdown/JSON
// and exact repo paths the journal attributes to the drive. Planning readiness
// remains the planning-entry gate's separate precondition; this resolver grants
// no lock, approval, code path or configuration change.
//
// A self-named broader problem slug buys only what that problem's merged
// ledger genuinely owns and the bounded same-slug documentation grammar:
// never the slug's breadth. Returns (nil, false) — unresolvable — when the
// bytes are not a parseable delivery ledger, so a consumer fails closed.
// Historical exact-path ownership and the own problem subtree retain their
// existing parser contract. Only the added plan and capture grammar requires
// the complete delivery shape and typed journal; older ledgers gain no new
// permission from their historical acceptance. The
// problems root the ledger and its owned subtree live under resolves from
// config (config-driven-portability-v1 §3.2, FR-1; default docs/problems).
func ResolveCommittedLedger(cfg config.Config, slug string, committed []byte) (*Scope, bool) {
	if slug == "" {
		return nil, false
	}
	problems := cleanRoot(cfg.Problems)
	led, defects := argusledger.ParseAs("boundary", problems+"/"+slug+"/delivery.md",
		schema.LedgerClasses["delivery-ledger"], committed)
	if led == nil || len(defects) > 0 {
		return nil, false
	}
	s := &Scope{
		exact:    map[string]bool{},
		prefixes: []string{problems + "/" + slug + "/"},
	}
	if validDeliveryScopeShape(slug, committed, led) {
		// Configured roots are literal paths, even when they contain characters
		// with meaning to path.Match. QuoteMeta also escapes its glob metacharacters.
		planRoot := regexp.QuoteMeta(path.Join(cleanRoot(cfg.Plans), slug) + "/")
		// Native ledger captures are flat Document JSON/body pairs. The suffix
		// uses the same 26-character Crockford alphabet as record.KindOfID;
		// record admission separately verifies their content and provenance.
		captureID := regexp.QuoteMeta("memory/problem/"+slug+"/Document-") + strings.Repeat("[0-9A-HJKMNP-TV-Z]", 26)
		s.globs = []string{
			planRoot + "[0-9][0-9][0-9]-plan.md",
			planRoot + "[0-9][0-9][0-9]-plan.json",
			captureID + ".json",
			captureID + ".md",
		}
	}
	// Draft roots are configurable and may live outside the legacy path
	// recognizer's named top-level directories. Recognize only exact effects
	// under that root; never grant its whole subtree from a committed ledger.
	draftPathRe := regexp.MustCompile(`^` + regexp.QuoteMeta(cleanRoot(cfg.Drafts)) + `/[A-Za-z0-9._/-]+$`)
	for _, id := range led.Order {
		for _, p := range externalEffectPaths(led.Entries[id], draftPathRe) {
			s.exact[p] = true
		}
	}
	return s, true
}

// ResolveFirstPR derives the declared scope for a brand-new problem's single
// first documentation PR — the slug-derived fixed set ∪ the fixed named
// allowlist (§3.2.1(b), problem:argus-boundary-enforcement#A-1). It consults no
// author-controlled same-PR input and admits no code path (every slug-derived
// member is a document or contract record keyed to that one slug, never a code
// path). A slug's own subtrees include its problem and PRD documents, working
// draft files, compiled PRD/design records, and native captures of its problem
// ledgers. The draft root resolves from
// config; memory subject roots are fixed. Always
// resolvable (a pure function of the config + slug); an empty slug yields an
// allowlist-only scope owning no slug subtree.
func ResolveFirstPR(cfg config.Config, slug string) *Scope {
	s := &Scope{exact: map[string]bool{}}
	for _, p := range firstPRAllowlist(cfg) {
		s.exact[p] = true
	}
	if slug == "" {
		return s
	}
	s.prefixes = []string{
		cleanRoot(cfg.Problems) + "/" + slug + "/",
		cleanRoot(cfg.PRDs) + "/" + slug + "/",
		cleanRoot(cfg.Drafts) + "/" + slug + "/",
		"memory/prd/" + slug + "/",
		"memory/design/" + slug + "/",
		"memory/problem/" + slug + "/",
	}
	s.globs = []string{
		cleanRoot(cfg.Brainstorms) + "/" + slug + "*.md",
		cleanRoot(cfg.Reviews) + "/" + slug + "-*.md",
	}
	return s
}

// problemSlugOf returns the owning problem slug of a normalized path under the
// configured problems root, or ("", false) for any other path — the only
// ledger-bearing subtree. The root resolves from config (default docs/problems);
// the pattern is compiled per distinct root and cached (Classify is on the hot
// path — every classified diff path).
func problemSlugOf(problemsRoot, t string) (string, bool) {
	if m := problemSlugRe(problemsRoot).FindStringSubmatch(t); m != nil {
		return m[1], true
	}
	return "", false
}

// problemSlugReCache memoizes the compiled owning-slug pattern per problems
// root so a stable-root run pays one compilation.
var problemSlugReCache sync.Map // problems-root string → *regexp.Regexp

func problemSlugRe(problemsRoot string) *regexp.Regexp {
	root := cleanRoot(problemsRoot)
	if v, ok := problemSlugReCache.Load(root); ok {
		return v.(*regexp.Regexp)
	}
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(root) + `/([^/]+)/`)
	problemSlugReCache.Store(root, re)
	return re
}

// fullPathRe matches a cleaned token that is a repo-relative path: a known
// top-level segment, a separator, then path characters (no ':' or '#', so a
// citation or a sha256 identity never matches).
var fullPathRe = regexp.MustCompile(`^(docs|internal|cmd|pkg|scripts|\.github|\.githooks|\.pyxis|\.claude)/[A-Za-z0-9._/-]+$`)

// bareFileRe matches a bare filename (an extension, no separator) — the "+ x,
// y" continuation shorthand the ledger writes for artifacts sharing the
// preceding path's directory (e.g. "000-base.md (+ effective.md, …)").
var bareFileRe = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9.]+$`)

// effectFieldRe captures the body of an entry's `- **External effect**:` line.
var effectFieldRe = regexp.MustCompile(`^\s*-\s*\*\*External effect\*\*:\s*(.*\S)\s*$`)

// externalEffectPaths extracts every repo-relative path an entry block's
// External-effect field names — the full-path tokens plus the "+ name"
// continuation filenames resolved against the preceding path's directory.
// Citations, sha256 identities, and prose are ignored (structural extraction:
// a planted directive in the body is never read as a path).
func externalEffectPaths(block []byte, draftPathRe *regexp.Regexp) []string {
	var out []string
	for _, line := range argusledger.ProseLines(block) {
		m := effectFieldRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var lastDir string
		plusSeen := false
		for _, raw := range externalEffectTokens(m[1], draftPathRe) {
			tok := clean(raw)
			switch {
			case tok == "":
				continue
			case fullPathRe.MatchString(tok) || draftPathRe.MatchString(tok):
				out = append(out, tok)
				lastDir = path.Dir(tok)
				plusSeen = false
			case tok == "+":
				plusSeen = true
			case plusSeen && lastDir != "" && bareFileRe.MatchString(tok):
				out = append(out, lastDir+"/"+tok)
			default:
				plusSeen = false
			}
		}
	}
	return out
}

// Preserve a delimited chunk only when it names one exact configured draft
// path. Other chunks keep the original FieldsSeq grammar, including quoted
// groups and Unicode whitespace. A closing quote never splits off a citation
// suffix: it remains in the chunk and fails the path recognizer.
func externalEffectTokens(field string, draftPathRe *regexp.Regexp) []string {
	var tokens []string
	for i := 0; i < len(field); {
		r, size := utf8.DecodeRuneInString(field[i:])
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		start := i
		var quote rune
		prefix := true
		for i < len(field) {
			r, size = utf8.DecodeRuneInString(field[i:])
			if quote != 0 {
				if r == quote {
					quote = 0
					prefix = false
				}
			} else {
				if unicode.IsSpace(r) {
					break
				}
				if prefix && strings.ContainsRune("`\"'", r) {
					quote = r
				} else if !strings.ContainsRune("*([<", r) {
					prefix = false
				}
			}
			i += size
		}
		chunk := field[start:i]
		if quote == 0 && draftPathRe.MatchString(clean(chunk)) {
			tokens = append(tokens, chunk)
			continue
		}
		for token := range strings.FieldsSeq(chunk) {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// clean strips the markdown, separator, and bracketing punctuation a token
// carries in prose (·, commas, semicolons, parens, emphasis, quotes), plus a
// stray trailing sentence dot — while preserving a leading dot so a
// dot-prefixed top-level path (.github, .pyxis, .claude) survives.
func clean(tok string) string {
	t := strings.Trim(tok, "*`'\"·,;:()[]<> \t")
	return strings.TrimRight(t, ".")
}

// norm canonicalizes a target to a forward-slash, cleaned, repo-relative path
// so classification is stable across "./x", "a//b", and backslash inputs.
func norm(p string) string {
	c := path.Clean(strings.ReplaceAll(p, "\\", "/"))
	return strings.TrimPrefix(c, "./")
}
