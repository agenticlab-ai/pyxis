# Coverage on read — the requirements-coverage join is computed, never committed

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(Draft while this PR is open; merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-09 |
| **Relevant-when** | asking which requirements have no design; running `docgate coverage`; changing `gen-index`/`verify-index` output; consuming `index.json` programmatically; reasoning about which plane a generated artifact belongs to |
| **Scope** | Where the FR-26 requirements-coverage view is materialized: it leaves the committed tech-design index and becomes an on-read `coverage` verb. **Not**: the core index graph (docgate-v2 §3.6 — unchanged), `Serves` semantics (serves-hardening), corpus mutability (fence-completion), the universe definition or the served/unserved computation (carried forward from the superseded design). |
| **Tracking** | issue #70 |
| **Builds-on** | docs/tech-design/docgate-v2.md · docs/tech-design/serves-hardening.md |
| **Supersedes** | docs/tech-design/index-coverage.md |
| **Serves** | prd:docgate/FR-28 |
| **Security/privacy impact** | none — deterministic computation over tracked files already read by the index; no new surface |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The coverage face (index-coverage, FR-26) is a **join** of two planes: the requirement *universe* (corpus IDs + every live ledger unit — PRD-plane data) against live designs' `serves[]` (design-plane data). The join is committed into `docs/tech-design/index.*` and byte-verified.

Committing a cross-plane join under one plane couples the two:

1. **Ownership surprise** — adding a PRD, a `docs/prds/` concern with no design and no implementation yet, produces a diff under `docs/tech-design/`. Nothing downstream of the requirement exists, yet a design-plane artifact moves. (It confused the operator on first contact; issue #70.)
2. **Conflict magnet** — every PRD PR *and* every design PR contends on one generated pair, `index.md`/`index.json` — the shared-append surface AP-23 says to remove structurally. Observed live: PR #67 (a new PRD ledger) regenerated the coverage bytes and conflicted the concurrently-open PR #68 on exactly those two files.
3. **Conceptual mismatch** — the demand side of the join is PRD-plane data with no home of its own; today it exists materialized only inside a design-plane artifact.

The superseded design foresaw the churn — its R3 recorded "every ledger amendment now touches index bytes" and *accepted* it as "the point of the diff." The operator has since reprioritized (issue #70): the plane coupling costs more than the maximal-currency it buys. This design records that reversal.

## 2. Decision

**Decision:** the requirements-coverage view is **computed on read, committed nowhere.** A new `docgate coverage` verb joins the universe against live designs' `serves` from committed inputs and prints it; `gen-index`/`index.*` carry only design-plane data (liveness, `serves_inverse`). Adding or amending a ledger therefore never touches `docs/tech-design/`; `index.*` regenerates only when *designs* change.

The load-bearing insight: a cross-plane join committed *anywhere* couples the planes — under `docs/tech-design/` (today) PRD changes touch the design plane; under `docs/prds/` (issue #70 option B) design changes touch the PRD plane. The only full decouple is to not commit the derived join at all. The **inputs** stay committed and per-plane — each ledger's `effective.source.json` already carries its unit universe, the flat corpus carries its IDs, designs carry their `serves` — so the view is reproducible on demand from tracked files by anyone. **This needs zero new committed PRD-side files** (issue #70 option A, at its cleanest): the PRD data is already materialized where it belongs.

This **supersedes** index-coverage wholly: that design's binding decision — "`gen-index` emits the coverage face into `index.*`, byte-verified" — is now false, and two live designs may not disagree (tech-design-conventions §2). What remains valid (the universe definition, the live-only join, the family grouping) is carried forward here unchanged.

**On reversing index-coverage §4's two rejections, explicitly:**

- *"A separate `coverage` verb and artifact"* was rejected on AP-11 ("widen the fence, don't grow a twin"). But `coverage` adds **no artifact** and **no verify machinery** — it is a read-only projection with nothing to keep in sync. AP-11 argues against a committed twin index; a stdout view is not that. And the alternative AP-11 pointed to — folding the join into the committed index — is precisely the AP-23 coupling issue #70 rules out.
- *"CI-only report (not committed) — consumers read the repo, not CI"* was rejected for the **core index**, correctly: programmatic consumers key on `index.json`. But coverage's consumer is a **human planner** asking "what's unserved?", answered by a deterministic verb on demand. Its **inputs remain committed** — "consumers read the repo" still holds for the data; only the derived join is on-read.

## 3. The design

### 3.1 The `coverage` verb

`docgate coverage` (merge-base-free) reads: corpus IDs from the shared serves-hardening extractor (one extractor, two consumers — AP-11); live ledger units from each `effective.source.json`; live designs' `serves[]` from the design headers (the same `serves_inverse` the index computes). It joins them — a requirement is *served* iff ≥1 **live** design's `serves[]` names it — and prints the served/unserved partition per family: a human table by default (counts first, unserved IDs verbatim), `--json` for the machine face. It writes nothing.

### 3.2 What leaves the committed index

`gen-index` drops the coverage computation and emission; `index.json` drops its `coverage` top-level key; `index.md` drops its `## Coverage` section. `verify-index` therefore no longer covers coverage — it byte-verifies only the design graph, which changes only when designs change. Because a top-level committed key is removed, `index.json`'s `schema_version` bumps `1 → 2` (AP-6: a breaking data change is versioned, not silent); the one-time regeneration that drops the key ships with the implementation.

**A loader fix rides along.** Superseding `index-coverage` surfaced that `LoadDesigns` skips every file whose name starts with `index` (intended for `index.md`/`index.json`), so `index-coverage.md` has been silently absent from *both* the lint set and the index graph since it merged — a merged design no gate ever checked. The implementation narrows the exclusion to `index.md`/`index.json` exactly; only then is `index-coverage.md` loaded and this design's `Supersedes` edge able to register. A one-character fix with an outsized correctness effect, and a caution for any future `index*`-named design.

### 3.3 CI visibility

The `docs-gate` workflow gains a non-gating step that runs `docgate coverage` and writes its output to `$GITHUB_STEP_SUMMARY`, so the planning view stays one click away on every PR without being committed bytes. It never fails the build (§3.4).

### 3.4 Boundaries and non-goals

- **Not a gate.** `coverage` reports; it never fails CI on an unserved requirement — *unserved ≠ work-now* (some corpus areas are engine-era by intent), carried forward from index-coverage §3.2. A coverage-as-gate is a separate future FR if ever wanted.
- **Universe and join unchanged.** What counts as a requirement (corpus IDs + live FR/UB/NFR/NR units), and the live-only serve rule, are inherited verbatim; this design moves *where the result lives*, not *what it is*.
- **The core index graph is untouched** — liveness, lineage, `serves_inverse`, staleness flags remain committed and byte-verified (docgate-v2 §3.6).

### 3.5 Invariants

- **DINV-1** — adding or amending a ledger leaves `docs/tech-design/index.*` byte-identical. (The issue #70 acceptance: PRD-plane changes never touch design-plane artifacts.)
- **DINV-2** — `coverage` is derived only from committed inputs (corpus, source maps, design headers): same inputs ⇒ same output. On-read replaces byte-verification — there is nothing stored to drift, and anyone can recompute.
- **DINV-3** — only **live** designs serve and only **live** units are targets: the view can never launder a supersession. (Carried forward from index-coverage DINV-3.)

### 3.6 Observability

The `coverage` verb's stdout and the CI job summary (§3.3) *are* the observability surface — the class-health view index-coverage rendered into a committed section, now produced on demand. `gen-index`/`verify-index` defects ride the existing stream unchanged.

### 3.7 Security & privacy

none — matches the header row: deterministic computation over tracked files already read by the index; nothing new crosses any boundary.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Keep committing coverage in `index.*`, accept the coupling (index-coverage's choice; issue #70 option C) | the operator ruling: the cross-plane coupling is an ownership surprise and a live conflict magnet (AP-23), costing more than the currency it buys |
| Commit the join under `docs/prds/coverage.*` (issue #70 option B) | mirrors the coupling — a design changing `Serves` would then touch `docs/prds/`; a cross-plane join has no honest committed home |
| A new per-ledger anchor manifest committed in each ledger dir (issue #70 option A, literal) | unnecessary — `effective.source.json` already carries the live-unit universe; adding a file materializes data that is already materialized |
| A committed `coverage.json` the index consumes by reference | a third projection with its own currency problem; the whole point is to stop committing the derived join |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Coverage is no longer browsable in the repo tree | a planner must run a verb instead of opening a file | the verb is deterministic and instant; the CI job summary keeps it one click away on every PR; inputs stay committed and greppable | fixture: `docgate coverage` output; CI summary render |
| R2 | Removing `index.json`'s `coverage` key breaks a programmatic consumer keying on it | a downstream reader errors | `schema_version` bumps `1 → 2` (AP-6); the key is days old (FR-26) with no known external consumer; the successor face is the `coverage --json` shape | goldens diff; schema_version assertion |
| R3 | On-read loses byte-verification of the coverage result | a drifted view could go unnoticed | nothing is stored to drift — the view is recomputed from committed inputs each read (DINV-2); the inputs *are* byte-verified where they live | determinism fixture: two reads, identical bytes |
| R4 | Transient `stale-serves` flag on index-coverage between the amendment and this design merging | a cosmetic index flag while the stack lands | the stack merges amendment → design in order; the flag is non-fatal and clears when this design supersedes index-coverage | index render across the two-PR train |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | `docgate coverage` over a fixture corpus + ledger + design set yields the expected served/unserved partition in both faces (table + `--json`) | Test | goldens | prd:docgate/FR-28 |
| A-2 | `gen-index` emits no `coverage` key or `## Coverage` section; `verify-index` is green over the coverage-free index; `index.json.schema_version` is `2` | Test | goldens | §3.2 · DINV-1 |
| A-3 | adding a ledger amendment to a fixture leaves `index.*` byte-identical | Test | before/after fixture diff | DINV-1 · prd:docgate/FR-28 |
| A-4 | superseding a design returns its solely-served anchors to unserved in `coverage` output | Test | supersession fixture | DINV-3 |
| A-5 | the `docs-gate` workflow publishes `docgate coverage` to the job summary and never fails on coverage | Inspection | workflow diff + a run's summary | §3.3 |
| A-6 | the `LoadDesigns` exclusion is narrowed to `index.md`/`index.json`, so `index-coverage.md` is loaded and appears in the index as superseded by `coverage-on-read` | Test | lint + index render over the corpus | §3.2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Should `coverage` ever gain a `--check` gate mode (fail when a *Must* requirement is unserved past some horizon)? | operator | nothing | open | deferred — *unserved ≠ work-now* stands; a coverage gate is a separate FR decided against real planning need |
| Q2 | Does `index.json`'s `schema_version 2` warrant a one-line migration note for consumers, or is the version bump enough? | implementation | nothing | open | version bump assumed sufficient (no known consumer); revisit if one appears |

## Principles

- **AP-23 — honored (the reason for this design).** Removing the committed cross-plane join eliminates the shared-append surface where parallel PRD and design authors collided; the fix is structural, not discipline.
- **AP-6 / INV-32 — honored.** The `index.json` contract evolves by an explicit `schema_version` bump on the key removal; nothing changes silently.
- **AP-11 / INV-30 — honored.** Still one binary, one predicate; `coverage` is a read-only projection with no committed twin and no verify machinery to keep in sync — the fence is not doubled.
- **AP-10 / AP-13 — honored.** The view stays generated and deterministic; on-read recomputation from committed inputs replaces the committed byte-verify without reintroducing hand-authorship.
- **AP-24 — honored.** Coverage informs prioritization and governs nothing — reporting only, as before.
- **AP-19 — honored.** Steady-state cost is zero: the verb runs on demand; no artifact is maintained.
- **Deviation:** this design **reverses** index-coverage's *application* of AP-11 (which read "widen the fence" as "fold the join into the committed index"). It does not deviate from AP-11 — the coverage verb remains one binary — but it re-decides that a committed cross-plane join was the wrong way to honor it, per the AP-23 coupling cost the superseded design's R3 had accepted.
