package checks

import (
	"context"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// Env is everything a diff-based check's run behavior may read: the resolved
// workspace and bindings, the parsed manifest, the compiled ecosystem defaults,
// and the proposal diff — extracted once by the ladder and shared across every
// check that needs it (design distributed-check-registration §3.1: "a single
// context value carrying everything a check may read"). A check reads only the
// fields its own logic uses. It is the diff-check view of the ladder's
// RunContext, built by adaptEnvCheck when the ladder dispatches the check.
type Env struct {
	Workspace string
	Bindings  schema.Bindings
	Manifest  *schema.Manifest
	Ecosystem *ecosystem.Defaults
	Diff      *diffset.DiffSet
}

// RunFunc is a diff-based check's natural signature: given the shared Env, it
// returns its CheckResult and any Findings, or an error the ladder maps to an
// error-class result. A check registers it through register(meta,
// adaptEnvCheck(id, fn)) so it dispatches through the same Get(id) as every
// other check — one seam, no per-kind switch.
type RunFunc func(ctx context.Context, env *Env) (*schema.CheckResult, []schema.Finding, error)
