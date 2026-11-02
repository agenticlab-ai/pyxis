package converge

// The archive-backed engine-verbatim repair and the generated-class write —
// extracted from Run's loop into independent callables (T1;
// existing-repo-onboarding-v2 §3.1, DINV-2) so a caller (adopt) can drive
// either class's writes for a chosen path set from a classification and the
// verified release archive, without invoking Run. Run itself calls exactly
// these two functions; its steady-state behavior is unchanged (NR-2) — this
// is a pure extraction, not a new write path.

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

const authorityWriteRefusal = "the live Argus authority contract is repository-owned; release metadata cannot authorize creating or replacing operator policy"

// RepairEngineVerbatim writes byte-exact repairs for every engine-verbatim
// finding in findings — any other class present is ignored, so a caller may
// pass a full Classification.Findings or a narrowed, caller-chosen path set
// (e.g. adopt's plan entries) — reading from the verified payload archive at
// archivePath. Each candidate file is re-hashed against finding.WantSHA256
// (the manifest's pin, carried by the classification) BEFORE any write: an
// archive that carries no such file, or whose bytes hash to something else,
// is a named refusal, never a write (DINV-1) — the pinned release metadata
// is defective, not the repo tree.
//
// Returns the repaired/refused sets in findings' order. A non-nil error means
// a write itself failed (or the context died); the sets returned alongside it
// are exactly what completed before the failure — mirroring Run's per-file
// atomicity (a killed caller leaves only converged-or-untouched files).
func RepairEngineVerbatim(ctx context.Context, root string, findings []Finding, archivePath string) ([]Action, []Refusal, error) {
	want := map[string]bool{}
	for _, f := range findings {
		if f.Class == relmanifest.EngineVerbatim && f.Path != payload.AuthorityContractTarget {
			want[f.Path] = true
		}
	}
	var extracted map[string]payloadEntry
	if len(want) != 0 {
		var err error
		extracted, err = extractFiles(archivePath, want)
		if err != nil {
			return nil, nil, err
		}
	}

	var actions []Action
	var refusals []Refusal
	for _, f := range findings {
		if err := ctx.Err(); err != nil {
			return actions, refusals, err
		}
		if f.Class != relmanifest.EngineVerbatim {
			continue
		}
		if f.Path == payload.AuthorityContractTarget {
			refusals = append(refusals, Refusal{Path: f.Path, Class: f.Class, Reason: authorityWriteRefusal})
			continue
		}
		ent, ok := extracted[f.Path]
		if !ok {
			refusals = append(refusals, Refusal{Path: f.Path, Class: f.Class,
				Reason: "the verified payload archive carries no such file — the pinned release metadata is defective; pin a coherent release"})
			continue
		}
		if got := sha256Hex(ent.b); got != f.WantSHA256 {
			refusals = append(refusals, Refusal{Path: f.Path, Class: f.Class,
				Reason: fmt.Sprintf("payload archive bytes hash %s but the manifest pins %s — the pinned release metadata is defective; nothing was written (fail closed)", got, f.WantSHA256)})
			continue
		}
		if err := writeTreeAtomic(treePath(root, f.Path), ent.b, ent.mode()); err != nil {
			return actions, refusals, fmt.Errorf("repair %s: %w", f.Path, err)
		}
		actions = append(actions, Action{Path: f.Path, Class: f.Class})
	}
	return actions, refusals, nil
}

// WriteGenerated writes desired[finding.Path] for every generated-class
// finding in findings — any other class present is ignored, so a caller may
// pass a full Classification.Findings or a narrowed, caller-chosen path set.
// desired is the classification's desired-bytes map (Classification.Generated)
// — the SAME bytes the classification compared the tree against, never a
// second render. A finding already named StateUngeneratable is a refusal,
// never a write (its bytes could not be computed at classification time).
//
// Returns the written/refused sets in findings' order. A non-nil error means
// a write itself failed (or the context died); the sets returned alongside it
// are exactly what completed before the failure.
func WriteGenerated(ctx context.Context, root string, findings []Finding, desired map[string][]byte) ([]Action, []Refusal, error) {
	var actions []Action
	var refusals []Refusal
	for _, f := range findings {
		if err := ctx.Err(); err != nil {
			return actions, refusals, err
		}
		if f.Class != relmanifest.Generated {
			continue
		}
		if f.Path == payload.AuthorityContractTarget {
			refusals = append(refusals, Refusal{Path: f.Path, Class: f.Class, Reason: authorityWriteRefusal})
			continue
		}
		if f.State == StateUngeneratable {
			refusals = append(refusals, Refusal{Path: f.Path, Class: f.Class, Reason: f.Detail})
			continue
		}
		if err := writeTreeAtomic(treePath(root, f.Path), desired[f.Path], 0o644); err != nil {
			return actions, refusals, fmt.Errorf("regenerate %s: %w", f.Path, err)
		}
		actions = append(actions, Action{Path: f.Path, Class: f.Class})
	}
	return actions, refusals, nil
}
