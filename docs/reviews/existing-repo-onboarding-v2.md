# Review: existing-repo-onboarding-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | request-changes — six blocking issues (a dangling `#D-11` behind a green gate, an unbuildable undo-detection schema, DINV-3 with no A-row, two overclaimed Serves, a softened FR-9, two omitted principles) plus four advisories; all applied in one revision → approve-with-advisories. |
| **Relevant-when** | auditing or consuming the review of existing-repo-onboarding-v1; asking why it bounced; tuning tech-design-review calibration |
| **Subject** | design:existing-repo-onboarding-v2 · shape: none · supersedes: existing-repo-onboarding-v1 |
| **Moment** | supersession *(the review ran against the draft that merged as v1 (#658) before dispositions could land; the corrections are applied in the superseding v2 this report rides — no decision of record changed)* |
| **Reviewer** | tech-design-review · .pyxis/authoring/tech-design-review.md (2026-07-24) |
| **Date(s)** | 2026-07-24 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(the v2 introducing PR)* |
| **Template version** | design-review-report/v1 |

## Round 1

Four blind lenses generated over the shared context pack; candidates survived independent cross-scrutiny before landing. The `lint-tech-designs` floor (header schema, Serves resolution/liveness, mandatory sections) is excluded by protocol. Confirmations recorded across the lenses: incident I5 is genuinely the two-list-drift/single-source lesson the design cites it for; the `ALT-1..6` ledger, the `S-n` seam map, `O-1/O-2`, and `G-1` all carry faithfully; `Deviation: none` re `prd:installer/NG-3` is true; no contradiction with the Builds-on designs.

### FND-1 — The undo Decision's `#D-11` anchor is dangling — it names the stage-3 record, but D-11 lives only in the stage-1 brainstorm

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment + coherence (independent convergence)
- **Evidence**: design:§2 (*"failing closed on operator-edited artifacts (`#D-11`)"*) and design:§3.5 (*"undo … (`design-brainstorm:…#D-11`; …)"*) attribute the undo ruling to `design-brainstorm:existing-repo-onboarding-tech-design#D-11` — but that record's Decisions are **D-1..D-6 only**; there is no D-11. The ruling lives at `brainstorm:existing-repo-onboarding#D-11` ("Undo semantics on a diverged tree"). The gate passes only because check-citations does a permissive whole-file `\bD-11\b` match that incidentally hits "D-1..D-11" and "D-7/D-11" elsewhere in the stage-3 record — so the citation is substantively dangling behind a green gate. — vs — `brainstorm:existing-repo-onboarding#D-11` (the true home).
- **Acceptance condition**: retarget both undo citations to `brainstorm:existing-repo-onboarding#D-11`.
- **Disposition**: applied (round 2)

### FND-2 — The `adoption-plan/v1` schema carries no per-added-path baseline, so undo's DINV-5 detection is unbuildable — A-7 cannot pass as drawn

- **Severity**: blocking (CAL-4 escalation — load-bearing for A-7)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.5/DINV-5 require undo to preserve-and-name *"a path in `added_paths` the operator has modified since adoption"*; detecting "modified since adoption" needs a baseline. design:§3.2's schema `added_paths: [ <path>, … ]` is a bare path list — no per-path hash. For the `add`/`chain` classes (the chained-hook wrapper, an added file) there is no manifest SHA and no generator to derive a baseline from, and the round-trip fixture leaves apply uncommitted (A-2 asserts HEAD equals the pre-adoption commit after undo), so no git blob exists at `epoch.commit` to diff against either. Undo cannot distinguish an operator-edited `add`/`chain` artifact from an untouched one. — vs — `prd:existing-repo-onboarding/UB-6 · FR-9` (the preserve-and-name behavior A-7 must prove) · #DINV-5.
- **Acceptance condition**: give the schema a per-added-path content baseline captured at apply (a hash per `added_paths` entry, at least for `add`/`chain` classes), and specify per-class detection (engine-verbatim → manifest SHA; generated → regenerate-and-compare; add/chain → stored hash).
- **Disposition**: applied (round 2)

### FND-3 — DINV-3 is enforced by no acceptance row

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:§3.8 DINV-3 (*"the `adoption-plan/v1` record is the single durable statement of what adoption agreed and did; `apply` and `undo` bind to it, and no second record of the same fact is kept"*) appears in no §6 A-row's Traces-to (A-rows cite #DINV-1/2/4/5/6, never #DINV-3). R4 (§5) intends A-7 to cover it, but A-7 traces only #DINV-5 and tests fail-closed-on-modification, not the single-record/binding property. — vs — the convention that every DINV is enforced by an acceptance row.
- **Acceptance condition**: add an A-row asserting DINV-3 (exactly one `adoption-plan/v1` record after apply and after undo; both operations bound to it) and trace `#DINV-3`.
- **Disposition**: applied (round 2)

### FND-4 — Overclaimed Serves: UB-1 (empty→init routing) and UB-5 (non-git refusal) are served but implemented by no §3 mechanism and no A-row

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the header Serves lists `…/UB-1 · …/UB-5`, but design:§3.1 only *"run[s] converge's classification over the non-empty tree"* — the empty-tree→init routing and the non-git refusal appear in no §3 mechanism and no A-row (A-1..A-10 exercise neither). §3.9 names only UB-2/UB-7 refusals. — vs — `prd:existing-repo-onboarding/UB-1` (Must: route to init, write no plan) and `/UB-5` (Must: refuse a non-git target before inspecting). The lint proved the anchors resolve; the substance is absent (the I4-inverse).
- **Acceptance condition**: add a §3.1 dispatch — an empty tree routes to `init` and writes no plan; a non-git target refuses by name before inspecting — plus an acceptance row asserting each.
- **Disposition**: applied (round 2)

### FND-5 — FR-9 softened: undo drops the PRD's "name any manual addition for the human to reverse" clause

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:§3.5 enumerates undo as unchain-hook + delete-`added_paths` + fail-closed on operator-edited *added files* (UB-6) — but the `collide-manual` additions (the lines the human added to their own `AGENTS.md`/`CLAUDE.md` at adopt time) are never in `added_paths` (adoption never touched that owned file), so undo has no path to surface them; §3.5 and A-7 are silent on them. — vs — `prd:existing-repo-onboarding/FR-9` (undo *"shall name any manual addition for the human to reverse"*; PRD §4.4).
- **Acceptance condition**: §3.5 states undo names the plan's `collide-manual` entries for the human to reverse (symmetric to adopt emitting them), with an acceptance row asserting it.
- **Disposition**: applied (round 2)

### FND-6 — Silently-omitted honored principles: AP-17 (fail-closed) and AP-20 (blast radius) are load-bearing yet absent from §Principles

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: the design honors `docs/architecture-principles.md#AP-17` (*"Fail closed on destructive … Ambiguity never deletes"*) — undo preserves-and-names on ambiguity (DINV-5), and the shallow-clone epoch fails closed (§3.4) — and `#AP-20` (*"Narrowest blast radius … enforced mechanically (guards), not by convention"*) — DINV-1's never-touch is *"a confinement, not a promise,"* the exact foreign-repo-write confinement the Builds-on `installer-v1` cites AP-20 for — yet neither is in §Principles. — vs — the governing rule "deviation-with-reason is legitimate; silent omission is not."
- **Acceptance condition**: add AP-17 (honored — fail-closed undo + shallow-clone gate-everything) and AP-20 (honored — never-touch confinement as the mechanically-enforced narrowest blast radius) to §Principles.
- **Disposition**: applied (round 2)

### FND-7 — The one converge change is under-counted: apply's "call converge's existing write primitives" — those primitives do not exist as callables

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.1 asserts the converge change is exposing the classification, and apply will *"call converge's existing write primitives."* Verified against `internal/converge/converge.go`: the scan is already a write-free exported callable (`Drift` runs `scanTree`), so exposing the read is near-nil — but the per-class writes live inline in `Run`'s loop (private `extractFiles`/`writeTreeAtomic`, entangled with archive acquisition + manifest re-hash); there is no `RepairVerbatim`/`WriteGenerated` primitive to call, and `Drift`'s `[]Finding` discards `genBytes`. Exposing the write is the surgery §3.7 says it avoids. — vs — the appetite estimate and DINV-2 (the converge work is larger and different in kind than "expose the classification").
- **Acceptance condition**: split the converge change into (a) the already-existing read exposure and (b) a NEW named exposure of archive-backed engine-verbatim + generated write primitives (returning the `Action` set); state apply consumes findings + genBytes + verified-archive-bytes, not `Drift`'s findings alone.
- **Disposition**: applied (round 2)

### FND-8 — Apply's step ordering and idempotency around the hook-path capture are unspecified; a re-apply or crash records the engine's own dir as `prior_hooks_path`

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:§3.1/§3.3 describe no ordering for apply's cross-step mutations (set `core.hooksPath` to the engine dir; record `hooks.prior_hooks_path`; …). If the prior path is not durably recorded *before* `core.hooksPath` is overwritten, an interrupted apply or a second idempotent apply (FR-11) reads the already-displaced `core.hooksPath` — now the engine's own dir — and records it as "prior"; undo then restores a self-referential path. The round-trip fixture runs apply once cleanly, masking it. — vs — `prd:existing-repo-onboarding/FR-11 · UB-4` (idempotent re-apply/re-adopt) and A-4 (undo restores the prior path).
- **Acceptance condition**: specify apply's ordering (capture + durably record `prior_hooks_path` before overwriting `core.hooksPath`) and the idempotency guard (skip the capture when `core.hooksPath` already resolves to the engine hook dir — the already-chained no-op).
- **Disposition**: applied (round 2)

### FND-9 — A-8 frames the epoch-scaling proof as a wall-clock "gate cost" measurement — the measurement-as-gate category error incident I5 (which this design cites) forbids

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:§6 A-8 (*"a fixed change-set against a growing corpus yields flat gate cost | Analysis (scaling test)"*) reads as a wall-clock curve, where §3.4's mechanism affords a deterministic observable (one `git diff` invocation; files-visited = |changed ∩ scan-set|, invariant to corpus). — vs — `docs/observability/incidents.md#I5` (*"a wall-clock/ambient-git assertion is a measurement, not a correctness check … Measurements record; they never gate"*) and the PRD's own count-based fit `prd:existing-repo-onboarding/NFR-1`.
- **Acceptance condition**: restate A-8's Verify/evidence to the deterministic observable (one `git diff` regardless of corpus; files gate-visited = |changed ∩ scan-set|) as the pass/fail; any wall-clock figure is a recorded measurement, never the verdict.
- **Disposition**: applied (round 2)

### FND-10 — DINV-1's "reads into" clause over-claims: §3.6 detection must read never-touch marker files, and no A-row checks a read-confinement

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality + coherence (independent convergence)
- **Evidence**: design:§3.8 DINV-1 (*"`adopt` never writes into, reads into, or configures any path in the plan's never-touch set"*) vs design:§3.6 (detection *"reuses the existing ecosystem marker registry (go/python/node marker files)"*) — the adopter's `go.mod`/`package.json` are manifest-complement (never-touch) yet must be read to build the plan; and every enforcing A-row (A-2/A-5/A-6) proves only byte-identity (not-written), never a read-confinement. — vs — the convention that a DINV states a checkable property that §3 delivers.
- **Acceptance condition**: scope DINV-1 to write/configure/mutate confinement (drop "reads into", or narrow to "never ingests adopter content into the managed/written set"), so it coheres with §3.6's necessary marker reads and A-2's byte-identity check.
- **Disposition**: applied (round 2)

## Round 2

Scope-locked re-review (protocol step 6): only the round-1 findings' acceptance conditions and regressions within the changed spans were examined. In this pre-engine bootstrap the author and reviewer are the same principal; all ten dispositions were applied in one revision — **landing in the superseding `existing-repo-onboarding-v2`, because v1 merged (#658) before the dispositions could land in it.** None required a new product ruling (every finding is a design-refinement faithfully expressing the already-settled stage-3 direction, not a change of direction).

**Acceptance conditions met (10/10):**

- **FND-1** (blocking) → both undo citations retargeted to `brainstorm:existing-repo-onboarding#D-11`.
- **FND-2** (blocking) → §3.2 schema gains `added_paths` as `{path, sha256}` entries; §3.5 + new DINV specify per-class "modified since adoption" detection (engine-verbatim → manifest SHA, generated → regenerate-compare, add/chain → stored hash).
- **FND-3** (blocking) → new A-11 asserts the single `adoption-plan/v1` record after apply and after undo, tracing `#DINV-3`.
- **FND-4** (blocking) → §3.1 gains the empty-tree→`init` and non-git-refusal dispatch; new A-12 asserts both.
- **FND-5** (blocking) → §3.5 states undo names the plan's `collide-manual` entries for reversal; A-7 extended to assert it.
- **FND-6** (blocking) → AP-17 and AP-20 added to §Principles (honored).
- **FND-7** (advisory) → §3.1/§3.7 split the converge change into read-exposure (near-existing, `Drift`) + a new named write-primitive exposure; apply consumes findings + genBytes + verified-archive.
- **FND-8** (advisory) → §3.3 specifies apply's ordering (record prior path before overwrite) + the already-chained idempotency no-op.
- **FND-9** (advisory) → A-8 restated to the deterministic count (one `git diff`; files-visited = |changed ∩ scan-set|).
- **FND-10** (advisory) → DINV-1 narrowed to write/configure/mutate confinement.

**Regressions in changed spans:** none — the new DINV-7 (per-class detection) and A-11/A-12 are traced and consistent; the retargeted `#D-11` resolves; the added principles are honestly honored; `docgate gate` re-runs green.

Two-round cap reached with all blocking findings resolved.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-24 |
| 2 | approve-with-advisories | 2026-07-24 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 6 blocking · 4 advisory
- **Dispositions**: 10 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
