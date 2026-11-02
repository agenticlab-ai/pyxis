// Package baseline implements the no-new-failures baseline (design
// codegate-v1 §3.10, basis FR-10): the grandfathered failure-identity set
// rung 2 evaluates a run's observed test failures against. A baseline is
// captured once by the operator-run `codegate capture-baseline` subcommand
// (DINV-6) and evolves only by human-lane edits thereafter — this package
// owns its file format, closed parsing, atomic writes, and the pure
// no-new-failures evaluation rung 2 runs against it.
package baseline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Version is the baseline file format's schema version stamp.
const Version = "1.0.0"

var baselineCreateTemp = os.CreateTemp

// Baseline is the committed grandfathered-failure file: failure identities
// plus capture provenance (design §3.10 — "who, when, the verb-output
// hash"). Written only by `codegate capture-baseline` (DINV-6); rung 2 only
// ever reads it.
type Baseline struct {
	Version    string     `json:"version"`
	Identities []string   `json:"identities"`
	Provenance Provenance `json:"provenance"`
}

// Provenance is a baseline's human-audit trail: how and when it was
// captured. Never consumed by Evaluate — informational only, for an
// operator reading the file in review.
type Provenance struct {
	CapturedAt      time.Time `json:"captured_at"`
	CapturedBy      string    `json:"captured_by,omitempty"`
	CodegateVersion string    `json:"codegate_version"`
	VerbOutputHash  string    `json:"verb_output_hash"`
}

// Parse parses a baseline from JSON, failing closed on unknown keys (UB-5:
// a "malformed" bound baseline must never be silently accepted) — the same
// closed-parse posture schema.UnmarshalManifest holds the manifest file to.
func Parse(data []byte) (*Baseline, error) {
	var b Baseline
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("baseline parse: %w", err)
	}
	if b.Version == "" {
		return nil, fmt.Errorf("baseline parse: missing version")
	}
	return &b, nil
}

// Load reads and parses the baseline file at path.
func Load(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline %q: %w", path, err)
	}
	b, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return b, nil
}

// Capture builds a freshly-captured baseline — the ONLY constructor
// `codegate capture-baseline` (DINV-6) uses. identities is normalized to a
// non-nil, empty-safe slice so a fully-green capture serializes as
// "identities": [] rather than the ambiguous JSON null.
func Capture(identities []string, codegateVersion, capturedBy, verbOutputHash string, capturedAt time.Time) *Baseline {
	ids := identities
	if ids == nil {
		ids = []string{}
	}
	return &Baseline{
		Version:    Version,
		Identities: ids,
		Provenance: Provenance{
			CapturedAt:      capturedAt,
			CapturedBy:      capturedBy,
			CodegateVersion: codegateVersion,
			VerbOutputHash:  verbOutputHash,
		},
	}
}

// WriteAtomicToFile writes b atomically (temp+rename, in the destination
// directory so the rename stays on one filesystem) to path. This is
// DINV-6's only writer: `codegate capture-baseline` calls it; `run` mode
// never imports this method's call path at all.
func (b *Baseline) WriteAtomicToFile(path string) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create baseline directory: %w", err)
	}

	f, err := baselineCreateTemp(dir, ".codegate-baseline-")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tempPath := f.Name()
	defer os.Remove(tempPath)

	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
