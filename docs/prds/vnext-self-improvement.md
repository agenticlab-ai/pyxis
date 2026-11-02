# PRD — vNext: Self-Improvement & Adaptability

**Status: draft v0.1 — operator-fed requirement (2026-07-02) · NOT built.**
Part of the [Pyxis PRD set](README.md); serves the
[product vision](../product-vision.md). **Extends
[vnext-requirements.md](vnext-requirements.md) by addition** — that document is
immutable (AP-7), so new requirements land here with new stable IDs; nothing
prior is modified. IDs continue the VNEXT sequence.

## Scope

The factory must be **adaptable**: it conforms to each organization's,
project's, and codebase's working style instead of imposing its own, and it
improves its own operating artifacts from observed outcomes. Two features:
one learns the *environment's* norms (VNEXT-14), one optimizes the *factory's
own* behavior (VNEXT-15). Both adapt through versioned, gated, evidence-backed
changes — the system tunes itself and never silently drifts.

## VNEXT-14 — Convention adoption (org / repo style profiles)

**What:** The factory adopts the org/project/codebase style — it writes
changes the way this team writes changes.

**Requirements:**
- A **style profile** per org and per repo, learned and maintained as
  versioned, closed-schema data. It covers at minimum: change/PR size norms
  (e.g. typical lines/files per PR and when to split work accordingly),
  commit granularity and message style, code style beyond what linters
  enforce (idioms, structure, naming patterns), review etiquette, test
  conventions, and documentation conventions.
- **Sources:** the existing codebase, the history of merged changes, human
  exemplars, and explicit configuration — gathered at onboarding and kept
  fresh by the periodic-maintenance stage.
- **Consumers:** implementation planning (task sizing respects the PR-length
  norm), worker charters and prompts (style context injected), quality gates
  (advisory style findings), and evidence packages (deviations from the
  profile are declared, not hidden).
- **Precedence is fixed:** human-authored policy and per-repo configuration >
  learned profile > engine defaults. The profile may *tighten* behavior
  toward local norms; it may never loosen a gate, a policy, or a principle
  (the policy engine always wins).
- The profile is data judged at explicit points and frozen for use (AP-21) —
  no per-task re-inference of "how does this team like its PRs."

## VNEXT-15 — Self-optimization (prompts, thresholds, policies)

**What:** The factory improves its own operating artifacts — prompt
optimization, threshold tuning, routing and policy calibration — from
measured outcomes.

**Requirements:**
- **Optimization targets are versioned artifacts**: prompt templates and
  skills, gate/policy thresholds, routing tables, style profiles. There is no
  in-place mutation: every adaptation is a normal change through the pipeline,
  lane-gated by autonomy level and risk tier; anything governance-adjacent is
  always human-lane (the self-unlock guard, generalized).
- **Learning signals** come from the traceability and observability planes:
  review-rejection rate and human edit-distance on proposals, gate/CI failure
  rates, e2e verification outcomes, feedback-round counts, incident lessons,
  and token/latency cost. Backend scorecards (VNEXT-7) are one instance of
  this loop, not a separate mechanism.
- **Method:** measure → propose → trial (staged rollout / A-B via the
  platform's cohort machinery) → adopt or revert on evidence. Minimum-sample
  and maturation guardrails apply (the calibration pattern); every adopted
  adaptation carries its evidence package (VNEXT-5) citing the metric it
  targets.
- **Bounded by construction:** optimization never edits governing documents
  (AP-24), never widens the factory's own permissions, and every adaptation is
  observable and reversible — a versioned diff in the repo, never learned
  state hidden inside the engine.
- **Effectiveness is measured** against the vision's metrics: interventions
  per feature and cost per shipped feature should fall as the system adapts;
  an adaptation that moves neither is reverted.

## Relationships & sequencing

- Consumes: VNEXT-3 (traceability — the outcome signals), VNEXT-5 (evidence
  packages), VNEXT-7 (scorecards — the routing instance of this loop), the
  platform loop (cohorts/A-B), VNEXT-9/10 (risk + policy gating of
  adaptations).
- VNEXT-14 can start **heuristic** at onboarding (profile derived from the
  existing codebase and merged-change history) before the full VNEXT-15
  learning loop exists.

## Principles

AP-8 (this is the learned counterpart of the sanctioned config surface —
config wins); AP-10/AP-21 (profiles and optimizations are codified, judged
once, frozen as data); AP-14 (adopt on observed evidence, not plausibility);
AP-19 (adaptation work is budgeted and stated, not ambient); AP-24 (governing
docs are never optimization targets). Deviation: none.
