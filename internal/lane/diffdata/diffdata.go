// Package diffdata derives the classifier's one proposal-derived input — the
// whole diff's paths and its diff-derived change classes — from a (base, head)
// revision pair (merge-lane-v1 §3.1; plan T4's wiring). The derivation reads
// the diff alone, never author-declared metadata (DINV-2): paths come from
// `git diff --name-status`, and the two hard-gated change classes mirror the
// doc system's own merged-content semantics — `governing-doc-edit` for any
// change touching a governing document, `ledger-frozen-entry-edit` for a
// modify/delete/rename of a merged PRD ledger entry or technical design
// (check-immutable's rule, applied to the proposal's own base..head range).
// The consumed proposal contributes only this data; the binary and policy load
// from the trusted mainline context upstream (DINV-9).
package diffdata

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
)

// The two diff-derived change classes the policy's floor hard-gates
// (merge-lane-v1 §3.1's self-unlock minimum; policy.SelfUnlockMinClasses).
const (
	ClassGoverningDocEdit      = "governing-doc-edit"
	ClassLedgerFrozenEntryEdit = "ledger-frozen-entry-edit"
)

// entryRe recognizes a PRD ledger entry file (000-base.md, NNN-*.md) — the
// dense-numbered, merged-immutable class check-immutable guards.
var entryRe = regexp.MustCompile(`^\d{3}-.*\.md$`)

// Runner executes one git read and returns its stdout — the injectable seam
// hermetic tests substitute. The production runner shells the real git.
type Runner func(ctx context.Context, dir string, args ...string) ([]byte, error)

// gitRunner is the production Runner.
func gitRunner(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// Derive computes the classifier Diff of the base..head range under root using
// the production git runner. See DeriveWith.
func Derive(ctx context.Context, root, base, head string, cfg config.Config) (classifier.Diff, error) {
	return DeriveWith(ctx, gitRunner, root, base, head, cfg)
}

// DeriveWith computes the whole-diff paths and diff-derived classes of the
// base..head range. The paths are every file the range touches (added,
// modified, deleted, or either side of a rename — a rename contributes both
// names, so no perimeter is escaped by moving a file). The classes derive from
// the same listing:
//
//   - governing-doc-edit: any change (including an append — the class is
//     conservative, and the floor path-gates these files regardless) touching
//     the vision, the principles registry, or the incident registry.
//   - ledger-frozen-entry-edit: a modify/delete/rename of a PRD ledger entry
//     under the PRD root or of a technical design under the design root —
//     content that existed at base and merged content is immutable for
//     (an added file is new, not frozen, and does not carry the class).
//
// An unparsable listing line is an error, never a silent skip (UB-1's
// fail-closed posture: the caller routes HUMAN on any derivation fault).
func DeriveWith(ctx context.Context, run Runner, root, base, head string, cfg config.Config) (classifier.Diff, error) {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(head) == "" {
		return classifier.Diff{}, fmt.Errorf("deriving the diff: base and head are both required (got base=%q head=%q)", base, head)
	}
	out, err := run(ctx, root, "diff", "--name-status", "--no-renames", base, head)
	if err != nil {
		return classifier.Diff{}, fmt.Errorf("deriving the diff %s..%s: %w", base, head, err)
	}

	var d classifier.Diff
	governing := false
	frozen := false
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			return classifier.Diff{}, fmt.Errorf("deriving the diff %s..%s: unparsable name-status line %q", base, head, line)
		}
		status := fields[0]
		paths := fields[1:]
		d.Paths = append(d.Paths, paths...)
		for _, p := range paths {
			if isGoverningDoc(p, cfg) {
				governing = true
			}
		}
		// Only content that existed at base can be frozen; an addition is a new
		// entry, never an edit of a merged one.
		if status != "" && status[0] != 'A' {
			for _, p := range paths {
				if isFrozenSurface(p, cfg) {
					frozen = true
				}
			}
		}
	}
	if governing {
		d.Classes = append(d.Classes, ClassGoverningDocEdit)
	}
	if frozen {
		d.Classes = append(d.Classes, ClassLedgerFrozenEntryEdit)
	}
	return d, nil
}

// isGoverningDoc reports whether p is one of the governing documents whose
// every change carries the governing-doc-edit class: the vision, the
// principles registry, and the incident registry (append-only; an append still
// reads as a change of the file — conservative by design, and the floor
// path-gates it either way). All three resolve from the configured governing-doc
// paths, never a fixed literal (config-driven-portability-v1 §3.2, FR-1/FR-3;
// DINV-1) — on a default config the set is byte-identical to the former literals
// (DINV-2).
func isGoverningDoc(p string, cfg config.Config) bool {
	switch p {
	case cfg.ProductVision, cfg.ArchitecturePrinciples, cfg.Incidents:
		return true
	}
	return false
}

// isFrozenSurface reports whether p names merged-immutable ledger content: a
// dense-numbered PRD ledger entry under the PRD root, or a technical design
// (not the generated index) under the design root — check-immutable's own
// surface, derived here from the diff for the lane's change class.
func isFrozenSurface(p string, cfg config.Config) bool {
	if strings.HasPrefix(p, cfg.PRDs+"/") {
		base := p[strings.LastIndex(p, "/")+1:]
		if entryRe.MatchString(base) {
			return true
		}
	}
	if strings.HasPrefix(p, cfg.TechDesigns+"/") && strings.HasSuffix(p, ".md") {
		base := p[strings.LastIndex(p, "/")+1:]
		if !strings.HasPrefix(base, "index") {
			return true
		}
	}
	return false
}
