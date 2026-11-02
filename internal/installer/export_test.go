package installer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// SetFailingSeedStageForTest swaps the package-level seed slot (t8Slots.Seed)
// for a stage that fails, returning a restore function. It lets the external
// test prove EVT-1 stays silent when a pipeline stage fails — the slot is a
// package var, so this set-and-restore is the sanctioned test seam without
// exposing the unexported stage types.
func SetFailingSeedStageForTest() (restore func()) {
	return SetFailingStageForTest("seed")
}

// SetFailingStageForTest swaps the named T8 pipeline slot ("seed" |
// "hooks" | "wiring" | "green-proof" | "welcome") for a
// stage that fails, returning a restore function — the general form beneath
// SetFailingSeedStageForTest, letting a rollback test (plan
// init-hardening/001 T4) inject a failure at any point in the tail and prove
// the unwind restores everything journaled up to that point.
func SetFailingStageForTest(name string) (restore func()) {
	return SetStageBodyForTest(name, func(context.Context, string) error {
		return fmt.Errorf("injected %s-stage failure (test)", name)
	})
}

// SetStageBodyForTest swaps the named T8 pipeline slot for a test-supplied
// body receiving the operand target root — the general seam beneath
// SetFailingStageForTest, letting a rollback test observe or mutate the tree
// (e.g. planting a foreign file) immediately before the stage fails.
func SetStageBodyForTest(name string, fn func(ctx context.Context, targetRoot string) error) (restore func()) {
	wrapped := func(ctx context.Context, run *initRun) error { return fn(ctx, run.opts.Target) }
	switch name {
	case "seed":
		old := t8Slots.Seed
		t8Slots.Seed = wrapped
		return func() { t8Slots.Seed = old }
	case "hooks":
		old := t8Slots.Hooks
		t8Slots.Hooks = wrapped
		return func() { t8Slots.Hooks = old }
	case "wiring":
		old := t8Slots.Wiring
		t8Slots.Wiring = wrapped
		return func() { t8Slots.Wiring = old }
	case "green-proof":
		old := t8Slots.GreenProof
		t8Slots.GreenProof = wrapped
		return func() { t8Slots.GreenProof = old }
	case "welcome":
		old := t8Slots.Welcome
		t8Slots.Welcome = wrapped
		return func() { t8Slots.Welcome = old }
	default:
		panic("installer: SetStageBodyForTest: unknown T8 stage " + name)
	}
}

// SetWireExecuteForTest swaps the T8 wiring dispatch seam so github-mode
// wiring runs a caller-supplied fake (a transcript runner) instead of the real
// gh CLI — the hermetic remote-but-owned bin (installer-v1 §3.11). fn receives
// exactly the declared operation table so a test can assert the executed set.
// Returns a restore function. (The out-of-package e2e matrix injects its fake
// through the production Options.WireRunner seam instead; this stays the
// in-package setter.)
func SetWireExecuteForTest(fn func(ctx context.Context, ops []wire.Op) (wire.Audit, error)) (restore func()) {
	old := wireExecute
	wireExecute = func(ctx context.Context, ops []wire.Op, _ wire.Runner, _ *slog.Logger, _ string, prior *wire.Audit) (wire.Audit, error) {
		audit, err := fn(ctx, ops)
		if prior != nil {
			audit.Operations = append(append([]wire.OpResult(nil), prior.Operations...), audit.Operations...)
			audit.Partial = prior.Partial || audit.Partial
			audit.HandOff = prior.HandOff + audit.HandOff
		}
		return audit, err
	}
	return func() { wireExecute = old }
}

func SetPublicationGitForTest(fn func(context.Context, string, ...string) (string, error)) (restore func()) {
	old := publicationGit
	publicationGit = fn
	return func() { publicationGit = old }
}
