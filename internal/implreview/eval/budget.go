package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// The band token/latency budgets and the decode-pinned run provenance, read as
// VERSIONED DATA (§3.5: "the band token/latency budgets as versioned data"; "a
// decode-pinned configuration recorded in the eval provenance"; the run runtime
// "budgeted as data beside the calibration table"). This file is the CLOSED-parse
// loader for both — an unknown key is an error (AP-6), exactly as the calibration
// table parses closed. Changing a budget or N is a reviewed data PR, never a code
// edit; a removal or a threshold relaxation is a reviewed decision (§3.5).

// BudgetsSchema / ProvenanceSchema pin the two data shapes this build parses.
const (
	BudgetsSchema    = "impl-review-eval-budgets/v1"
	ProvenanceSchema = "impl-review-eval-provenance/v1"
)

// BandBudget is one band's COST budget the review runtime is held to — the token
// ceiling and the latency bound (NFR-1/2). These are the cost budgets (distinct
// from band.BandBudgets, which is the diff-READ byte budget the truncation family
// uses); both are data. Zero on the skip band (it runs no lenses).
type BandBudget struct {
	// MaxTokens is the token ceiling a review in this band is budgeted for.
	MaxTokens int `json:"max_tokens"`
	// MaxLatencyMs is the latency bound (ms) a review in this band is budgeted
	// for.
	MaxLatencyMs int `json:"max_latency_ms"`
}

// Budgets is the parsed band-budget data (budgets.json). It carries one
// BandBudget per band, keyed by the report.Band value. The gated live suite
// reads it to hold a real run to its band budget; the hermetic harness asserts it
// LOADS and covers the three bands (so the data stays present and well-formed).
type Budgets struct {
	Schema string `json:"schema"`
	// Note is the human-readable rationale carried in the data file; informative.
	Note string `json:"note"`
	// Bands maps a band (skip/standard/deep) to its cost budget.
	Bands map[string]BandBudget `json:"bands"`
}

// Budget returns the cost budget for a band and whether the band is present. A
// band absent from the data has no budget entry — the live suite treats that as a
// misconfiguration (a review in an unbudgeted band has no ceiling), never an
// unbounded run.
func (b *Budgets) Budget(band string) (BandBudget, bool) {
	bg, ok := b.Bands[band]
	return bg, ok
}

// LoadBudgets reads and parses the band-budget data at path (the canonical file
// is .pyxis/skills/impl-review/fixtures/budgets.json), CLOSED. It requires the
// three bands to be present (skip/standard/deep) so the data is complete — a
// missing band is a data defect surfaced here.
func LoadBudgets(path string) (*Budgets, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("eval: read budgets %s: %w", path, err)
	}
	var b Budgets
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("eval: parse budgets %s: %w", path, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("eval: parse budgets %s: trailing data", path)
	}
	if b.Schema != BudgetsSchema {
		return nil, fmt.Errorf("eval: budgets %s: wrong schema %q (want %q)", path, b.Schema, BudgetsSchema)
	}
	for _, band := range []string{report.BandSkip, report.BandStandard, report.BandDeep} {
		if _, ok := b.Bands[band]; !ok {
			return nil, fmt.Errorf("eval: budgets %s: missing band %q", path, band)
		}
	}
	return &b, nil
}

// Decode is the decode-pinned configuration a run records (§3.5) — the parameters
// the live lenses run under, so a run is reproducible. The hermetic harness does
// not consume these (it injects a deterministic fake); the gated live suite pins
// them against the real backend.
type Decode struct {
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	Seed        int     `json:"seed"`
}

// RunUnder is the model + prompts + calibration versions a run evaluated under —
// the provenance the payload's report.Provenance mirrors.
type RunUnder struct {
	Model          string `json:"model"`
	PromptsVersion string `json:"prompts_version"`
	CalVersion     string `json:"cal_version"`
}

// Provenance is the parsed run provenance (provenance.json): the fixed per-family
// run count N (>= 5), the caught-every-run rule, the decode-pinned config, and
// the versions the run evaluated under. The harness reads N and caught-every-run
// from here (never a hard-coded constant) so the protocol itself is versioned
// data (§3.5).
type Provenance struct {
	Schema string `json:"schema"`
	Note   string `json:"note"`
	// RunsPerFamily is N — the fixed number of times each family runs per release.
	// It MUST be >= MinRunsPerFamily (the design floor of 5).
	RunsPerFamily int `json:"runs_per_family"`
	// CaughtEveryRun is the pass rule: a family passes only if caught on EVERY
	// run. It MUST be true — a run configuration that relaxed it is a reviewed
	// decision the loader refuses (an intermittent miss is a defect, never a
	// rerun-until-green — §3.5).
	CaughtEveryRun bool     `json:"caught_every_run"`
	Decode         Decode   `json:"decode"`
	RunUnder       RunUnder `json:"run_under"`
}

// MinRunsPerFamily is the design floor for N (§3.5: "N >= 5 times per family").
// The provenance's runs_per_family must be at least this; a smaller value is a
// data defect the loader refuses, so the protocol can never silently run fewer
// than the floor.
const MinRunsPerFamily = 5

// LoadProvenance reads and parses the run provenance at path (the canonical file
// is .pyxis/skills/impl-review/fixtures/provenance.json), CLOSED, and enforces
// the two protocol floors: runs_per_family >= MinRunsPerFamily, and
// caught_every_run == true. These are the design's non-negotiables (§3.5), so a
// data file that weakened either is refused here rather than silently honored.
func LoadProvenance(path string) (*Provenance, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("eval: read provenance %s: %w", path, err)
	}
	var p Provenance
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("eval: parse provenance %s: %w", path, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("eval: parse provenance %s: trailing data", path)
	}
	if p.Schema != ProvenanceSchema {
		return nil, fmt.Errorf("eval: provenance %s: wrong schema %q (want %q)", path, p.Schema, ProvenanceSchema)
	}
	if p.RunsPerFamily < MinRunsPerFamily {
		return nil, fmt.Errorf("eval: provenance %s: runs_per_family=%d is below the floor of %d (N >= 5, §3.5)",
			path, p.RunsPerFamily, MinRunsPerFamily)
	}
	if !p.CaughtEveryRun {
		return nil, fmt.Errorf("eval: provenance %s: caught_every_run must be true — an intermittent miss is a defect, never a rerun-until-green (§3.5)", path)
	}
	return &p, nil
}
