# Review-feedback capture — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Technical direction for the review-feedback ledger — where it lives + its schema, how a disposition is determined, how capture reaches it from conversation + the code host, the cross-citation grammar, and the privacy posture. |
| **Relevant-when** | designing or changing the review-feedback ledger / its capture writer / its consumer wiring; checking prior design rulings on where human-review feedback is stored, how entries close, or how they cite; reconciling with `review-report/v1`, the Argus ledgers, or the amendment `Learned-from` field |
| **Basis** | `prd:review-feedback-capture` (merged ledger, 000) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-13 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | docs/tech-design/review-feedback-capture.md |
| **Prior records** | brainstorm:review-feedback-capture |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/review-feedback-capture/effective.md` | the basis — FR-1..FR-9, NR-1..NR-4, UB-1..6, the four parked seams (P-1..P-3, Q5) |
| `docs/brainstorms/review-feedback-capture.md` | stage-1 provenance (D-1..D-7): federate-not-subsume, closable, consumer-gated |
| `docs/architecture-principles.md` | AP/RW registry for panel scoring — esp. AP-1/RW-3 (medium-independence), AP-6 (contractual), AP-23/AP-27 (parallel authors, addition-not-shared-center), AP-10 (codify), AP-17 (fail-closed/open) |
| `docs/tech-design/prd-amendment-model.md` | the **merged** `Learned-from` contract (`review-thread` class) our FR-4 must resolve into — evolves only by supersession |
| `docs/tech-design/prd-review-artifacts.md` | `review-report/v1` — the M2 residual + calibration home we cross-link (never touch, NG-5) |
| `docs/tech-design/argus-artifacts.md` | the Argus `A-n`/`E-n` ledgers we cross-link (never touch, NG-5) |
| `docs/tech-design/brainstorm-record-model.md` · `design-brainstorm-record-model.md` | the append-only markdown record-class precedent (home/lifecycle/citation grammar/freeze-at-merge) our ledger can inherit |
| `docs/tech-design/local-work-tracker.md` | the files-as-truth localfs adapter precedent (medium-neutral, header-scan queries) |
| `docs/domain-model.md` | `FeedbackRound`/`ChangeProposal` feedback threads, `DomainEvent` feedback family, the ports/adapters map |
| `docs/observability/incidents.md` | I1 (Serves-cites-committed-FR; graduation-gate bootstrap), I3 (fail-open vs consequential) |

**Overlaps surfaced:**

- **`prd-amendment-model` (live design, merged)** — defines `Learned-from`'s `review-thread` class; our FR-4 resolves it to a ledger anchor. **Not a covering design** (it doesn't decide our storage/resolution); constrains us: any change to `Learned-from`'s semantics is a *supersession*, never a silent edit (§P-2 / FND-5 of prd-review).
- **`review-report/v1` + Argus `A-n`/`E-n` (live designs)** — the existing partial captures we **federate** with; NG-5 forbids touching them. Cross-link targets, not storage decisions.
- **The record-class models (brainstorm / design-brainstorm)** — a proven append-only markdown provenance-class shape (flat home, dense anchors, citation grammar, freeze-at-merge). A strong reuse candidate for our home/schema — but built for *human-authored session records*, not machine-computed signal.
- **`local-work-tracker`** — the files-as-truth, medium-neutral adapter precedent (relevant to code-host comment ingestion + repo-portable storage).
- *Net: no live design covers the direction; several constrain or offer reuse. The idea is live.*

## Design questions

- **DQ-1**: The ledger's **physical home + entry schema** — where does the review-feedback ledger live on disk, in what format (markdown record-class vs structured data), and per-subject vs global?
- **DQ-2**: How is an entry's **disposition + outcome determined** — writer-tagged at resolution, inferred from the merge/amendment, or human-confirmed? (basis P-1)
- **DQ-3**: The **capture-source interface** — how conversation (persist comments) and the code host (ingest comments) feed the writer; is there one capture-source port both implement?
- **DQ-4**: The **cross-citation grammar** — the `review-feedback:` family, how an entry cites its resolving artifact + links M2/`E-n`, and how `Learned-from: review-thread` resolves (respecting the merged amendment model). (basis P-2)
- **DQ-5**: The **derived-measure definitions** — rejection-rate denominator, edit-distance unit, disposition counts — computable from the ledger. (basis Q5)
- **DQ-6**: The **privacy/redaction + access posture** for captured verbatim comments (mandatory Security subsection). (basis P-3)

## Panel log

- **DQ-1**: opens/replaces — **fired** — a new data-class contract (storage shape + granularity) is a fresh seam; ≥2 structurally distinct shapes exist (markdown record-class vs structured data). See ALT-1, ALT-2, ALT-3.
- **DQ-2**: opens/replaces — **fired** — how an entry closes (open→disposed) is a mechanism seam; ≥2 distinct shapes (writer-tags-at-resolution vs inferred-by-fold). See ALT-4, ALT-5, ALT-6.
- **DQ-3**: opens/replaces — **fired** — a new consumer-side capture port is a fresh seam; ≥2 distinct shapes (one CaptureSource port vs bespoke per-source paths). See ALT-7, ALT-8, ALT-9.
- **DQ-4**: extends — **skipped** — `review-feedback:` is a sibling in the existing citation-grammar family, and the resolution check extends docgate's registry-born-check pattern; no new seam.
- **DQ-5**: extends — **skipped** — measure definitions extend FR-5's contract, not a seam; storing edit-distance at write keeps them ledger-native.
- **DQ-6**: extends — **skipped** — a Security-subsection policy decision (redaction/access), no new mechanism; relies on the existing commit-path secret gate.

## Decisions

### D-1 — Storage shape: markdown record-class + folded machine projection, per-subject

- **Question**: Where/how does the ledger live — markdown record-class (ALT-1), structured JSONL (ALT-2), or the markdown + folded-projection hybrid (ALT-3)?
- **Answer**: The **hybrid (ALT-3)** — per-subject append-only markdown entries + a generated machine projection (the PRD-ledger fold pattern) for native FR-5 signals.
- **Reasoning**: keeps the human-readable/diff-reviewable face the verbatim comments + the P-3 review want, gets FR-5's signals machine-native, and reuses the proven fold + byte-verify machinery rather than standing up a new JSONL class. ALT-1 hybridized in; ALT-2 rejected.
- **Anchor**: 2026-07-13 · DQ-1 pick
- **Status**: live

### D-2 — Disposition determination: writer-tags at resolution (event-driven)

- **Question**: How is an entry's disposition + outcome determined — writer-tags-at-resolution (ALT-4), inferred-by-fold (ALT-5), or human-confirmed (ALT-6)?
- **Answer**: **Writer-tags at resolution (ALT-4)** — the capture writer sets the typed disposition + outcome-ref when it *observes* the resolving event; no inference. conversation gives a precise comment→edit link; code-host falls back to a coarser observable (PR merged / thread resolved / declined), never a guess.
- **Reasoning**: the dispositions are observable at events (AP-18 observe-don't-declare); inference (ALT-5) is fuzzy and a wrong auto-close is consequential (I3 fail-closed) + drags in §5.9 evals; human-confirm (ALT-6) violates the no-toil / no-dependency guardrails (NG-4).
- **Anchor**: 2026-07-13 · DQ-2 pick
- **Status**: live

### D-3 — Capture seam: one CaptureSource port + two adapters

- **Question**: How do the two sources feed the writer — one CaptureSource port (ALT-7), bespoke per-source paths (ALT-8), or event-mediated (ALT-9)?
- **Answer**: **One CaptureSource port (ALT-7)** — the writer declares a consumer-side `CaptureSource` interface (comment verbatim, author, timestamp, subject anchor, source-id); conversation + code-host are two adapters; the writer core imports neither. Each adapter uses its medium's own mechanism internally (conversation explicit capture; code-host webhook/poll).
- **Reasoning**: medium-independence is founding (AP-1/AP-3, ports on the consumer side; RW-3); two real adapters justify the seam; a third source is a new adapter, not core code. Bespoke (ALT-8) leaks the medium into core; event-mediation (ALT-9) interposes bus semantics that make UB-1's no-loss harder.
- **Anchor**: 2026-07-13 · DQ-3 pick
- **Status**: live

### D-4 — Cross-citation grammar + enforced resolution

- **Question**: The citation family, outcome-ref, cross-links, and how `Learned-from: review-thread` resolves — convention-only or enforced?
- **Answer**: A `review-feedback:<subject-stem>#<entry-anchor>` family (sibling to prd:/design:/brainstorm:). Each entry's `outcome` names a **stable** resolving ref (merged sha / PR number / amendment id) — never the working-branch sha (squash-merge orphans it). Cross-links to reviewer M2 / Argus `E-n` are one-directional (NG-5). `Learned-from: review-thread` resolves to a `review-feedback:` anchor via the amendment model's existing free ref slot — **no model change** (F-6). **Enforced**: an in-scope docgate `review-feedback:` resolution check (registry-born, extends the check pattern) fails a `review-thread`/`outcome` ref that doesn't resolve — closing the AP-10 prompt-vs-codify gap on FR-4/M4/DOD-5.
- **Reasoning**: the feature *is* the provenance edge; convention-only resolution (cheaper) would let M4 silently drift; the lint enforces it within FR-4's scope with no supersession. Stable outcome-refs survive squash-merge.
- **Anchor**: 2026-07-13 · DQ-4 pick ("go with your suggestion")
- **Status**: live

### D-5 — Derived-measure definitions (ledger-native)

- **Question**: Define FR-5's three measures so they are genuinely "computable from the ledger alone."
- **Answer**: (1) **disposition counts** per type, per subject class — native; (2) **applied/decline ratio over disposed entries** — native (the feedback-level rejection signal); (3) **human edit-distance recorded at write time** — the writer computes the change's size when it tags an `applied` outcome and freezes `edit_distance` in the entry (AP-21 judge-once) → native. The all-proposals "review-rejection rate" (needs the all-reviewed denominator) is the optimizer's downstream job (NG-1), combining our ledger with the proposal count.
- **Reasoning**: storing edit-distance at write keeps "from the ledger alone" honest without re-reading resolving diffs; the cross-source all-proposals rate belongs to the out-of-scope optimizer, so no PRD change is needed.
- **Anchor**: 2026-07-13 · DQ-5 pick
- **Status**: live

### D-6 — Privacy/redaction posture: verbatim, repo-internal, secret-gate backstop

- **Question**: The privacy/redaction + access posture for captured verbatim comments (P-3).
- **Answer**: Verbatim capture (no paraphrase); repo-internal with no new egress/access surface (matches the Argus-ledger "impact none" precedent — the comments are already on the repo's own collaboration surface); the redaction backstop is the repo's existing commit-path secret hygiene (IDEN-4-class — the ledger write goes through the normal commit). No bespoke redaction now (a stronger scrub is a WR). Caveat: code-host ingestion may capture a third-party commenter's handle, already public on the source; still no new egress.
- **Reasoning**: no new data class beyond what the collaboration surface already exposes; the existing secret gate covers the sharp edge; bespoke redaction is unwarranted for a repo-internal, factory-read ledger.
- **Anchor**: 2026-07-13 · DQ-6 pick
- **Status**: live

## Alternatives

### ALT-1 — Markdown record-class, per-subject

- **Shape**: the ledger is a new append-only **markdown** provenance class (like `review-report/v1` / brainstorm records) — one file per reviewed subject, typed entry sections, dense anchors; signals computed by a parser/fold over the structure (the PRD-ledger pattern).
- **Trade-offs**: wins on human-readable provenance in diffs, one uniform provenance shelf, greppable citations, and massive reuse of the proven record-class machinery (home/lifecycle/grammar/freeze). Costs: FR-5's signals need a parser/fold, not a native read.
- **Principles**: AP-6 honored (contractual, human-granularity); AP-5 honored (greppable chain); AP-23/AP-27 honored (per-subject = addition, no shared hot file); AP-10 staged (lint at graduation); RW-2 honored (inherit the lesson, not re-derive).
- **Incidents**: I1 matched — our `Serves` cites committed FRs (FR-1..9); if it graduates to a docgate class (WR-3), the first gate can't gate itself (operator-review backstop). None else checked (incidents.md, 2026-07-13).
- **Verdict**: hybridized into ALT-3 — its markdown human-face is kept; the folded machine projection is added
- **From**: DQ-1 · panel

### ALT-2 — Structured data (append-only JSONL), per-subject

- **Shape**: the ledger is machine-native **JSONL** (like `events.jsonl` / the `effective.source.json` face) — append-only lines, schema-versioned records; a renderer produces a human face on demand.
- **Trade-offs**: wins on FR-5 (signals computed natively) and machine-nativity — and the usual "markdown for human review in PRs" cost is *weaker here* because the beneficiary is the factory loop, not a human reading the ledger (basis D-3). Costs: breaks from the corpus's uniform markdown-record culture; raw JSON diffs are hostile when a human *does* look (privacy review of verbatim comments, P-3).
- **Principles**: AP-6 honored fully (machine-native contractual); AP-5 honored; AP-23/AP-27 honored (append-only lines); AP-25 neutral.
- **Incidents**: I1 checked (same Serves discipline); I3 matched — the append is additive (fail-open correct); a consequential path (disposition-close) must still fail closed. None else checked (incidents.md, 2026-07-13).
- **Verdict**: rejected — the hybrid (ALT-3) gets FR-5's machine-nativity without abandoning the human markdown face or the corpus's record-class uniformity
- **From**: DQ-1 · panel

### ALT-3 — Markdown record-class + folded machine projection, per-subject *(chosen)*

- **Shape**: per-subject append-only **markdown** entries (ALT-1's human face) **plus** a generated machine projection (an `effective.source.json`-style face) a fold produces for native FR-5 signals — the PRD-ledger pattern applied to feedback. The projection is never hand-authored.
- **Trade-offs**: gets both — human-readable/greppable/diff-reviewable provenance AND machine-native signal computation — reusing the fold + byte-verify tooling. Cost: a fold + its verify-gate is more machinery than a single face.
- **Principles**: AP-6 honored (both faces contractual); AP-5 honored; AP-10 honored (the fold codifies the parse, not prompt discipline); AP-23/AP-27 honored (per-subject = addition); RW-2 honored (inherit the PRD-ledger fold pattern).
- **Incidents**: I1 matched (Serves cites committed FRs; the self-gating-bootstrap caution applies to the fold's own gate at graduation); I3 checked (fold deterministic; misparse fails closed). None else checked (incidents.md, 2026-07-13).
- **Verdict**: chosen (D-1) — the human face + machine projection resolves the ALT-1↔ALT-2 tension with a proven pattern
- **From**: DQ-1 · panel (synthesis of ALT-1 + the FR-5 need)

### ALT-4 — Writer-tags at resolution (event-driven) *(chosen)*

- **Shape**: the writer sets disposition + outcome-ref on the observed resolving event — the comment's edit merges (→ `applied` + commit), an author declines-with-reason (a captured act), the subject merges with no tied change (→ `no-action`), an incident cites it (→ `became-incident`). Deterministic.
- **Trade-offs**: precise, no guessing; needs a trigger on each resolving event. conversation has a precise comment→edit link; code-host uses a coarser observable where a 1:1 comment→commit link isn't available.
- **Principles**: AP-18 honored (key on observed state); AP-13 honored (verified artifact); AP-17 honored (additive capture fails open, a close is deterministic — not a guess); RW-3 honored (the event trigger lives at the adapter boundary, not in core).
- **Incidents**: I3 matched — a wrong auto-disposition would be the consequential-guess failure I3 warns of; event-observation avoids it. None else checked (incidents.md, 2026-07-13).
- **Verdict**: chosen (D-2)
- **From**: DQ-2 · panel

### ALT-5 — Inferred by fold

- **Shape**: a fold/analysis infers each disposition by matching the captured comment against what changed in the repo; no per-event hook.
- **Trade-offs**: works over repo state without triggers; but comment→diff matching is fuzzy and a wrong auto-disposition is consequential; needs §5.9 evals.
- **Principles**: AP-14 strained (acceptance by fuzzy inference, not observation); AP-17 strained (a consequential guess); AP-19 neutral.
- **Incidents**: I3 matched — this is exactly the consequential-guess path I3 warns against. None else checked (incidents.md, 2026-07-13).
- **Verdict**: rejected — fuzzy and consequential; inference is not the mechanism
- **From**: DQ-2 · panel

### ALT-6 — Human-confirmed

- **Shape**: a human sets each disposition (the author/reviewer marks it).
- **Trade-offs**: accurate; but adds ongoing human toil and makes closure depend on human presence.
- **Principles**: the basis's no-human-dependency (NG-4) and zero-toil guardrails are violated; AP-19 strained.
- **Incidents**: none checked (incidents.md, 2026-07-13).
- **Verdict**: rejected — conflicts with the basis's no-toil / no-dependency guardrails
- **From**: DQ-2 · panel

### ALT-7 — One CaptureSource port + two adapters *(chosen)*

- **Shape**: the writer declares `CaptureSource` (consumer-side) yielding {comment, author, timestamp, subject-anchor, source-id}; conversation and code-host adapters implement it; each uses its medium's own mechanism internally (explicit capture / webhook / poll).
- **Trade-offs**: medium-neutral, testable (a stub adapter), extensible (new source = new adapter); one small interface. Cost: one interface layer (justified by two real adapters).
- **Principles**: AP-1/AP-3 honored (medium-independent core, ports on the consumer side); RW-3 honored (medium workarounds die at the port boundary); AP-6 honored (the yielded record is contractual).
- **Incidents**: I1 checked (Serves cites committed FRs). None else checked (incidents.md, 2026-07-13).
- **Verdict**: chosen (D-3)
- **From**: DQ-3 · panel

### ALT-8 — Bespoke per-source paths

- **Shape**: conversation and code-host each write to the ledger directly; no shared interface.
- **Trade-offs**: less abstraction now; but couples the writer to each medium and a third source is new bespoke code.
- **Principles**: AP-1 violated (medium leaks into core); RW-3 violated (no port boundary); AP-3 strained.
- **Incidents**: none checked (incidents.md, 2026-07-13).
- **Verdict**: rejected — violates the founding medium-independence posture
- **From**: DQ-3 · panel

### ALT-9 — Event-mediated

- **Shape**: sources emit `feedback_captured` DomainEvents; the writer consumes the AP-5 event stream and folds events into entries.
- **Trade-offs**: decouples capture from the writer, reuses event infra; but interposes delivery/ordering semantics and makes UB-1's no-loss guarantee harder than a direct port.
- **Principles**: AP-5 honored (events first-class); AP-1 honored; AP-22 / UB-1 strained (at-least-once delivery + dedup required).
- **Incidents**: none checked (incidents.md, 2026-07-13).
- **Verdict**: rejected — bus semantics complicate no-loss; the webhook-feeding-adapter case lives inside ALT-7 anyway
- **From**: DQ-3 · panel

## Seam map

- **S-1**: the review-feedback ledger storage class — **opens** — behind it: the entry schema + granularity (DQ-1) — feeds: the design's §2 Decision + Scope + `Serves: prd:review-feedback-capture/FR-1,FR-3,FR-7`
- **S-2**: the resolving-event trigger + disposition writer — **opens** — behind it: per-source event observation (conversation edit-merge / code-host resolve), adapter-mediated — feeds: the design's Scope + `Serves: prd:review-feedback-capture/FR-3,FR-4`
- **S-3**: the `CaptureSource` port — **opens** — behind it: two adapters (conversation explicit-capture, code-host webhook/poll), writer core imports neither — feeds: the design's Scope + `Serves: prd:review-feedback-capture/FR-1,FR-2` + the AP-3 port/adapter map
- **S-4**: the docgate `review-feedback:` resolution check — **extends** — behind it: the registry-born-check pattern (distributed-check-registration); resolves entry `outcome` refs + `Learned-from: review-thread` → `review-feedback:` anchors — feeds: the design's Scope + `Serves: prd:review-feedback-capture/FR-4` (the M4 provenance guarantee)

## Facts

- **F-1**: The basis's beneficiary is the factory's learning/traceability loop, not a human reading the ledger — weakens the "markdown-for-human-review" pull that chose markdown for `review-report/v1` — source: brainstorm:review-feedback-capture#D-3
- **F-2**: `review-report/v1` chose markdown over JSON-first *because the bootstrap's human surface (the operator reviewing reports in PRs) is now* — a reason that binds reports more than a factory-read feedback ledger — source: brainstorm:reviewer-artifacts-tech-design; `docs/tech-design/prd-review-artifacts.md`
- **F-3**: The PRD ledger already demonstrates "human markdown + folded machine projection (`effective.source.json`)" — a hybrid storage precedent — source: `docs/tech-design/prd-amendment-model.md`
- **F-4**: `Learned-from`'s `review-thread` class is defined in the merged amendment model; resolving it to a ledger anchor changes its semantics only by supersession — source: `docs/tech-design/prd-amendment-model.md`; brainstorm:review-feedback-capture#P-2
- **F-5**: AP-27 names a shared append point a design smell for parallel authors — a single global ledger file is a hot shared center; per-subject granularity makes each capture pure addition — source: `docs/architecture-principles.md` AP-27/AP-23
- **F-6**: docgate's `validate-amendments` checks the `Learned-from` *class* against the closed vocabulary (`review-thread` is valid) but leaves the *ref* free-form (non-empty only) — a `review-thread:` ref pointing at a `review-feedback:` anchor needs no model change, but nothing today enforces it resolves — source: `internal/docgate/fold/fold.go` (Learned-from validation)

## Fog

none

## Out of scope

- **O-1**: the all-proposals review-rejection rate (rejected ÷ all reviewed) — this design provides only the ledger-native numerator signals; computing the rate is the optimizer's (NG-1), needing the external all-reviewed denominator (from D-5)

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-13 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 D-1·D-2·D-3 · §4 ALT-1..9 · Serves/Scope S-1..4 + Basis · §3.2 O-1), zero blocking fog |
| **Amend route** | — |
