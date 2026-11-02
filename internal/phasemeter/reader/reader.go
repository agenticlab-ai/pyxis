// Package reader is the pipeline-measurement operator-facing read
// (docs/tech-design/pipeline-measurement-v2.md §3.11, FR-9): it queries a
// run's stored records, computes each phase's efficiency signals from the
// frozen artifacts + git (internal/phasemeter/efficiency, T9), derives
// each model's dollar cost from the vendored price snapshot
// (internal/phasemeter/price, T11), and joins all three — reconciled
// spend, cost, and efficiency — into one per-phase View (design §3.11:
// "a reader (query -> efficiency -> present)").
package reader

import (
	"fmt"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/efficiency"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/price"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/store"
)

// View is one phase's operator-facing measurement: tokens by model, dollar
// cost, and efficiency signals — or the harness's declared unavailable
// markers (FR-9). It carries every field the per-phase presentation needs,
// so a caller never reaches back into record.Record.
type View struct {
	RunID     string
	FeatureID string
	PhaseID   string
	Harness   string

	Models      []ModelView
	Efficiency  record.Efficiency
	Unavailable []record.Unavailable
}

// ModelView is one model's spend within a phase: its token quartet, the
// derived dollar cost (or Unpriced, UB-2), and how tightly it reconciles
// to the harness's own accounting (DINV-4) — the reader shows the tier
// exactly as the record carries it, never recomputing reconciliation
// itself (design §3.1: reconciliation is the recorder's concern).
type ModelView struct {
	Model              string
	Tokens             record.TokenQuartet
	Cost               float64
	Unpriced           bool
	ReconciliationTier record.ReconciliationTier
}

// Read is the reader's query -> efficiency -> cost pipeline (design
// §3.11). It loads runID's records from st in their recorded (phase)
// order, computes each phase's efficiency signals from repoRoot's frozen
// artifacts + git (SourceFor, T9's efficiency.Compute), derives each
// model's dollar cost from prices (T11's price.Table.Derive), and returns
// one View per phase. prices may be nil — every model's cost then reads
// Unpriced rather than a fabricated figure. A run with no records yields
// an empty, non-nil-error slice: nothing recorded is reported, not
// faulted, mirroring the recorder's own "zero phases is not a fault"
// convention.
func Read(st *store.Store, prices *price.Table, repoRoot, runID string) ([]View, error) {
	recs, err := st.LoadRun(runID)
	if err != nil {
		return nil, fmt.Errorf("phasemeter/reader: read %q: %w", runID, err)
	}

	views := make([]View, len(recs))
	for i, rec := range recs {
		eff, cerr := efficiency.Compute(SourceFor(repoRoot, rec.FeatureID, rec.PhaseID))
		if cerr != nil {
			return nil, fmt.Errorf("phasemeter/reader: read %q: phase %q: %w", runID, rec.PhaseID, cerr)
		}
		if prices != nil {
			rec = prices.Derive(rec)
		} else {
			rec = markUnpriced(rec)
		}
		views[i] = toView(rec, eff)
	}
	return views, nil
}

// markUnpriced forces every one of rec's Spend entries Unpriced, Cost 0 —
// used when Read has no price table at all, so a model's cost reads as
// declared-unpriced rather than whatever value happened to already be on
// the stored record (UB-2: never a silent zero presented as measured).
// rec's own Spend map is never mutated.
func markUnpriced(rec record.Record) record.Record {
	if len(rec.Spend) == 0 {
		return rec
	}
	spend := make(map[string]record.ModelSpend, len(rec.Spend))
	for model, ms := range rec.Spend {
		ms.Cost = 0
		ms.Unpriced = true
		spend[model] = ms
	}
	rec.Spend = spend
	return rec
}

// toView joins rec's spend and eff into one operator-facing View. Models
// are sorted by name for deterministic presentation — record.Record.Spend
// is a Go map and iterates in randomized order otherwise.
func toView(rec record.Record, eff record.Efficiency) View {
	models := make([]ModelView, 0, len(rec.Spend))
	for model, ms := range rec.Spend {
		models = append(models, ModelView{
			Model:              model,
			Tokens:             ms.TokenQuartet,
			Cost:               ms.Cost,
			Unpriced:           ms.Unpriced,
			ReconciliationTier: ms.ReconciliationTier,
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Model < models[j].Model })

	return View{
		RunID:       rec.RunID,
		FeatureID:   rec.FeatureID,
		PhaseID:     rec.PhaseID,
		Harness:     rec.Harness,
		Models:      models,
		Efficiency:  eff,
		Unavailable: rec.Unavailable,
	}
}
