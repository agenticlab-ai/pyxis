package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// ForbiddenArtifactsCheckID is this check's registry and finding CheckID.
const ForbiddenArtifactsCheckID = "forbidden-artifacts"

func init() {
	register(schema.Check{
		ID:                ForbiddenArtifactsCheckID,
		Rung:              "0",
		AuthorIndependent: true,
		InputSet:          []string{"diff"},
	}, adaptEnvCheck(ForbiddenArtifactsCheckID, runForbiddenArtifacts))
}

// runForbiddenArtifacts flags three diff-scoped hygiene floors over content
// the proposal diff actually introduces (design codegate-v1 §3.7): a
// committed binary, a file over the ecosystem/manifest size ceiling, and a
// baked absolute filesystem path in added content. A deletion or a
// content-preserving rename introduces nothing new and is skipped —
// FileChange.ContentIntroduced is the single gate for all three floors, so a
// move/rename refactor never trips them.
func runForbiddenArtifacts(ctx context.Context, env *Env) (*schema.CheckResult, []schema.Finding, error) {
	start := time.Now()
	if env.Diff == nil {
		return nil, nil, fmt.Errorf("%s: no diff set provided", ForbiddenArtifactsCheckID)
	}
	if env.Ecosystem == nil {
		return nil, nil, fmt.Errorf("%s: no ecosystem defaults provided", ForbiddenArtifactsCheckID)
	}

	var cfg *schema.ForbiddenArtifactsConfig
	if env.Manifest != nil {
		cfg = env.Manifest.ForbiddenArtifacts
	}
	rules, err := env.Ecosystem.ResolveForbiddenArtifactRules(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", ForbiddenArtifactsCheckID, err)
	}

	var findings []schema.Finding
	for _, f := range env.Diff.Files {
		if !f.ContentIntroduced() {
			continue
		}

		if f.Binary {
			findings = append(findings, schema.Finding{
				CheckID: ForbiddenArtifactsCheckID,
				Class:   "forbidden-binary",
				Rung:    "0",
				Path:    f.Path,
				Message: "the proposal diff commits a binary file",
			})
		}

		// Size is diff-scoped to a NEWLY-ADDED file: the floor blocks committing
		// a new oversized artifact, not a one-line edit to a pre-existing large
		// file (a >1 MiB go.sum / vendored / generated file). Keying on
		// whole-file size for a modify would red a legitimate diff over bulk the
		// diff never introduced (R1 red-blindness).
		if f.Status == diffset.StatusAdded && rules.MaxFileBytes > 0 && f.SizeBytes > rules.MaxFileBytes {
			findings = append(findings, schema.Finding{
				CheckID: ForbiddenArtifactsCheckID,
				Class:   "forbidden-size",
				Rung:    "0",
				Path:    f.Path,
				Message: fmt.Sprintf("file is %d bytes, over the %d-byte forbidden-artifact ceiling", f.SizeBytes, rules.MaxFileBytes),
			})
		}

		for _, line := range f.AddedLines {
			for _, p := range rules.AbsolutePathPatterns {
				if !p.MatchString(line.Content) {
					continue
				}
				findings = append(findings, schema.Finding{
					CheckID: ForbiddenArtifactsCheckID,
					Class:   "forbidden-path",
					Rung:    "0",
					Path:    f.Path,
					Line:    line.Line,
					Message: "the proposal diff bakes in an absolute filesystem path",
				})
				break
			}
		}
	}

	result := &schema.CheckResult{
		CheckID:  ForbiddenArtifactsCheckID,
		State:    "pass",
		Duration: schema.DurationMS(time.Since(start)),
	}
	if len(findings) > 0 {
		result.State = "fail"
		result.Message = fmt.Sprintf("%d forbidden-artifact finding(s) in the proposal diff", len(findings))
	}
	return result, findings, nil
}
