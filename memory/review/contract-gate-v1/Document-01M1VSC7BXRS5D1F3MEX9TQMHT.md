# Review: contract-gate-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | approve after a round-2 close: two blocking findings (the reused checks' feature-scoping mechanism unspecified; AP-23 honored-but-omitted) and three advisories (zero-token explicitness, fold-enrichment migration, one-decision justification) all applied. |
| **Relevant-when** | auditing or consuming the review of contract-gate-v1; asking why it was approved; tuning tech-design-review calibration against findings on a docgate-hosted gate design |
| **Subject** | design:contract-gate-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-07-17 |
| **Date(s)** | 2026-07-17 |
| **Verdict** | approve-with-advisories |
| **Tracking** | agenticlab-ai/pyxis — the introducing PR (filed with this design) |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — the zero-token property (NFR-3) is served but not stated in any binding element

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:Serves lists `prd:contract-gate/NFR-3` (zero model tokens) — vs — design:#DINV-5 (as drafted) bound network-free and clock/randomness-free but not model-free; the substance (deterministic mechanical checks) implements NFR-3, but no binding element labels it, so an implementer could add a model call without tripping an invariant.
- **Acceptance condition**: state the zero-token / no-model property in a binding element.
- **Disposition**: applied — DINV-5 now binds "no model invocation — zero tokens" and cites NFR-3.

### FND-2 — the reused tree-wide checks' feature-scoping mechanism is unspecified

- **Severity**: blocking (CAL-4 — load-bearing for A-1 / FR-1)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.9 (as drafted) says "the verb assembles the marked subset and runs it **feature-scoped**" — vs — `prd:contract-gate/FR-1` (a feature-scoped gate) and the fact that the three reused checks (`check-citations`, `GenCoverage`, the stale flags) are **tree-wide** today (`design-brainstorm:contract-gate-tech-design#F-2`). Bridging tree-wide checks to one feature is a core design mechanism the draft left implicit — A-1 is not buildable without it named.
- **Acceptance condition**: name how the verb scopes the reused checks to one feature (a scope parameter vs verb-side unit selection), and confirm their tree-wide behavior for existing callers is unchanged.
- **Disposition**: applied — §3.9 gained a "Feature-scoping the reused checks" paragraph: the verb selects the feature's units from the machine faces (the `prd:<feature>` family + the feature's serves-inverse) and runs each check over that subset; tree-wide behavior for existing callers is byte-unchanged (NR-2/NR-3).

### FND-3 — the fold-enrichment migration (re-fold existing projections) is unaddressed

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.6 adds per-requirement fields to the machine face "additive schema evolution" — vs — existing folded projections carry no such fields until re-folded; the draft did not say how they get populated, a gap an implementer hits on the first existing ledger.
- **Acceptance condition**: state that existing projections gain the fields on their next fold / a one-time re-fold populates them.
- **Disposition**: applied — §3.6 now states migration is a re-fold (regeneration, not a data migration), additive, changing no existing field's meaning.

### FND-4 — one-decision-per-document: five sub-decisions bundled without justification

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§2 + design:§3.6–§3.10 carry five `**Decision:**` callouts (home, inputs, output schema, registration, trigger) — vs — the tech-design-template's RW-5 rule (one independently-revisable decision per document); D-2 (fold enrichment) and D-3 (report retention) are arguably revisable on a different cadence than the tool-home spine, so the one-document boundary needed an explicit justification.
- **Acceptance condition**: justify the single-document bundling (they co-revise as one mechanism) or split the independently-revisable ones.
- **Disposition**: applied — §3.2 now states the five rulings co-revise as one mechanism, and names the condition under which the fold enrichment would supersede as its own docgate-schema design.

### FND-5 — AP-23 is honored by the design but omitted from Principles

- **Severity**: blocking (CAL-2 — principles honesty: a relevant honored principle silently omitted)
- **Lens**: upstream-alignment
- **Evidence**: design:Principles cites AP-27 (derived registration) — vs — design:§3.9's self-registration (no shared enumeration/dispatch/help line edited) is precisely AP-23's conflict-magnet-removal charter, honored here by extending the DCR pattern; citing AP-27 while omitting its sibling AP-23 understates the principles the design actually honors.
- **Acceptance condition**: cite AP-23 (honored) in Principles.
- **Disposition**: applied — Principles now carries an AP-23 (honored) bullet.

## Round 2

*Scope-locked to the five changed spans (DINV-5, §3.9 feature-scoping paragraph, §3.6 migration note, §3.2 co-revise note, Principles AP-23); the unchanged remainder is out of scope by protocol.*

- **FND-1** — closed: DINV-5 binds the zero-token property; NFR-3 now has a binding element. No regression: DINV-5's other clauses intact.
- **FND-2** — closed: §3.9 names the feature-scoping mechanism (verb-side unit selection from the machine faces); A-1/FR-1 is now buildable as specified, and NR-2/NR-3 (tree-wide behavior unchanged) is honored. No regression: the registration paragraph above it is intact.
- **FND-3** — closed: §3.6 states the re-fold migration; DINV-7 (additive) unaffected.
- **FND-4** — closed: §3.2 justifies the one-mechanism bundling and names the supersession condition; the boundaries list is otherwise intact.
- **FND-5** — closed: AP-23 (honored) cited; the Deviation line stays "none."

No new finding on unchanged content (would be a protocol violation). All five round-1 findings dispositioned `applied`; lint + gate re-run green.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-17 |
| 2 | approve-with-advisories | 2026-07-17 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 3 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
