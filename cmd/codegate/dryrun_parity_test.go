//go:build dryrunparity

package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// localReferenceVerdict is a real `codegate run` verdict captured on a
// non-CI machine against a freshly built fixture-go workspace — the "local
// run" half of design codegate-v1's portability claim (§3.1, §3.11, A-9;
// prd:implementation-verification/FR-15, NFR-3). Captured 2026-07-19 by the
// identical procedure this test's headless half runs below. Re-capture it
// whenever a change to internal/verify or fixture-go could shift a rung's or
// check's state; a stale reference only ever shows up as a visible,
// non-blocking mismatch in the advisory dry-run workflow — this test never
// gates a merge (required-check registration stays parked, basis WR-5).
const localReferenceVerdict = "../../internal/verify/testdata/dryrun-parity/local-verdict.json"

// TestDryRunParity_MatchesLocalReference is EVT-5, the harness's own
// portability demonstration (A-9): it runs codegate's real CLI entry point
// headlessly, exactly as TestRunCmd_EndToEnd_GreenDegraded does, then diffs
// the resulting verdict RUNG-FOR-RUNG (and check-for-check) against
// localReferenceVerdict. Bindings, timing, and run identity are excluded by
// design — an independently produced run legitimately differs there (NFR-2's
// "byte-for-byte modulo timing" claim, generalized to modulo identity too:
// the CI run's fixture commit is necessarily a distinct commit from the
// local capture's). Gated behind the dryrunparity build tag so it never
// joins the default `go test ./...` the docs-gate CI context runs (NR-1) —
// the advisory workflow (.github/workflows/codegate-dry-run.yml) builds and
// runs it explicitly, continue-on-error, never a required check.
func TestDryRunParity_MatchesLocalReference(t *testing.T) {
	localData, err := os.ReadFile(localReferenceVerdict)
	if err != nil {
		t.Fatalf("read local reference verdict: %v", err)
	}
	var local schema.Verdict
	if err := json.Unmarshal(localData, &local); err != nil {
		t.Fatalf("unmarshal local reference verdict: %v", err)
	}

	ws := gitFixtureWorkspace(t, nil)
	code := runCmd([]string{"--workspace", ws, "--diff-base", "HEAD"})
	if code != invoke.ExitPass {
		t.Fatalf("headless run exit = %d, want %d (pass-class)", code, invoke.ExitPass)
	}
	ci := readVerdict(t, ws)

	mismatches := dryRunParityDiff(&local, ci)
	logEVT5(local.State, ci.State, mismatches)

	if len(mismatches) > 0 {
		t.Errorf("dry-run parity mismatch (local vs headless CI run):\n%s", strings.Join(mismatches, "\n"))
	}
}

// dryRunParityDiff compares two verdicts RUNG-FOR-RUNG: only the top-level
// state and each rung's/check's state must agree — see the test's own
// doc comment for why bindings/timing/identity are excluded. Returns one
// line per mismatch (a state divergence, or a rung/check present on only one
// side); nil means full parity.
func dryRunParityDiff(local, ci *schema.Verdict) []string {
	var diffs []string
	if local.State != ci.State {
		diffs = append(diffs, fmt.Sprintf("overall state: local=%s ci=%s", local.State, ci.State))
	}

	seen := map[string]bool{}
	for name, lr := range local.Rungs {
		seen[name] = true
		cr, ok := ci.Rungs[name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("rung %s: present locally, absent in CI", name))
			continue
		}
		diffs = append(diffs, diffRung(name, lr, cr)...)
	}
	for name := range ci.Rungs {
		if !seen[name] {
			diffs = append(diffs, fmt.Sprintf("rung %s: absent locally, present in CI", name))
		}
	}
	return diffs
}

// diffRung compares one rung's aggregate state and its checks' individual
// states between two verdicts.
func diffRung(name string, local, ci *schema.Rung) []string {
	var diffs []string
	if local.State != ci.State {
		diffs = append(diffs, fmt.Sprintf("rung %s state: local=%s ci=%s", name, local.State, ci.State))
	}
	seen := map[string]bool{}
	for id, lc := range local.Checks {
		seen[id] = true
		cc, ok := ci.Checks[id]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("rung %s check %s: present locally, absent in CI", name, id))
			continue
		}
		if lc.State != cc.State {
			diffs = append(diffs, fmt.Sprintf("rung %s check %s: local=%s ci=%s", name, id, lc.State, cc.State))
		}
	}
	for id := range ci.Checks {
		if !seen[id] {
			diffs = append(diffs, fmt.Sprintf("rung %s check %s: absent locally, present in CI", name, id))
		}
	}
	return diffs
}

// logEVT5 emits design codegate-v1's EVT-5 structured record (§3.4: "dry-run
// parity … land in the harness's own CI") as one JSON line the advisory
// workflow's log carries regardless of pass/fail, so a maintainer can see
// exactly which rung or check diverged without re-running anything.
func logEVT5(localState, ciState string, mismatches []string) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("EVT-5",
		"event", "codegate.dryrun_parity",
		"parity", len(mismatches) == 0,
		"local_state", localState,
		"ci_state", ciState,
		"mismatches", mismatches,
	)
}
