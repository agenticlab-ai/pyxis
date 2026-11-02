// Package schema: incremental.go carries T8's addition to the verdict shape
// (design codegate-v1 §3.8) — CarriedPass, the per-rung provenance a rung
// gets when its result is spliced forward from a prior verdict instead of
// re-executed, and LoadVerdict, the verdict-artifact reader `--incremental`
// needs to find that prior verdict in the first place.
package schema

import (
	"encoding/json"
	"fmt"
	"os"
)

// CarriedPass marks a rung whose result this run did not execute: it was
// copied forward from a prior verdict because that rung had already recorded
// a pass and its registry-declared input set did not change under the fix's
// own diff (design codegate-v1 §3.8, DINV-8). ladder.PlanIncremental is the
// sole writer of this type.
type CarriedPass struct {
	FromRunID string `json:"from_run_id"`
}

// LoadVerdict reads and parses a verdict artifact from path. Unlike
// UnmarshalManifest, parsing is lenient (no DisallowUnknownFields): a verdict
// is a machine-written artifact, never hand-authored, and an incremental
// invocation's own qualification rule (matching codegate_version, among
// other bindings) is what actually gates whether an older or newer shape may
// be trusted — a strict parse here would only turn a version skew DINV-8
// already handles into a second, confusing failure mode. Any error here —
// missing file, invalid JSON, incomplete bindings — is the caller's cue to
// silently fall back to a full run: DINV-8 treats "no prior verdict" and "a
// prior verdict that doesn't parse" as the identical case.
func LoadVerdict(path string) (*Verdict, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load verdict: %w", err)
	}
	var v Verdict
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("load verdict: parse: %w", err)
	}
	if err := v.Bindings.BindingsPresent(); err != nil {
		return nil, fmt.Errorf("load verdict: %w", err)
	}
	if v.RunID == "" {
		return nil, fmt.Errorf("load verdict: missing run_id")
	}
	return &v, nil
}
