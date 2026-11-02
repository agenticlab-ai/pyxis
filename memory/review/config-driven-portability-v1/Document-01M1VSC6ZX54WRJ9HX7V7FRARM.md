# Review: config-driven-portability-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories — four blocking findings (a misframed AP-25 citation, a workflow class-transition that implied an undefined migration, a dangling A-7 acceptance anchor, and §4 dual-citing the chosen decisions as their own rejected alternatives) all applied in round 2; two contract-quality advisories declined with reason. |
| **Relevant-when** | auditing or consuming the review of design:config-driven-portability-v1; asking why it was approved; tuning tech-design-review calibration against a record-fed design |
| **Subject** | design:config-driven-portability-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · 2026-07-23 |
| **Date(s)** | 2026-07-23 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

Four lenses (contract-quality · coherence · upstream-alignment · feasibility-completeness) generated candidates over the draft and ran adversarial survival; the lint floor and §0 were excluded. Seven findings survived.

### FND-1 — AP-25 cited for the wrong property (the hook delegation is single-source, not vendor-neutrality)

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:§Principles/AP-25 — "the git-hooks delegate resolution to `pyxis` rather than carrying content" — vs — `docs/architecture-principles.md` AP-25 ("Agent-facing surfaces are vendor-neutral … a vendor surface (`.claude/`, `.cursor/`, a CLI config) may only point at it, adding invocation glue, never content"): a git-hook is not a vendor CLI surface, and the delegation is an AP-11 (single-source) property, correctly cited elsewhere.
- **Acceptance condition**: cite AP-25 only for the vendor CLI-surface pointers; attribute the hook delegation to AP-11.
- **Disposition**: applied

### FND-2 — the workflow "moves … to generated" implies an undefined operand migration

- **Severity**: blocking (CAL-4 — load-bearing for A-1, the DoD)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.1 — "the installed workflow moves from the engine-verbatim ownership class to the **generated** class" — vs — design:§3.4 boundaries + basis WR-3 (onboarding/upgrade deferred): "moves" implies an operand-side class migration the design neither defines nor tests, while phase-1 has no operand carrying an old-class workflow (the pyxis repo is source-mode; adoption is phase-2).
- **Acceptance condition**: reframe as a class-policy declaration (born-generated at first install), and defer migrating an already-installed operand's old-class workflow to phase-2.
- **Disposition**: applied

### FND-3 — R2 traces to a non-existent acceptance row; the hook-liveness property has no A-row

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:§5/R2 — "the hook reads live, so it cannot go stale | A-6, A-7" — vs — design:§6 (A-1…A-6 only): R2's verification cites `A-7`, a dangling anchor, and the runtime-resolution "cannot go stale" claim is unverified.
- **Acceptance condition**: add A-7 verifying a hook's runtime branch/remote resolution tracks config across two configs.
- **Disposition**: applied

### FND-4 — §4 dual-cites the chosen decisions (D-2/D-4) as their own rejected alternatives

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:§4 — "A dedicated target-facing 'resolver' port … (`design-brainstorm:…#D-2`)" and "Symlink-resolved path validation (`design-brainstorm:…#D-4`)" — vs — the record's D-2/D-4, which are live decisions the design also cites in §3.2/§3.3 as *chosen*: the same anchor names both the chosen ruling and the alternative it rejected, blurring which is which.
- **Acceptance condition**: cite the questions `#DQ-2`/`#DQ-4` in §4.
- **Disposition**: applied

### FND-5 — the derived-map pure-function property is asserted but not pinned by an acceptance row

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§3.2 — "computed as a **pure function** … with no cached or materialized derived state" — vs — design:§6 (no A-row verifies the pure-function/no-cache property).
- **Acceptance condition**: add an acceptance row asserting the derived maps recompute per use, not cache.
- **Disposition**: declined — A-1 (self-host on a non-default config) fails on a cached-but-wrong derivation, and the engine reads config once per run so mid-run cache-drift cannot occur; a dedicated cache test is defensive, not load-bearing for acceptance.

### FND-6 — the read-only "no file move" boundary is asserted but not pinned by an acceptance row

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§3.4 — "Implementation must not infer that a relocated path implies any file *move*" — vs — design:§6 (no A-row verifies filesystem non-mutation).
- **Acceptance condition**: add an acceptance row asserting no filesystem mutation when resolving a relocated path.
- **Disposition**: declined — the engine's gates/resolvers are read-only by construction; A-1 relocates the corpus externally and runs read-only checks; a no-mutation test asserts the absence of a capability the design never grants.

### FND-7 — A-2 reads as the sole coherence proof, under-scoping a grep

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§6/A-2 — "The coherence inspection finds zero target-facing literals in engine code outside the schema defaults" — vs — design:§3.2 (derived maps computed at runtime) + A-1: a literal grep cannot see a constant-folded derivation; A-1 is the full proof.
- **Acceptance condition**: clarify A-2 is a first-pass and A-1 (self-host on a non-default config) is the full coherence proof.
- **Disposition**: applied

## Round 2

Scope-locked re-review of the changed spans (§3.1, §3.4, §4, §6 A-2/A-7, Principles AP-25). Each round-1 blocking finding's fix verified resolved: AP-25 now cites only the vendor pointers with the hook delegation attributed to AP-11 (FND-1); §3.1 reads as a class-policy declaration with operand-upgrade migration deferred in §3.4 (FND-2); A-7 exists and R2's `A-6, A-7` both resolve (FND-3); §4 cites `#DQ-2`/`#DQ-4` (FND-4); A-2 carries the first-pass clarification (FND-7). No regression introduced in the changed spans; docgate green. The two declined advisories (FND-5, FND-6) remain recorded.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-23 |
| 2 | approve-with-advisories | 2026-07-23 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 4 blocking · 3 advisory
- **Dispositions**: 5 applied · 2 declined · 0 withheld-as-violation

## Overlay corrections

none
