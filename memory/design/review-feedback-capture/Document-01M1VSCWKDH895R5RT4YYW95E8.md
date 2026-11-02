# Review-feedback capture — technical design

| Field | Value |
|---|---|
| **Status** | Draft — design review pending *(→ Approved before merge; merge is the freezing event; "superseded" is computed by the index, never written)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-13 |
| **Relevant-when** | implementing or changing the review-feedback ledger / its capture writer, `CaptureSource` port, adapters, fold, or consumer wiring; deciding where a human review comment + its disposition is stored or how it cites; reconciling with `review-report/v1`, the Argus ledgers, or the amendment `Learned-from` field |
| **Scope** | The review-feedback ledger's storage class + fold, the `CaptureSource` port + its two adapters, the writer's disposition-at-resolution mechanism, the `review-feedback:` citation grammar + its docgate resolution check, the ledger-native measure definitions, and the privacy posture. **Not**: the self-optimization engine / scorecards / any UI (NG-1..3), any change to `review-report/v1` or the Argus ledgers (NG-5), the amendment model (D-4/F-6). |
| **Tracking** | filed with the introducing PR |
| **Builds-on** | prd-amendment-model · brainstorm-record-model · distributed-check-registration |
| **Supersedes** | — |
| **Serves** | prd:review-feedback-capture/FR-1 · prd:review-feedback-capture/FR-2 · prd:review-feedback-capture/FR-3 · prd:review-feedback-capture/FR-4 · prd:review-feedback-capture/FR-5 · prd:review-feedback-capture/FR-6 · prd:review-feedback-capture/FR-7 · prd:review-feedback-capture/FR-8 · prd:review-feedback-capture/FR-9 |
| **Security/privacy impact** | low — a new repo-internal markdown data class holding verbatim review comments (author handles already present in the collaboration surface); no new egress or network surface; redaction backstopped by the existing commit-path secret gate (§3.9) |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The merged `review-feedback-capture` PRD binds *what* — capture each human review comment (doc-review + code host) as a closable, computable, federated ledger that serves one consumer — but leaves the load-bearing *how* to design: where an entry lives and in what schema (P-1's sibling, the record's DQ-1), how its disposition + outcome is determined (P-1, DQ-2), how the two sources reach the writer medium-neutrally (DQ-3), how an entry cites and how `Learned-from: review-thread` resolves without touching the merged amendment model (P-2, DQ-4), how FR-5's measures stay "computable from the ledger alone" (Q5, DQ-5), and the privacy posture for verbatim comments (P-3, DQ-6). Without these fixed, implementation improvises a storage format, a capture path per medium, and an unenforced provenance edge — the exact drift the feature exists to end. The `/tech-design-brainstorm` session resolved all six (`design-brainstorm:review-feedback-capture-tech-design` D-1..D-6); this design freezes them.

## 2. Decision

**Decision:** The review-feedback ledger is a **per-subject directory of append-only markdown entry files plus a docgate-folded machine projection** (`design-brainstorm:review-feedback-capture-tech-design#D-1`) — the PRD-ledger pattern, giving a human-readable/diff-reviewable face and a native machine face for FR-5. Entries are written **only by a coded writer**, fed through **one consumer-side `CaptureSource` port** with two adapters — doc-review and code-host — so the writer core imports no adapter (`#D-3`, AP-1/AP-3/RW-3). The writer **tags each entry's typed disposition + stable outcome-ref at the observed resolving event** — never by inference (`#D-2`, AP-18/I3). Entries cite as `review-feedback:<subject-stem>#<entry-anchor>`, carry a **stable** outcome-ref (merged sha / PR / amendment id, never a working-branch sha) and **one-directional** cross-links to reviewer M2 / Argus `E-n` (NG-5); `Learned-from: review-thread` resolves to a `review-feedback:` anchor through the amendment model's **existing free ref slot** (no model change — `#D-4`/F-6), **enforced** by a registry-born docgate resolution check (`#D-4`, AP-10). FR-5's measures are **ledger-native** — disposition counts + applied/decline ratio over disposed entries + an `edit_distance` frozen at write (`#D-5`, AP-21). Captured comments are **verbatim, repo-internal, secret-gate-backstopped** (`#D-6`).

The reasoning: every ruling reuses a proven pyxis primitive rather than inventing one — the fold + byte-verify from the PRD ledger, the ports-and-adapters hexagon, the citation-grammar family, the registry-born check pattern, and the record-class freeze rules — so the net-new surface is one storage class, one port, one writer, one check, and one fold, each argued in the brainstorm's fired panels.

## 3. The design

### 3.1 The ledger home, entry schema, and fold *(#D-1, #ALT-3, #S-1)*

Per reviewed subject, the ledger lives at **`docs/feedback/<subject-stem>/`**, holding one **dense-numbered markdown file per captured entry** (`<NNN>-<source>.md`) plus a generated projection **`effective.source.json`** the fold produces for FR-5. `<subject-stem>` is the reviewed artifact's slug (e.g. a PRD/design slug, or a proposal identity for a code PR).

One file per entry makes each capture **pure addition** to the subject directory (AP-27/AP-23 — no shared append point); an entry file is **append-only within itself**: capture writes the initial block (id · source · author · timestamp · subject-anchor · the comment **verbatim**), and the disposition transition (§3.4) is **appended** later, never rewriting prior content (PRD NFR-2).

**Decision:** the machine projection (`effective.source.json`) is generated by `docgate fold docs/feedback/<subject-stem>` and **byte-verified in CI** (the PRD-ledger contract) — never hand-authored; a wrong projection is fixed at the entry or the fold, never the output. The fold regenerates on each entry write (capture and disposition), byte-verified by `verify-generated` in CI and a desk hook — the PRD-ledger freshness contract; a stale projection reds the gate.

- **DINV-1** — an entry file is append-only: capture writes its initial block once; the disposition transition appends; prior content is never rewritten. The projection is derived, never authored.

### 3.2 Boundaries and non-goals

- Inherits the PRD's `NG-1..NG-5`: no optimizer engine, scorecards, or UI; `review-report/v1` and the Argus ledgers are **read-only cross-link targets**, never modified.
- **O-1 (from the record):** the all-proposals review-rejection rate (rejected ÷ all reviewed) is **not** computed here — this design emits the ledger-native numerator signals; the rate is the optimizer's (NG-1), needing the external all-reviewed denominator.
- The **amendment model is not touched** (D-4/F-6): resolution rides its existing free ref slot.

### 3.3 The `CaptureSource` port and its adapters *(#D-3, #ALT-7, #S-3)*

**Decision:** the writer declares a consumer-side port `CaptureSource` yielding a schema-versioned `CapturedComment{comment, author, timestamp, subject_anchor, source_id}` (AP-6). Two adapters implement it — `doc-review` (a confirm-time hook persisting the annotation the skill today discards) and `code-host` (webhook/poll comment ingestion) — each using its medium's mechanism internally. The writer core imports neither adapter; the boundary is a lint rule, not convention.

- **DINV-2** — the writer core imports no adapter package; a new capture source is a new adapter, never new core code (AP-1/AP-3/RW-3).
- **DINV-6** — ledger entries are written only by the coded writer; no code path or guide permits hand-authoring (FR-9).
- Capture is **off the review critical path** (PRD NFR-1) and **fails open** — a failed ledger write never blocks or loses the comment; it is queued for retry (PRD UB-1/UB-2), idempotent on comment identity (UB-4).

### 3.4 Disposition determination *(#D-2, #ALT-4, #S-2)*

**Decision:** the writer sets an entry's typed disposition (`applied` · `declined-with-reason` · `became-incident` · `no-action`) + outcome-ref **from an observed resolving event**, never by inference:

- **doc-review** — the annotation→edit link is precise: the propose-and-confirm edit that merges is the `applied` outcome (+ its stable commit/amendment ref); an author decline is a captured `declined-with-reason`.
- **code-host** — where a 1:1 comment→commit link is absent, the writer uses a **coarser observable** (the proposal merged / the thread marked resolved / a decline), never a guess.
- A comment that produces no change closes `no-action` with a reason (UB-3); one whose disposition is not yet observable stays **open, flagged pending** (UB-5); an entry whose subject anchor no longer resolves is **retained + flagged stale** (UB-6).

- **DINV-3** — every disposition transition names the observed resolving event it was set from; a transition citing no event is invalid, so inference has no admissible path (I3 — no consequential guess).

### 3.5 Citation, outcome-refs, and enforced resolution *(#D-4, #S-4)*

**Decision:** entries cite as **`review-feedback:<subject-stem>#<entry-anchor>`** — a sibling of the `prd:`/`design:`/`brainstorm:` family, resolving by grep. Each entry's `outcome` names a **stable** ref (the merged/squash sha, the PR number, or an amendment id) — **never a working-branch sha** (squash-merge orphans it). Cross-links to reviewer M2 / Argus `E-n` are **one-directional** (we cite them; NG-5 forbids editing them to cite back). `Learned-from: review-thread` resolves to a `review-feedback:` anchor through the amendment model's existing free ref slot (docgate validates the *class*, not the *ref* — F-6), so **no amendment-model supersession**.

**Decision:** a **registry-born docgate check** (`check-review-feedback-resolution`, extending `distributed-check-registration`) fails any entry `outcome` ref or `Learned-from: review-thread` ref that does not resolve — moving the FR-4/M4 provenance guarantee from convention to gate (AP-10).

- **DINV-4** — an entry `outcome` is a stable ref, never a working-branch sha.
- **DINV-5** — a `review-feedback:` / `Learned-from: review-thread` ref that fails to resolve fails the gate; the provenance edge is enforced, not conventional.

### 3.6 The ledger-native measures *(#D-5)*

The fold's projection exposes, from the ledger alone: **disposition counts** per type; the **applied/decline ratio** over disposed entries (the feedback-level rejection signal); and `edit_distance` **frozen in the entry at write** — the writer computes the applied change's size when it tags the `applied` outcome and stores it (AP-21 judge-once), so no later diff read is needed. The consumer (reviewer-calibration / M2) reads these (FR-6). The all-proposals rejection rate is out of scope (§3.2, O-1).

### 3.7 Observability

The PRD's tracking plan is realized as domain events: `feedback_captured` (a file is written), `feedback_disposed` (a disposition transition appends), `feedback_signal_read` (a consumer reads a derived signal) — feeding PRD metrics M1/M3/M4 and M2. The inspection surfaces are the artifacts themselves: the append-only entry files (greppable, AP-5), the byte-verified projection, and the resolution check's pass/fail. No hidden runtime state.

### 3.8 Security & privacy

Echoes the header row: **low**. Comments are captured **verbatim** (contractual data, no paraphrase) into **repo-internal** markdown — the same collaboration surface (doc-review annotations, code-host comments on the repo's own PRs) they already live on; this adds **no new data class beyond operator handles already present** (the Argus-ledger "impact none" precedent) and **no new egress or network surface** (code-host ingestion is inbound). The **redaction backstop is the existing commit-path secret gate** (IDEN-4-class) — the ledger write goes through the normal commit, so a captured secret is caught by the scan that already gates every commit; **no bespoke redaction** (a stronger scrub is a waiting-room item). **Caveat:** code-host ingestion may capture a third-party commenter's handle, already public on the source PR; still no new egress.

### 3.9 Invariants

DINV-1..DINV-6 above. All are property-testable: append-only entry files (DINV-1), the adapter-import boundary lint (DINV-2), every disposition citing its resolving event (DINV-3), stable outcome-refs (DINV-4), the resolution gate (DINV-5), and the coded-writer-only producer (DINV-6).

## 4. Alternatives considered

*Materialized from the record's `ALT-n` ledger (three fired panels); each row cites its entry.*

| Alternative | Verdict / why |
|---|---|
| Markdown record-class only (`#ALT-1`) | hybridized into the chosen shape — its human face kept, a folded projection added for FR-5 |
| Structured JSONL only (`#ALT-2`) | rejected — machine-native but breaks the corpus's markdown-record uniformity and is diff-hostile for the verbatim/privacy review |
| **Markdown + folded projection (`#ALT-3`)** | **chosen (D-1)** — both faces, reusing the fold + byte-verify tooling |
| Inferred-by-fold disposition (`#ALT-5`) | rejected — fuzzy comment→diff matching; a wrong auto-close is consequential (I3); drags in §5.9 evals |
| Human-confirmed disposition (`#ALT-6`) | rejected — adds ongoing human toil and a human-presence dependency (PRD NG-4 + zero-toil guardrail) |
| **Writer-tags at resolution (`#ALT-4`)** | **chosen (D-2)** — deterministic, observed-event, no guessing |
| Bespoke per-source capture (`#ALT-8`) | rejected — leaks the medium into core (AP-1/RW-3 violation) |
| Event-mediated capture (`#ALT-9`) | rejected — bus delivery/ordering semantics complicate UB-1's no-loss; the webhook case lives inside the code-host adapter anyway |
| **One `CaptureSource` port (`#ALT-7`)** | **chosen (D-3)** — medium-neutral, testable, extensible |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | code-host's coarser observable mis-tags a disposition (no 1:1 comment→commit link) | a wrong `applied`/`no-action` | never a guess — a non-observable disposition stays `open`/pending (DINV-3, UB-5); the signal is advisory + calibration is propose-and-confirm | disposition fixtures; UB-5 test |
| R2 | the fold + resolution check + one port is more machinery than a single flat file | build/maintenance cost | every piece reuses an existing primitive (fold, registry-check, hexagon); net-new is minimal | design inspection; A-1/A-4 |
| R3 | `edit_distance` computed at write adds capture cost | latency/cost on the write path | capture is off the critical path (NFR-1); the metric is a cheap diff-size, computed once (AP-21) | NFR-1 test; cost analysis |
| R4 | per-entry files proliferate under a busy subject | directory noise | dense-numbered, greppable; a per-subject dir bounds it; volume watched at review (the record-class R4 posture) | directory-size review |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | `docs/feedback/<subject-stem>/` holds one append-only markdown file per entry (schema per §3.1) + a `docgate fold`-generated, byte-verified `effective.source.json` | Test | fold + verify-generated run | prd:review-feedback-capture/FR-1 · FR-7 · #DINV-1 |
| A-2 | a `CaptureSource` port with doc-review + code-host adapters; the writer core imports no adapter (lint-enforced) and is the sole entry producer (no hand-authoring); capture is off-critical-path and fails open | Test | port + adapter tests; import-boundary lint; UB-1 failure-injection | prd:review-feedback-capture/FR-1 · FR-2 · FR-9 · #DINV-2 · #DINV-6 |
| A-3 | the writer tags every entry's disposition from an observed resolving event; a non-observable one stays pending; a dangling subject stays flagged-stale; no inference path exists | Test | disposition fixtures (UB-3/5/6) | prd:review-feedback-capture/FR-3 · #DINV-3 |
| A-4 | entry `outcome` refs are stable; `Learned-from: review-thread` resolves to a `review-feedback:` anchor; a dead ref **reds** `check-review-feedback-resolution`; the amendment model is unchanged | Test | resolution-check pass/fail fixtures; amendment-model diff = empty | prd:review-feedback-capture/FR-4 · #DINV-4 · #DINV-5 |
| A-5 | disposition counts + applied/decline ratio + stored `edit_distance` compute from the projection alone; the reviewer-calibration consumer reads ≥1 signal and cites ≥1 entry | Demo | projection query + consumer demo | prd:review-feedback-capture/FR-5 · FR-6 |
| A-6 | captured comments are verbatim + repo-internal; the commit-path secret gate is the redaction backstop; `review-report/v1` + Argus ledgers unchanged (cross-links one-directional) | Inspection | diff review; cross-link resolution | prd:review-feedback-capture/FR-8 · §3.8/§3.9 |
| A-7 | this design passes `docgate lint-tech-designs` and ships with the regenerated index | Test | gate output in the introducing PR | conventions §3.2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | `edit_distance` unit — changed lines vs characters vs a normalized token count | implementation | nothing (a metric detail; FR-5 is satisfied either way) | open | leaning: changed lines, the cheapest stable diff-size |
| Q2 | Whether `check-review-feedback-resolution` graduates to a hard CI gate immediately or rides the advisory period first (the shared docgate-graduation trigger) | operator | nothing | open | leaning: advisory first, matching the class posture |
| Q3 | The `<subject-stem>` identity for a code-host proposal (PR number vs a content-addressed id) — a naming detail behind the CaptureSource adapter | implementation | nothing | open | |

## Principles

- **AP-1 / AP-3 — honored.** The `CaptureSource` port is consumer-side; the writer core imports no adapter; a new source is a new adapter.
- **AP-5 — honored.** Greppable append-only entries + domain events extend the provenance/trace plane; the `review-feedback:` grammar is the citation edge.
- **AP-6 — honored.** Both faces are contractual: typed markdown entries + a schema-versioned projection.
- **AP-10 — honored.** The fold codifies the parse; the resolution check codifies the provenance guarantee — enforcement, not prompt discipline.
- **AP-17 — honored.** Capture (additive) fails open; disposition (consequential) is deterministic on an observed event, never a guess.
- **AP-21 — honored.** `edit_distance` is judged once, at write, and frozen in the entry.
- **AP-23 / AP-27 — honored.** One file per entry in a per-subject directory: every capture is pure addition, never an edit to a shared center.
- **RW-3 — honored.** Medium specifics (confirm hook, webhook, poll) live inside the adapters, dying at the port boundary.
- **AP-13 — honored.** The projection is byte-verified and the resolution check + append-only entries are the verified inspection surfaces — success is a verified artifact, never a prose claim.
- **AP-19 — honored.** Capture and disposition fire only on observed events; an idle ledger costs nothing.
- **Incidents:** **I1** honored — `Serves` cites committed FRs (FR-1..9); the resolution check's own first gate cannot gate itself, so operator review is its bootstrap backstop. **I3** honored — no consequential inference; misparse/non-resolution fails closed.
- **Deviation:** none.
