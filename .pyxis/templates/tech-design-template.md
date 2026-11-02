# <Decision title> — technical design

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | <one accountable person> |
| **Approver(s)** | <who must approve before merge — a named reviewer is mandatory when Security/privacy impact is yes or the surface is data/infra/governance. Authority is the merge review record; this row names the bar> |
| **Date** | <YYYY-MM-DD> |
| **Relevant-when** | <load this document when… — designing or changing <the decided mechanism>; implementing against it; superseding it. Concrete nouns, never pronouns. e.g. "designing or changing cross-service write paths; triaging duplicate-effect defects; superseding the retry model"> |
| **Scope** | <what this decides — and, explicitly, what it does not. e.g. "retry and idempotency semantics for cross-service writes. Not: transport selection, queue technology"> |
| **Tracking** | <issue / PR link> |
| **Builds-on** | <prior design(s) this consulted or extends, or —> |
| **Supersedes** | <design slug(s) this replaces, or — · declaring it here is what retires the target in the index; the old document is never edited> |
| **Serves** | <typed anchors this design implements: `prd:<feature>/FR-3` · `corpus:VNEXT-8` · `design:<slug>` (whole design — prefer `design:<slug>#A-n` / `#DINV-n` / `#<section>` when the served mechanism is narrower) · `doc:<path#anchor>`; — only for document-system/governance/infrastructure work. e.g. `prd:payments/FR-4 · prd:payments/NR-2`> |
| **Security/privacy impact** | <none, or: yes — <summary> · reviewer: <named>. Yes is **mandatory** when this touches identity, permissions, secrets, user data, network boundaries, model/provider egress, billing, or production controls — and pairs with the mandatory `Security & privacy` subsection, which must agree with this row> |
| **Template version** | tech-design-template/v1 |

## How to use this template

*Guidance paragraphs look like this one — italic. Delete every guidance
paragraph when instantiating. The conventions this template instantiates are
`docs/tech-design/tech-design-conventions.md`; the live exemplar of the form
is `docs/tech-design/prd-amendment-model.md` (which predates this template
and is grandfathered — its header lacks the newer rows).*

*The rules that matter:*

- ***One decision per document*** *(RW-5). If you find yourself writing two
  §2s, you are writing two documents. Small decisions still get their own
  doc — the index keeps many small documents navigable; nothing keeps one
  tangled document decidable. One decision means one **independently-revisable
  question**: subordinate choices that would only ever change together ride
  inside §3; choices revisable separately get their own documents, because
  the document boundary is the supersession boundary. Requirements accumulate
  designs over time — overlapping `Serves` across documents is expected, not
  a smell.*
- ***Immutable at merge.*** *While the introducing PR is open this is a
  draft — edit freely. Once merged, it changes only by a successor document
  naming it in `Supersedes`. Do not renumber sections, ever: §3 is the one
  section that grows, by appending `3.x` subsections; §4–§7 keep their
  numbers forever.*
- ***Decisions are called out.*** *Open §2 (and any load-bearing choice in
  §3) with a `**Decision:**` paragraph — the renderer distinguishes them,
  and reviewers navigate by them. State the decision first, argue after.*
- ***Prose is not uniformly binding.*** *`**Decision:**` callouts,
  invariants, and acceptance rows **bind** implementation; alternatives,
  rationale, and risks explain — they create no obligation unless an
  acceptance row or invariant references them. Write accordingly: if a
  constraint must hold, put it in a binding element, not a paragraph.*
- ***Contradiction requires supersession.*** *If this design makes any
  binding statement of a live predecessor false, use `Supersedes` — not
  only `Builds-on`. `Builds-on` is for additive, dependent, or
  constraining decisions that remain simultaneously true with their
  target; two live designs never disagree.*
- ***The Principles section is mandatory*** *— cite every AP/RW/INV this
  design honors or deviates from, by ID, with reasoning.
  Deviation-with-reason is legitimate; silent omission is not.*
- ***Frozen prose states no volatile facts.*** *Never write a count, range,
  enumeration, or repo path a later change can falsify — name the concept
  and point at the live surface (an index, a config key, a directory) that
  owns the current value. Immutable documents cannot take corrections
  (#54 D3 found this failure four ways).*
- ***Cite requirements, not vibes.*** *`Serves` carries the anchors this
  design implements; body text cites principle and invariant IDs. A design
  that serves nothing and cites nothing is an essay. Design-local IDs
  (`A-n`, `R-n`, `Q-n`) are per-document — cite them externally as
  `<slug>#R1`; invariants carry `DINV-n`. Use `Builds-on` for dependency or reasoning lineage;
  `Serves: design:<slug>` only when this document implements support for a
  mechanism the target defines — and prefer a narrower anchor
  (`design:<slug>#A-1`) where one exists.*

## 0. Read to rule — the human reviewer's section (~N min)

*Audience: the gate-signing human reviewer — and no machine: automated
consumers (the unmanned `tech-design-review`, the stage-6 compiler, the
generated index, traceability, workers, stage flows) skip this section
entirely; it is informative, never normative, and nothing downstream may cite
into it. Write it **last**, from this design's own content — every routed item
names its body anchor (§/row id) and resolves in this document. The contract
(`read-to-rule-v1` §3.1 · design `read-to-rule-tech-design-v1`): the four
blocks below, in order, each present or asserted empty as
`none — <one-line assertion>`; at most 40 markdown source lines from the
heading through the skip map's last line; ranked calls at most 7 rows (healthy
designs typically carry 3–7), ordered by irreversibility × blast radius.
Required for every design; a design that genuinely routes nothing carries the
whole-section skip line `Skipped: <reason>` (the class has no tiers). Source
the four blocks from this design's own binding elements — ranked calls from the
`**Decision:**` callouts (§2 and load-bearing §3.x) paired with the `#ALT-n`
each displaced; numbers from acceptance/DINV bounds; open questions from §7.
Fill the reading-budget hint in the heading. §0 is refreshed only by a
superseding design (whole-document), never amended in place. Delete this
guidance paragraph when instantiating; the italic notice line below is contract
boilerplate — keep it verbatim.*

*The body is the contract; this section only routes attention — on any conflict
the body governs; machines ignore it.*

**Judge these** *(each row: the call · the alternative it displaced · what disagreeing concretely means, with its anchor)*

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | <the `**Decision:**` that could have gone the other way> | <the `#ALT-n` it displaced> | <what a veto means for the reviewer> → <§/row> |

**Sanity-check the numbers** — <the acceptance thresholds, DINV bounds, and tolerances worth a gut-check, each anchored — or: none — <assertion>>

**The author's open questions** — <the §7 `Q-n` calls still low-confidence and awaiting the reviewer, each anchored — or: none — <assertion>>

**Skip the rest** — <each content class this section omits and the mechanism that backstops it — e.g. schema/structure → docgate; substance/coherence/reuse → the unmanned `tech-design-review`; implementation defects → codegate + downstream gates; rationale/alternatives → §4 (explanatory)>

## 1. Problem

*What is broken, missing, or undecided — and why now. Name the failure mode
this design prevents; if there is a cheapest-moment argument, make it here.*

## 2. Decision

*One `**Decision:**` paragraph stating the choice, then the reasoning. A
reader who stops after §2 should know what was decided and why it is safe.
e.g. — "**Decision:** Every cross-service write carries a client-generated
idempotency key, and receivers deduplicate on it: retries are safe by
construction, and duplicate submissions collapse to one effect."*

**Decision:** <the decision, in two or three sentences>.

## 3. The design

*The substance, in `### 3.x` subsections — mechanics, schemas, boundaries,
lifecycle. This is the only section that grows: append new `3.x`
subsections; never renumber existing ones. Use ` ```mermaid ` fences for
topology and flows — the review surface draws them. Mark subordinate
choices with `**Decision:**` callouts; mark honest limits plainly (a named
debt beats a silent assumption).*

### 3.1 <first aspect — e.g. key lifecycle: generation, namespacing, expiry>

### 3.2 Boundaries and non-goals

*Nearly every design earns this subsection — it is what stops an
implementing agent from overextending the decision. Answer four prompts:*

- **Not solved here** — <e.g. cross-region key replication>
- **Owned by a future design** — <e.g. retry backoff policy — separate, independently-revisable>
- **Implementation must not infer** — <e.g. that dedupe implies ordering; keys guarantee at-most-once, never sequence>
- **Must remain unchanged** — <e.g. existing non-keyed read paths and their latencies>

### 3.3 Invariants

*The properties that must hold whatever the implementation does — often more
useful than examples, and they map directly to property tests. Invariants
are binding, so they carry stable anchors: `DINV-n` (design invariant —
`INV-n` is the domain registry and `I1…` the incident registry, both
taken), cited externally as `<slug>#DINV-1`. One that proves load-bearing
beyond this design graduates to the domain model's INV registry.*

- **DINV-1** — <e.g. a retry produces at most one durable effect>
- **DINV-2** — <e.g. a duplicate request records a no-op, never an error>
- **DINV-3** — <e.g. an idempotency key is scoped to exactly one operation class>

### 3.4 Observability

*Mandatory by name in every design (position free; AP-5 — observability is
designed in, never bolted on): how the mechanism is seen working and seen
**broken**. `none — <reason>` is a legal fill for designs with no runtime
surface (most document-system designs); anything that executes answers all
five.*

- **Events/logs** — <what is emitted, and when>
- **Metrics** — <what is measured, and the dashboard that shows it>
- **Alerts** — <what fires, on what threshold, routed to whom>
- **Debugging surface** — <how an operator or agent diagnoses misbehavior>
- **Trace/correlation** — <how actions carry trace context end-to-end>

### 3.5 Security & privacy

*Mandatory by name in every design; must **agree with the header row** — a
lintable pair: header `none` ⇒ this subsection reads `none — <reason>`;
header `yes` ⇒ the analysis lives here — surfaces touched (identity,
permissions, secrets, user data, network boundaries, model/provider
egress, billing, production controls), data handled, threat sketch, and
mitigations traced to binding elements (`A-n`, `DINV-n`).*

<none — <reason>, or the analysis>

*Add further `3.x` subsections as the design demands them — the recurring
situational concerns: **failure modes** (a `| Failure | System behavior |
Detection | Recovery |` table — operational, where risks are anticipatory);
**compatibility / migration / cutover** (can old and new coexist? what
happens mid-rollout?); **rollback / reversibility**; **performance and cost
constraints**; **testing strategy** (the seams under test — the interface
is the test surface; each dependency binned **in-process** (test directly)
· **local-substitutable** (a real stand-in, e.g. an in-memory medium) ·
**remote-but-owned** (a port with production + test adapters) ·
**true-external** (an injected mock) — the bin determines the strategy, and
one adapter means a hypothetical seam, two a real one: don't introduce a
port nothing varies across). None are mandatory boilerplate — but a concern
the design plainly touches and silently skips is review feedback waiting to
happen. (Observability and Security & privacy are absent from this menu
because they are mandatory by name, above.)*

## 4. Alternatives considered

*Every alternative a reviewer would reasonably raise, each with the honest
reason it lost — a design with one option is an announcement, not a
decision.*

| Alternative | Why rejected |
|---|---|
| <option> | <cost, risk, mismatch — honest> |
| <e.g. dedupe by payload hash, no client keys> | <e.g. distinct requests with identical payloads conflate; no client-side retry contract> |
| <e.g. at-most-once delivery, no retries> | <e.g. drops under transient failure become user-visible loss> |

## 5. Risks

*A mitigation without verification is intent — the last column answers "how
will we know the mitigation exists?"*

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | <what goes wrong — e.g. a key reused across semantically different requests> | <e.g. valid request suppressed as a duplicate> | <mechanism, not intention — e.g. keys namespaced per operation class; cross-class reuse rejected, not deduplicated> | <e.g. unit test + cross-class integration fixture> |

## 6. Acceptance

*The observable bar this design's implementation must clear — structured so
it is traceable and executable, not prose-only. `Verify` uses the PRD's
method enum — Test / Demo / Inspection / Analysis / Eval / Human judgment
(named owner + rubric) — with design-typical refinements welcome as
qualifiers: failure injection, load test, security review. Mechanically
checkable where possible; named inspection otherwise. These rows become the
implementing work's definition of done.*

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | <e.g. 1,000 duplicate submissions across concurrent clients produce exactly one effect> | Test | <e.g. CI fixture report> | <e.g. `prd:payments/FR-4` · §2> |
| A-2 | <e.g. a retried request after receiver restart still deduplicates — keys survive restart> | Test | <e.g. restart fixture result> | <e.g. `prd:payments/FR-4` · #DINV-1> |

## 7. Open questions

*What this design deliberately leaves undecided — the structure enforces the
rule: an open question with no owner and no blocking gate is a TBD in
costume.*

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | <e.g. do keys expire, and on what retention evidence?> | <who> | <implementation / launch / none> | open | <e.g. decided at implementation against the retention fixtures> |

## Principles

*One bullet per cited ID: honored or deviated, with the reasoning. End with
an explicit `**Deviation:**` line — "none" is a statement, not a default.
This section is deliberately unnumbered: it is the cross-document footer
contract, located by name in every design.*

- **AP-<n> — honored.** <how — e.g. "AP-22 — honored: re-running any write converges; the losing duplicate records a no-op, never an error">
- **Deviation:** none.
