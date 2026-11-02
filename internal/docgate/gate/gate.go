// Package gate implements the docgate verbs as functions returning defect
// lists (DINV-3: verbs never write; only cmd/docgate writes, and only the
// two artifact paths). Desk and CI call the same umbrella (DINV-4).
package gate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/rewriteauth"
)

var entryRe = regexp.MustCompile(`^\d{3}-[a-z0-9-]+\.md$`)

// diffSpan renders the git-diff revision arguments for a history check: the
// merge base against the working tree (head == "") or against an explicit
// pushed sha — one place, so every check scopes identically (#357).
func diffSpan(mergeBase, head string) []string {
	if head == "" {
		return []string{mergeBase}
	}
	return []string{mergeBase, head}
}

func def(verb, file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: op, Reason: reason, Remedy: remedy}
}

// LedgerDirs finds every ledger directory under docs/prds (directories only —
// the flat placeholder corpus is exempt by operator ruling, PRD NG-1/NR-1).
func LedgerDirs(root string, cfg config.Config) ([]string, error) {
	prds := filepath.Join(root, filepath.FromSlash(cfg.PRDs))
	items, err := os.ReadDir(prds)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", prds, err)
	}
	var dirs []string
	for _, it := range items {
		if it.IsDir() {
			dirs = append(dirs, filepath.Join(prds, it.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// ReadEntries loads a ledger directory's entries, sorted by name.
func ReadEntries(dir string) ([]ledger.Entry, []schema.Defect) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, []schema.Defect{def("lint-structure", dir, "read", err.Error(), "the ledger directory must be readable")}
	}
	var entries []ledger.Entry
	for _, it := range items {
		name := it.Name()
		if it.IsDir() || !entryRe.MatchString(name) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, []schema.Defect{def("lint-structure", name, "read", err.Error(), "the entry must be readable")}
		}
		entries = append(entries, ledger.Entry{Name: name, Bytes: b})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

// LintStructure checks ledger layout: exactly one 000-base, dense monotonic
// numbering, parseable headers with template-version pins (FR-8, UB-1).
func LintStructure(dir string, entries []ledger.Entry) []schema.Defect {
	var defects []schema.Defect
	rel := filepath.Base(dir)
	if len(entries) == 0 || entries[0].Name != "000-base.md" {
		return append(defects, def("lint-structure", rel, "layout",
			"missing 000-base.md", "a ledger starts at 000-base.md instantiated from prd-template/v1"))
	}
	seen := map[int]string{}
	max := 0
	for _, e := range entries {
		seqStr, ok := ledger.Seq(e.Name)
		if !ok {
			defects = append(defects, def("lint-structure", e.Name, "layout",
				"filename is not NNN-<slug>.md", "dense three-digit numbering, kebab-case slug"))
			continue
		}
		n, _ := strconv.Atoi(seqStr)
		if prev, dup := seen[n]; dup {
			defects = append(defects, def("lint-structure", e.Name, "layout",
				fmt.Sprintf("duplicate sequence %03d (also %s)", n, prev),
				fmt.Sprintf("renumber onto the next free number (%03d)", maxKey(seen)+1)))
			continue
		}
		seen[n] = e.Name
		if n > max {
			max = n
		}
	}
	for n := 0; n <= max; n++ {
		if _, ok := seen[n]; !ok {
			defects = append(defects, def("lint-structure", rel, "layout",
				fmt.Sprintf("numbering gap: %03d is missing", n), "the sequence is dense and monotonic"))
		}
	}
	return defects
}

func maxKey(m map[int]string) int {
	max := 0
	for k := range m {
		if k > max {
			max = k
		}
	}
	return max
}

// VerifyGenerated recomputes the fold and byte-compares the committed
// projection artifacts (FR-3).
func VerifyGenerated(dir string, res fold.Result) []schema.Defect {
	var defects []schema.Defect
	check := func(name string, want []byte) {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			defects = append(defects, def("verify-generated", filepath.Base(dir)+"/"+name, "compare",
				"projection artifact missing", "run `docgate fold` and commit the regenerated artifacts"))
			return
		}
		if !bytes.Equal(got, want) {
			defects = append(defects, def("verify-generated", filepath.Base(dir)+"/"+name, "compare",
				"committed artifact differs from the recomputed fold",
				"never hand-edit generated artifacts: fix the op or the fold, then `docgate fold` and recommit"))
		}
	}
	check("effective.md", res.EffectiveMD)
	check("effective.source.json", res.SourceJSON)
	return defects
}

func statusWord(s byte) string {
	switch s {
	case 'M':
		return "modified"
	case 'D':
		return "deleted"
	default:
		return "renamed"
	}
}

// VerifyRebase proves every rebase checkpoint is a semantic no-op: the fold
// of everything before it equals the fold of the checkpoint alone, byte-wise
// after masking the generated banner line (FR-7, design §3.6).
func VerifyRebase(entries []ledger.Entry) []schema.Defect {
	var defects []schema.Defect
	for i, e := range entries {
		if !ledger.IsRebase(e.Name) {
			continue
		}
		before := fold.Fold(entries[:i])
		alone := fold.Fold(entries[i : i+1])
		if !bytes.Equal(maskBanner(before.EffectiveMD), maskBanner(alone.EffectiveMD)) {
			defects = append(defects, def("verify-rebase", e.Name, "compare",
				"rebase checkpoint is not semantically identical to the projection preceding it",
				"a rebase is a no-op snapshot: regenerate it from the projection, changing nothing"))
		}
	}
	return defects
}

func maskBanner(md []byte) []byte {
	lines := strings.Split(string(md), "\n")
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, "**GENERATED — DO NOT EDIT.**") {
			continue
		}
		out = append(out, l)
	}
	return []byte(strings.Join(out, "\n"))
}

// Diff renders an amendment PR's folded effect: before/after the last
// amendment — affected IDs, computed blast radius, invalidation levels (FR-6).
func Diff(entries []ledger.Entry) (string, []schema.Defect) {
	res := fold.Fold(entries)
	if len(res.Amendments) == 0 {
		return "identity fold: no amendments; projection equals the base.\n", res.Defects
	}
	last := res.Amendments[len(res.Amendments)-1]
	var b strings.Builder
	fmt.Fprintf(&b, "amendment %s — computed blast radius: %s\n", last.Name, last.ComputedTier)
	fmt.Fprintf(&b, "affected: %s\n", strings.Join(last.Affected, " · "))
	for _, id := range last.Affected {
		level := "Stale"
		if last.ComputedTier == "contract-changing" {
			level = "Blocked / Needs-recompile"
		}
		fmt.Fprintf(&b, "  %-10s invalidation: %s\n", id, level)
	}
	before := fold.Fold(entries[:len(entries)-1])
	fmt.Fprintf(&b, "projection: %d bytes → %d bytes; fingerprint %s → %s\n",
		len(before.EffectiveMD), len(res.EffectiveMD), short(before.Fingerprint), short(res.Fingerprint))
	return b.String(), res.Defects
}

func short(fp string) string {
	if len(fp) > 19 {
		return fp[:19] + "…"
	}
	return fp
}

func hasAnyPrefix(p string, prefixes []string) bool {
	for _, pre := range prefixes {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}

// Gate is the umbrella: every applicable verb over the given ledger dirs
// (FR-2..FR-8). mergeBase may be empty (desk use before any push). The diff
// span ends at the working tree; pre-push uses GateAt with the pushed sha.
func Gate(root, mergeBase string, dirs []string, cfg config.Config) []schema.Defect {
	return GateAt(root, mergeBase, "", dirs, cfg)
}

// GateAt is Gate with an explicit diff-span end: head == "" keeps the
// working-tree scope (desk/CI); a pushed sha scopes every history check to
// the committed range <mergeBase>..<head>, so shared-clone tree state cannot
// leak into a push verdict (#357).
func GateAt(root, mergeBase, head string, dirs []string, cfg config.Config) []schema.Defect {
	// Pyxis is moving its own contract review to memory. Retire its legacy
	// document checks, while keeping registered non-document checks such as
	// Argus attribution active. Read identity
	// from the trusted base (or HEAD for a base-free desk run), never from a
	// candidate worktree edit.
	identityRef := mergeBase
	if identityRef == "" {
		identityRef = "HEAD"
	}
	if pyxis, err := pyxisAtMergeBase(root, identityRef); err == nil && pyxis {
		var defects []schema.Defect
		rc := Ctx{Root: root, MergeBase: mergeBase, Head: head, Cfg: cfg}
		for _, chk := range Checks() {
			if !chk.RunOnPyxisSource || (chk.NeedsMergeBase && mergeBase == "") {
				continue
			}
			defects = append(defects, chk.Run(rc)...)
		}
		return defects
	}
	var defects []schema.Defect
	// The install-epoch scan filter, computed ONCE per run and shared by every
	// tree-local check (existing-repo-onboarding-v2 §3.4). An unreachable epoch
	// names its cause here — the umbrella is the authoritative gate run — and
	// fails closed to a whole-corpus scan.
	scan, scanDefects := scanScope(root, cfg)
	defects = append(defects, scanDefects...)
	// Registered repo-wide checks (self-contributed from their own files,
	// verb-sorted) dispatch through the registry — no per-check enumeration line
	// here to conflict on. Each check reads its own applicability off the
	// descriptor; base-requiring checks skip when no merge base is present.
	rc := Ctx{Root: root, MergeBase: mergeBase, Head: head, Cfg: cfg, Scan: scan}
	for _, chk := range Checks() {
		if chk.NeedsMergeBase && mergeBase == "" {
			continue
		}
		defects = append(defects, chk.Run(rc)...)
	}
	// The per-ledger-directory checks keep their explicit loop: they share one
	// read + fold per directory, which the flat registry does not model.
	for _, dir := range dirs {
		entries, edef := ReadEntries(dir)
		defects = append(defects, edef...)
		if entries == nil {
			continue
		}
		defects = append(defects, LintStructure(dir, entries)...)
		if len(entries) == 0 || entries[0].Name != "000-base.md" {
			continue
		}
		res := fold.Fold(entries)
		defects = append(defects, res.Defects...)
		defects = append(defects, VerifyRebase(entries)...)
		defects = append(defects, VerifyGenerated(dir, res)...)
	}
	// The review-feedback ledgers are a distinct storage class under their own
	// root: each subject directory folds to a byte-verified projection (a stale
	// projection reds the gate, review-feedback-capture §3.1).
	fdirs, ferr := FeedbackDirs(root, cfg)
	if ferr != nil {
		defects = append(defects, def("fold", cfg.Feedback, "read", ferr.Error(), "the feedback root must be readable"))
	}
	for _, dir := range fdirs {
		entries, edef := ReadFeedbackEntries(dir)
		defects = append(defects, edef...)
		if len(entries) == 0 {
			continue
		}
		res := feedback.Fold(filepath.Base(dir), entries)
		defects = append(defects, res.Defects...)
		defects = append(defects, VerifyFeedbackGenerated(dir, res)...)
	}
	return applyDirectRewriteAuthorization(root, mergeBase, head, defects)
}

func applyDirectRewriteAuthorization(root, mergeBase, head string, defects []schema.Defect) []schema.Defect {
	return filterDirectRewriteAuthorization(rewriteauth.Verify(root, mergeBase, head), defects)
}

func filterDirectRewriteAuthorization(auth rewriteauth.Result, defects []schema.Defect) []schema.Defect {
	if !auth.Active {
		return defects
	}
	if auth.Err != nil {
		marker := auth.Marker
		if marker == "" {
			marker = rewriteauth.MarkerPath
		}
		return append(defects, def("direct-rewrite-authorization", marker, "verify",
			auth.Err.Error(), "restore the exact operator-authorized change set or remove the direct rewrite"))
	}
	kept := defects[:0]
	for _, defect := range defects {
		if !auth.AllowsDocgate(defect.Verb) {
			kept = append(kept, defect)
		}
	}
	return kept
}

// scanScope computes the install-epoch scan filter ONCE per gate run
// (existing-repo-onboarding-v2 §3.4, DINV-6): a single `git diff --name-only
// <epoch>` (the epoch commit against the working tree — committed AND
// uncommitted change in one call) intersected with each tree-local check's
// scan-set, so an adopted repository's pre-existing corpus is grandfathered and
// gate cost scales with the changed set, not the corpus (NFR-1). No epoch set →
// the zero scope, every path in scope (the un-adopted / self-host case,
// unchanged). An epoch set but UNREACHABLE (a shallow clone lacking the commit)
// FAILS CLOSED: the returned scope is inactive so the whole corpus is judged —
// never silently grandfathered (ABUSE-2) — and a defect names the cause. The
// per-check grandfather exception lists are independent and still apply.
func scanScope(root string, cfg config.Config) (ScanScope, []schema.Defect) {
	epoch := strings.TrimSpace(cfg.Epoch.Commit)
	if epoch == "" {
		return ScanScope{}, nil
	}
	out, err := exec.Command("git", "-C", root, "diff", "--name-only", epoch).Output()
	if err != nil {
		return ScanScope{}, []schema.Defect{def("check-epoch", filepath.FromSlash(".pyxis/config.yaml"), "epoch",
			fmt.Sprintf("the install epoch %s is unreachable (%v) — judging the whole corpus, not only post-epoch changes", short(epoch), err),
			"restore the epoch commit (e.g. `git fetch --unshallow`) or re-run `adopt` to reset the epoch")}
	}
	changed := map[string]bool{}
	for l := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			changed[filepath.ToSlash(l)] = true
		}
	}
	return ScanScope{epochActive: true, changed: changed}, nil
}

// corpusScanFiles is the tree-local content checks' scan-set: the governed
// markdown corpus — the docs tree (minus the reviews subtree when skipReviews),
// the root convention file, and the templates directory. When an install epoch
// scopes the run it returns only the paths changed since the epoch, gathered by
// iterating the changed set (NOT by walking the corpus), so the files-visited
// count is |changed ∩ scan-set|, invariant to corpus size (§3.4, NFR-1, A-8).
// With no active epoch the whole corpus is walked — byte-for-byte the pre-epoch
// behaviour. Absolute paths, sorted, either way.
func corpusScanFiles(root string, cfg config.Config, scan ScanScope, skipReviews bool) []string {
	if scan.epochActive {
		return scopedCorpusFiles(root, cfg, scan, skipReviews)
	}
	return walkCorpusFiles(root, cfg, skipReviews)
}

// walkCorpusFiles enumerates the whole governed corpus (the no-epoch path).
func walkCorpusFiles(root string, cfg config.Config, skipReviews bool) []string {
	var files []string
	reviewsDir := filepath.Join(root, filepath.FromSlash(cfg.Reviews))
	_ = filepath.WalkDir(filepath.Join(root, "docs"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipReviews && path == reviewsDir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err == nil {
		files = append(files, filepath.Join(root, "AGENTS.md"))
	}
	tdir := filepath.Join(root, filepath.FromSlash(cfg.TemplatesDir))
	if items, err := os.ReadDir(tdir); err == nil {
		for _, it := range items {
			if !it.IsDir() && strings.HasSuffix(it.Name(), ".md") {
				files = append(files, filepath.Join(tdir, it.Name()))
			}
		}
	}
	sort.Strings(files)
	return files
}

// scopedCorpusFiles returns the epoch's changed corpus files by iterating the
// changed set (cost ∝ the changed set), keeping only paths that (a) belong to
// the governed corpus and (b) still exist — a deletion is changed but has
// nothing to scan.
func scopedCorpusFiles(root string, cfg config.Config, scan ScanScope, skipReviews bool) []string {
	var files []string
	for rel := range scan.changed {
		if !inCorpus(rel, cfg, skipReviews) {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if fi, err := os.Stat(abs); err != nil || fi.IsDir() {
			continue
		}
		files = append(files, abs)
	}
	sort.Strings(files)
	return files
}

// inCorpus reports whether a repository-relative (slash) path is a member of
// the governed markdown corpus walkCorpusFiles would enumerate: a *.md under
// docs/ (minus the reviews subtree when skipReviews), the root convention file,
// or a *.md directly under the templates directory (non-recursive, as the walk
// reads it via ReadDir).
func inCorpus(rel string, cfg config.Config, skipReviews bool) bool {
	if rel == "AGENTS.md" {
		return true
	}
	tdir := filepath.ToSlash(cfg.TemplatesDir)
	if path.Dir(rel) == tdir && strings.HasSuffix(rel, ".md") {
		return true
	}
	if strings.HasPrefix(rel, "docs/") && strings.HasSuffix(rel, ".md") {
		if skipReviews {
			reviews := filepath.ToSlash(cfg.Reviews)
			if rel == reviews || strings.HasPrefix(rel, reviews+"/") {
				return false
			}
		}
		return true
	}
	return false
}
