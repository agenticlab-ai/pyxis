package release

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// The release preconditions (installer-v1 §3.10): the build REFUSES to
// publish while the staged payload would ship a repo that cannot work
// installed — the green-merge-is-not-a-working-artifact lesson, checked
// mechanically. Each precondition is named, fail-closed, and asserted by
// Pack before publication:
//
//	a. retired-resolver-references  — no staged file references a retired
//	   per-tool shell resolver (CheckRetiredResolvers).
//	b. workflow-toolchain-steps     — no staged operand workflow carries a
//	   source-toolchain step (CheckWorkflowToolchainFree): an installed
//	   repo has no engine source to set up, vet, or test.
//	c. every-staged-file-classes    — relmanifest.Walk enforces DINV-6
//	   during manifest derivation; an unclassed file fails Pack by name.
//
// PreconditionError carries the name plus every finding, so one run names
// every problem (the accumulate-defects house style).
type PreconditionError struct {
	Name     string   // the precondition's stable name (above)
	Findings []string // one line per violation, sorted
}

func (e *PreconditionError) Error() string {
	return fmt.Sprintf("release precondition %s failed (installer-v1 §3.10) — refusing to pack:\n  %s",
		e.Name, strings.Join(e.Findings, "\n  "))
}

// RetiredResolvers names the per-tool shell resolver scripts the installer
// design retires (installer-v1 §3.10 "Shims"): after the T12 migration every
// payload call site speaks the exec grammar (`pyxis exec <tool> -- …`), and
// these files survive only as one-line delegates. A staged file that still
// REFERENCES one of them would ship an operand invoking a resolver whose
// fetch/parse logic is gone — the drift class two of these resolvers already
// exhibit today (they verify nothing). The task charter names three;
// codegate-resolve.sh is the fourth (the same per-tool resolver class T12
// collapses), and code-gate-resolve.sh the fifth — the code-gate mirror's
// source-mode pre-push shim (code-gate-mirror.md §3.6), the same class.
var RetiredResolvers = []string{
	"argus-stamp-resolve.sh",
	"code-gate-resolve.sh",
	"codegate-resolve.sh",
	"docgate-resolve.sh",
	"feedback-capture-resolve.sh",
}

// CheckRetiredResolvers scans every staged regular file's CONTENT for the
// retired resolver names. The retired files themselves are exempt from the
// scan (matching by basename): post-T12 they remain in the payload as
// one-line exec-grammar delegates, and a file does not "reference" itself by
// carrying its own header — every OTHER staged file must be silent about
// them (hooks, workflows, skills, scripts, tests alike).
func CheckRetiredResolvers(stagedRoot string) error {
	var findings []string
	err := walkStagedFiles(stagedRoot, func(rel string, content []byte) {
		if isRetiredResolver(rel) {
			return
		}
		for _, name := range RetiredResolvers {
			if bytes.Contains(content, []byte(name)) {
				findings = append(findings, fmt.Sprintf("%s references retired resolver %s (T12 migrates call sites to the exec grammar)", rel, name))
			}
		}
	})
	if err != nil {
		return err
	}
	if len(findings) > 0 {
		sort.Strings(findings)
		return &PreconditionError{Name: "retired-resolver-references", Findings: findings}
	}
	return nil
}

func isRetiredResolver(rel string) bool {
	base := filepath.Base(rel)
	return slices.Contains(RetiredResolvers, base)
}

// toolchainPatterns are the source-toolchain markers no operand workflow may
// carry (installer-v1 §3.10: "no Go setup, vet, or race suite" — an
// installed repo has no engine source). Patterns are data; matching is a
// plain substring scan over each staged workflow's bytes.
var toolchainPatterns = []string{"setup-go", "go vet", "go test", "gofmt"}

// CheckWorkflowToolchainFree scans every staged file under the operand
// workflow directory (.github/workflows/ — the overlay-mapped set) for
// source-toolchain steps.
func CheckWorkflowToolchainFree(stagedRoot string) error {
	dir := filepath.Join(stagedRoot, filepath.FromSlash(overlayDstDir))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil // no mapped workflows staged — nothing to scan
	}
	if err != nil {
		return fmt.Errorf("release: read staged workflows %s: %w", overlayDstDir, err)
	}
	var findings []string
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		rel := overlayDstDir + "/" + e.Name()
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("release: read staged workflow %s: %w", rel, err)
		}
		for _, pat := range toolchainPatterns {
			if bytes.Contains(content, []byte(pat)) {
				findings = append(findings, fmt.Sprintf("%s carries source-toolchain step %q — the installed-repo variant runs the gate through the exec grammar only", rel, pat))
			}
		}
	}
	if len(findings) > 0 {
		sort.Strings(findings)
		return &PreconditionError{Name: "workflow-toolchain-steps", Findings: findings}
	}
	return nil
}

// walkStagedFiles visits every regular file under root with its
// payload-relative slash path and content, in lexical walk order.
func walkStagedFiles(root string, visit func(rel string, content []byte)) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return fmt.Errorf("release: relativize %s: %w", p, err)
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("release: read staged %s: %w", rel, err)
		}
		visit(filepath.ToSlash(rel), content)
		return nil
	})
}
