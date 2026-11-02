// Package checks implements the v1 repo-verb check units — format, lint,
// build, and exec-probe (rungs 0, 1, and 4) — each a self-contained file that
// registers itself with the ladder's derived registry AND with this
// package's own executor lookup. That pairing is the one seam a new check's
// file touches (AP-27): registry.Register supplies the ladder-derivation
// metadata (rung, verb deps, author-independence, input set); this package's
// funcs map supplies what actually runs.
package checks

import (
	"context"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// RunContext is the single context value carrying everything a registered check
// may read (design distributed-check-registration §3.1). A verb check reads
// Workspace/Verbs/LogDir; a diff-based check reads Bindings/Manifest/Ecosystem/
// Diff — the ladder populates every field once so any check dispatches through
// the same Get(id), with no per-kind switch.
type RunContext struct {
	Workspace string
	Verbs     map[string]*schema.Verb // the resolved (ecosystem defaults + manifest overrides) verb set
	LogDir    string                  // absolute path; full verb output is written here, referenced by Finding.LogRef
	Bindings  schema.Bindings         // resolved run bindings (diff base, head sha, …) for diff-scoped checks
	Manifest  *schema.Manifest        // the parsed manifest
	Ecosystem *ecosystem.Defaults     // the compiled ecosystem defaults
	Diff      *diffset.DiffSet        // the proposal diff, extracted once by the ladder (nil if unavailable)
}

// Outcome is a check's contribution to the verdict: its own result plus zero
// or more findings. A clean pass carries no findings — findings exist only to
// explain a fail or error state (FR-2: compact feedback, not decoration).
type Outcome struct {
	Result   schema.CheckResult
	Findings []schema.Finding
	// Shrinkage is rung 2's observed-baseline-shrinkage count (design
	// codegate-v1 §3.6): the number of baseline entries this run did not
	// reproduce. Every other check leaves it zero; the ladder folds it onto
	// the RUNG (schema.Rung.Shrinkage), not this check result, since
	// shrinkage describes the no-new-failures evaluation as a whole rather
	// than any single check's pass/fail.
	Shrinkage int
}

// Func is the executable behind one registered check. The ladder calls it
// only when every declared verb dependency is bound; an unbound dependency is
// the ladder's own skip:verb-absent path (FR-6), never a check's concern.
type Func func(ctx context.Context, rc RunContext) Outcome

var (
	mu    sync.RWMutex
	funcs = map[string]Func{}
)

// register wires one check's metadata into the shared ladder registry and its
// executable into this package's lookup. registry.Register panics on a
// duplicate ID before funcs is ever touched, so the two maps can never
// disagree on which check IDs exist.
func register(meta schema.Check, fn Func) {
	registry.Register(&meta)
	mu.Lock()
	defer mu.Unlock()
	funcs[meta.ID] = fn
}

// Get returns the executable registered for id, or nil if none is registered.
func Get(id string) Func {
	mu.RLock()
	defer mu.RUnlock()
	return funcs[id]
}

// adaptEnvCheck bridges a diff-based check's natural RunFunc signature (it reads
// a pre-built Env and returns result/findings/error) to the uniform Func the
// ladder dispatches, so a diff check registers through the same register(meta,
// fn) seam as a verb check. A returned error becomes an error-class result
// (fail closed); the shared Env is built from the RunContext the ladder
// populated.
func adaptEnvCheck(id string, fn RunFunc) Func {
	return func(ctx context.Context, rc RunContext) Outcome {
		env := &Env{
			Workspace: rc.Workspace,
			Bindings:  rc.Bindings,
			Manifest:  rc.Manifest,
			Ecosystem: rc.Ecosystem,
			Diff:      rc.Diff,
		}
		result, findings, err := fn(ctx, env)
		if err != nil {
			return Outcome{Result: schema.CheckResult{CheckID: id, State: "error", Message: err.Error()}}
		}
		return Outcome{Result: *result, Findings: findings}
	}
}
