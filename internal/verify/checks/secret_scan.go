package checks

import (
	"context"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
	"time"
)

// SecretScanCheckID is this check's registry and finding CheckID.
const SecretScanCheckID = "secret-scan"

func init() {
	register(schema.Check{
		ID:                SecretScanCheckID,
		Rung:              "0",
		AuthorIndependent: true,
		// The check reads only the proposal diff, never the whole tree — the
		// same honest input set design codegate-v1 §3.8 defaults rung 3 to,
		// for the identical reason (this check's logic never touches
		// anything outside the diff).
		InputSet: []string{"diff"},
	}, adaptEnvCheck(SecretScanCheckID, runSecretScan))
}

// runSecretScan matches ecosystem-table secret patterns (plus any
// manifest-declared extra patterns) against the proposal diff's ADDED lines
// only (design codegate-v1 §3.7): diff-scoped so a secret already sitting in
// the tree outside this diff is brownfield reality the baseline philosophy
// covers, not a fresh finding on every run.
func runSecretScan(ctx context.Context, env *Env) (*schema.CheckResult, []schema.Finding, error) {
	start := time.Now()
	if env.Diff == nil {
		return nil, nil, fmt.Errorf("%s: no diff set provided", SecretScanCheckID)
	}
	if env.Ecosystem == nil {
		return nil, nil, fmt.Errorf("%s: no ecosystem defaults provided", SecretScanCheckID)
	}

	patterns, err := env.Ecosystem.ResolveSecretPatterns(secretScanExtraPatterns(env.Manifest))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", SecretScanCheckID, err)
	}

	var findings []schema.Finding
	for _, f := range env.Diff.Files {
		for _, line := range f.AddedLines {
			matched := false
			for _, p := range patterns {
				if p.MatchString(line.Content) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			findings = append(findings, schema.Finding{
				CheckID: SecretScanCheckID,
				Class:   "secret",
				Rung:    "0",
				Path:    f.Path,
				Line:    line.Line,
				Message: "the proposal diff adds a line matching a secret pattern",
			})
		}
	}

	result := &schema.CheckResult{
		CheckID:  SecretScanCheckID,
		State:    "pass",
		Duration: schema.DurationMS(time.Since(start)),
	}
	if len(findings) > 0 {
		result.State = "fail"
		result.Message = fmt.Sprintf("%d line(s) in the proposal diff match a secret pattern", len(findings))
	}
	return result, findings, nil
}

// secretScanExtraPatterns reads the manifest's secret_scan extension point
// (schema.SecretScanConfig), tolerating a nil manifest or an absent section.
func secretScanExtraPatterns(m *schema.Manifest) []string {
	if m == nil || m.SecretScan == nil {
		return nil
	}
	return m.SecretScan.ExtraPatterns
}
