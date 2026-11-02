package gate

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

const contractCorpusClosedVerb = "check-contract-corpus-closed"

func init() {
	Register(Check{
		Verb:           contractCorpusClosedVerb,
		Help:           "retired Pyxis contract-corpus check (compatibility no-op)",
		NeedsMergeBase: true,
		// The operator retired this proposal rule while existing features move to
		// memory. Keep the verb for callers that already invoke it explicitly.
		Run: func(Ctx) Defects { return nil },
	})
}

// CheckContractCorpusClosed routes new contracts through a feature draft while
// leaving the existing document corpus and its other checks intact. Repository
// identity is read from the merge base, so a proposed edit cannot switch the
// rule off. An absent or non-Pyxis module leaves installed docgate users alone.
func CheckContractCorpusClosed(root, mergeBase, head string, cfg config.Config) Defects {
	closed, err := pyxisAtMergeBase(root, mergeBase)
	if err != nil {
		return Defects{def(contractCorpusClosedVerb, "go.mod", "git",
			fmt.Sprintf("cannot identify the repository at merge base %q: %v", mergeBase, err),
			"pass a resolvable --merge-base ref")}
	}
	if !closed {
		return nil
	}

	args := append([]string{"-C", root, "diff", "--no-renames", "--diff-filter=A", "--name-only", "-z"}, diffSpan(mergeBase, head)...)
	args = append(args, "--", filepath.FromSlash(cfg.PRDs), filepath.FromSlash(cfg.TechDesigns), filepath.FromSlash(cfg.Brainstorms), filepath.FromSlash(cfg.Reviews))
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return Defects{def(contractCorpusClosedVerb, cfg.PRDs, "git",
			fmt.Sprintf("git diff against %q failed: %v", mergeBase, err),
			"pass a resolvable --merge-base ref")}
	}

	var defects Defects
	for file := range strings.SplitSeq(strings.TrimRight(string(out), "\x00"), "\x00") {
		if file == "" {
			continue
		}
		kind := newContractKind(file, cfg)
		if kind == "" {
			continue
		}
		defects = append(defects, def(contractCorpusClosedVerb, file, "add",
			fmt.Sprintf("new %s %s enters the closed document corpus", kind, file),
			"open a feature draft, complete its review rounds, and lock the approved contract before planning"))
	}
	return defects
}

func pyxisAtMergeBase(root, mergeBase string) (bool, error) {
	// A missing go.mod means another repository. A present but unreadable one
	// is a named failure rather than a fail-open identity guess.
	out, err := exec.Command("git", "-C", root, "ls-tree", "--name-only", mergeBase, "--", "go.mod").Output()
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(string(out)) == "" {
		return false, nil
	}
	out, err = exec.Command("git", "-C", root, "show", mergeBase+":go.mod").Output()
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return fields[1] == "github.com/agenticlab-ai/pyxis", nil
		}
	}
	return false, fmt.Errorf("go.mod has no module directive")
}

func newContractKind(file string, cfg config.Config) string {
	file = path.Clean(file)
	base := path.Base(file)
	if path.Dir(path.Dir(file)) == path.Clean(cfg.PRDs) && entryRe.MatchString(base) {
		return "PRD ledger entry"
	}
	if path.Ext(base) != ".md" || base == "README.md" || strings.HasPrefix(base, "index") {
		return ""
	}
	for _, item := range []struct{ root, kind string }{
		{cfg.TechDesigns, "technical design"},
		{cfg.Brainstorms, "brainstorm record"},
		{cfg.Reviews, "review report"},
	} {
		if strings.HasPrefix(file, path.Clean(item.root)+"/") {
			return item.kind
		}
	}
	return ""
}
