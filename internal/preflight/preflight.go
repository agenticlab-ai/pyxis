// Package preflight is init's checked front door (design init-hardening-v1
// §3.1; plan init-hardening/001 T1): one predicate home for every
// environment and admission check init refuses on pre-write, preview will
// report in full (T9), and doctor's prerequisite rows will delegate to
// (T10) — DINV-2's one-evaluator-per-predicate, made structural rather than
// a convention three packages could quietly drift apart on (incident I5).
//
// The registry is a small ordered list of Checks (design §3.1's numbered
// list — evaluation order is meaningful and never resorted): each is a row
// of id, an applicability predicate over the resolved-or-prospective knobs
// (nil = always applicable, the shape every environment/admission check
// that runs before elicitation takes), and a read-only probe. A probe
// yields an Outcome — pass, refuse (with remedy and, where one exists,
// route), or advise (a warning that never blocks, DINV-7) — data every
// consumer renders identically (AP-10) rather than three independent
// prose implementations.
//
// T1 seeded the registry with its first check, Admission (DINV-5). T2's
// Toolchain and Artifact-source checks (design §3.1 checks 2 and 5) and
// T3's GitHub-mode, scope-prediction, and cross-knob-coherence checks
// (checks 3/4/6) each landed in the evaluation-order position the design
// assigns — the full §3.1 list, no contract reshaped.
package preflight

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// Status is a check outcome's class — the three the design names.
type Status string

const (
	// StatusPass means the check found nothing to refuse or advise on.
	StatusPass Status = "pass"
	// StatusRefuse means the check blocks: no write may proceed.
	StatusRefuse Status = "refuse"
	// StatusAdvise means the check found something worth naming that never
	// blocks (DINV-7 territory — a later check, T3's scope prediction).
	StatusAdvise Status = "advise"
)

// Outcome is one check's result — the registry's per-row contract (design
// §3.1): a pass carries nothing further; a refuse carries the cause, the
// enumerated conflicting evidence, the remedy, and — where one exists — the
// route to the command that does handle the target; an advise carries a
// warning that never blocks. GitInit and Branch are admission-specific
// pipeline facts (meaningful only when Check == AdmissionID and Status ==
// StatusPass) — admission is the one check whose pass still hands its
// caller a decision (create the repository, or not) rather than a plain
// "proceed".
type Outcome struct {
	Check  string // the owning check's id
	Status Status

	Reason    string   // refuse: the human-readable cause
	Conflicts []string // refuse: the enumerated conflicting evidence
	Remedy    string   // refuse: the fix, runnable where possible
	Route     string   // refuse: the follow-up command that handles the target ("" = none exists)

	Warning string // advise: the non-blocking warning

	GitInit bool   // admission pass only: true when the caller must git init
	Branch  string // admission pass only: the branch to create/commit onto
}

// Input is what a check's Applicability and Probe evaluate against — the
// target directory, the release pin + artifact-source override, and the
// resolved-or-prospective knobs. System and Knobs are their zero values
// before elicitation resolves them; checks that run pre-elicitation
// (Admission, Toolchain, Artifact source) either never consult them
// (Toolchain) or tolerate the zero value as "nothing to check yet"
// (Artifact source — see probeArtifactSource). Growing this struct as
// later checks land (T3) is additive and never breaks an
// already-registered check.
type Input struct {
	Target string
	// System carries the release pin pair + artifact-source override
	// (installer.Options.Pin + .ArtifactSource) — available before
	// elicitation, unlike Knobs, so the checks that need only these two
	// fields (Artifact source) run in the same pre-elicitation pass
	// Admission and Toolchain do.
	System config.System
	Knobs  config.Knobs

	// Coordinate is the supplied owner/name GitHub coordinate — a promptless
	// flag/answers value (prd:init-hardening FR-7) or one the interactive
	// exchange elicited and folded here (FR-6); "" means none arrived. The
	// GitHub-mode check consults it only when the target carries no
	// configured remote to parse one from (design §3.7's resolution
	// order) — the remote always wins when both are present.
	Coordinate string

	// WireRunner is the gh transport the GitHub-mode and scope-prediction
	// checks probe through (presence, auth, existence, scopes); nil uses
	// the production wire.GHRunner{}. The same seam installer.Options.
	// WireRunner threads to wire.Execute at apply time — one gh boundary,
	// never a second.
	WireRunner           wire.Runner
	Integrations         integrations.Config
	ForgejoClientFactory func(integrations.ResolvedRepository) (*forgejo.Client, error)
}

// Check is one registry row: a stable id, an applicability predicate over
// the resolved-or-prospective knobs (nil means always applicable — the
// environment/admission checks that run before the first question), and a
// read-only probe. Checks run in registration order; Registry's order is
// the design's numbered evaluation order and is never resorted.
type Check struct {
	ID            string
	Applicability func(config.Knobs) bool
	Probe         func(ctx context.Context, in Input) (Outcome, error)
}

// Registry returns every check in evaluation order (design §3.1's numbered
// list — the complete set: 1 admission, 2 toolchain, 3 GitHub mode,
// 4 scope prediction, 5 artifact source, 6 cross-knob coherence).
func Registry() []Check {
	return []Check{
		admissionCheck,
		toolchainCheck,
		processBoundaryCheck,
		githubModeCheck,
		scopePredictionCheck,
		artifactSourceCheck,
		crossKnobCoherenceCheck,
	}
}

// Report is every evaluated check's outcome, in evaluation order.
type Report []Outcome

// Run evaluates every applicable check in Registry(), in order, against in.
// Every outcome is collected — pass, refuse, and advise alike (AP-16: no
// check skips silently) — so a caller can render the whole pass in one
// shot even though the returned error carries only the FIRST refusing
// outcome (FR-16's "a user fixes everything in one pass": the report holds
// every problem, the error names the one that governs the exit). A
// probe's own infra failure aborts the run immediately — there is nothing
// left to check honestly once a probe itself cannot run.
func Run(ctx context.Context, in Input) (Report, error) {
	var rep Report
	var refused *Outcome
	for _, c := range Registry() {
		if in.Integrations.SchemaVersion != "" && (c.ID == GitHubModeID || c.ID == ScopePredictionID) {
			continue
		}
		if c.Applicability != nil && !c.Applicability(in.Knobs) {
			continue
		}
		out, err := c.Probe(ctx, in)
		if err != nil {
			return rep, fmt.Errorf("preflight: check %s: %w", c.ID, err)
		}
		out.Check = c.ID
		rep = append(rep, out)
		if out.Status == StatusRefuse && refused == nil {
			refused = &out
		}
	}
	if in.Integrations.SchemaVersion != "" && in.Knobs.Tracker != "" {
		rows, err := IntegrationChecks(ctx, in)
		if err != nil {
			return rep, fmt.Errorf("preflight integrations: %w", err)
		}
		for _, out := range rows {
			rep = append(rep, out)
			if out.Status == StatusRefuse && refused == nil {
				copy := out
				refused = &copy
			}
		}
	}
	if refused != nil {
		return rep, &RefusalError{Outcome: *refused}
	}
	return rep, nil
}
