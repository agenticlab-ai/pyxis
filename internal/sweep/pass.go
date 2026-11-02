// This file owns the maintenance sweep's one bounded pass (maintenance-sweep-v1
// §3.1): the fixed-order composition the thin main delegates to. One invocation
// is one pass — pin the evaluated state (HEAD + dirty), load the closed-parse
// policy, run the three detectors in fixed order each isolated, and assemble the
// one dual-face report. The pass fails closed: an unreadable repository state or
// a malformed policy short-circuits to a named pass-level fault with zero
// detector output, and a detector fault is named in-band while the remaining
// detectors complete (DINV-3, DINV-4). The pass performs no write beyond the
// report the caller emits (DINV-1). Stdlib only save the shared config surface.
package sweep

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Run executes one maintenance sweep pass over the checkout at root and returns
// the one report the caller renders to its two faces. runner is the toolchain
// seam the detectors' go/git reads flow through; a nil runner defaults to the
// production ExecRunner. Run never returns an error: every fault is named inside
// the report (pass-level in Report.Faults, detector-level in the DetectorResult)
// and read out by the pinned exit ranking — the fail-loud-in-band contract
// (DINV-3, DINV-4).
func Run(ctx context.Context, root string, runner Runner) Report {
	if runner == nil {
		runner = ExecRunner{}
	}

	// Pin the evaluated state first. An unreadable repository state is a
	// pass-level fault that certifies nothing — no detector runs, and the
	// report names the fault with an empty commit (DINV-3).
	state, err := evaluatedState(ctx, root, runner)
	if err != nil {
		return NewReport(EvaluatedState{}, nil, fmt.Sprintf("unreadable repository state: %v", err))
	}

	// Load the shared config surface (capability paths + rollup keys the
	// staleness detector reads) and the closed-parse policy. A malformed config
	// or policy — an unknown key, a wrong schema version, a reasonless waiver —
	// is a pass-level fault with zero detector output (prd UB-2, DINV-3).
	cfg, cfgDefects := config.Load(root)
	policy, polDefects := LoadPolicyFile(root, cfg.MaintenancePolicy)
	if faults := defectFaults(cfgDefects, polDefects); len(faults) > 0 {
		return NewReport(state, nil, faults...)
	}

	// The evaluated state the detectors are pure functions of (§3.2).
	st := State{Root: root, Config: cfg, Runner: runner}

	// Run the three detectors in the fixed canonical order, each isolated: a
	// panic in one is caught and named as that detector's fault, never crashing
	// the pass or masking the others (DINV-4). Findings/faults are named in-band.
	detectors := []DetectorResult{
		isolate(DetectorUnwiredPackages, func() DetectorResult { return DetectUnwired(ctx, st, policy) }),
		isolate(DetectorTouchesUnderdelivery, func() DetectorResult { return DetectTouchesUnderdelivery(root, policy) }),
		isolate(DetectorRollupStaleness, func() DetectorResult { return DetectRollupStaleness(ctx, st, policy) }),
	}

	return NewReport(state, detectors)
}

// evaluatedState pins the exact repository state the pass evaluated: HEAD's
// commit and whether the working tree is dirty, both read through the runner
// seam so the pin is deterministic and the pass touches no subprocess it does
// not own. An empty HEAD or any git error is returned so the caller names the
// unreadable-state fault (§3.3).
func evaluatedState(ctx context.Context, root string, runner Runner) (EvaluatedState, error) {
	head, err := runner.Run(ctx, root, "git", "rev-parse", "HEAD")
	if err != nil {
		return EvaluatedState{}, err
	}
	if len(head) == 0 {
		return EvaluatedState{}, fmt.Errorf("git rev-parse HEAD returned no commit")
	}

	// --porcelain is the stable, locale-independent dirty signal: any line means
	// the tree carries uncommitted or untracked changes (§3.3's dirty annotation).
	status, err := runner.Run(ctx, root, "git", "status", "--porcelain")
	if err != nil {
		return EvaluatedState{}, err
	}
	return EvaluatedState{Commit: head[0], Dirty: len(status) > 0}, nil
}

// defectFaults flattens the config and policy parse defects into pass-level
// fault strings in a fixed order (config before policy, each in file order) so
// the report's fault list is a pure function of the input (DINV-2).
func defectFaults(groups ...[]schema.Defect) []string {
	var out []string
	for _, g := range groups {
		for _, d := range g {
			out = append(out, d.String())
		}
	}
	return out
}

// isolate runs one detector, converting a panic into that detector's named
// in-band fault so a single detector can never crash the pass or hide the
// others — the isolation §3.1 requires and DINV-4 contracts. Detectors already
// name their expected faults in-band; this catches only the unexpected.
func isolate(name string, fn func() DetectorResult) (res DetectorResult) {
	defer func() {
		if r := recover(); r != nil {
			res = DetectorResult{Name: name, Fault: fmt.Sprintf("%s: panic: %v", name, r)}
		}
	}()
	return fn()
}
