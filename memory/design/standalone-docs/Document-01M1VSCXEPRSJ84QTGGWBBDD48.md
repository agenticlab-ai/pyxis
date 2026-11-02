# Standalone doc PRs — governed contract classes ship one-per-PR, implementation stacks

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(Draft while this PR is open; merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-10 |
| **Relevant-when** | opening a PR that changes a PRD ledger, a technical design, or the incident registry; wondering why the gate rejected a bundled change; deciding what may ride with implementation code |
| **Scope** | The `check-standalone-docs` gate verb (FR-30): what counts as a governed contract class, the code/second-class bundling it forbids, and the generated-projection + rollup carve-outs. **Not**: the one-ledger-entry cap (FR-27), the rollup requirement (FR-10), design immutability (FR-17), the governing-doc rules (FR-14/18). |
| **Tracking** | filed with this PR · motivating case: PR #82 |
| **Builds-on** | docs/tech-design/docgate-v2.md |
| **Supersedes** | — |
| **Serves** | prd:docgate/FR-30 |
| **Security/privacy impact** | none — reads the merge-base name diff over tracked paths; no new surface |
| **Template version** | tech-design-template/v1 |

## 1. Problem

"One concern per PR" (AGENTS.md) keeps review honest and concurrent authoring collision-free: every PRD amendment, technical design, and incident change ships as its own PR, and implementation stacks on the approved contract. `check-single-amendment` (FR-27) enforces one slice — at most one PRD ledger entry per PR — but nothing stops that entry riding with a technical design and a pile of implementation code. PR #82 did exactly that (an amendment, a new design, the impl, and the hook) and passed the gate green.

The cost is not cosmetic. Bundling a contract with the code that implements it means the contract is never reviewed on its own merits — FR-16's "PRD precedes design" is satisfied trivially (both land at once) rather than in fact (approve, then build). A bundle also mixes two review altitudes — a requirements decision and an implementation — in one diff, the review-honesty problem the discipline exists to prevent.

## 2. Decision

**Decision:** a merge-base gate verb, `check-standalone-docs`, fails a diff that touches a **governed contract class** if it also touches implementation code or a *second* governed class.

The three governed classes, each an authored contract:
- **PRD ledger entry** — an `NNN-<slug>.md` one directory below the PRDs root.
- **Technical design** — an authored `*.md` under the tech-designs path, excluding the generated `index.*`.
- **Incident registry** — the configured incidents file.

Two carve-outs, forced by the rest of the gate and the plane model:
- **Generated projections ride with their source.** `effective.*` (with its amendment) and `index.*` (with a design) are not governed classes — they are generated, and `verify-generated`/`verify-index` already require them in the same PR.
- **Rollups ride with code.** `docs/techspec.md` and `docs/prd.md` are not governed classes — `check-rollups` (FR-10) *requires* them alongside a capability change.

Everything else — `AGENTS.md`, the wiki, the system maps, the governing docs — is neither governed-contract nor code here: it does not trigger the verb and does not count as a second class (those docs carry their own rules where they have them).

**The verdict, precisely:** collect the governed classes the diff touches. Fail if **more than one** class is present (two contracts in one PR), or if **any** class is present **and** the diff also touches a capability path (a contract bundled with its implementation). A pure-code, pure-rollup, or single-class change passes. FR-27 still caps ledger entries; this verb is orthogonal — it governs cross-class and doc↔code bundling.

## 3. The design

### 3.1 The check

`check-standalone-docs` is merge-base-gated, so it rides pre-push and CI like `check-immutable`/`check-single-amendment`. It reads `git diff --name-only -z` against the merge base (NUL-separated, so a non-ASCII path cannot fail open — the FR-27 lesson) and classifies each path in priority order: a generated projection or a rollup → exempt; else a governed class by its path shape; else a capability path → code. It then applies the two-part verdict of §2. The defect names the classes (and, for the code case, a few offending paths) and the remedy: split into one PR per class, stacked contract → design → implementation.

### 3.2 What rides where — the stack it enforces

The discipline it makes mechanical is the shape used to land this very change:
- **contract PR** — the amendment + its `effective.*` (one governed class, no code).
- **design PR** — the design + its `index.*` (one governed class, no code), stacked on the contract.
- **implementation PR** — the gate code + the `techspec.md` rollup (no governed class), stacked on the design.

Each passes `check-standalone-docs`; the bundle that is PR #82 fails it.

### 3.3 Boundaries and non-goals

- **Not the one-entry cap.** Two ledger entries in one PR is FR-27's job; this verb sees "PRD ledger" as a single class regardless of entry count.
- **Not the governing docs.** vision/principles have `check-governing`; maps and `AGENTS.md` have no standalone rule and are not swept in here.
- **Carve-out drift is the maintenance surface.** The exempt set (generated projections, rollups) is defined from config paths and the `NNN`/`index.*` shapes; a new committed generated surface must be added to the exemption or it will read as a governed class (§3.5).

### 3.4 Invariants

- **DINV-1** — a PR touching exactly one governed class and no capability path passes; a governed class plus any capability path fails; two governed classes fail.
- **DINV-2** — the generated projections (`effective.*`, `index.*`) and the rollups (`docs/techspec.md`, `docs/prd.md`) never count as a governed class, so amendment-with-projection, design-with-index, and code-with-rollup all pass.
- **DINV-3** — the check is orthogonal to FR-27: it never inspects entry *count*, only class *membership* and code presence.

### 3.5 Observability

`check-standalone-docs` emits a named defect (classes + remedy) on the shared `--defects-json` channel, exactly as the sibling verbs; there is no new surface. A carve-out gap surfaces as a false-positive defect on a legitimate PR — the signal to widen the exemption.

### 3.6 Security & privacy

none — the check reads the merge-base name diff over tracked paths; nothing new crosses any boundary.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Status quo — FR-27 only | FR-27 caps entry count but not doc↔code or cross-class bundling; PR #82 is the live gap |
| A warning, not a gate | the value is that the contract is *approved* alone; a warning is ignored under churn, and the point is a hard split |
| Per-file "one changed file per PR" | too blunt — a contract legitimately carries its generated projection; the unit is the *class*, not the file |
| Fold it into `check-single-amendment` | different predicate (class membership + code presence vs entry count) and wider scope (designs, incidents); one predicate per concern (AP-11) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | A false positive blocks a legitimate combination | author friction | the carve-out set is explicit and config-derived; the defect names exactly what tripped it, so the fix (split, or widen the carve-out) is obvious | fixture per green/red case; the real repo green |
| R2 | A new committed generated surface reads as a governed class | spurious red on regen-only PRs | generated shapes are matched by path (`NNN`/`index.*`) + config rollups; a new generated surface adds its exemption in the same change | §3.3; the carve-out fixture |
| R3 | Authors split code and docs across commits on one branch to evade it | none — the check is diff-scoped, not commit-scoped | it inspects the whole merge-base diff | diff-scope reasoning; the real-repo fixture |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | an amendment + a design in one diff → red naming both classes | Test | fixture | prd:docgate/FR-30 · DINV-1 |
| A-2 | a governed class + a capability path → red naming the class and code | Test | fixture | prd:docgate/FR-30 · DINV-1 |
| A-3 | a lone amendment with `effective.*`, a lone design with `index.*`, and a pure code+rollup change → each green | Test | fixtures | DINV-2 |
| A-4 | the verb rides the gate umbrella (merge-base branch) and is a standalone `check-standalone-docs` verb | Inspection | gate wiring + main dispatch | §3.1 |
| A-5 | this change lands as the three-PR stack it enforces, each PR green under the new verb | Inspection | the contract/design/impl PRs | §3.2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Should the system maps and governing docs join the governed set later? | operator | nothing | open | out of scope now — they carry their own rules (`check-governing`) or none; revisit if map/governing PRs prove they need the same standalone discipline |

## Principles

- **AP-23 — honored (the reason for this design).** The contract, the design, and the code become separately-authored, separately-reviewed PRs; parallel authors collide on nothing and the review altitudes never mix.
- **AP-11 / INV-30 — honored.** One binary, one more read-only predicate over the same merge-base diff the sibling checks use; no new machinery, no twin.
- **AP-13 — honored.** "This PR is one concern" becomes a verifiable verdict, not a reviewer's memory of AGENTS.md.
- **AP-24 — honored.** The verb gates structure and governs no content; the documents remain the sole authority.
- **AP-19 — honored.** Deterministic name-diff arithmetic; zero steady-state cost.
- **Deviation:** none.
