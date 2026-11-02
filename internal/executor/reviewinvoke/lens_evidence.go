package reviewinvoke

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	implruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

const lensResultSchema = "impl-review-lens-result/v1"
const lensEvidenceSchema = "impl-review-lens-evidence/v1"
const lensFence = "```impl-review-lens\n"

var errLensAggregateInvalid = errors.New("lens evidence: aggregate handoff is unavailable or malformed")
var errLensPersistence = errors.New("lens evidence: native completion could not be retained")

func lensEvidenceFailureClass(err error) string {
	if errors.Is(err, errLensPersistence) {
		return worker.ReviewFailureInfrastructure
	}
	if errors.Is(err, errLensAggregateInvalid) {
		return worker.ReviewFailureArtifactUnverified
	}
	return worker.ReviewFailureArtifactUntrusted
}

type lensResult struct {
	Schema     string               `json:"schema_version"`
	Lens       string               `json:"lens"`
	PackDigest string               `json:"pack_digest"`
	Status     string               `json:"status"`
	Reason     string               `json:"reason,omitempty"`
	Candidates []implreport.Finding `json:"candidates"`
}

type retainedLens struct {
	Lens         string               `json:"lens"`
	ContextID    string               `json:"native_context_id"`
	InvocationID string               `json:"native_task_invocation_id,omitempty"`
	PackDigest   string               `json:"shared_pack_digest"`
	Status       string               `json:"status"`
	Reason       string               `json:"reason,omitempty"`
	Candidates   []implreport.Finding `json:"candidates"`
}

type lensEvidence struct {
	Schema       string         `json:"schema_version"`
	PackDigest   string         `json:"shared_pack_digest"`
	PackPath     string         `json:"pack_path,omitempty"`
	ReviewerRoot string         `json:"reviewer_root"`
	ContextID    string         `json:"context_id"`
	Lenses       []retainedLens `json:"lenses"`
}

func decodeLensJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("lens evidence: surplus JSON: %v", err)
	}
	return nil
}

func parseNativeLens(output string) (lensResult, error) {
	var result lensResult
	if len(output) > 8<<20 {
		return result, fmt.Errorf("lens evidence: native lens output exceeds 8 MiB")
	}
	if strings.Count(output, lensFence) != 1 {
		return result, fmt.Errorf("lens evidence: expected one native lens machine block")
	}
	_, rest, _ := strings.Cut(output, lensFence)
	data, tail, closed := strings.Cut(rest, "\n```")
	if !closed || strings.TrimSpace(tail) != "" {
		return result, fmt.Errorf("lens evidence: incomplete native lens machine block")
	}
	if err := decodeLensJSON([]byte(data), &result); err != nil {
		return result, fmt.Errorf("lens evidence: native lens result: %w", err)
	}
	if result.Schema != lensResultSchema || result.Status != "completed" || result.Reason != "" || result.Candidates == nil {
		return result, fmt.Errorf("lens evidence: %s did not complete its judgment: status=%q", result.Lens, result.Status)
	}
	return result, nil
}

// Pack manifests hash the body, not the complete file. Verify both the declared
// digest and the bytes so children never compare a manifest-inclusive hash.
func reviewPackDigest(data []byte) (string, error) {
	header, body, ok := bytes.Cut(data, []byte("\n\n"))
	if !ok {
		return "", fmt.Errorf("lens evidence: missing pack manifest")
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(body))
	declared := ""
	for line := range strings.SplitSeq(string(header), "\n") {
		if value, found := strings.CutPrefix(line, "# digest: "); found {
			if declared != "" {
				return "", fmt.Errorf("lens evidence: duplicate pack digest")
			}
			declared = value
		}
	}
	if declared != digest {
		return "", fmt.Errorf("lens evidence: declared digest differs from pack body")
	}
	return digest, nil
}

func (p *preparedReviewInputs) verifyLenses(ctx context.Context, state string, run port.RunResult) error {
	if p == nil {
		return nil
	} // lifecycle-only fixtures never claim a real pack
	p.verifiedLenses = nil
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := os.OpenRoot(state)
	if err != nil {
		return fmt.Errorf("lens evidence: open runtime: %w", err)
	}
	defer func() { _ = root.Close() }() // read-only handle; no state to flush
	pack, err := readReviewData(ctx, root, filepath.Join("review-evidence", "pack.txt"))
	if err != nil {
		return err
	}
	digest, err := reviewPackDigest(pack)
	if err != nil {
		return err
	}
	data, err := readReviewData(ctx, root, filepath.Join("review-evidence", "lenses.json"))
	if err != nil {
		return fmt.Errorf("lens evidence: required lens outcomes: %w", err)
	}
	var evidence lensEvidence
	if err := decodeLensJSON(data, &evidence); err != nil {
		return fmt.Errorf("lens evidence: retained outcomes: %w", err)
	}
	if evidence.Schema != lensEvidenceSchema || evidence.PackDigest != digest || len(evidence.Lenses) != 4 || run.ReviewChildren == nil || len(run.ReviewChildren.Children) != 4 {
		return fmt.Errorf("lens evidence: all four native outcomes over the canonical pack are required")
	}
	if run.SessionID == "" || evidence.ReviewerRoot != run.SessionID {
		return fmt.Errorf("lens evidence: reviewer root mismatch")
	}
	native := make(map[string]lensResult, 4)
	contexts := make(map[string]bool, 4)
	// A harness whose parent never sees a child's context reports the name the
	// parent does see; the retained outcome may then cite that name instead.
	invocations := make(map[string]string, 4)
	candidates := 0
	for _, child := range run.ReviewChildren.Children {
		if !child.Completed || child.ContextID == "" || child.ContextID == run.SessionID || contexts[child.ContextID] {
			return fmt.Errorf("lens evidence: missing, failed or reused native child context")
		}
		contexts[child.ContextID] = true
		if child.InvocationID != "" {
			if _, reused := invocations[child.InvocationID]; reused {
				return fmt.Errorf("lens evidence: reused native child invocation %q", child.InvocationID)
			}
			invocations[child.InvocationID] = child.ContextID
		}
		result, err := parseNativeLens(child.Output)
		if err != nil {
			return err
		}
		switch result.Lens {
		case implreport.LensSpecFidelity, implreport.LensDesignConformance, implreport.LensSecurityAbuse, implreport.LensCorrectnessCritic:
		default:
			return fmt.Errorf("lens evidence: unknown lens %q", result.Lens)
		}
		if result.PackDigest != digest {
			return fmt.Errorf("lens evidence: %s reviewed another pack", result.Lens)
		}
		native[child.ContextID] = result
		candidates += len(result.Candidates)
	}
	seen := make(map[string]bool, 4)
	for _, lens := range evidence.Lenses {
		context := lens.ContextID
		if context == "" {
			context = invocations[lens.InvocationID]
		}
		child, exists := native[context]
		if !exists || seen[lens.Lens] || lens.Lens != child.Lens || lens.Status != "completed" || lens.Reason != "" || lens.PackDigest != digest || !reflect.DeepEqual(lens.Candidates, child.Candidates) {
			return fmt.Errorf("lens evidence: %s retained outcome differs from its native child", lens.Lens)
		}
		seen[lens.Lens] = true
	}
	report, err := persist.ParseHandoff(run.Output)
	if err != nil {
		return fmt.Errorf("%w: %w", errLensAggregateInvalid, err)
	}
	if len(report.Rounds) == 0 {
		return fmt.Errorf("%w: aggregate has no round", errLensAggregateInvalid)
	}
	round := report.Rounds[len(report.Rounds)-1]
	nativeContext, err := implruntime.ContextID(run.SessionID)
	if err != nil {
		return fmt.Errorf("lens evidence: native reviewer root: %w", err)
	}
	if round.ContextID != nativeContext || round.ContextID != evidence.ContextID || round.Candidates != candidates {
		return fmt.Errorf("lens evidence: aggregate context or candidate count differs from native outcomes")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	payload, err := report.Marshal()
	if err != nil {
		return fmt.Errorf("lens evidence: canonical aggregate: %w", err)
	}
	nativeBytes, err := json.Marshal(struct {
		Retained lensEvidence               `json:"retained"`
		Native   *port.NativeReviewChildren `json:"native"`
	}{evidence, run.ReviewChildren})
	if err != nil {
		return fmt.Errorf("lens evidence: native completion: %w", err)
	}
	p.verifiedLenses = &implreview.NativeLensEvidence{
		SchemaVersion: implreview.NativeLensEvidenceSchema, PackDigest: digest,
		EvidenceDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(nativeBytes)), AggregateDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(payload)),
	}
	return nil
}

func (p *preparedReviewInputs) recordLenses(ctx context.Context, processes implreview.ProcessCoordinator, admitted implreview.AdmittedProcess) error {
	if p == nil {
		return nil // lifecycle-only fixtures have no source pack or native panel
	}
	writer, ok := processes.(implreview.NativeLensEvidenceCoordinator)
	if !ok || p.verifiedLenses == nil {
		return fmt.Errorf("%w: verified native panel or protected writer unavailable", errLensPersistence)
	}
	if err := writer.RecordNativeLensEvidence(ctx, admitted, *p.verifiedLenses); err != nil {
		return fmt.Errorf("%w: %w", errLensPersistence, err)
	}
	return nil
}
