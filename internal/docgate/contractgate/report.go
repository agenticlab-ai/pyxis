package contractgate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

// ReportSchemaVersion pins the defect-report schema — a closed, versioned
// dual-face record (design contract-gate-v1 §3.8). Additive-evolvable: a later
// version adds tagged fields, never renames or removes one (AP-5/AP-6). Distinct
// from the verdict's SchemaVersion: the report is its own artifact with its own
// schema, so the two evolve independently.
const ReportSchemaVersion = "1"

// ReportGeneratedBy names the producer stamped into every report, so a consumer
// reading a committed report knows which tool + mode emitted it. Fixed (no clock,
// no run-specific text) so the byte content is reproducible (DINV-2/DINV-5).
const (
	reportGeneratedBy = "docgate contract-gate"
)

// DefaultReportDir is the sink a bounce's report is emitted to when the verb is
// given no explicit --report-out (design §3.8 open-Q2: the exact home is an
// implementation call). It is a gitignored scratch path under .pyxis/ — the gate
// is read-only and never commits (DINV-1); the DURABLE, immutable-history copy is
// the one the HOST retains to git (DINV-2). Keeping the default out of git avoids
// the gate's scratch output masquerading as a committed event record. Repo-
// relative; the verb joins it under the repo root.
const DefaultReportDir = runlogs.ContractGate

// ReportDefect is one failed-check defect as it appears in the report's machine
// face — the closed, downstream-parseable shape of a single contract bounce
// (design §3.8): the check that red, the specific offender (the offending
// artifact + the concrete reason naming the requirement / citation / stale input
// / malformed join), and what unblocks it (the remedy). Derived 1:1 from a
// schema.Defect a check produced, so the two faces can never diverge — both read
// the same defect list.
type ReportDefect struct {
	// Check is the contract-scope check that produced this defect (its stable id,
	// e.g. "structural-completeness"), read from the defect's Op.
	Check string `json:"check"`
	// Offender is the specific artifact the defect is about — the offending
	// machine face / document (a requirement's source map, a serving design file),
	// read from the defect's File. The precise unit/citation/anchor is named in
	// Reason.
	Offender string `json:"offender"`
	// Reason names the concrete defect — the requirement ID lacking a fit
	// criterion, the dangling citation + its source location, the stale input +
	// what superseded it, the malformed join.
	Reason string `json:"reason"`
	// Unblock is the concrete remedy — what a person does to clear this defect.
	// Populated from the check-produced defect's Remedy (every check names one).
	Unblock string `json:"unblock"`
}

// ReportCheck bundles one failed check's defects in the report (design §3.8):
// the check id, the verdict class its defects contributed, the blocking-vs-
// advisory severity at the report's trigger, and every defect it named. A
// passing check contributes nothing — only the failed checks appear, so the
// report enumerates exactly the bounce.
type ReportCheck struct {
	// Check is the failed check's stable id.
	Check string `json:"check"`
	// Class is the verdict class this check's defects contributed on failure.
	Class Class `json:"class"`
	// Severity is the check's blocking-vs-advisory disposition at the report's
	// trigger (FR-9/FR-13). A report reader distinguishes the defects that stopped
	// the run from those merely surfaced.
	Severity Severity `json:"severity"`
	// Defects are the offenders this check named, each with its unblock.
	Defects []ReportDefect `json:"defects"`
}

// DefectReport is the one dual-face event record a bounce emits (design §3.8):
// "at run T, contract C carried defects D". It is an EVENT RECORD, not a live
// projection — it does not stale when the contract is fixed; a re-gate produces a
// NEW record and the prior persists as history (DINV-2). The machine face is
// this struct serialized as JSON (a downstream consumer parses it); the human
// face is Markdown() derived from the SAME fields, so the two never diverge. Its
// content is deterministic — no clock, no run-specific text — so it is
// byte-reproducible across runs over identical inputs (DINV-5); the run-time
// ("at run T") lives in git's commit metadata when the host retains it, not in
// the report body.
type DefectReport struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedBy   string `json:"generated_by"`
	ToolVersion   string `json:"tool_version"`
	// Feature is the feature slug the bounced contract is scoped to.
	Feature string `json:"feature"`
	// Trigger is the context the gate ran under (planning-entry / contract-doc-pr),
	// copied from the verdict — the severity lens the recorded dispositions were
	// judged against (§5.7, §3.10).
	Trigger Trigger `json:"trigger"`
	// Verdict is the overall verdict class this report records (never "clean" —
	// a clean verdict emits no report, DINV-6).
	Verdict Class `json:"verdict"`
	// Blocking is whether the bounce stopped the run (any blocking defect) — false
	// for an advisory-only bounce whose defects are surfaced but non-blocking.
	Blocking bool `json:"blocking"`
	// Summary is the verdict's short reason line — routes attention; the checks
	// carry the authoritative detail.
	Summary string `json:"summary,omitempty"`
	// Evaluated is the identity of every input the verdict evaluated (fingerprints
	// + refs), copied verbatim from the verdict so a disputed bounce is reproducible
	// against the exact contract state that produced it (design §3.4/§3.8).
	Evaluated []InputIdentity `json:"evaluated"`
	// Checks are the failed checks and their defects — the bounce enumerated.
	Checks []ReportCheck `json:"checks"`
}

// ReportForVerdict builds the dual-face report for a non-clean verdict, bundling
// every failed check's defects with the check id, offender, and unblock (design
// §3.8). It returns (report, true) for a failing verdict and (zero, false) for a
// clean one — a clean verdict has no report (DINV-6), so callers gate emission on
// the bool rather than materializing an empty record. Pure and deterministic:
// it reads the verdict's own fields and orders the checks by the verdict's stable
// (name-sorted) CheckResults order, no clock or map iteration (DINV-5).
func ReportForVerdict(v Verdict) (DefectReport, bool) {
	if v.Clean() {
		return DefectReport{}, false
	}
	r := DefectReport{
		SchemaVersion: ReportSchemaVersion,
		GeneratedBy:   reportGeneratedBy,
		ToolVersion:   schema.ToolVersion,
		Feature:       v.Feature,
		Trigger:       v.Trigger,
		Verdict:       v.Class,
		Blocking:      v.Blocking,
		Summary:       v.Reason,
		Evaluated:     append([]InputIdentity(nil), v.Evaluated...),
		Checks:        []ReportCheck{},
	}
	for _, res := range v.CheckResults {
		if res.Passed || len(res.Defects) == 0 {
			continue // only failed checks with named defects enter the report
		}
		rc := ReportCheck{Check: res.Check, Class: res.Class, Severity: res.Severity, Defects: make([]ReportDefect, 0, len(res.Defects))}
		for _, d := range res.Defects {
			rc.Defects = append(rc.Defects, ReportDefect{
				Check:    res.Check,
				Offender: d.File,
				Reason:   d.Reason,
				Unblock:  d.Remedy,
			})
		}
		r.Checks = append(r.Checks, rc)
	}
	return r, true
}

// JSON is the report's machine face — the closed schema serialized as
// indented JSON a downstream consumer parses (design §3.8). Deterministic byte
// output (struct field order is fixed; the check/defect slices are already in the
// verdict's stable order), so a golden test can byte-pin it. A trailing newline
// makes it a well-formed text file.
func (r DefectReport) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal defect report: %w", err)
	}
	return append(b, '\n'), nil
}

// Markdown is the report's human face — a rendered Markdown enumeration of the
// SAME fields the machine face carries (design §3.8), for a person reading the
// bounce. Both faces derive from the one DefectReport, so they never diverge.
// Deterministic: no clock, the checks/defects are in the verdict's stable order.
func (r DefectReport) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Contract-gate defect report — %s\n\n", r.Feature)

	// A stable header table — the event-record identity (which tool, which
	// contract state) a person reads first (mirrors the review-report family).
	b.WriteString("| Field | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| **Feature** | %s |\n", r.Feature)
	fmt.Fprintf(&b, "| **Trigger** | %s |\n", r.Trigger)
	fmt.Fprintf(&b, "| **Verdict** | %s |\n", r.Verdict)
	blocking := "no (advisory only — non-blocking at this trigger)"
	if r.Blocking {
		blocking = "yes"
	}
	fmt.Fprintf(&b, "| **Blocking** | %s |\n", blocking)
	if r.Summary != "" {
		fmt.Fprintf(&b, "| **Summary** | %s |\n", r.Summary)
	}
	fmt.Fprintf(&b, "| **Generated by** | %s (%s) |\n", r.GeneratedBy, r.ToolVersion)
	fmt.Fprintf(&b, "| **Report schema** | %s |\n", r.SchemaVersion)
	b.WriteString("\n")

	// The inputs the verdict evaluated — the exact contract state, so a disputed
	// bounce is reproducible (design §3.4).
	if len(r.Evaluated) > 0 {
		b.WriteString("## Inputs evaluated\n\n")
		for _, in := range r.Evaluated {
			fp := in.Fingerprint
			if fp == "" {
				fp = "(no fingerprint)"
			}
			fmt.Fprintf(&b, "- **%s** `%s` — %s\n", in.Kind, in.Ref, fp)
		}
		b.WriteString("\n")
	}

	// The bounce itself — every failed check, each defect's offender + unblock.
	b.WriteString("## Defects\n\n")
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "### %s — %s (%s)\n\n", c.Check, c.Class, c.Severity)
		for _, d := range c.Defects {
			fmt.Fprintf(&b, "- **Offender:** `%s`\n", d.Offender)
			fmt.Fprintf(&b, "  - **Defect:** %s\n", d.Reason)
			fmt.Fprintf(&b, "  - **Unblock:** %s\n", d.Unblock)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// EmitReport writes both faces of a non-clean verdict's report atomically into
// dir (design §3.8 / plan T5): `<feature>.json` (the machine face) and
// `<feature>.md` (the human face). It returns the two written paths. It is the
// gate's ONLY write, and it writes ONLY under dir (the designated sink, DINV-1):
// no git, no network, no write elsewhere. A clean verdict is a no-op — nothing is
// materialized (DINV-6), so the caller must gate on the verdict; EmitReport
// returns (nil, nil) for a clean verdict as a defensive second guard.
//
// The filenames are STABLE — the feature slug, no timestamp — so the byte content
// is reproducible for the tests and a re-gate overwrites the scratch copy rather
// than accreting timestamped files (the durable, immutable-history copy is the
// host's git commit, DINV-2, not this scratch sink). Each face is written
// temp+rename so a reader never observes a half-written report.
func EmitReport(dir string, v Verdict) (paths []string, err error) {
	r, ok := ReportForVerdict(v)
	if !ok {
		return nil, nil // clean verdict — nothing to emit (DINV-6)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create report sink %s: %w", dir, err)
	}
	base := reportBaseName(v.Feature)

	jsonBytes, err := r.JSON()
	if err != nil {
		return nil, err
	}
	jsonPath := filepath.Join(dir, base+".json")
	if err := atomicWrite(jsonPath, jsonBytes); err != nil {
		return nil, err
	}

	mdPath := filepath.Join(dir, base+".md")
	if err := atomicWrite(mdPath, []byte(r.Markdown())); err != nil {
		return nil, err
	}
	return []string{jsonPath, mdPath}, nil
}

// reportBaseName is the stable, filesystem-safe base name for a feature's report
// files — the slug with any path separator neutralized so a slug can never escape
// the sink directory (a defect report is scratch, never a path-traversal vector).
func reportBaseName(feature string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, feature)
	if safe == "" {
		safe = "contract"
	}
	return safe + "-defect-report"
}

// atomicWrite writes b to path via a temp file in the same directory + rename, so
// a concurrent reader observes either the old file or the complete new one, never
// a partial write. The temp lives in path's directory (rename is atomic only
// within a filesystem).
func atomicWrite(path string, b []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	// On any failure past creation, remove the temp so the sink is never littered
	// with partial writes.
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp into %s: %w", path, err)
	}
	return nil
}
