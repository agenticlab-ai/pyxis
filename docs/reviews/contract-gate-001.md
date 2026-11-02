# Review: prd:contract-gate/001 — review report

| Field | Value |
|---|---|
| **In one line** | approve after a scope-locked round-2 close of four blocking findings and one advisory: the amendment now makes exact independent human attestation—not a nonexistent inferred binding universe—the compatibility authority, binds approval to the exact disposition, repairs readiness and absent-record semantics, and validates disposition PRs continuously. |
| **Relevant-when** | auditing or consuming the review of the contract-gate currency-compatibility amendment; asking why the narrow stale-edge exception is approved; designing its disposition and approval machine faces |
| **Subject** | prd:contract-gate/001 |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-29 |
| **Date(s)** | 2026-07-29 |
| **Verdict** | approve |
| **Tracking** | agenticlab-ai/pyxis — the stacked amendment PR filed with this report |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — no authoritative consumed-binding universe makes the design-dependency Must unverifiable

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-15/FR-16 required equality with a subject machine face's complete consumed-binding set, while FR-18 forbade prose inference — vs — `design-brainstorm:planner-driven-model-routing-tech-design-3#D-7` requires exact consumed-anchor mapping, and the current design index exposes whole-document `builds_on[]` but no edge-specific consumed-anchor universe.
- **Acceptance condition**: either define the independent complete machine universe, or explicitly make a disposition-bound independent human attestation authoritative for scope completeness and compatibility while removing any claim that the gate infers consumed bindings; keep exact anchor/fingerprint validation and fail-closed absence.
- **Disposition**: applied — FR-15/FR-16 now assign scope-completeness and compatibility to the exact independent human attestation; FR-16/FR-18 confine the gate to structural, freshness, authority, and attestation validation; DOD-14 carries the reviewer rubric and §6.3 names the human-error residual.

### FND-2 — approval-reference presence does not prove independent approval of the exact disposition

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-15/FR-16 initially required only a typed human principal and present approval reference — vs — D-7 requires an independently reviewed disposition, AP-13 requires exact-artifact evidence, and incident I2/AP-26 reject an intermediate status or reference as proof of the goal object.
- **Acceptance condition**: require one closed resolvable attestation binding the exact disposition fingerprint, edge, mapping digest, compatible verdict, reviewer authority, and policy; prove reviewer independence from the author; reject unrelated, stale, unauthorized, revoked, non-approved, and status-only evidence.
- **Disposition**: applied — FR-15 defines the separate exact attestation and canonical non-self-referential record fingerprint; FR-16 and UB-10 validate exact binding, authority, revocation, and author/reviewer independence; EVT-5 preserves the identities and policy fingerprint.

### FND-3 — folded §8 certifies the superseded pre-amendment contract

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: amendment 001 adds FR-14..FR-19, NR-4, DOD-11..DOD-15, and DOD-14's named human rubric, while folded §8 still listed FR-1..FR-13, NR-1..NR-3, DOD-1..DOD-10, and no human-judgment criteria — vs — the PRD template's contract-readiness gate must verify the live contract, not trust stale counts.
- **Acceptance condition**: supersede §8 wholesale with the exact live FR/NR/DoD sets, every changed trace, the new risks, and DOD-14's owner/rubric; identify every superseded row accurately.
- **Disposition**: applied — §8 now evaluates the exact live sets (`FR-1`, `FR-3..FR-8`, `FR-11..FR-19`; `NR-1..NR-4`; live DOD rows), names FR-2/FR-9/FR-10 and DOD-2/DOD-4/DOD-8/DOD-9 superseded, and records DOD-14's human owner/rubric.

### FND-4 — zero dispositions has two incompatible verdict contracts

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: FR-14 and DOD-11 classify an absent disposition as an uncovered stale edge with a currency defect, matching D-7, while UB-9 also triggered on zero records and required an ambiguity defect naming competing records that do not exist.
- **Acceptance condition**: keep zero records solely on FR-14's ordinary uncovered-currency path; restrict UB-9 to multiple, duplicate, conflicting, or multi-edge claims; separate their fixtures and DoD expectations.
- **Disposition**: applied — UB-9 excludes zero records and explicitly routes absence to FR-14; DOD-11 covers uncovered currency while DOD-13 covers the narrowed ambiguity failures.

### FND-5 — the continuous trigger excludes the disposition PR that activates the exception

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: the amendment's §5.5 interface claimed the CI context covers PRD/design/disposition PRs, but live FR-9 covered only PRD or design PRs and DOD-4 carried the same older scope — vs — D-7 makes the independently reviewed disposition a critical contract input whose invalid state must fail before activation.
- **Acceptance condition**: supersede FR-9 and DOD-4 so the same candidate-contract gate runs on currency-disposition PRs, with malformed, expired, ambiguous, unrelated, and unauthorized evidence blocking and coverage-only gaps advisory.
- **Disposition**: applied — FR-19 and DOD-15 add the disposition PR class and exact approval-integrity fixtures; §5.5 cites the live row and UB-7..UB-10.

## Round 2

*Scope-locked to FR-15/FR-16/FR-18/FR-19, UB-9/UB-10, ABUSE-4, EVT-5, DOD-11/DOD-13..DOD-15, §5.4, §5.5, §6.2, §6.3, Q5, and §8; unchanged content was out of scope.*

- **FND-1** — closed: the contract no longer claims a nonexistent machine completeness oracle. The exact independent human attestation owns completeness/compatibility; the gate validates declared anchors, fingerprints, authority, and attestation integrity only. No regression: DOD-14 and R5 make the semantic residual explicit and attributable.
- **FND-2** — closed: exact disposition/edge/mapping/policy binding, author/reviewer independence, authorization/revocation, and status-only refusal are all contractual and fixture-verifiable. No regression: Q5 leaves only adapter-neutral representation to design.
- **FND-3** — closed: §8 reflects the actual live and superseded sets and DOD-14's rubric. The first scope-locked read caught FR-9 mistakenly left in the live list; that changed-span reference was corrected before the round-2 verdict.
- **FND-4** — closed: zero records has one uncovered-currency path; ambiguity starts at competing or multi-edge records.
- **FND-5** — closed: FR-19/DOD-15 and §5.5 agree on all three PR classes and disposition-integrity failures. The first scope-locked read caught stale FR-9/UB-9 references inside the replaced §5.5 span; they were corrected to FR-19 and UB-10-inclusive ranges before verdict.

No new finding on unchanged content. All round-1 findings are applied and closed; the changed-span regression screen is clean.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-29 |
| 2 | approve | 2026-07-29 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 4 blocking · 1 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
