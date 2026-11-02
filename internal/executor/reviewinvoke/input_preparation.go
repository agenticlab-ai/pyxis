package reviewinvoke

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
)

type reviewInputPreparation struct {
	workdir, state, revision, base, charter string
}

// These authoritative bytes stay in the host process, outside the reviewer's
// writable runtime. Files supplied to the reviewer are only staging copies.
type preparedReviewInputs struct {
	inputs         pack.ReviewInputs
	verifiedLenses *implreview.NativeLensEvidence
}

// Prepare deterministic inputs before launching a model. Selecting the relevant
// governing design bindings remains reviewer judgment; no placeholder design is
// invented to make an incomplete pack pass.
func (i *Invoker) materializeReviewInputs(ctx context.Context, in reviewInputPreparation) (*preparedReviewInputs, error) {
	if i.prepareInputs != nil {
		return i.prepareInputs(ctx, in)
	}
	started := time.Now()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.charter) == "" {
		return nil, fmt.Errorf("review inputs: missing frozen task charter")
	}
	if in.base == "" {
		remote, mainline := i.remote, i.mainline
		if remote == "" {
			remote = config.Defaults().Remote
		}
		if mainline == "" {
			mainline = config.Defaults().Branch
		}
		for _, ref := range []string{"refs/remotes/" + remote + "/" + mainline, "refs/heads/" + mainline} {
			base, err := gitOut(ctx, i.repoDir, "rev-parse", "--verify", ref+"^{commit}")
			if err != nil {
				continue
			}
			in.base, err = gitOut(ctx, in.workdir, "merge-base", base, in.revision)
			if err != nil {
				return nil, fmt.Errorf("review inputs: resolve exact revision base: %w", err)
			}
			break
		}
		if in.base == "" {
			return nil, fmt.Errorf("review inputs: missing mainline base")
		}
	}
	diff, err := exec.CommandContext(ctx, "git", "-C", in.workdir, "diff", "--no-ext-diff", "--no-textconv", in.base, in.revision, "--").Output()
	if err != nil {
		return nil, fmt.Errorf("review inputs: exact revision diff: %w", err)
	}
	calibration, err := os.ReadFile(filepath.Join(in.workdir, ".pyxis", "skills", "impl-review", "calibration.md"))
	if err != nil {
		return nil, fmt.Errorf("review inputs: calibration: %w", err)
	}
	root := filepath.Join(in.state, "review-evidence", "input-root", "review")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("review inputs: create private root: %w", err)
	}
	prepared := &preparedReviewInputs{inputs: pack.ReviewInputs{
		Proposal: diff, CodegateVerdict: fmt.Appendf(nil, "All required repository checks were green at exactly %s when the trusted host admitted this review.\n", in.revision),
		Charter: []byte(in.charter), Calibration: calibration,
	}}
	for _, input := range []struct {
		name string
		data []byte
	}{
		{"proposal.diff", prepared.inputs.Proposal},
		{"codegate.txt", prepared.inputs.CodegateVerdict},
		{"charter.md", prepared.inputs.Charter},
		{"calibration.md", prepared.inputs.Calibration},
	} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(root, input.name), input.data, 0o600); err != nil {
			return nil, fmt.Errorf("review inputs: write %s: %w", input.name, err)
		}
	}
	if i.log != nil {
		i.log.Info("impl-review deterministic inputs prepared", "revision", in.revision, "base", in.base, "input_root", filepath.Dir(root), "inputs", 4, "duration_ms", time.Since(started).Milliseconds())
	}
	return prepared, nil
}

// Verify after the runtime quiesces, before accepting or publishing its report.
// Rebuilding with host-held inputs also detects assemble-then-restore attacks
// and packs assembled from a different input root or proposal subject.
func (p *preparedReviewInputs) verify(ctx context.Context, state, subject string) error {
	if p == nil {
		return nil // synthetic lifecycle fixtures do not prepare real inputs
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := os.OpenRoot(state)
	if err != nil {
		return fmt.Errorf("host-prepared review inputs: open evidence: %w", err)
	}
	defer func() { _ = root.Close() }() // read-only handle, no durable state to flush
	expected := p.inputs
	expected.Subject = subject
	expected.DesignBindings, err = readReviewData(ctx, root, filepath.Join("review-evidence", "input-root", "review", "design-bindings.md"))
	if err != nil {
		return fmt.Errorf("host-prepared review inputs: design bindings: %w", err)
	}
	want, err := pack.Build(expected)
	if err != nil {
		return fmt.Errorf("host-prepared review inputs: rebuild pack: %w", err)
	}
	got, err := readReviewData(ctx, root, filepath.Join("review-evidence", "pack.txt"))
	if err != nil {
		return fmt.Errorf("host-prepared review inputs: assembled pack: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("host-prepared review inputs: assembled pack differs from the authoritative inputs")
	}
	return nil
}

func readReviewData(ctx context.Context, root *os.Root, path string) ([]byte, error) {
	const limit = 8 << 20
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := root.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("%s must be a regular file of at most 8 MiB", path)
	}
	f, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	b, readErr := io.ReadAll(io.LimitReader(f, limit+1))
	closeErr := f.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(b) > limit {
		return nil, fmt.Errorf("%s exceeds 8 MiB", path)
	}
	return b, nil
}
