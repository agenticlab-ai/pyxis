package gate

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "check-single-amendment",
		Help:           "a PR adds at most one ledger entry vs --merge-base",
		NeedsMergeBase: true,
		Run:            func(c Ctx) Defects { return CheckSingleAmendment(c.Root, c.MergeBase, c.Head, c.Cfg) },
	})
}

// CheckSingleAmendment fails a diff that adds more than one PRD ledger entry
// (FR-27): "one concern per PR" (AGENTS.md) made code for the ledger — one
// amendment, or one new base, per change. Only Added NNN-<slug>.md files one
// directory below the PRDs root count; the generated effective.* projection,
// the flat placeholder corpus (NG-1), and modifications of existing entries
// (already fenced by check-immutable) do not.
func CheckSingleAmendment(root, mergeBase, head string, cfg config.Config) []schema.Defect {
	args := append([]string{"-C", root, "diff", "--diff-filter=A", "--name-only", "-z"}, diffSpan(mergeBase, head)...)
	args = append(args, "--", filepath.FromSlash(cfg.PRDs))
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return []schema.Defect{def("check-single-amendment", cfg.PRDs, "git",
			fmt.Sprintf("git diff against %q failed: %v", mergeBase, err),
			"pass a resolvable --merge-base ref (e.g. origin/main)")}
	}
	prdsRoot := filepath.Base(cfg.PRDs)
	var added []string
	// -z yields NUL-separated, unquoted paths. Without it, git octal-escapes
	// and double-quotes any non-ASCII path (core.quotepath), so filepath.Base
	// keeps a trailing quote, entryRe misses it, and the entry goes silently
	// uncounted — a fail-open guard bypass.
	for _, path := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if path == "" {
			continue
		}
		// A ledger entry is <prds>/<feature>/NNN-<slug>.md — exactly one
		// directory below the PRDs root. The flat placeholder corpus (NG-1,
		// directly under <prds>) and the effective.* projection are excluded.
		if entryRe.MatchString(filepath.Base(path)) && filepath.Base(filepath.Dir(filepath.Dir(path))) == prdsRoot {
			added = append(added, path)
		}
	}
	if len(added) <= 1 {
		return nil
	}
	sort.Strings(added)
	return []schema.Defect{def("check-single-amendment", cfg.PRDs, "diff",
		fmt.Sprintf("PR adds %d ledger entries (%s) — one amendment per PR", len(added), strings.Join(added, ", ")),
		"split the change so each PR adds exactly one ledger entry; land the amendments in separate PRs")}
}
