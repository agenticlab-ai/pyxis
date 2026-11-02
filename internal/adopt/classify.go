package adopt

// Classification → plan entries (design existing-repo-onboarding-v2 §3.1):
// adopt consumes converge's ownership-class classification as the single
// source of tree-vs-manifest truth (AP-11; DINV-2) — no second classifier.
// entriesFromFindings maps each converge.Finding onto the plan's own class
// enum; neverTouchEntries derives the never-touch set as the manifest's
// complement (a property, not a second list: design-brainstorm:
// existing-repo-onboarding-tech-design#F-3) over the tracked file set at a
// bounded, top-level-segment granularity, so its cost is a function of the
// repository's top-level breadth, never its full corpus size.
//
// Neither function resolves a collision on its own: chain and collide-manual
// arrive only from the hook-path and root convention-file detectors T5/T7
// add on top of this baseline pass (design §3.3). The one collision this
// pass CAN carry through unresolved is the installed workflow's fixed
// target — additive-adjacent, not a new class — which workflow.go's
// ResolveWorkflowEntries (T6) redirects to a distinct filename when that
// target already names a file. Every other path this pass finds
// engine-verbatim or generated is planned as such regardless of state
// (missing or modified): those remain converge's own reserved, byte-pinned
// namespace, never a collision surface a third party plausibly already
// owns.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// ErrAuthoritySurfaceInvalid refuses unhealthy repository-owned authority
// before drafting or applying adoption. Only the operator may establish or
// correct delegated policy; adoption never creates or modifies it.
var ErrAuthoritySurfaceInvalid = errors.New("adopt: repository-owned Argus authority needs operator repair; adoption never creates or modifies delegated policy")

// entriesFromFindings maps converge's classification findings onto the
// plan's acted-on entries: engine-verbatim and generated findings carry
// straight through regardless of State — including a generated finding
// Drift names StateUngeneratable purely because its read-only face has no
// archive to render a template-sourced generated file from (converge.go's
// own documented fresh-tree asymmetry); apply (T8) drives the SAME class
// through converge's write primitives WITH archive access, where the same
// path renders cleanly. A missing repo-owned path is new content adopt will
// add (e.g. .pyxis/config.yaml on a tree that has none yet), except delegated
// authority, which only the operator may establish. A repo-owned
// finding in any OTHER state means the existing surface cannot be trusted —
// UB-7's refusal (in practice unreachable here for .pyxis/config.yaml, whose
// validity Inspect already gates before classifying, but named defensively
// for any other repo-owned path a future manifest might list).
func entriesFromFindings(findings []converge.Finding) ([]Entry, error) {
	entries := make([]Entry, 0, len(findings))
	for _, f := range findings {
		switch f.Class {
		case relmanifest.EngineVerbatim:
			entries = append(entries, Entry{Path: f.Path, Class: ClassEngineVerbatim})
		case relmanifest.Generated:
			entries = append(entries, Entry{Path: f.Path, Class: ClassGenerated})
		case relmanifest.RepoOwned:
			if f.Path == payload.AuthorityContractTarget {
				return nil, fmt.Errorf("%w: %s: %s", ErrAuthoritySurfaceInvalid, f.Path, f.Detail)
			}
			if f.State != converge.StateMissing {
				return nil, fmt.Errorf("adopt: %s: %w: %s", f.Path, ErrConfigSurfaceInvalid, f.Detail)
			}
			entries = append(entries, Entry{Path: f.Path, Class: ClassAdd})
		}
	}
	return entries, nil
}

// neverTouchEntries derives the never-touch set as the manifest's complement
// over tracked (the pre-adoption `git ls-files` list): for every top-level
// path segment tracked has that no manifest-listed path also begins with,
// emit one entry — "name/**" for a segment that is a directory (any tracked
// path nests under it), the bare name for a segment that is itself a
// tracked file. This is deliberately coarse (bounded by the tree's top-level
// breadth, not its full file count — NFR-1's cost posture): a partially
// manifest-touched top-level directory (e.g. docs/, when the manifest only
// lists docs/tech-design/index.md) is excluded wholesale rather than
// finding its own untouched leaves, since the never-touch GUARANTEE is
// enforced structurally (apply only ever writes a path an entry names,
// §3.1) — this set is the reviewable summary of that guarantee, not the
// mechanism itself.
func neverTouchEntries(manifest []relmanifest.PayloadFile, tracked []string) []Entry {
	touched := make(map[string]bool, len(manifest))
	for _, pf := range manifest {
		touched[topSegment(pf.Path)] = true
	}

	var entries []Entry
	seen := make(map[string]bool)
	for _, t := range tracked {
		seg := topSegment(t)
		if touched[seg] || seen[seg] {
			continue
		}
		seen[seg] = true
		entries = append(entries, Entry{Path: neverTouchLabel(t, seg), Class: ClassNeverTouch})
	}
	return entries
}

// topSegment returns p's first slash-separated path segment.
func topSegment(p string) string {
	segment, _, _ := strings.Cut(p, "/")
	return segment
}

// neverTouchLabel renders seg as a bare filename when fullPath IS seg (a
// top-level tracked file), else as the "seg/**" directory glob — the same
// spelling the adoption-plan schema's own fixtures use for a never-touch
// entry (internal/adopt/testdata/golden-plan.json's "src/**").
func neverTouchLabel(fullPath, seg string) string {
	if fullPath == seg {
		return seg
	}
	return seg + "/**"
}
