# Context pack — the shared pipeline-context compiler — PRD

| Field | Value |
|---|---|
| **In one line** | A compiler that assembles the stable governing context a pipeline stage needs — the vision and principles, the frozen subject, the stage's calibration — into one deterministic, cache-friendly, authoring-trail-free artifact, so every authoring and review session reads it once instead of re-crawling the corpus. |
| **Relevant-when** | planning, building, or reviewing the context-pack compiler, its input-class declaration, its freshness or determinism guarantees, or the independence boundary that keeps a cold reviewer cold; triaging a session that read stale or leaked context, a non-deterministic pack, or a pack that missed a governing input; changing the pipeline stages, the governing-doc set, the calibration files, or how sessions are handed their reading. |
| **Status** | Draft → Problem review → Solution review → Launch review → Launched |
| **Tier** | T2 — internal tooling over repo documents; reversible (the artifact is regenerable scratch); no factory-control, security, money, or migration surface. One property carries high risk within the tier: the independence boundary (§5.1 FR-4). |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi — via the introducing PR's operator review (solo operator; one real gate). |
| **Contributors** | @mdarabi (product / engineering); the orchestrating session (synthesis). |
| **Informed** | future repo contributors and every pipeline drive that consumes the pack. |
| **Date** | 2026-07-14 created · 2026-07-14 last updated |
| **Appetite** | Bounded increment: the compiler plus its input-class declaration and freshness guarantee, proven on one representative multi-session stage. Human attention: this PRD's review, the design review, and the operator's read of the before/after evidence. Machine work: the build (a docgate-family verb) and the measurement run. Excluded: model routing, review-tiering policy, any change to what reviewers conclude, and a token-telemetry plane — the measurement rides ordinary session accounting. |
| **Target release** | No calendar target. Usable once one pipeline stage runs through the pack with the token reduction recorded. |
| **Tracking** | — *(the introducing PR / epic, once filed)* |
| **Artifacts** | The session-cost observation behind §2 (the stage just run: one authoring session, three reviewers × two rounds, two disposition sessions, ≈2M tokens) — recorded in this PR's discussion, not a separate corpus. |
| **Template version** | prd-template/v1 |

## 1. Summary — the one-pager

- **Problem** — Every pipeline session — each authoring pass, each independent reviewer, each disposition — starts cold and re-reads the same stable governing material (the vision, the principles, the frozen subject, the stage's calibration) from scratch before it can do any work. On a multi-session stage that re-orientation is paid N times over, and it dominates the token cost of running the factory.
- **For whom** — The operator running pipeline drives under a token budget, and every authoring or review session the pipeline spawns.
- **Why now** — The pipeline has proven it produces correct work through many cold sessions per stage (the independence is load-bearing — it is what catches the holes an author misses). That same coldness re-pays the corpus every session; with drives now routine, the re-orientation cost is the dominant, and most compressible, line item.
- **Direction** — Compile the stable, shared context for a given stage and subject **once**, into a single deterministic artifact a session reads in one pass. Determinism makes the artifact a byte-identical prefix across the sessions of a stage, so a provider-side prompt cache amortizes even the one read across them. A closed boundary keeps the artifact free of the authoring trail, so a reviewer fed the pack stays exactly as independent as one that crawled the sources itself.
- **Appetite** — The compiler + input-class declaration + freshness guarantee, proven on one stage; as in the header.
- **Success** — Governing-context input tokens per pipeline session drop by at least 60% across a representative multi-session stage, versus the same stage run without the pack, measured from session accounting.
- **Guardrail** — Reviewer independence is preserved: a cold reviewer handed the pack reaches the same class of findings as one reading the sources directly, and no authoring-trail content ever appears in a pack (zero tolerance).
- **Top risks** — (1) the pack drifts stale from its sources and a session rules on outdated governing content; (2) an independence leak — authoring-trail material reaches a cold reviewer through the pack; (3) the caching assumption does not hold (non-identical prefixes or missed TTL) and the saving is smaller than projected.

## 2. Problem space

### 2.1 Problem statement

A pipeline stage is run by several independent sessions — an authoring pass, one or more cold reviewers, disposition passes — and each must be cold to do its job honestly (a reviewer that has seen the author's reasoning cannot independently find its flaws). Coldness has a cost: before any session can act, it re-reads the same stable corpus — the governing documents, the frozen subject under work, the stage's calibration rules. That reading is identical across the sessions of a stage and largely identical across stages, yet it is paid fresh every time. The result is that the majority of a stage's token spend buys re-orientation, not judgment, and it scales with the number of sessions rather than with the difficulty of the work. The cost of doing nothing: every drive is several times more expensive than the judgment in it warrants, and the operator's budget caps throughput well below what the pipeline could otherwise deliver.

### 2.2 Users and segments

| Segment | Role and context | Domain exp. | Tech exp. | Key goals | Pain today | Evidence |
|---|---|---|---|---|---|---|
| **P1 (primary)** | The operator running drives against a token budget | master | master | more pipeline throughput per token; predictable per-stage cost | re-orientation dominates spend; a routine stage costs millions of tokens | the stage just run (§ header Artifacts) |
| **P2** | Each authoring / review / disposition session the pipeline spawns | — | — | reach working context in one read, not a corpus crawl | every session re-crawls the same files before it can start | the drive prompts to date (each hands a session a file list to read) |
| **Anti-segment** | A session that legitimately needs the authoring trail (a disposition pass reading its own review findings) | — | — | — | — | served by the existing per-session prompt, not by a cold pack — the pack is for the shared, stable context only |

### 2.3 Jobs and desired outcomes

| # | When… | I want to… | So I can… | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | I spawn several sessions for one stage | hand each the same governing context without each re-reading the corpus | pay for the shared context roughly once per stage, not once per session | each prompt lists the files to read; each session reads them cold | the drive prompts |
| J2 | I hand a cold reviewer its subject | give it the governing context and the subject without leaking the author's reasoning | keep the review independent while still cutting its reading cost | reviewers crawl source files; independence is preserved by prompt discipline alone | round-2 reviews caught real holes — independence must survive any change |
| J3 | A governing document or calibration file changes | be sure every session reads the current version, never a stale snapshot | trust that a pack never rules on outdated context | sessions read the live files, so freshness is automatic today | the append-only governing docs |

### 2.4 Evidence

| Claim | Source | Confidence |
|---|---|---|
| A single stage costs on the order of 2M tokens, dominated by many cold sessions | the tech-design stage just run: author + 3 reviewers × 2 rounds + 2 disposition passes | High |
| Independence comes from withholding the authoring trail, not from re-reading the corpus | the round-2 merge-lane reviewer caught FND-12/13 having read only its subject + calibration cold | High |
| The stable governing corpus is re-read near-identically by every session of a stage | the drive prompts each instruct a session to read the same governing set | High |
| Byte-identical context prefixes across concurrent sessions are eligible for provider-side prompt caching | provider prompt-cache behavior (identical prefix within TTL) | Medium |

### 2.5 The journey today

| Phase | User does | User feels / pain | Frontstage | Backstage |
|---|---|---|---|---|
| Spawn | writes a prompt listing the governing files + subject | repetitive; the same list every time | the session prompt | the corpus on disk |
| Orient | each session reads the corpus cold | slow, expensive, paid N times | session reads | vision, principles, subject, calibration |
| Work | the session finally does its judgment | the only part worth paying for | the ruling / finding | — |

### 2.6 Opportunity and why now

- **Size** — every stage × every session in it × the corpus each re-reads; the compressible fraction is most of a stage's spend.
- **Alternatives users have** — keep crawling (today); hand-paste a digest per stage (manual, drift-prone, no caching guarantee); shrink the corpus (loses the context that makes judgment good).
- **Why us / strategic fit** — pyxis already treats context as artifacts on disk; compiling a scoped view of them is native, and it strengthens the token-consciousness the product holds as a standing constraint.
- **Why now** — drives are routine and the review fan-out is proven necessary; re-orientation is now the dominant recurring cost.
- **Cost of doing nothing** — throughput stays capped by re-orientation, not by judgment; the budget buys re-reading.

**Decision:** Problem gate — via the introducing PR's operator review.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | *(pending)* | *(pending)* |

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | compiling the shared governing context once per stage | most of a session's re-orientation cost disappears, at no loss of the context it needs | M1 |
| G2 | keeping the pack free of the authoring trail | cold sessions stay as independent as when they crawled sources | M2 |
| G3 | making the pack's bytes deterministic | concurrent sessions share a cacheable prefix, amortizing even the single read | M3 |

### 3.2 Goals → signals → metrics

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M1 (primary) — governing-context input tokens per pipeline session | G1 | tokens a session spends loading shared governing context | the corpus re-read in full per session (measured on a reference stage) | ≥ 60% reduction across the reference stage, first stage run through the pack | EVT-1 |
| M2 — independence preservation | G2 | the class of findings a cold reviewer reaches, pack vs source-crawl, on a fixed subject | — | same finding class; zero authoring-trail content present in any pack (zero tolerance) | EVT-2 |
| M3 — cache-eligible share | G3 | share of a stage's sessions whose pack prefix is byte-identical | — | 100% of a stage's sessions receive byte-identical pack bytes for the same (stage, subject) | EVT-3 |

### 3.3 Guardrails and decision rule

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Authoring-trail content in a pack | zero — zero tolerance | block the feature; the pack is not shippable until the exclusion holds |
| Finding-class regression for a cold reviewer on the pack vs source-crawl | any regression | do not route reviewers through the pack until closed |
| Staleness: a pack served that does not match its current sources | zero | fail closed — regenerate or refuse; never serve a stale pack |

**Ship rule (pre-registered):** M1 ≥ target AND M2 clean (same finding class, zero leak) AND no staleness breach → adopt; M1 below target with M2/M3 clean → keep for cost but re-tune; any M2 breach → do not ship the reviewer path.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Choosing which model runs a session (model routing) | orchestration/harness policy; owned by the model-routing effort, not this machinery |
| NG-2 | Deciding when to run a cold reviewer vs a human review (review tiering) | an operating convention between operator and orchestrator, not a repo compiler |
| NG-3 | Deciding whether a session runs inline or as a subagent | harness behavior, upstream of what context a session reads |
| NG-4 | Changing calibration content or what a reviewer concludes | the pack transports calibration verbatim; it never edits or summarizes judgment inputs |
| NG-5 | A token-telemetry / observability plane | measurement rides ordinary session accounting; a metrics surface is separate future work |
| NG-6 | Compressing or summarizing governing documents to save tokens | summarization loses the context that makes judgment good; the pack selects and concatenates, never rewrites |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | The drive guide and skills referencing the pack as their standing input | after the compiler ships and the reference stage proves the saving |
| WR-2 | Broadening the declared input set per stage as new stages ship | as pipeline stages are added |
| WR-3 | A cost line in the delivery ledger fed by M1 | when a token-accounting convention lands |

## 4. Solution space

### 4.1 Approach and alternatives considered

The compiler takes a **stage** and a **subject** and emits one artifact holding that stage's declared **stable context classes** — the governing documents, the frozen subject under work, and the stage's calibration — selected and concatenated verbatim, in a fixed order, with a manifest header naming exactly what it contains and the source revision it was built from. It writes nothing from the **authoring trail** (the delivery journal, prior dispositions, the orchestration). Its output is a pure function of its declared inputs at a repo revision: identical inputs produce identical bytes. It is regenerable scratch, never committed.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | a deterministic compiler that selects declared stable-context classes into one manifest-headed artifact per (stage, subject) | — |
| B | hand-assembled digest per stage | manual, drift-prone, no determinism or freshness guarantee, no cache eligibility |
| C | shrink or summarize the corpus so every read is cheaper | loses judgment-bearing context (NG-6); degrades the work the pipeline exists to do |
| Do nothing | keep crawling per session | §2.6's cost — re-orientation stays the dominant spend |

### 4.2 Scope perimeter

- **In (plan of record)** — the compiler; a per-stage **declaration** of which stable context classes belong in that stage's pack (as data, not hardcoded); the closed exclusion of authoring-trail classes; the deterministic, manifest-headed artifact; a freshness guarantee (a served pack matches its current sources or is refused); the measurement of M1 on a reference stage.
- **Future (believed next, not committed)** — WR-1..WR-3.
- **Out** — NG-1..NG-6.

### 4.3 Experience: flows and states

*The compiler's surface is a command plus a produced artifact; states are the artifact-visible conditions. Accessibility/localization: n/a — no graphical surface.*

**Flow F1 — compile and hand off (serves J1, J2)**

| Step | Actor does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | requests a pack for (stage, subject) | selects the stage's declared stable classes, verifies each source resolves, emits the manifest-headed artifact | ideal · error (a declared source missing → refuse, name it) | the input-class declaration |
| 2 | hands the artifact path to each session of the stage | each session reads one file to reach full governing context | ideal · partial (a stage with no calibration → header says so, no empty section) | the manifest header |
| 3 | a source changed since a pack was built | the stale pack is refused or regenerated; never served as-is | error (stale → fail closed) | the freshness guarantee |

**Flow F2 — the independence boundary (serves J2)**

| Step | Actor does | System responds | States covered | Design |
|---|---|---|---|---|
| 1 | requests a pack over a subject currently under review | includes the subject and calibration; **excludes** the authoring trail by construction | ideal | the closed exclusion set |
| 2 | routes a cold reviewer through the pack | the reviewer sees governing context + subject, never the author's reasoning | ideal | manifest lists contents; a reviewer can confirm no trail is present |

### 4.4 Scenarios

1. **The archetype.** A stage spawns an author and two cold reviewers. The compiler builds one pack for (that stage, that subject); all three read it in one pass; the two reviewers run concurrently and their identical pack prefix hits the cache. Governing-context tokens are paid roughly once for the stage instead of three times.
2. **The stale catch.** A principle is appended between two sessions of a stage. The second session's pack no longer matches its sources; the compiler refuses to serve the stale pack and regenerates, so the session reads the new principle — never the old one.
3. **The leak that cannot happen.** A pack is requested for a subject mid-review. Because the authoring-trail classes are outside the compiler's declared input set, no journal entry or prior disposition can enter the pack; the cold reviewer stays cold by construction, not by remembering to exclude.

**Decision:** Solution gate — via the introducing PR's operator review. Zero TBDs may pass.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | *(pending)* | *(pending)* |

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When the compiler is invoked for a stage and a subject, it shall emit one artifact containing that stage's declared stable-context classes, in a fixed order, each source's content included verbatim. | one read replaces the corpus crawl (J1) | Must | med | for a reference stage the artifact contains exactly the declared classes' current content, in the declared order, byte-for-byte from source | Test | G1, J1, F1 |
| FR-2 | The compiler shall carry a manifest header naming each included source and the repo revision the artifact was built from. | a reader (and a reviewer) can see exactly what it holds | Must | low | every included source and the build revision appear in the header; nothing unlisted appears in the body | Inspection | J2, F1 |
| FR-3 | Which stable-context classes belong to a stage's pack shall be declared as data, not fixed in code. | coverage extends as stages ship, without a code change (WR-2) | Must | low | changing the declaration changes a stage's pack contents with no code edit | Test | WR-2 |
| FR-4 | The compiler shall include only classes in its declared stable-context set, and shall never include any authoring-trail class (the delivery journal, prior dispositions, orchestration notes). | the independence boundary — a cold reviewer must stay cold (J2) | Must | high | over any subject, no authoring-trail content appears in the produced pack; the exclusion is a closed set an out-of-set class cannot pass | Test | G2, J2, F2 |
| FR-5 | Given identical declared inputs at a repo revision, the compiler shall produce byte-identical output. | determinism makes the pack a cacheable shared prefix (J1) | Must | med | two builds of the same (stage, subject) at the same revision are byte-identical | Test | G3, J1 |
| FR-6 | When a served pack does not match the current content of its declared sources, the compiler shall refuse it or regenerate it, and shall never serve a stale pack. | freshness the crawl gives for free must not be lost (J3) | Must | high | a source changed after a build causes the stale pack to be refused or rebuilt; a session never reads outdated governing content | Test | G1, J3, F1 |
| FR-7 | Where a stage declares no source for a class, the compiler shall record the absence in the manifest rather than emit an empty or fabricated section. | honest partial state; no invented context | Should | low | a stage lacking calibration yields a header noting the absence and no empty body section | Inspection | F1 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | The existing docgate fold/gate verbs and their outputs | the compiler adds a capability; it must not alter existing doc-system behavior | med | the docgate gate and fold produce identical results before and after this feature | Test | §2 |
| NR-2 | Sessions that legitimately need the authoring trail (e.g. a disposition pass) keep receiving it through their own prompt | the pack is additive; it does not remove any existing context path | med | a disposition session still reaches its review findings; the pack does not gate that path | Demo | 2.2 anti-segment |
| NR-3 | The governing documents and calibration files themselves | the compiler reads them; it must never write, reorder, or mutate a source | high | after any compile, all source files are byte-unchanged | Test | NG-4, NG-6 |

### 5.2 Unwanted behavior and failure handling

| ID | If… | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If a declared source does not resolve | refuse to emit a partial pack and name the missing source | an error naming the source | Must | med | a missing declared source produces no artifact and a named error | Test | FR-1 |
| UB-2 | If a subject is passed that does not exist | refuse and name the subject; emit nothing | an error naming the subject | Must | low | an unknown subject yields no artifact and a clear error | Test | FR-1 |
| UB-3 | If the declaration references an authoring-trail class | refuse the declaration as invalid (the exclusion is closed) | an error naming the disallowed class | Must | high | a declaration naming a trail class fails validation; no pack is built | Test | FR-4 |
| UB-4 | If a source changed mid-stage | rebuild or refuse per FR-6; never mix old and new across a stage's sessions | a regenerated or refused pack | Must | high | a mid-stage source change is reflected or refused, never silently stale | Test | FR-6 |

### 5.3 Permissions and abuse cases

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | Someone (or a flawed declaration) routes the author's reasoning to a cold reviewer to bias its verdict | adding an authoring-trail class to a stage's declaration | a reviewer sees the answer key; independence — the pipeline's trust basis — collapses | high | FR-4 + UB-3 (closed exclusion set, validated) | Test |
| ABUSE-2 | A stale pack is served so a session rules on superseded governing content | serving a pack built before a governing change | a ruling made against outdated principles/subject | med | FR-6 (freshness, fail-closed) | Test |

### 5.4 Data

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| context pack | the compiled per-(stage, subject) artifact | derived from the repo docs; never authoritative | regenerable scratch, gitignored | pure function of declared inputs at a revision; refused if stale | internal |
| stage input-class declaration | which stable classes a stage's pack holds | the repo (config data) | repo history | closed set; no authoring-trail class admissible | internal |

- **Migration** — none: new artifact.
- **Parallel-run / cutover** — none: additive; existing per-session reading is untouched until sessions opt into the pack.
- **Rollback** — stop invoking the compiler; sessions read sources directly as today. No state to unwind (the pack is scratch).
- **Decommission** — delete the scratch; sources are untouched.
- **Egress** — the pack's content is the same repo content already sent to the model provider by every session today; no new destination or class. The pack changes *how often* content is sent (once per stage vs once per session), never *what leaves* or *where*.

### 5.6 Non-functional requirements

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Cost | The pack shall reduce governing-context input tokens per session across a reference stage. | ≥ 60% reduction vs the no-pack baseline on the reference stage (M1) | Analysis |
| NFR-2 | Determinism | The compiler shall be a pure function of its declared inputs at a revision. | byte-identical output for identical inputs (FR-5) | Test |
| NFR-3 | Operability | A reader shall be able to tell from the artifact alone what it contains and how current it is. | the manifest header names every source and the build revision (FR-2) | Inspection |
| NFR-4 | Maintainability | Adding a stage or changing a stage's classes shall be a data change, not a code change. | a declaration edit changes pack contents with no code edit (FR-3) | Inspection |
| NFR-5 | Security | The compiler shall not read or emit any file outside its declared stable-context set. | over any input, no out-of-set file is read or included (FR-4) | Test |

### 5.7 Instrumentation — the tracking plan

*Events are observed from ordinary session accounting and artifact inspection; no telemetry plane (NG-5).*

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | pack_read | a session loads a pack instead of crawling | stage, subject, pack token size, sessions in the stage | session accounting | M1 |
| EVT-2 | independence_check | a cold reviewer runs on a pack vs a source-crawl on a fixed subject | finding class each way; authoring-trail bytes present (must be 0) | the two review runs | M2 |
| EVT-3 | pack_prefix | a stage's sessions receive their packs | byte-identical across the stage's sessions? (bool) | the produced artifacts | M3 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must FRs pass their fit criteria | FR-1..6 | Test | CI run + artifact inspection | yes |
| DOD-2 | No non-regression row broken | NR-1..3 | Test | docgate gate/fold identical; sources byte-unchanged | yes |
| DOD-3 | The independence boundary holds: zero authoring-trail content in any pack, over the abuse vectors | FR-4, UB-3, ABUSE-1, §3.3 | Test | the exclusion test + a leak-attempt fixture, red-first then green | yes |
| DOD-4 | Freshness holds: no stale pack is ever served | FR-6, UB-4, ABUSE-2, §3.3 | Test | a mid-stage source-change fixture | yes |
| DOD-5 | The reference stage shows M1 ≥ target with M2 clean | §3.2 | Analysis | before/after token accounting + the independence check | yes |
| DOD-6 | Determinism proven | FR-5, NFR-2 | Test | byte-identical rebuild | yes |

## 6. Delivery

### 6.1 Phasing and milestones

| Phase | Purpose | Scope | Exit criterion |
|---|---|---|---|
| 1 — build | the compiler exists | compiler + input-class declaration + exclusion + freshness + determinism | DOD-1..4, DOD-6 green |
| 2 — prove | the saving is real and independence survives | one reference stage run through the pack | DOD-5 green (M1 ≥ 60%, M2 clean) |
| 3 — adopt | drives use it by default | the drive guide / skills reference the pack (WR-1) | a stage runs unattended through the pack at the recorded saving |

### 6.2 Dependencies

| Dependency | Type | Needed by | Status | Owner |
|---|---|---|---|---|
| docgate verb-set (the compiler is a sibling verb) | system | Phase 1 | confirmed — exists | repo |
| the governing docs, frozen subjects, and calibration files as stable sources | system | Phase 1 | confirmed — exist | repo |
| a provider-side prompt cache (for the M3 amortization) | vendor | Phase 2 | confirmed — available | provider |

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | independence leak — a trail class reaches a cold reviewer (obituary #2) | L | H | FR-4 closed exclusion + UB-3 + ABUSE-1 test, red-first | @mdarabi |
| R2 | stale pack served — a session rules on outdated context (obituary #1) | M | M | FR-6 fail-closed freshness + UB-4 fixture | @mdarabi |
| R3 | the cache assumption underdelivers (non-identical prefixes / TTL misses) — saving < projected (obituary #3) | M | M | FR-5 determinism; M3 measures the actual identical-prefix share; M1 still counts the single-read saving even at zero cache | @mdarabi |
| A1 | session token accounting is a faithful enough measure of M1 without a telemetry plane | H (assumed) | M | the reference-stage before/after uses the same accounting both ways, so the delta is sound even if absolute counts are approximate | @mdarabi |

### 6.4 Release criteria

| Lens | Criterion | Evaluated by |
|---|---|---|
| Functionality | all Must FRs pass | CI |
| Reliability | independence + freshness fixtures pass (DOD-3, DOD-4) | failure/leak injection |
| Performance | M1 ≥ 60% on the reference stage | token accounting |
| Supportability | the manifest header makes any pack self-describing | inspection |

### 6.5 Rollout and reversibility

- **Flag** — adoption is opt-in per session: a session either reads the pack or crawls sources; nothing forces the pack until Phase 3.
- **Kill switch** — stop invoking the compiler; sessions crawl sources exactly as today (no state to unwind — the pack is scratch).
- **Experiment** — the reference-stage before/after is the controlled comparison.
- **Cutover/migration steps** — none.

| Stage | Audience | Minimum evidence | Advance when | Roll back when |
|---|---|---|---|---|
| build | this repo | DOD-1..4, DOD-6 green | the compiler is correct and safe | any DOD-3 leak |
| reference stage | one real stage | DOD-5 (M1 ≥ target, M2 clean) | the saving is proven without losing independence | M2 breach |
| default | all drives | a stage runs through the pack unattended at the saving | operator adopts | a staleness or leak breach |

### 6.6 Launch readiness

| Area | Item | Owner | Status |
|---|---|---|---|
| Docs | the compiler's usage + the input-class declaration documented with the implementation | @mdarabi | ☐ |
| Support / enablement / legal / pricing / marketing / platform / localization | skipped: solo-operator open-source tooling — no support org, billing, or comms surface | — | n/a |

**Decision:** Launch gate — §6.4 criteria green, rollback (stop invoking) rehearsed.

| Reviewer | Role | Date | Verdict |
|---|---|---|---|
| @mdarabi | operator | *(pending)* | *(pending)* |

### 6.7 Post-launch review

- **Review trigger** — the first five stages run through the pack · **Owner** — @mdarabi
- **Verdict recorded** — adopt / iterate / kill, with the M1/M2/M3 evidence — on the header's Tracking ref.

## 7. Alignment

### 7.1 FAQ

**Customer**

- **Q:** Does the pack change what my reviewers conclude? — **A:** No. It transports the same governing documents, subject, and calibration verbatim; it never summarizes or edits them (NG-4, NG-6). A reviewer reads the same content, in one file instead of several.
- **Q:** How does it save tokens? — **A:** The shared context of a stage is compiled once and read once per session instead of re-crawled; because the bytes are identical across a stage's concurrent sessions, a provider cache can amortize even that single read.

**Internal**

- **Q:** Doesn't feeding a reviewer a pre-built pack risk contaminating its independence? — **A:** The opposite, if the boundary holds: the pack is a closed set of *stable* classes and structurally cannot contain the authoring trail (FR-4, UB-3). Independence is preserved by construction, not by a prompt reminder.
- **Q:** What stops a stale pack from serving old context? — **A:** FR-6: a pack that no longer matches its sources is refused or rebuilt; a stale pack is never served.
- **Q:** Why not just summarize the corpus to make every read cheap? — **A:** Summarizing loses the judgment-bearing detail the pipeline depends on (NG-6). The pack selects and concatenates; it never rewrites.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | Whether the freshness guarantee is best met by refuse-on-stale or always-regenerate (or both by context) | design | solution gate → design | open |
| Q2 | Whether the pack is one artifact or a small set (e.g. governing vs subject) while remaining a single cacheable prefix | design | design | open |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-14 | Compile a deterministic, manifest-headed pack of declared stable classes; exclude the authoring trail by construction | hand digest; summarize corpus; do nothing | determinism enables caching, the closed exclusion preserves independence, verbatim selection preserves judgment | @mdarabi |
| 2026-07-14 | Scope to the compiler; model routing, review tiering, and inline-vs-subagent are non-goals | one broad token-efficiency feature | those are orchestration/harness policy, not repo machinery; the compiler is the buildable core | @mdarabi |
| 2026-07-14 | Measure from session accounting; no telemetry plane | build a token-observability surface | the before/after delta is sound on ordinary accounting; a metrics plane is separable future work | @mdarabi |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-07-14 | 000-base drafted for operator review | @mdarabi (via the orchestrating session) |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §2 problem gate signed (T2) | ☐ | pending operator review |
| Zero solution-blocking TBDs or open questions (§7.2) | ✅ | Q1/Q2 are design-owned, not solution-blocking |
| Every Must FR has a fit criterion and a verification method (§5.1) | ✅ | FR-1..6 each carry both |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan (§5.1, §6.5) | ✅ | FR-4/FR-6/NR-3 high → the leak + staleness fixtures and the opt-in/kill-switch rollout |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ✅ | NG-1..6; NR-1..3 |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ✅ | J1→FR-1/FR-5; J2→FR-4/FR-2; J3→FR-6 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage | ✅ | F1→FR-1/2/6, UB-1/2/4, NR-1/3; F2→FR-4, UB-3, ABUSE-1 |
| Every primary metric (§3.2) has instrumentation (§5.7) or a measurement plan | ✅ | M1→EVT-1; M2→EVT-2; M3→EVT-3 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | ✅ | DOD-1..6 each cite sources |
| Risks and assumptions have owners (§6.3) | ✅ | R1..R3, A1 owned |
| Human-judgment criteria name an owner and a rubric (§5.8) | ✅ | none — all DoD rows are Test/Analysis/Inspection |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ✅ | no n/a rows |

## Superseded & retired

None — this is the base.
