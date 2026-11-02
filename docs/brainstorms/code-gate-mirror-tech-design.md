# code-gate mirror — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The stack-scoped, single-source code-quality gate that makes implement-now's local gate a faithful mirror of required CI — closing the local-green/remote-red drift (FR-8 · AP-11). |
| **Relevant-when** | designing or changing the local↔remote code-gate mirror, the per-repo code-gate check definition, or how `executor.gate` and the remote CI derive from one source; checking prior rulings on gate single-sourcing, hermeticity of blocking checks, or the code-gate ↔ codegate boundary. |
| **Basis** | `prd:implement-now` (FR-8, UB-5 — the mirror Musts) · adjacent: `prd:implementation-verification` WR-5 (remote-gate wiring, reserved) |
| **Authority** | Operator |
| **Date(s)** | 2026-07-21 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/code-gate-mirror.md |
| **Prior records** | design-brainstorm:implement-now-tech-design · design-brainstorm:implementation-verification-tech-design |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/prds/implement-now/effective.md | Basis — FR-8/UB-5 (the mirror Musts), risk R3 |
| docs/tech-design/implement-now.md | live design owning FR-8/DINV-4 — asserts the mirror property, delegates the mechanism |
| docs/tech-design/codegate-v1.md | live design — closest neighbor; defers this exact seam (WR-5/WR-7, `#F-2`, `#ALT-6`) |
| docs/prds/implementation-verification/effective.md | codegate's basis — WR-5 (remote-gate wiring) reserved for "when the operator opens the remote-gate phase" |
| docs/architecture-principles.md | AP-11 (one predicate, one place — governs) · AP-25 (vendor-neutral agent surface) |
| docs/observability/incidents.md | I4 (a post-merge contract/gate gap costs a supersession — adjacent); the drift episode is the I5 candidate |
| .pyxis/config.yaml | the `executor.gate` block + its documented, mechanism-forced deviation |
| internal/verify/checks/rung0_latency_test.go | the non-hermetic check that stranded #600 |
| docs/tech-design/index.md | routing — live designs Serving implement-now / codegate |

**Overlaps surfaced:**
- **codegate-v1 (live)** — designs "the identical pinned artifact runs in the local loop today and as the remote gate later (AP-11 — one gate implementation, both surfaces)", but its Scope *excludes* remote-gate wiring (WR-5); `#F-2` forbids inferring codegate replaces the host's CI-mirror local gate ("it coexists; their DINV-4 stands"); `#ALT-6` (ladder subsumes required CI now) **rejected**. → It does **not** decide this seam; it names a future design (this one). Not `covered`.
- **implement-now (live)** — Serves FR-8; DINV-4 asserts "the local gate runs the same deterministic checks required by CI" but delegates the mechanism. This design supplies that mechanism (co-serves FR-8/UB-5 — see DQ-1).
- **repo-layout-and-system-footprint (live)** — owns the `.pyxis/config.yaml` schema and the umbrella-verb / desk-parity convention (`docgate-resolve.sh`, shared by pre-push + CI). The config-grammar change lands against it.
- **I4** — adjacent lesson: a contract/gate gap surfacing only post-merge forced a supersession; argues for settling the seam and the Serves set pre-merge here.

## Design questions

- **DQ-1**: Which basis owns the code-gate mirror, and what does the design `Serve` — co-serve `prd:implement-now`/FR-8+UB-5, promote `prd:implementation-verification`/WR-5, or both?
- **DQ-2**: Where does the canonical per-repo code-gate check set live, and how do the local gate **and** required CI both derive from it as one source? *(seam)*
- **DQ-3**: What is the code-gate runner's relationship to the existing `codegate` binary (reuse the `internal/verify` core vs a thin tool-wrapper), and what is it named? *(seam / module boundary)*
- **DQ-4**: How is the target repo's stack — and thus its check set — determined: explicit per-repo config vs ecosystem detection?
- **DQ-5**: The **hermeticity invariant** — stated how, enforced how (convention / lint / conformance-test), and what is `TestRung0Latency`'s disposition? *(seam — a new conformance rule)*
- **DQ-6**: Surface scope for v1 — is pre-push / desk parity included now, or deferred?

## Panel log

*One line per `DQ-n`, appended as each is resolved (DINV-7).*

- **DQ-1**: extends (requirement-ownership / Serves) — **skipped** — no code seam opened; the question is only which committed Must to cite.
- **DQ-2**: opens (the `code-gate` verb + per-repo check-list config + the both-surfaces invocation contract) — **fired** — a new command, a new config surface, a new execution contract; 3 shapes scored (ALT-1..3).
- **DQ-3**: extends (the S-1 verb) — **skipped** — a reuse-vs-reinvent + naming choice inside the already-opened command; operator pre-ruled reuse.
- **DQ-4**: extends (the S-2 config seam) — **skipped** — how the config's list is populated, not a new seam.
- **DQ-5**: opens (a gate-admission rule — correctness-only vs measurement) — **fired** — 3 enforcement shapes scored (ALT-4..6); operator sharpened the chosen shape to a category rule (measurements are not tests).
- **DQ-6**: extends (the S-1 verb invocation) — **skipped** — pre-push is another caller of the already-opened command, not a new seam.

## Decisions

*Appended as the authority rules each `DQ-n`; `D-n` only from a live answer this session (DINV-1).*

### D-1 — What requirement the design serves

- **Question**: Which basis does the design `Serve` — co-serve `prd:implement-now`/FR-8+UB-5, promote `prd:implementation-verification`/WR-5, or both?
- **Answer**: Co-serve `prd:implement-now`/**FR-8 + UB-5** (the committed "local gate mirrors required CI" Musts). Do **not** serve codegate's WR-5 — that is codegate-the-binary's remote delivery, a distinct and coexisting future.
- **Reasoning**: FR-8/UB-5 are committed Musts → valid anchors, no `/prd-amend` friction, a clean path to ready-for-design (I4's lesson). This design is the *mechanism* realizing FR-8; the partition with implement-now's own design is drive-side (it consumes a mirror gate) vs mechanism (this defines it). Per the operator's not-supersede ruling the blocking set is config-driven, not codegate's ladder, so WR-5 stays reserved for the codegate-absorbs-the-verb future (WR-7).
- **Anchor**: 2026-07-21 · this session (DQ-1: "go with your recommendation")
- **Status**: live

### D-2 — The single-source mechanism

- **Question**: Where does the canonical per-repo code-gate check set live, and how do the local gate and required CI both derive from it as one source?
- **Answer**: **(ALT-1)** The per-repo `.pyxis/config.yaml` code-gate block is the one canonical check list; a single `code-gate` verb reads it and runs the checks; the local executor gate, the pre-push hook, and required CI all invoke that same verb — same command, same list, both surfaces.
- **Reasoning**: one command + one list is structurally drift-proof (both surfaces run the identical code path); it mirrors the proven `docgate-resolve.sh` single-source already used for the docs half; a Go verb can express the *code* checks argv-only config cannot (gofmt, `-race`) — the doc-gate (docgate) stays a separate, already-single-sourced gate that composes alongside (O-1 independence); the config home keeps the list per-repo/per-stack (D-1). ALT-2 keeps a policeable copy; ALT-3 cannot vary per repo.
- **Anchor**: 2026-07-21 · this session (DQ-2: "go with 1")
- **Status**: live

### D-3 — Runner reuse + name

- **Question**: Does the `code-gate` verb reuse codegate's `internal/verify` runner, and what is it named?
- **Answer**: **Reuse** the `internal/verify` runner as a library — the verb is a thin front-end (read the config checklist → run each check via the shared runner → report). Name it **`code-gate`**. The existing `codegate` binary stays untouched.
- **Reasoning**: no point reinventing a tested check-runner (timeouts, output capture, sandboxing); `code-gate` matches the operator's own term. Accepted caveat — a near-homonym with `codegate`: mental model is `code-gate` = the required-checks mirror (now, blocking), `codegate` = the deep verification ladder (future, advisory), converging at WR-7.
- **Anchor**: 2026-07-21 · this session (DQ-3: "go with your recommendation")
- **Status**: live

### D-4 — Where the checklist comes from

- **Question**: Is the check set determined by explicit per-repo config, or by stack detection / built-in defaults?
- **Answer**: **(a) Explicit** — the gate runs only what the repo's config lists; no gate-time detection, no baked-in defaults. Repo-setup (`pyxis init`) may *scaffold* a visible starter list into the config, but that is editable content the author owns, never a fallback the gate reaches for on its own.
- **Reasoning**: honors the ruling that each repo owns its checks (source of truth in config, not universal core); transparent; sidesteps detection ambiguity on polyglot repos. Init-time scaffolding keeps ergonomics without hiding the source.
- **Anchor**: 2026-07-21 · this session (DQ-4: "go with a")
- **Status**: live

### D-5 — Keeping measurements out of the blocking gate

- **Question**: How is the hermeticity invariant enforced, and what is the disposition of the wall-clock latency test?
- **Answer**: **(ALT-5)** The blocking gate holds only **correctness checks** — each a pure function of the code/declared input (same input → same verdict, any machine). Performance/latency is a **measurement**: recorded via benchmark/observability, never a blocking pass/fail. Operator's sharpening: "performance testing as a unit test" is a **category error** and the root of the #600 strand — always ask what property a check establishes. Disposition: the wall-clock budget assertion in `TestRung0Latency` is **removed** (not relocated as an assertion); a deterministic smoke on a fixed fixture may stay; a real rung-0 latency NFR is verified by a separate statistical perf process — a follow-up owned by the check's team (#380), not this design.
- **Reasoning**: convention already existed (the repo's hermetic-tests rule, F-11) yet the offender shipped, so a structural category rule is needed; measurements have a proper home (observability), so excluding them from the gate loses nothing and removes the drift class at its root.
- **Anchor**: 2026-07-21 · this session (DQ-5: "go with (2)" + the category-error refinement)
- **Status**: live

### D-6 — Surfaces in v1

- **Question**: Which surfaces run the code-gate in v1 — executor + CI only, or also pre-push?
- **Answer**: **All three** — the executor's local gate (implement-now drive-to-green), required remote CI, and the pre-push hook (human desk parity). One verb, three callers.
- **Reasoning**: FR-8's executor mirror is the core; CI is the "remote" half (nothing to mirror without it); pre-push is nearly free (one more caller, exactly as `docgate` already does) and completes AP-11 desk parity so a human pushing by hand isn't surprised by CI.
- **Anchor**: 2026-07-21 · this session (DQ-6: "all three")
- **Status**: live

### D-7 — Execution economy: required vs advisory checks, and verdict reuse (review-born)

- **Question**: (from FND-3, review) running the full mirror at drive-to-green + pre-push + remote is 3×; how to cut the overhead without reintroducing local/remote drift?
- **Answer**: (a) checks are typed **required** (block; the mirror — DINV-1 — scopes to this set) or **advisory** (never block; may run remote-only; the nice-to-have bucket, outside the mirror), keeping the required set lean; (b) **verdict reuse keyed by content fingerprint** — safe because required checks are hermetic (a verdict is valid for a tree hash), so the local gate runs once per distinct tree, shared across drive-to-green and pre-push (an executor-gated tree makes pre-push a no-op; a human runs pre-push once); (c) **remote re-runs** the required set as the authoritative backstop (trust boundary; off the critical path).
- **Reasoning**: faithful to FR-8's exact wording ("the same deterministic checks **required by** CI" already scopes to required); a lighter *required* set run locally would recreate the very drift this design kills, so overhead comes from caching + advisory demotion, never a weaker local required gate.
- **Anchor**: 2026-07-21 · review (FND-3 — docs/reviews/code-gate-mirror.md)
- **Status**: live

## Alternatives

*Appended when a panel fires; ≥ 2 scored shapes per fired `DQ-n`.*

### ALT-1 — Config-sourced list, one shared verb

- **Shape**: the per-repo `.pyxis/config.yaml` code-gate block is the canonical check list; one `code-gate` verb reads it and runs each check; `executor.gate`, pre-push, and required CI all invoke that one verb. One representation, one execution path.
- **Trade-offs**: wins on can't-drift (single shared code path) + expressiveness (a Go verb runs gofmt/`-race` that argv-only config can't) + per-repo/per-stack (config is human-editable). Costs a new verb + config schema; the CI step becomes "run the verb" (less explicit than visible YAML — but that opacity *is* the single-source guarantee).
- **Principles**: AP-11 honored (one predicate, one place — one verb, one list, both surfaces); AP-25 honored (a CLI verb any harness/CI invokes); AP-5/AP-6 (config parses closed).
- **Incidents**: matches the `docgate-resolve.sh` precedent (docs half already single-sourced this way); I3 (the verb must fail-closed on a malformed/ambiguous config, never silently skip).
- **Verdict**: chosen — proven pattern, structurally drift-proof, honors the per-repo/stack ruling (D-2).
- **From**: DQ-2 · panel

### ALT-2 — Config-sourced list, generated CI workflow

- **Shape**: config holds the list; a generator writes the CI file's code steps from it; a "check-regenerated" CI gate fails on hand-edits. One source, a derived second representation; the local surface keeps its own runner.
- **Trade-offs**: wins on remote readability (each check a visible YAML step). Costs a generated copy to police, two representations honest only by a checker, and it unifies only the *list*, not execution (local + remote still run it two ways).
- **Principles**: AP-11 strained ("one place" becomes "one source, two copies," honest only by a checker); AP-6 (closed parsing on the generator input).
- **Incidents**: I1 (the WR-stub loophole — a derived artifact that passes its checker yet drifts from intent; generated-honest-by-check is fragile).
- **Verdict**: rejected — adds a policeable copy and unifies only the list, for a readability gain the verb's `--list`/logging also gives.
- **From**: DQ-2 · panel

### ALT-3 — Checks baked into a shipped Go program

- **Shape**: the check list is encoded in a pyxis-shipped binary (codegate-style); both surfaces run the binary; the stack is fixed or detected in code.
- **Trade-offs**: wins on typed, testable checks in real Go. Costs: the list can't vary per repo/stack without rebuilding pyxis. (If the binary instead *reads* per-repo config, it collapses into ALT-1.)
- **Principles**: AP-25 strained (a baked-in universal check set is not the per-repo adapter surface the code-gate must be); AP-11 honored only in isolation.
- **Incidents**: none checked (registry 2026-07-21) — no entry bears on baked-vs-config.
- **Verdict**: rejected — conflicts with the per-repo/per-stack requirement (D-1 basis; operator ruling). Retained to bound the space.
- **From**: DQ-2 · panel

### ALT-4 — Convention only

- **Shape**: state the hermeticity rule in docs; reviewers uphold it; fix today's offender. No mechanism.
- **Trade-offs**: cheapest; zero structural guarantee — the next measurement-as-test slips in.
- **Principles**: AP-11 relies on discipline to hold "one predicate" (weak).
- **Incidents**: I5 candidate — the #600 drift happened *under* the repo's already-written hermetic-tests rule (F-11), so convention alone is demonstrably insufficient.
- **Verdict**: rejected — the offender violated an existing convention; convention didn't hold.
- **From**: DQ-5 · panel

### ALT-5 — Separate category: gate = correctness only; measurements are benchmarks/observability

- **Shape**: the blocking gate admits only correctness checks (same input → same verdict, any machine). Measurements (wall-clock, memory, cost) are a distinct category — recorded via benchmarks/observability, never asserted as pass/fail. A real perf NFR is a statistical perf-regression process outside the unit suite.
- **Trade-offs**: structural guarantee (a measurement-assertion has no home in the gate by definition); cheap (a policy + reclassify the offender); fits the repo's advisory-vs-blocking split. Cost: someone must own the measurement surface (out of this design's scope).
- **Principles**: AP-11 honored ("same input → same verdict" is definitional to a mirror); AP-5/AP-6 (measurements belong in observability); FR-8 DoD ("same input") satisfied by construction.
- **Incidents**: I5 — the #600 strand *is* a measurement asserted in the correctness gate; this removes the category error at the root.
- **Verdict**: chosen — sharpened by the operator: "performance testing as a unit test" is a category error; measurements record, correctness asserts.
- **From**: DQ-5 · panel

### ALT-6 — Conformance test (double-run)

- **Shape**: a harness runs each blocking check twice and fails on divergent verdicts.
- **Trade-offs**: mechanical non-determinism detection; awkward to wrap every check; misses a check that's non-hermetic yet stable on one machine (wall-clock under low load passes twice locally).
- **Principles**: AP-11 supported mechanically, but coverage is incomplete.
- **Incidents**: none checked (registry 2026-07-21).
- **Verdict**: rejected as the primary mechanism (imperfect, heavy); may ride later as a non-blocking guard (a design `Q-n`).
- **From**: DQ-5 · panel

## Seam map

*Appended as directions settle; feeds the design's Scope and Serves.*

- **S-1**: the `code-gate` verb (CLI) — **opens** — behind it: read the per-repo check list, run each check, emit one exit/verdict — feeds: the design's Scope (the verb + its contract) · Serves prd:implement-now/FR-8.
- **S-2**: the per-repo code-gate config block (`.pyxis/config.yaml`) — **replaces** the argv-only `executor.gate` list — behind it: the canonical, stack-specific check set (typed, closed-parse) — feeds: Scope (config schema) · touches `repo-layout-and-system-footprint` (owns the config schema).
- **S-3**: the required-CI invocation — **replaces** the hand-listed `docs-gate.yml` code steps with a call to the verb — behind it: CI runs the same verb over the same config, coexisting with the untouched `docgate` doc-gate step — feeds: Scope (the remote surface).
- **S-4**: the pre-push hook (desk) — **extends** — behind it: the human's `git push` runs the same verb, adding code-gate alongside the existing `docgate` pre-push — feeds: Scope (the desk surface).
- **S-5**: the verdict record (content-fingerprint → verdict) — **new** (D-7) — behind it: a hermetic required check's verdict cached by tree hash, shared across local surfaces so the required gate runs once per change (reuses codegate's verdict-as-state where available) — feeds: Scope (execution economy).

## Facts

- **F-1**: FR-8 (Must, high): "The executor's local gate shall run the same deterministic checks required by CI"; DoD — "a change failing a required CI check **also fails the local gate on the same input**". The "on the same input" clause entails determinism/hermeticity. — source: docs/prds/implement-now/effective.md:230.
- **F-2**: UB-5 (Must, high): "a change passes the local gate but would fail required CI" → "fail it at the local gate too (the gate mirrors required CI)". — source: docs/prds/implement-now/effective.md:263.
- **F-3**: AP-11 — "One predicate, one place … The local quality gate mirrors CI exactly, or it is not a mirror"; graduated from harbormaster P1 (a real strand-red incident). — source: docs/architecture-principles.md:107.
- **F-4**: The local gate today = `executor.gate {build: go build ./...; vet: go vet ./...; test: go test ./...}`, whitespace-split into argv with **no shell**; the config comment explicitly declares "gofmt and the -race / docgate checks … are not mirrored here and stay CI-authoritative" — a **documented, mechanism-forced deviation**. — source: .pyxis/config.yaml:79-95.
- **F-5**: Required remote CI (`docs-gate`) runs `gofmt -l` · `go vet ./...` · `go test -race ./...` · the `docgate` umbrella (via `docgate-resolve.sh`, the **same shim pre-push runs**). The docs half is already single-sourced; the code half is two independent hand-authored lists. — source: .github/workflows/docs-gate.yml.
- **F-6**: The empirical strand: implement-now task T10 (#600) drove green through the local gate but `docs-gate` stayed red at `go test -race ./...` → `internal/verify/checks` → `TestRung0Latency_FixtureAndDogfoodRepo/dogfood`: rung-0 neutral checks 20.2s over the NFR-1 10s budget (secret-scan 13.9s). — source: CI run 29856451948.
- **F-7**: `TestRung0Latency` is **non-hermetic**: `dogfoodDiff` builds its input from `git log -n 10 HEAD` of the actual checkout (size varies per branch), and `measureRung0` asserts a **wall-clock** budget — so its verdict varies by machine and by `-race` for identical code, and cannot be mirrored. — source: internal/verify/checks/rung0_latency_test.go:38-75,106-123.
- **F-8**: codegate-v1 (Approved 2026-07-16) — "the identical pinned artifact runs in the local loop today and as the remote gate later (AP-11)"; Scope **excludes** remote-gate wiring (WR-5); `#F-2` forbids inferring codegate replaces the host CI-mirror gate ("it coexists; DINV-4 stands"); `#ALT-6` (subsume required CI now) rejected. codegate has a stack-aware `internal/verify` + `ecosystem` core. — source: docs/tech-design/codegate-v1.md:10,26,44,145.
- **F-9**: `prd:implementation-verification`/WR-5 reserves "Remote CI delivery: workflow wiring, required-check registration, enforcement rollout … when the operator opens the remote-gate phase." — source: docs/prds/implementation-verification/effective.md (WR-5).
- **F-10**: FR-8 is **already** served by the implement-now design (index.md:33) — so this design co-serves a *mechanism*, it does not fill an inverse-coverage hole. — source: docs/tech-design/index.md:33.
- **F-11**: The repo already mandates hermetic unit tests ("no clock/randomness without injection") — yet `TestRung0Latency` shipped anyway, and its own comment concedes the valid p95 latency claim is "a Phase-B measure across real dogfood runs" while it asserts a single-run wall-clock budget: a self-acknowledged category error under an existing convention. — source: AGENTS.md (Go testing rules); internal/verify/checks/rung0_latency_test.go:16-25.
- **F-12**: FR-8's wording is "the same deterministic checks **required by** CI" — the mirror is scoped to the *required* set by the requirement itself; advisory/informational checks were never "required CI." — source: docs/prds/implement-now/effective.md:230.

## Fog

- none

## Out of scope

- **O-1**: Realizing codegate's WR-5 (codegate-the-binary running as the remote gate) — out. This design builds a *distinct, coexisting* config-driven code-gate; codegate-v1 stays live and unsuperseded (its `#F-2`). The eventual convergence — codegate's ladder absorbing the code-gate verb — is WR-7's trigger, not this design. (from D-1)
- **O-2**: Building any performance/measurement lane, and the actual fix to `TestRung0Latency`'s wall-clock assertion — out. This design rules the blocking gate is correctness-only and that the assertion cannot sit in it; reclassifying the measurement (benchmark/observability) and any statistical perf-NFR process is a follow-up owned by the check's team (#380). (from D-5)

## Candidate glossary terms

- **T-1**: code-gate — the stack-scoped code-quality gate predicate (the per-repo check set that both the local loop and required CI run). · avoid: conflating with **codegate** (the binary + its check-ladder, one possible implementation).
- **T-2**: doc-gate — the universal pyxis document-system gate (`docgate`), identical across every repo the factory drives. · avoid: "the gate" (ambiguous between the two predicates).
- **T-3**: gate mirror — the property that a local gate runs the identical predicate as required CI, verdict-for-verdict, on the same input (AP-11). · avoid: "parity" (used loosely for near-enough).
- **T-4**: hermetic check — a gate check whose verdict is a pure function of the declared input (no wall-clock, ambient git history, network, or machine-speed dependence). · avoid: "deterministic" alone (a wall-clock assertion is deterministic given the clock, yet not hermetic).
- **T-5**: measurement (vs check) — an observation of a run's cost/speed/size (wall-clock, memory, tokens, dollars), recorded and trended via benchmark or observability; never a blocking pass/fail. · avoid: "performance test" (invites asserting a measurement in the correctness suite — the #600 category error).

## Parked product gaps

- none

## Gate

*Exactly one entry, at session end (DINV-4).*

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-21 |
| **Authority** | Operator |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog |
| **Amend route** | — |
