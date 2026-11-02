// Package ladder: incremental.go implements T8's incremental re-verification
// (design codegate-v1 §3.8, DINV-8): the prior verdict is the ONLY state a
// `--incremental` invocation may lean on, and it counts only when every
// DINV-1 binding except the head revision still matches. Qualification,
// rerun-set planning, and carried-pass execution all live here so Run's
// full-ladder path (ladder.go) stays exactly what it always was — run() is
// the one shared executor both paths drive, so a full and an incremental
// verdict can never disagree about how a rung that DOES execute is
// evaluated.
package ladder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// QualifiesForIncremental reports whether prior counts as DINV-8's
// incremental state against the current run's bindings: every DINV-1
// binding EXCEPT the head revision must match exactly, prior's own bindings
// must be complete, and prior must itself be a structurally valid verdict.
// A nil prior never qualifies — the caller's silent full-run fallback for a
// missing or unparseable prior verdict starts here (schema.LoadVerdict
// already collapses "missing" and "unparseable" to a nil/err return, and the
// caller passes that straight through as prior == nil).
func QualifiesForIncremental(prior *schema.Verdict, bindings schema.Bindings) bool {
	if prior == nil {
		return false
	}
	if err := prior.Bindings.BindingsPresent(); err != nil {
		return false
	}
	if err := prior.ValidateVerdictInvariants(); err != nil {
		return false
	}
	if prior.RunID == "" {
		return false
	}

	pb := prior.Bindings
	return pb.DiffBase == bindings.DiffBase &&
		pb.ManifestHash == bindings.ManifestHash &&
		pb.BaselineHash == bindings.BaselineHash &&
		pb.CodegateVersion == bindings.CodegateVersion &&
		pb.GitVersion == bindings.GitVersion
	// HeadSHA is deliberately excluded from the comparison: it is the one
	// binding a fix cycle is EXPECTED to move (DINV-8).
}

// RerunSet is T8's incremental plan: for every currently-registered rung,
// either it executes this run (Rerun), or its prior result is spliced into
// the new verdict unexecuted (Carry). The two maps are built together by
// PlanIncremental from the same per-rung decision, so they can never
// disagree about a given rung.
type RerunSet struct {
	Rerun map[string]bool
	Carry map[string]*schema.Rung
}

// PlanIncremental computes the rerun set (design codegate-v1 §3.8) for an
// ALREADY-qualified prior verdict (the caller runs QualifiesForIncremental
// first): the previously-failed rung and every rung not recorded pass
// (unreached or errored) always rerun; a rung recorded pass carries forward
// only when its registry-declared input set — the union of its checks'
// InputSet, "tree" if any check declares it, else "diff" — did not change
// under the fix itself.
//
// fixDeltaPaths is what `git diff --name-only <prior head> <new head>`
// reports — what the fix touched. newDiffPaths is the subset of
// fixDeltaPaths that was NOT already part of the proposal diff as of the
// prior run (diff_base..prior_head): a "diff"-tagged rung's honest
// composition signal per §3.8's conservative v1 default — a content edit to
// a file the diff already carried doesn't move it, a path newly entering the
// diff does. Content-level tracking within an already-diffed file is
// explicitly out of scope for v1 ("a future supersession by design" — the
// plan's Out of scope); DINV-8's binding match plus the previously-
// failed/not-reached rules are the primary safety net this default leans on.
func PlanIncremental(prior *schema.Verdict, rungs []*registry.Rung, fixDeltaPaths, newDiffPaths map[string]bool) *RerunSet {
	plan := &RerunSet{Rerun: map[string]bool{}, Carry: map[string]*schema.Rung{}}

	for _, r := range rungs {
		priorRung, ok := prior.Rungs[r.Name]
		if !ok || priorRung.Skipped || priorRung.State != "pass" || !checksMatchRegistry(r, priorRung) || inputSetChanged(r, fixDeltaPaths, newDiffPaths) {
			plan.Rerun[r.Name] = true
			continue
		}

		// The provenance run id must be where the rung ACTUALLY executed, not
		// merely the immediately preceding verdict: across a chain of several
		// --incremental invocations that all carry the same untouched rung
		// forward, prior itself may already be carrying it (priorRung.Carried
		// != nil). Stamping prior.RunID in that case would replace true
		// provenance with a hop that never ran the rung either — and since
		// scratch retention (design §3.4) guarantees only the current run's
		// scratch plus ONE prior verdict survive, that hop's own verdict may
		// already be reaped, making the reference permanently unresolvable.
		// Propagating the original FromRunID keeps it resolvable indefinitely.
		fromRunID := prior.RunID
		if priorRung.Carried != nil {
			fromRunID = priorRung.Carried.FromRunID
		}

		carried := *priorRung // safe to alias Checks: prior is freshly loaded and owned solely by this call
		carried.Carried = &schema.CarriedPass{FromRunID: fromRunID}
		carried.Duration = 0 // nothing executed this run
		plan.Carry[r.Name] = &carried
	}

	return plan
}

// inputSetChanged reports whether r's declared input set intersects what the
// fix changed. A "tree" rung (any check whose InputSet includes "tree" — a
// whole-repo command's honest scope, design §3.8) is invalidated by any
// non-empty fix delta; a "diff" rung is invalidated only when the fix
// introduced a path the proposal diff did not already carry.
func inputSetChanged(r *registry.Rung, fixDeltaPaths, newDiffPaths map[string]bool) bool {
	if len(fixDeltaPaths) == 0 {
		return false
	}
	if rungReadsWholeTree(r) {
		return true
	}
	return len(newDiffPaths) > 0
}

// rungReadsWholeTree reports whether any check registered on r declares a
// "tree" input set.
func rungReadsWholeTree(r *registry.Rung) bool {
	for _, c := range r.Checks {
		for _, in := range c.InputSet {
			if in == "tree" {
				return true
			}
		}
	}
	return false
}

// checksMatchRegistry reports whether priorRung's recorded checks are
// exactly the set the CURRENT registry declares for r — i.e. that splicing
// priorRung forward verbatim would still satisfy the ladder's own
// completeness invariant (DINV-4) against today's registry. A matching
// CodegateVersion binding (DINV-8) is meant to guarantee this by
// construction — same binary, same compiled-in registry — but codegate's own
// version constant is a fixed dev-string across many commits during active
// development (this repo dogfoods codegate on itself), so a prior verdict
// from a build whose registry has since gained or lost a check can still
// reach here with a qualifying binding set. Carrying such a rung forward
// unexecuted would leave the verdict missing a check ValidateCompleteness
// expects (or carrying a check the ladder no longer recognizes) and panic
// rather than run: "any doubt means a full run" applies at this per-rung
// granularity too, so a shape mismatch forces exactly this rung to rerun.
func checksMatchRegistry(r *registry.Rung, priorRung *schema.Rung) bool {
	if len(priorRung.Checks) != len(r.Checks) {
		return false
	}
	for _, c := range r.Checks {
		if _, ok := priorRung.Checks[c.ID]; !ok {
			return false
		}
	}
	return true
}

// RunIncremental is the `--incremental` entry point (design codegate-v1
// §3.8). It qualifies prior against bindings (DINV-8); on a nil prior, any
// binding mismatch, or any failure to compute the fix's own diff, it
// silently runs the identical full ladder Run would — --incremental never
// errors and never launders a stale result. On a qualifying prior it plans
// the rerun set from the fix's own diff (the prior verdict's head SHA to
// this run's head SHA) and executes only what the plan calls for, splicing
// every carried rung's prior result into the new verdict verbatim
// (provenance-stamped) in the same ladder order and first-failure-stop
// discipline Run enforces — a carried rung is always pass-class, so it can
// never itself trigger a stop.
func RunIncremental(ctx context.Context, workspace string, manifest *schema.Manifest, bindings schema.Bindings, prior *schema.Verdict) *schema.Verdict {
	if !QualifiesForIncremental(prior, bindings) {
		return Run(ctx, workspace, manifest, bindings, "full")
	}

	fixDeltaPaths, err := changedPaths(ctx, workspace, prior.Bindings.HeadSHA, bindings.HeadSHA)
	if err != nil {
		return Run(ctx, workspace, manifest, bindings, "full")
	}
	priorDiffPaths, err := changedPaths(ctx, workspace, prior.Bindings.DiffBase, prior.Bindings.HeadSHA)
	if err != nil {
		return Run(ctx, workspace, manifest, bindings, "full")
	}
	newDiffPaths := map[string]bool{}
	for p := range fixDeltaPaths {
		if !priorDiffPaths[p] {
			newDiffPaths[p] = true
		}
	}

	plan := PlanIncremental(prior, registry.GetLadder(), fixDeltaPaths, newDiffPaths)

	verdict := run(ctx, workspace, manifest, bindings, "incremental", plan)
	verdict.CarriedFrom = prior.RunID
	return verdict
}

// changedPaths returns the set of paths `git diff --name-only` reports
// between from and to, NUL-delimited so a path containing a newline can
// never merge with its neighbor. It mirrors the small, package-private git
// runners internal/verify/invoke and internal/verify/diffset each already
// keep for their own plumbing, rather than reaching across a package
// boundary for a two-line git invocation.
func changedPaths(ctx context.Context, workspace, from, to string) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", workspace, "diff", "--name-only", "-z", from, to)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff --name-only %s %s: %w: %s", from, to, err, strings.TrimSpace(stderr.String()))
	}

	paths := map[string]bool{}
	for _, p := range strings.Split(stdout.String(), "\x00") {
		if p != "" {
			paths[p] = true
		}
	}
	return paths, nil
}
