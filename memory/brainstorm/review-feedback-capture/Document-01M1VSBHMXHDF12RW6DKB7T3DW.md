# Review-feedback capture — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Capture the human-review feedback loop — the human's review comment (via doc-review or the code host), the system's response, and the outcome — as a portable, learnable record the self-optimization and traceability planes can read. |
| **Relevant-when** | brainstorming or contracting how Pyxis records human review feedback and its resolution; deciding where feedback/learning signal (edit-distance, rejection rate, dispositions) is captured; reconciling `review-report/v1`, the Argus ledgers, `FeedbackRound`, or the amendment `Learned-from` field with a feedback-capture substrate |
| **Idea** | *(verbatim, operator, 2026-07-12)* "one of the missing pieces in pyxis human-review process is we are not capturing the human's comment and the system response to learn from in the future. for example when a human reviews a prd (through doc-review or github ui) and provides feedback, we do not store it anywhere in the pyxis system and what happened after that. so that feedback loop goes unrecorded" |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-12 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | `docs/prds/review-feedback-capture/` |
| **Prior records** | brainstorm:authoring-reviewers, brainstorm:reviewer-artifacts-tech-design, brainstorm:argus-tech-design, brainstorm:autonomous-developer |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/product-vision.md` | governing layer — §11 self-improvement, §6 evidence/traceability/escalation, §4 human review stages 2/5/9, §10 metrics |
| `docs/prds/vnext-self-improvement.md` | VNEXT-15 self-optimization — the named consumer of feedback signal ("review-rejection rate and human edit-distance on proposals … feedback-round counts") |
| `docs/prds/vnext-requirements.md` | VNEXT-3 traceability (the outcome-signal spine), VNEXT-5 evidence packages, VNEXT-13 human escalation |
| `docs/prds/README.md` | requirements index — confirm no existing capture feature; adjacents OBS-6, KNOW-4, GATE-4/6 |
| `docs/domain-model.md` | `FeedbackRound` (Port), `ChangeProposal` feedback threads, `DomainEvent` feedback family, `JudgmentPoint` feedback-resolution, INV-21 closes-reference |
| `docs/prds/observability-and-spend.md` | OBS-6 prompt logging & provenance — the redacted-capture-for-review precedent |
| `docs/prds/quality-and-merge.md` | GATE-6 bounded feedback recovery loop; GATE-4 risk-tiered approval |
| `docs/tech-design/prd-amendment-model.md` · `docs/wiki/glossary.md` | the `Learned-from` typed evidence classes incl. `review-thread` / `operator-decision`; closable-finding + scope-lock vocabulary |
| `docs/tech-design/prd-review-artifacts.md` | `review-report/v1` schema — disposition, verdict history, post-approve overlay, calibration home |
| `docs/tech-design/argus-artifacts.md` | problem-ledger `A-n` / delivery-ledger `E-n` — the human-capture precedent (Argus-driven only) |
| `.pyxis/skills/doc-review/` (PLAN-3 / BENCH-4) | the annotation-discard behavior at the capture source (zero git mutations of its own) |

**Overlaps surfaced:**

- **brainstorm:authoring-reviewers** — the unmanned reviewers are self-contained and assume **no** human review; human review is "an optional overlay on the same data," never a dependency. *Ruling to respect (not a refusal):* this feature captures that overlay as **learning fuel**; it must never make the pipeline depend on human review.
- **`review-report/v1`** (prd-review-artifacts · reviewer-artifacts) — already records per-finding disposition (applied / declined-with-reason), append-only verdict history, and post-approve overlay corrections (the M2 residual); calibration lives at `.pyxis/skills/<reviewer>/calibration.md`. **Partial overlap:** the reviewer's *own* review is captured; the *general* human review (doc-review annotations, code-host comments on any doc, non-reviewer-mediated) is not. → feed-into, don't duplicate.
- **`FeedbackRound`** (Port `pr-review-feedback-loop`) + **GATE-6** — the **react** loop (bounded rounds → escalate `feedback-stalled`); it routes feedback, it doesn't remember it. Complementary — the feature is the memory side of the same surface.
- **`ChangeProposal` carries feedback threads** + **`DomainEvent` feedback family** (events.jsonl) — feedback is already modeled on the proposal and there are feedback events; candidate capture rails.
- **amendment `Learned-from: review-thread` / `operator-decision`** — PRD amendments already must cite *why*, including "review-thread," but the thread content is not in-corpus (an unresolvable pointer). → the feature can make it resolvable.
- **Argus `A-n` / `E-n`** (argus-artifacts) — captures human rulings/answers/dispositions, but Argus-driven problems only and breaking-calls only. → generalize beyond Argus; feed-into.
- **VNEXT-15 self-optimization** + **VNEXT-3 traceability** — the specified consumers; the capture they assume was never built. Not refusals — the gap this feature fills.
- **OBS-6 prompt logging** (redacted capture, weekly review) — an existing capture-for-review precedent; a sibling data class.

*Net: no covering capability and no blocking refusal — the idea is live.*

## Framings

*2–3 genuinely distinct theses of what the feature is; operator picks or hybridizes.*

- **A — The learning corpus** *(purpose-first).* The feature **is the capture substrate `VNEXT-15` self-optimization consumes**: a typed record of ⟨human feedback → response → outcome⟩ whose reason to exist is to make the factory's own artifacts improve (reviewer calibration, prompts, thresholds, Argus authority) from measured human-correction signal. Success is *downstream* (interventions-per-feature falls; reviewer M2 residual trends to 0). Implications: capture must yield machine-computable signal (edit-distance, rejection rate, disposition counts); it is scoped by what the learners can actually consume; it lives near the observability + calibration plane. **Risk:** scoping to today's learners under-captures; VNEXT-15 is itself unbuilt.
- **B — The provenance node** *(structure-first).* The feature **is the missing upstream edge of the traceability chain (AP-5)**: raw idea → PRD ruling → **human review comment → change** → merged code → incident. As the brainstorm record made "idea → ruling" greppable, this makes "review comment → what changed" a resolvable citation — upgrading `Learned-from: review-thread` from a dead pointer to a real edge. Success is *queryability* ("every human-caused change names the comment that caused it"). Implications: it is a citation grammar + resolvable anchors riding existing artifacts (amendments, review reports, the introducing PR), provenance-class not governed-kind. **Risk:** minimal capture (just enough to resolve a citation) may not carry enough content to *learn* from.
- **C — The unified feedback ledger** *(artifact-first).* The feature **is one new first-class capture class** that every human-review touchpoint feeds — doc-review annotations, code-host PR/issue comments, reviewer overlays, Argus answers — a typed, append-only, per-subject ledger where each entry is a **closable item** (comment → disposition → outcome anchor), written by a coded writer, riding the subject's PR. Success is *one substrate, not three* (reconciling `review-report/v1` and Argus `E-n` as feed-ins, not rivals). Implications: adapter-mediated capture; a new writer; a home decision. **Risk:** a store built before a named consumer is the write-only-ledger trap; scope creep into review-management.

**Destination:** **Hybrid — one unified feedback ledger (framing C), scope-gated by a named consumer (framing A), each entry a resolvable provenance node (framing B).** A single append-only capture substrate every human-review touchpoint feeds; each entry a closable comment→disposition→outcome item that doubles as a traceability edge (upgrading `Learned-from: review-thread` from a dead pointer); scope bounded by naming the first consumer (reviewer calibration / the M2 residual) so it is never write-only. Folds `review-report/v1` M2, Argus `E-n`, and `FeedbackRound` into one substrate rather than adding a fourth. — operator pick, 2026-07-12 (recommendation adopted; see D-1).

## Decisions

### D-1 — What the feature fundamentally is (problem framing)

- **Question**: Of three distinct theses — learning corpus (A), provenance node (B), unified ledger (C) — what is this feature's center of gravity?
- **Answer**: The hybrid — build the unified feedback ledger (C) as the artifact, gate its scope by a named first consumer (A), and shape each entry as a resolvable provenance node (B).
- **Reasoning**: "We don't store it anywhere" is answered by an artifact; the corpus's no-write-only-ledger instinct demands a named consumer; the `Learned-from: review-thread` seam is cheap to make resolvable — and one substrate folds the three partial captures (review-report M2, Argus `E-n`, `FeedbackRound`) into one.
- **Anchor**: 2026-07-12 · framing pick
- **Status**: live

### D-2 — Scope boundary: capture substrate + one proven consumer

- **Question**: How much of the loop does this feature build — capture-only, capture + prove one consumer, or capture + the first optimization loop?
- **Answer**: Capture substrate + exactly one proven consumer, nothing more. IN: the ledger (schema + coded writer), adapter-mediated capture at doc-review + code-host, the derived signals defined as computable from the ledger (rejection rate, edit-distance, disposition counts), and wiring the one existing reviewer-calibration / M2-residual consumer to prove it. OUT: see O-1..O-4.
- **Reasoning**: capture is the genuinely missing piece; the optimizer is a separately-specified vNext program that would balloon appetite and pull in cohort/A-B machinery; naming *and* wiring one real consumer discharges the write-only risk without building the whole loop.
- **Anchor**: 2026-07-12 · turn 2 ("the boundary holds")
- **Status**: live

### D-3 — Primary beneficiary (segment)

- **Question**: Whose need does the captured record primarily serve, and who is it explicitly not for?
- **Answer**: Primary = the factory's own learning + traceability loops (reviewer-calibration now; VNEXT-15/VNEXT-3 later). The human reviewer is the *source* of the signal; the "closed loop / what happened after" is an incidental secondary benefit, never a design driver. Anti-segment = teams wanting a human-review workflow/management product (see O-3).
- **Reasoning**: naming the factory-loop as beneficiary keeps the feature honest to D-1's learning-input purpose and stops drift toward a review-UX product; the human is upstream (produces signal), the factory downstream (consumes it) — which is why O-4 holds.
- **Anchor**: 2026-07-12 · turn 3 ("go with the recommendation")
- **Status**: live

### D-4 — The primary jobs

- **Question**: What must the substrate do for its beneficiary?
- **Answer**: Adopt all three. **J1 Capture** — the human comment verbatim, attributed (who/when), subject-linked. **J2 Close the loop** — each comment reaches a typed disposition + outcome anchor (`applied`→commit/amendment · `declined-with-reason` · `became-incident` · `no-action`+reason); every entry is *closable*, per the reviewers' closable-finding discipline. **J3 Serve the signal** — rejection rate, edit-distance, disposition counts, computable from the ledger, for the reviewer-calibration consumer.
- **Reasoning**: the three jobs are D-1's three framings made concrete as capabilities (C=capture, B=provenance/close, A=serve); closability is what prevents capture-and-forget.
- **Anchor**: 2026-07-12 · turn 4 ("adopt all")
- **Status**: live

### D-5 — The bet and observable success

- **Question**: What is the bet, and how do we observe success without claiming the downstream optimizer's outcome?
- **Answer**: **Bet** — capturing the human-review loop (comment + response + outcome) as a closable, computable ledger wired to the one existing consumer turns lost human review into measurable fuel, and makes every human-caused change traceable to its cause. **Four checks**: (1) capture coverage at the two sources; (2) closability (~100% of entries reach a typed disposition); (3) consumer-proof (the reviewer-calibration/M2 path computes ≥1 signal from, and cites, the ledger); (4) provenance (a human-caused change resolves back to its entry). **Excluded metric**: "M2 residual falls / reviewers improve" — that is O-1 / VNEXT-15's payoff.
- **Reasoning**: judges the feature on what it builds (capture, closure, consumption, traceability), not on the learning outcome it scoped out — honest and verifiable.
- **Anchor**: 2026-07-12 · turn 5 ("adopt the bet and the four checks")
- **Status**: live

### D-6 — Federate, not subsume (home of record + cross-links)

- **Question**: Does D-1's "one substrate" mean migrating review-report M2 and Argus `E-n` into the new ledger (subsume), or a home-of-record for the uncaptured comment plus cross-links (federate)?
- **Answer**: Federate. The new ledger is the **home of record** for the genuinely-uncaptured general human-review comment at the two human sources (doc-review + code-host); the existing captures (reviewer M2, Argus `E-n`) stay untouched and everything cross-links via the shared citation grammar. Capture-sources scope thereby = **2 captured here** (doc-review, code-host) + **2 linked** (reviewer M2, Argus `E-n`).
- **Reasoning**: the missing thing is the general comment; M2/Argus already capture their slices; subsume would re-open two merged designs (large blast radius, against the immutability / one-concern posture). Federation is still "one place to look," achieved by citations (AP-5), not migration. Refines D-1's "folds into one" → "one home of record + cross-links."
- **Anchor**: 2026-07-12 · turn 6 ("go federate, existing captures untouched")
- **Status**: live

### D-7 — Appetite: medium, foundational, hard-capped

- **Question**: How much is this problem worth (spend/scope, never time)?
- **Answer**: Medium and foundational, hard-capped at "capture + prove one consumer." In-budget: one new capture class (schema + coded writer), the two source adapters (doc-review annotation persistence + code-host comment ingestion), the derived-signal definitions, one consumer wiring (reviewer-calibration/M2), the cross-citation grammar. Over-budget signal = any optimizer/scorecard/UI work or modifying existing captures (O-1..O-5). Adds ~zero ongoing human toil; low rollout/support (additive data, no gate/merge-path change). T2-ish rigor.
- **Reasoning**: VNEXT-3 / VNEXT-15 stand on it, so build it well; the hard cap keeps it a bounded learning input, not the whole loop.
- **Anchor**: 2026-07-12 · turn 7 ("confirm the bound")
- **Status**: live

## Facts

- **F-1**: VNEXT-15 self-optimization names its learning signals as "review-rejection rate and human edit-distance on proposals … feedback-round counts, incident lessons, and token/latency cost" — source: `docs/prds/vnext-self-improvement.md` §VNEXT-15
- **F-2**: VNEXT-3 end-to-end traceability is "the queryable chain from raw idea to incident … the substrate the next three items stand on" — source: `docs/product-vision.md` §11 · `docs/prds/vnext-requirements.md` VNEXT-3
- **F-3**: `review-report/v1` (in `docs/reviews/`, one per subject per moment) records per-finding disposition (applied / declined-with-reason), append-only verdict history, and post-approve overlay corrections (the M2 residual); calibration is data at `.pyxis/skills/<reviewer>/calibration.md` — source: `docs/tech-design/prd-review-artifacts.md` · brainstorm:reviewer-artifacts-tech-design · brainstorm:authoring-reviewers
- **F-4**: The unmanned reviewers are designed with **no** assumption of human review; human review is "an optional overlay on the same data, never a dependency" — source: brainstorm:authoring-reviewers
- **F-5**: `FeedbackRound` (Port `pr-review-feedback-loop`) is the **react** loop — human review comments dispatch a bounded resolve-and-reply loop, round cap → escalate `feedback-stalled`; it routes feedback, it does not persist it — source: `docs/domain-model.md` §3.3, §5
- **F-6**: `ChangeProposal` (GitHub PR) "carries risk lane, approvals, checks rollup, **feedback threads**"; `DomainEvent` has a `feedback` recovery family emitted to events.jsonl — source: `docs/domain-model.md` §3.3, §3.7
- **F-7**: The PRD amendment envelope requires a typed `Learned-from` evidence field whose closed classes include `review-thread` and `operator-decision`, each with a structured reference — but the referenced thread content is not itself captured in-corpus — source: `docs/wiki/glossary.md` (amendment template) · `docs/tech-design/prd-amendment-model.md`
- **F-8**: Argus captures human input as append-only `A-n` (problem ledger: pre-rulings + breaking-call answers) and typed `E-n` dispositions (`answer-received`, `proposal-disposed`) in the delivery ledger — but only for Argus-driven problems — source: `docs/tech-design/argus-artifacts.md`
- **F-9**: `doc-review` makes zero git mutations of its own — annotations drive propose-and-confirm edits and are then discarded; the resulting edit survives, the comment does not — source: `.pyxis/skills/doc-review/` (PLAN-3 / BENCH-4)
- **F-10**: OBS-6 (prompt logging & provenance — redacted capture, weekly review) is the existing "capture for later review" precedent in the corpus — source: `docs/prds/observability-and-spend.md` · `docs/prds/README.md`

## Fog

none

## Out of scope

- **O-1**: the VNEXT-15 self-optimization *engine* (measure→propose→trial→adopt + cohort/A-B) — a separately-specified vNext program; this feature only produces the signal it consumes (from D-2)
- **O-2**: backend scorecards (VNEXT-7) — the routing instance of the optimization loop, not this feature (from D-2)
- **O-3**: any review-management or analytics UI/dashboard — the ledger is data; a human-facing surface is a separate concern (from D-2)
- **O-4**: any change to the reviewers' "no human dependency" posture — human review stays an optional overlay; this feature captures it, never makes the pipeline depend on it (from D-2)
- **O-5**: migrating or modifying `review-report/v1` or `argus-artifacts` (the reviewer M2 residual, Argus `A-n`/`E-n`) — they stay untouched; this feature only *adds* cross-links to them (from D-6)

## Candidate glossary terms

- **T-1**: review-feedback ledger *(working name — final name set at prd-new)* — the home-of-record for human review comments captured at human-facing surfaces (doc-review, code-host), each a closable comment→disposition→outcome entry cross-linked to the existing captures; the class this feature introduces. · avoid: bare "feedback" (overloaded — `FeedbackRound` the react-loop / PR feedback threads / feedback events / `feedback-stalled`); "review record" (collides with `review-report/v1`)

## Parked technical constraints

- **P-1**: how a J2 disposition/outcome is *determined* — writer-tagged at resolution, inferred from the merge/amendment, or human-confirmed — is a stage-3/4 mechanism, not resolved here (from D-4)
- **P-2**: the cross-citation grammar — how a ledger entry cites the resolving amendment/commit and links reviewer M2 / Argus `E-n`, and how `Learned-from: review-thread` resolves to a ledger anchor — is a stage-3 seam (from D-6)
- **P-3**: privacy/redaction + access posture for captured comments — verbatim human text may carry identities, @mentions, or sensitive context; the tech-design's mandatory Security subsection decides it (precedent: OBS-6 redacted capture; argus-artifacts repo-internal, impact none) (surfaced at D-7)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-12 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1 D-1 · §2.2 D-3 · §2.3 D-4 · §3.1 D-5 · §3.4 O-1..O-5 · appetite D-7), zero blocking fog |
