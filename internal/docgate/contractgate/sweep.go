package contractgate

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// The dogfood sweep (plan T8; prd:contract-gate/M2, design contract-gate-v1
// §3.11): the "local CI" over the repo's OWN contracts. It runs the mechanical
// gate over EVERY live feature contract in the repo — each PRD ledger directory
// that carries a folded `effective.source.json` — under the planning-entry
// trigger (the hard bar: every defect class blocking, the same lens plan-new's
// precondition applies), and reports a per-feature summary: the verdict class
// and, on a bounce, the named defects. It is read-only on every path (DINV-1):
// it only reads committed machine faces and runs the pure check subset, writing
// nothing (no report emission, no git, no network).
//
// The sweep is the honesty proof of M2: every bounce it surfaces must be a REAL
// contract defect. The disposition of the current corpus is recorded at
// docs/plans/contract-gate/dogfood-disposition.md — the checked-in artifact the
// operator reads; the sweep itself stays deterministic so that record is
// reproducible.

// Sweep gates every live feature contract in root under the planning-entry
// trigger and returns one closed SweepResult. It discovers the feature set (the
// PRD ledger directories carrying a folded source map, excluding the
// review-feedback ledger which is not a PRD contract), gates each, and reports
// per feature its verdict class, whether it blocked, and the named defects on a
// bounce. Deterministic: features are sorted, and each per-feature verdict is
// itself deterministic (DINV-5). Read-only on every path (DINV-1).
//
// A fail-closed config edge (a malformed .pyxis/config.yaml or severity-map)
// short-circuits into ConfigError with no per-feature entries — the whole sweep
// fails closed, mirroring the per-feature run's UB-6 handling.
func Sweep(root string) SweepResult {
	cfg, cdef := config.Load(root)
	if len(cdef) > 0 {
		return SweepResult{ConfigError: "the pyxis config does not parse: " + cdef[0].Reason}
	}
	sev, sdef := LoadSeverityMap(root)
	if len(sdef) > 0 {
		return SweepResult{ConfigError: "the severity-map policy does not parse: " + sdef[0].Reason}
	}

	features, ferr := liveFeatures(root, cfg)
	if ferr != "" {
		return SweepResult{ConfigError: ferr}
	}

	res := SweepResult{Trigger: TriggerPlanningEntry, Features: make([]FeatureVerdict, 0, len(features))}
	for _, f := range features {
		v := runResolved(root, f, TriggerPlanningEntry, cfg, sev)
		fv := FeatureVerdict{
			Feature:  f,
			Class:    v.Class,
			Blocking: v.Blocked(),
			Reason:   v.Reason,
			Defects:  bounceDefects(v),
		}
		res.Features = append(res.Features, fv)
		res.Total++
		switch {
		case v.Blocked():
			res.Blocking++
		case !v.Clean():
			res.Advisory++ // a non-clean but non-blocking (advisory-only) bounce
		default:
			res.Clean++
		}
	}
	return res
}

// SweepResult is the closed outcome of a dogfood sweep over the repo's live
// contracts: the trigger it applied (always planning-entry — the hard bar), one
// FeatureVerdict per live feature, and the tallies. Blocking counts the features
// that BLOCK (the exit-code driver: the sweep fails iff Blocking > 0). A
// fail-closed config/severity-map short-circuits into ConfigError with no
// per-feature entries. Serialized as data a consumer (or the disposition record)
// reads verbatim (AP-21); additive-evolvable via new tagged fields.
type SweepResult struct {
	// Trigger is the severity lens every feature was judged under — planning-entry
	// (the hard "must be whole to plan" bar the sweep applies as local CI).
	Trigger Trigger `json:"trigger"`
	// Features is one verdict summary per live feature, in feature (sorted) order.
	Features []FeatureVerdict `json:"features"`
	// Total is the number of live feature contracts gated.
	Total int `json:"total"`
	// Clean / Advisory / Blocking partition Total: cleanly-passing, non-blocking
	// bounces (advisory-only), and blocking bounces. Clean+Advisory+Blocking=Total.
	Clean    int `json:"clean"`
	Advisory int `json:"advisory"`
	Blocking int `json:"blocking"`
	// ConfigError names a fail-closed config/severity-map defect; non-empty means
	// the whole sweep failed closed with no per-feature entries.
	ConfigError string `json:"config_error,omitempty"`
}

// FeatureVerdict is one feature's line in the sweep: its slug, verdict class,
// whether it blocked, the verdict's reason line, and — on a bounce — the named
// defects (each carrying its check, offender, and reason). A clean feature
// carries an empty Defects slice. The fields derive verbatim from the feature's
// Verdict so the sweep never diverges from a direct `contract-gate <feature>`.
type FeatureVerdict struct {
	// Feature is the feature slug.
	Feature string `json:"feature"`
	// Class is the feature's overall verdict class (clean on a pass).
	Class Class `json:"class"`
	// Blocking is whether the feature's verdict blocked (the sweep fails iff any
	// feature blocks). An advisory-only bounce is non-blocking with Class != clean.
	Blocking bool `json:"blocking"`
	// Reason is the verdict's short reason line (empty on clean).
	Reason string `json:"reason,omitempty"`
	// Defects are the named offenders on a bounce (empty on clean) — each the
	// check that red, the offending artifact, and the concrete reason.
	Defects []SweepDefect `json:"defects,omitempty"`
}

// SweepDefect is one named defect in a feature's bounce — the check that red,
// the offending artifact, and the concrete reason — the minimum a person needs
// to see WHICH contract defect the sweep caught. Derived from the verdict's
// CheckResults, so the sweep names exactly what the feature's own verdict does.
type SweepDefect struct {
	Check    string   `json:"check"`
	Offender string   `json:"offender"`
	Reason   string   `json:"reason"`
	Severity Severity `json:"severity,omitempty"`
}

// Failed reports whether the sweep should fail (exit non-zero): a fail-closed
// config/severity-map error, or any feature that blocks. An advisory-only bounce
// leaves it false — the defect is surfaced but does not fail the sweep, exactly
// as the per-feature exit code behaves (FR-9).
func (r SweepResult) Failed() bool {
	return r.ConfigError != "" || r.Blocking > 0
}

// bounceDefects flattens a non-clean verdict's failed-check defects into the
// sweep's named-offender shape, in the verdict's stable (name-sorted) check
// order (DINV-5). A clean verdict yields nil (no defects to name).
func bounceDefects(v Verdict) []SweepDefect {
	if v.Clean() {
		return nil
	}
	var out []SweepDefect
	for _, res := range v.CheckResults {
		if res.Passed || len(res.Defects) == 0 {
			continue // only failed checks with named defects
		}
		for _, d := range res.Defects {
			out = append(out, SweepDefect{
				Check:    res.Check,
				Offender: d.File,
				Reason:   d.Reason,
				Severity: res.Severity,
			})
		}
	}
	return out
}

// liveFeatures enumerates the repo's live feature slugs — every directory under
// the PRD root that carries a folded `effective.source.json` (a resolvable
// contract), excluding the review-feedback ledger (a subject ledger under a
// different root that is not a PRD contract). Sorted for determinism (DINV-5).
// Read-only. A missing PRD root is a config error (the corpus is absent).
func liveFeatures(root string, cfg config.Config) ([]string, string) {
	prds := filepath.Join(root, filepath.FromSlash(cfg.PRDs))
	items, err := os.ReadDir(prds)
	if err != nil {
		return nil, "the PRD corpus is unreadable: " + err.Error()
	}
	var out []string
	for _, it := range items {
		if !it.IsDir() {
			continue
		}
		dir := filepath.Join(prds, it.Name())
		// Skip the review-feedback ledger tree — it lives under its own root but
		// could be nested under PRDs in a misconfigured repo; a feedback subject is
		// not a PRD contract and has no folded PRD source map.
		if gate.IsFeedbackDir(root, cfg, dir) {
			continue
		}
		// A live feature is one whose contract is folded — its source map exists.
		// A ledger dir without one is not yet a resolvable contract (the sweep is
		// over LIVE contracts, so it is skipped rather than reported unresolvable).
		if _, err := os.Stat(filepath.Join(dir, "effective.source.json")); err != nil {
			continue
		}
		out = append(out, it.Name())
	}
	sort.Strings(out)
	return out, ""
}
