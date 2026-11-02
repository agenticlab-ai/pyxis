package gate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// ledgerHistoryVerb is the arm's verb — it names both the descriptor and the
// parse defects the shared parse emits on the arm's behalf, so a malformed
// ledger names this arm and not the sibling cross-document one.
const ledgerHistoryVerb = "check-ledger-history"

func init() {
	Register(Check{
		Verb:             ledgerHistoryVerb,
		Help:             "merged Argus ledger entries (A-n/E-n) are add-only vs --merge-base",
		NeedsMergeBase:   true,
		RunOnPyxisSource: true,
		Run:              func(c Ctx) Defects { return CheckLedgerHistory(c.Root, c.MergeBase, c.Cfg) },
	})
}

// CheckLedgerHistory is the Argus ledger-lint history arm
// (docs/tech-design/argus-ledger-lint.md §3.1): it compares each in-scope
// ledger's entry blocks keyed by anchor between the merge base and the working
// tree. Every A-n/E-n entry merged at the base must survive present-and-byte-
// identical; a changed block is an edit, an absent one a removal (FR-1). A new,
// strictly-higher-numbered anchor is a legal append, invisible to the
// comparison because it iterates the base anchors, not the working ones (FR-2)
// — and the delivery ledger's `## Derived state` region, which legally changes
// on every append, is never part of the anchor-keyed maps.
//
// It is base-requiring (NeedsMergeBase true): the umbrella skips it with no
// merge base, exactly as check-immutable degrades (FR-5, UB-1). Report-only —
// it never writes (DINV-4). The problems root the ledgers live under resolves
// from config (config-driven-portability-v1 §3.2, FR-1; default docs/problems).
func CheckLedgerHistory(root, mergeBase string, cfg config.Config) []schema.Defect {
	problems := filepath.ToSlash(filepath.Clean(cfg.Problems))
	baseFiles, err := lsTreeLedgers(root, mergeBase, cfg.Problems)
	if err != nil {
		return []schema.Defect{def(ledgerHistoryVerb, problems, "git",
			fmt.Sprintf("listing ledgers at %q failed: %v", mergeBase, err),
			"pass a resolvable --merge-base ref (e.g. origin/main)")}
	}
	var defects []schema.Defect
	for _, p := range baseFiles {
		class, ok := argusledger.ClassForPath(cfg.Problems, p)
		if !ok {
			continue
		}
		// The base content of a previously-merged ledger. A read failure here is
		// a git problem, not a ledger defect — surface it, do not fail silently.
		baseBytes, err := gitShow(root, mergeBase, p)
		if err != nil {
			defects = append(defects, def(ledgerHistoryVerb, p, "git",
				fmt.Sprintf("reading %s at %q failed: %v", p, mergeBase, err),
				"pass a resolvable --merge-base ref (e.g. origin/main)"))
			continue
		}
		baseLedger, bdef := argusledger.ParseAs(ledgerHistoryVerb, p, class, baseBytes)
		if baseLedger == nil {
			// A base that predates the guard may be malformed; it cannot
			// establish the merged-anchor set, so there is nothing to enforce
			// against and reporting it would false-red an untouched legacy base.
			// The working-tree parse below is where a malformed ledger is caught
			// (UB-3). Discard the base parse defects.
			_ = bdef
			continue
		}
		// The working-tree content. A missing file is a whole-ledger removal:
		// every merged anchor is a removal defect.
		wtBytes, werr := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if werr != nil {
			for _, id := range baseLedger.Order {
				defects = append(defects, removalDefect(p, id))
			}
			continue
		}
		wtLedger, wdef := argusledger.ParseAs(ledgerHistoryVerb, p, class, wtBytes)
		if wtLedger == nil {
			// A malformed working-tree ledger is a named parse defect (UB-3),
			// never a false green that would let a disturbed entry through.
			defects = append(defects, wdef...)
			continue
		}
		for _, id := range baseLedger.Order {
			wb, present := wtLedger.Entries[id]
			if !present {
				defects = append(defects, removalDefect(p, id))
				continue
			}
			if !bytes.Equal(baseLedger.Entries[id], wb) {
				defects = append(defects, editDefect(p, id))
			}
		}
	}
	return defects
}

func editDefect(path, anchor string) schema.Defect {
	return def(ledgerHistoryVerb, path, "history",
		fmt.Sprintf("merged entry %s#%s changed vs merge base — merged ledger entries are append-only", path, anchor),
		"revert the edit; evolve the ledger by appending a new higher-numbered entry instead")
}

func removalDefect(path, anchor string) schema.Defect {
	return def(ledgerHistoryVerb, path, "history",
		fmt.Sprintf("merged entry %s#%s removed or renumbered vs merge base — merged ledger entries are append-only", path, anchor),
		"restore the entry at its original anchor; a merged entry is never deleted or renumbered")
}

// lsTreeLedgers lists the in-scope Argus ledger paths tracked at the merge base
// (<problemsRoot>/*/problem.md and <problemsRoot>/*/delivery.md), repo-relative
// with forward slashes and stably sorted for deterministic output. The problems
// root is the configured location (default docs/problems).
func lsTreeLedgers(root, mergeBase, problemsRoot string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "ls-tree", "-r", "--name-only", "-z",
		mergeBase, "--", filepath.FromSlash(problemsRoot)).Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for p := range strings.SplitSeq(strings.TrimRight(string(out), "\x00"), "\x00") {
		if p == "" {
			continue
		}
		if _, ok := argusledger.ClassForPath(problemsRoot, p); ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, nil
}
