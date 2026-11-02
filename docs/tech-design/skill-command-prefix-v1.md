# skill-command-prefix — `pyxis-` harness skill rename and prefix gate — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(Draft while this PR is open; approval is the introducing PR's operator review; merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-30 |
| **Relevant-when** | designing or changing the `pyxis-` harness skill rename train, the skill-prefix conformance gate, occurrence-matching for skill basenames, or how converge regenerates harness skill views after rename; implementing against this design; superseding the rename/gate model |
| **Scope** | How every harness skill gets a canonical `pyxis-<basename>` identity (live rewrite rules, per-skill PR unit, converge view regeneration) and how the post-train prefix conformance test activates on the closing PR. **Not**: engine CLI rename; rewriting frozen merged docs; a docgate skill-prefix check; inventing per-harness name variants. |
| **Tracking** | — *(the introducing PR, once filed)* |
| **Builds-on** | docs/tech-design/ecosystem-help-v1.md (live-surface derivation; rename changes help by surface alone) · docs/tech-design/code-gate-mirror.md (test rides code-gate via Go harness) · docs/tech-design/repo-layout-and-system-footprint.md (system tree + content purity) |
| **Supersedes** | — |
| **Serves** | prd:skill-command-prefix/FR-1 · prd:skill-command-prefix/FR-2 · prd:skill-command-prefix/FR-3 · prd:skill-command-prefix/FR-4 · prd:skill-command-prefix/FR-5 · prd:skill-command-prefix/UB-1 · prd:skill-command-prefix/UB-2 · prd:skill-command-prefix/UB-3 · prd:skill-command-prefix/NR-1 · prd:skill-command-prefix/NFR-1 |
| **Security/privacy impact** | none — in-repo path/content rename and a tree-shape conformance test; no new credential, network, or privilege surface |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict
the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **Prefix gate = scripts-test / payload conformance** — no docgate verb; closing PR lands the test | #ALT-1 (docgate `check-skill-prefix`) | Enforcement umbrella moves; CI/docs-only path behavior changes → §2, §3.4, `design-brainstorm:skill-command-prefix-tech-design#D-1` |
| 2 | **Per-skill PR rewrites all live identity occurrences** of that stem (exhaustive under the system tree; structural outside it) | Skill-local-only + shared refs on closing (#ALT rejected by D-3) | Closing PR grows; mid-train residuals return → §3.1, `#D-3`/`#D-5` |
| 3 | **Harness views regenerate via converge** — never hand-edited | Parallel hand-maintained vendor trees | Second identity source; M1(c) drifts → §3.2, `#D-2` |
| 4 | **Occurrence matching = structural + system-tree exhaustive** — not denylist whole-token | #ALT-4 aggressive replace | Short stems (`help`) corrupt prose → §3.3, `#D-5` |

**Sanity-check the numbers** — after the closing PR: every skills-tree directory/`name:` is `pyxis-*` (A-1); the conformance test is present and green (A-4); zero unprefixed skill identity remains on the live vendor surface (A-2).

**The author's open questions** — none — G-1 (test home) resolved in §3.4 as the help-purity pattern (scripts-test + Go wrapper).

**Skip the rest** — schema → docgate design lint; substance/reuse → `tech-design-review` report on this PR; implementation defects → code-gate / rename-train PRs.

## 1. Problem

The approved contract (`prd:skill-command-prefix`) requires every harness skill to carry one canonical `pyxis-<basename>` identity, a hard cut, a per-skill PR train, and a mechanical prefix gate that is inactive mid-train and active after the closing PR — plus a safe rewrite rule so short basenames do not corrupt English prose. Stage 3 decided the enforcement umbrella, the rewrite unit, how generated harness views stay honest, and the occurrence-matching rule. Without those bindings, implementers re-open the docgate-vs-test debate and the "skill-local vs whole-repo" rewrite debate on every skill PR.

## 2. Decision

**Decision:** Deliver the namespace cut as a **per-skill live rewrite train** plus a **closing conformance test**: each skill PR renames that skill to `pyxis-<basename>` and rewrites every allowed live occurrence of its identity (structural forms everywhere live; **all** skill-basename references under the live system tree); regenerates harness skill views via existing converge reconcile (never hand-edit generated views); hard-cuts with no aliases. The closing PR lands a **scripts-test / Go-payload conformance test** (help-purity pattern) that refuses non-`pyxis-*` skill directories/`name:` pairs, and confirms the Argus/`/pyxis-argus` glossary split. No docgate skill-prefix check. (design-brainstorm:skill-command-prefix-tech-design#D-1…#D-5)

This is safe because converge already keys generated views off canonical directory names, help already derives from live frontmatter (ecosystem-help-v1), and the gate's mid-train absence is encoded by landing the test only on the closing PR (UB-2). The basis §6.1 "skill-local / shared refs" wording is narrower than this train unit — proceed-anyway per stage-3 D-4 (optional later PRD amend).

## 3. The design

### 3.1 Per-skill rename PR (identity rewrite unit)

For each skill whose pre-rename basename is `<stem>`:

1. Rename the canonical skills-tree directory and frontmatter `name:` to `pyxis-<stem>` (FR-1; stem unchanged — basis D-10).
2. Rename the matching authoring guide filename and update skill-glue pointers (FR-1).
3. Rewrite **live** occurrences of that skill's identity per §3.3 (D-3, D-5).
4. Regenerate harness skill views via `ReconcileHarnessSurfaces` / converge so vendor-visible names match (D-2).
5. Hard-cut: no alias restoring `<stem>` (FR-2, UB-1).

**Closing PR owns:** the prefix conformance test (§3.4), glossary confirm (FR-5), and any leftover view drift backstop (D-2).

**Decision:** Each skill PR owns all allowed live occurrences of that skill's identity; shared-ref deferral to closing for that skill's own strings is refused. (design-brainstorm:skill-command-prefix-tech-design#D-3)

### 3.2 Harness views

**Decision:** Generated harness skill views are **derived** from the canonical skills tree by existing converge reconcile — never a second hand-edited identity source. Prefer regenerating on each skill PR that renames; closing PR is the backstop. (design-brainstorm:skill-command-prefix-tech-design#D-2)

### 3.3 Occurrence matching

**Decision:** Matching is **ALT-3 structural forms** on all live surfaces outside the system tree, plus **exhaustive skill-basename references under the live system tree** (paths and content). Outside the system tree, do not bare-English whole-token replace. Frozen merged documents are never rewritten for historical command strings (NG-2). (design-brainstorm:skill-command-prefix-tech-design#D-5/#D-4)

Structural forms include at least: skill directories, authoring filenames, frontmatter `name:`, slash-command citations (`/<stem>`), and path segments that locate the skill. Under the system tree, any remaining reference to that skill's basename as skill identity is rewritten in the same PR.

### 3.4 Prefix conformance gate (FR-4)

**Decision:** FR-4 is enforced by a **conformance test** in the scripts-test harness, reached by CI via a Go test wrapper in the payload package (same pattern as help purity) — **not** a docgate check. The test is absent until the closing PR (UB-2); once present it fails closed if any skills-tree directory basename or frontmatter `name:` violates `pyxis-*`, and fails if the closing precondition still sees an unprefixed skill (UB-3). (design-brainstorm:skill-command-prefix-tech-design#D-1; G-1 resolved here)

### 3.5 Boundaries and non-goals

- **Not solved here** — engine CLI verb renames (NG-1); frozen-doc historical string rewrites (NG-2 / O-2); per-harness divergent names (NG-4); docgate registration of a skill-prefix check (#ALT-1 rejected).
- **Must remain unchanged** — engine binaries/verbs and frozen merged provenance docs (NR-1).
- **Implementation must not infer** — that generated harness views may be hand-patched instead of regenerated (O-1); that bare English outside the system tree is in scope for automatic replace (O-3).

### 3.6 Invariants

- **DINV-1** — **One identity.** For every skill on the skills tree after its rename PR, directory basename = frontmatter `name:` = `pyxis-<pre-rename-stem>`; matching authoring guide filename uses the same stem.
- **DINV-2** — **Hard cut.** No alias, symlink, or alternate `name:` restores a skill's former unprefixed identity after its rename PR merges.
- **DINV-3** — **Live-only rewrite.** Automatic rewrite never targets frozen merged documents solely to replace historical unprefixed skill strings.
- **DINV-4** — **Gate activation.** The prefix conformance test does not exist in-tree before the closing PR; after that PR it refuses non-`pyxis-*` skills-tree entries.
- **DINV-5** — **Derived views.** Vendor-visible skill directories in generated harness views are produced by converge reconcile from the canonical tree, not independently authored.

### 3.7 Observability

none — rename and conformance are tree/CI artifacts with no new runtime telemetry; M1 and acceptance rows are proven by inspection/test evidence on the train and closing PR.

### 3.8 Security & privacy

none — matching the header row: in-repo content/path mutations under existing review; the conformance test reads the skills tree only; no new credential, egress, or privilege.

### 3.9 Testing strategy

- **Prefix conformance (#DINV-4 / FR-4)** — *in-process*: scripts-test suite + Go wrapper (payload package), green only after closing PR lands it; sandbox fixture adding a non-`pyxis-*` skill must fail.
- **Identity unity (#DINV-1)** — *inspection* on each skill PR + closing checklist.
- **Hard cut (#DINV-2)** — *inspection*: no alias artifacts in the skill PR diff.
- **View regeneration (#DINV-5)** — *inspection/demo*: after rename, generated harness views expose `pyxis-<stem>` (or closing backstop clears drift).
- **Help fallout** — existing help purity / routing evals continue to derive from the live surface (ecosystem-help-v1); no help catalog edits required for rename alone.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| **Docgate `check-skill-prefix`** (design-brainstorm:skill-command-prefix-tech-design#ALT-1) | Authority chose scripts-test/payload conformance (#ALT-2 / D-1); avoids growing the docgate check set for a tree-shape rule already pattern-matched by help purity. |
| **Skill-local refs only; shared refs on closing** | Authority required all live occurrences of that skill's basename per skill PR (D-3); §6.1 wording proceed-anyway (D-4). |
| **Hand-edit generated harness views** | Creates a second identity source; converge already regenerates from directory names (D-2 / O-1). |
| **Aggressive whole-token replace + denylist** (#ALT-4) | False positives on short stems; denylist maintenance; rejected for ALT-3 + system-tree exhaustive hybrid (D-5). |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Mid-train mixed names confuse operators (basis R1) | temporary dual catalog | expected under UB-2; train keeps each skill hard-cut; help/pipeline update with that skill's PR | per-skill PR review |
| R2 | Short-stem over-rewrite under the system tree | false positives inside system-tree prose | D-5 still targets skill-identity references; PR review of system-tree diffs; structural forms remain the floor outside the tree | skill PR diff review |
| R3 | Conformance test absent from docs-only workflows until code-gate runs | FR-4 not seen on a pure-docs path | closing PR is not docs-only — it lands the test under code-gate; docgate stays out by decision | closing PR includes test files |
| R4 | Basis §6.1 text lags D-3 delivery shape (P-1) | ledger/prose drift | proceed-anyway (D-4); optional later `/prd-amend` | tracked as P-1 |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | After each skill's rename PR, that skill's directory basename = frontmatter `name:` = `pyxis-<stem>`, and the matching authoring guide filename uses the same stem | Inspection | skill PR diff | prd:skill-command-prefix/FR-1 · #DINV-1 |
| A-2 | After each skill's rename PR, no alias restores the unprefixed identity; after the full train + close, no unprefixed pyxis skill remains invocable on the vendor surface | Inspection | skill PR + closing checklist | FR-2 · UB-1 · M1 · #DINV-2 |
| A-3 | Live routing surfaces named by FR-3 cite only prefixed forms for renamed skills once each skill's PR (and close for residuals) has landed | Inspection | diffs to pipeline map, help fixtures/evals, living rollup, editable wiki as touched | FR-3 |
| A-4 | After the closing PR, the prefix conformance test is present and fails a sandbox non-`pyxis-*` skill; the live skills tree is entirely `pyxis-*` | Test | harness transcript on closing PR | FR-4 · UB-3 · #DINV-4 |
| A-5 | Glossary distinguishes Argus the persona from `/pyxis-argus` at M1 | Inspection | glossary entry | FR-5 |
| A-6 | Generated harness views are regenerated via converge (not hand-edited) and expose the prefixed skill names for renamed skills | Inspection | view reconcile evidence in skill/closing PRs | #DINV-5 · M1(c) |
| A-7 | Engine CLI verbs and frozen merged docs are not rewritten solely for historical command strings | Inspection | train + closing diffs | NR-1 · #DINV-3 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| — | none — G-1 (conformance test home) resolved in §3.4 as scripts-test + Go payload wrapper (help-purity pattern) | — | — | closed | §3.4 |

## Principles

- **AP-10 — honored.** Prefix legality is a mechanical conformance test, not prompt judgment.
- **AP-23 — honored (by refusal).** No new shared docgate enumeration line; the chosen gate is not a docgate check (#ALT-1 rejected).
- **AP-25 — honored.** Canonical identity lives on the vendor-neutral skills/authoring surface; vendor views are generated pointers.
- **AP-27 — honored.** Help catalog continues as a derived aggregate over the renamed surface (ecosystem-help-v1).
- **RW-5 — honored.** One independently-revisable question: the rename+gate delivery mechanism (rewrite unit, view regeneration, occurrence matching, and gate umbrella travel together).
- **Deviation:** none beyond the content-vs-code gate split already chosen in D-1 (enforcement rides code-gate rather than docgate) — argued and accepted by the authority.
