package efficiency

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

// Source names the frozen corpus artifacts that feed one phase's
// efficiency signals (design §3.10). All paths are relative to RepoRoot.
// A phase supplies only the fields its own artifact kind produces — e.g.
// a feature-brainstorm phase sets AnchorPaths and LatencyPaths and leaves
// the rest zero; a prd-review phase sets ReviewPaths instead. An unset
// field contributes nothing: Compute never fabricates a signal for a
// source it wasn't given (prd:pipeline-measurement §5.5 — "a corpus
// artifact absent -> the signal reads none, not fabricated").
type Source struct {
	RepoRoot string

	AnchorPaths  []string // brainstorm-class records: D-n/G-n/superseded-D-n
	AmendmentDir string   // a PRD ledger dir (docs/prds/<feature>): amendment count
	ReviewPaths  []string // review reports (docs/reviews/*.md): FND-n
	PlanPath     string   // a plan doc (docs/plans/<feature>/*.md): the Coverage matrix
	LatencyPaths []string // paths bounding the phase's git latency
}

// Compute derives one phase's efficiency signals deterministically from
// src: no model invocation, no runtime hook (DINV-1) — every signal comes
// from reading frozen files and git history. Compute is pure over its
// inputs, so the same Source content (byte-identical files, byte-identical
// git history) yields a byte-identical record.Efficiency regardless of
// which harness produced the artifacts (FR-4; plan pipeline-measurement/001
// T9 AC1).
func Compute(src Source) (record.Efficiency, error) {
	var eff record.Efficiency

	superseded := 0
	for _, p := range src.AnchorPaths {
		doc, err := readOptional(filepath.Join(src.RepoRoot, p))
		if err != nil {
			return record.Efficiency{}, fmt.Errorf("efficiency: compute: %w", err)
		}
		eff.Questions += Questions(doc)
		eff.Fog += Fog(doc)
		superseded += SupersededDecisions(doc)
	}

	amendments := 0
	if src.AmendmentDir != "" {
		n, err := AmendmentCount(filepath.Join(src.RepoRoot, src.AmendmentDir))
		if err != nil {
			return record.Efficiency{}, fmt.Errorf("efficiency: compute: %w", err)
		}
		amendments = n
	}
	eff.Rework = superseded + amendments

	for _, p := range src.ReviewPaths {
		doc, err := readOptional(filepath.Join(src.RepoRoot, p))
		if err != nil {
			return record.Efficiency{}, fmt.Errorf("efficiency: compute: %w", err)
		}
		eff.Defects += Defects(doc)
	}

	if src.PlanPath != "" {
		doc, err := readOptional(filepath.Join(src.RepoRoot, src.PlanPath))
		if err != nil {
			return record.Efficiency{}, fmt.Errorf("efficiency: compute: %w", err)
		}
		eff.CoverageRatio = CoverageRatio(doc)
	}

	lat, err := Latency(src.RepoRoot, src.LatencyPaths)
	if err != nil {
		return record.Efficiency{}, fmt.Errorf("efficiency: compute: %w", err)
	}
	eff.LatencySeconds = int64(lat / time.Second)

	return eff, nil
}

// Join computes and attaches efficiency signals to every record in
// records whose PhaseID has an entry in sources, returning a new slice —
// records itself is never mutated. records is expected to be one run's
// worth (e.g. store.LoadRun's result), so phase_id alone disambiguates
// the join within that scope; that, plus the run scope the caller already
// loaded, is the "run/phase id" join key design §3.10 specifies. A record
// whose PhaseID has no matching source passes through unchanged.
func Join(records []record.Record, sources map[string]Source) ([]record.Record, error) {
	out := make([]record.Record, len(records))
	copy(out, records)
	for i, rec := range out {
		src, ok := sources[rec.PhaseID]
		if !ok {
			continue
		}
		eff, err := Compute(src)
		if err != nil {
			return nil, fmt.Errorf("join: phase %q: %w", rec.PhaseID, err)
		}
		out[i].Efficiency = eff
	}
	return out, nil
}

// readOptional reads path, treating a missing file as empty content
// rather than an error — a corpus artifact absent reads as no signal,
// never a fault (prd:pipeline-measurement §5.5).
func readOptional(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return b, nil
}
