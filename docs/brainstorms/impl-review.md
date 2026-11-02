# impl-review — brainstorm record

| Field | Value |
|---|---|
| **In one line** | The third unmanned reviewer — LLM code review of each implementation worker's proposal, run once at the proposal boundary (codegate ladder green, before PR-raise, warm worker context), judging the diff against its charter/design/verdict; findings closable, verdict downgrade-only toward the merge lane, report riding the PR as evidence. |
| **Relevant-when** | brainstorming or contracting machine code review of proposals (the proposal-class reviewer); its seam with implement-now's worker exit, codegate's verdict, stage-9's human review, or the merge lane; checking prior rulings on review placement (boundary vs in-loop vs PR-check), review depth tiering, or the WR-2 calibration corpus |
| **Idea** | "what is the best way to perform built-in code review skill for each implementation worker? should it be part of the verification pipeline after all simpler/faster tests (e.g. lint, unit tests, etc) or it should be somewhere else" · "run the brainstorm for the impl-review feature" (operator, 2026-07-16, verbatim; the placement question was explored in-session first — the boundary answer and its reasoning ride this record as the idea's pre-thinking, decisions land only from this session's rulings) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-16 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/impl-review/ |
| **Prior records** | brainstorm:authoring-reviewers · brainstorm:implementation-verification · design-brainstorm:implementation-verification-tech-design |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/brainstorms/authoring-reviewers.md (frozen) | the reviewer-program record: D-1 no-human assumption, D-2 the verdict protocol, D-3 escalation layering, D-4 **one class per feature** (this feature is the reserved third slot), D-5 the lens-pack + calibration-as-anti-nitpick shape, D-6 independent evolution |
| docs/prds/prd-review/effective.md + tech-design-review (headers/NG/WR) | the sibling contracts: NG-6 one-class-per-feature; WR-3 shared verdict grammar parked for the engine era; FR-2 floor-exclusion; the advisory→ten-review adoption ramp |
| docs/prds/implementation-verification/effective.md (authored+merged this session) | the deterministic sibling: WR-2 (judgment rungs deferred; trigger = "a judgment-check calibration corpus exists and deterministic-rung trust is established"), NFR-2 determinism, NFR-5 zero tokens — the contract that rules judgment OUT of the ladder |
| docs/tech-design/implement-now.md + codegate-v1.md (live) | the seam ground: the Worker's exit path (ladder green → PR) has no reviewer step; codegate's verdict + finding schema are the review's inputs; integration = the additive-amendment class already queued for the VerificationGate (the P-1 mechanics precedent) |
| docs/prds/vnext-requirements.md | **VNEXT-7 names the idea**: "multi-agent review for risky code (agent A implements, agent B critiques)"; VNEXT-9: risk decides whether human review is required; VNEXT-5: evidence packages (the report is a producer); VNEXT-3: traceability (charter↔diff↔finding) |
| docs/product-vision.md §4/§6 (in context) | stage 9 = the **human** implementation review; trust model: evidence packages + high-quality escalations; the AUTO lane's floor |
| docs/wiki/glossary.md + merge-lane vocabulary (in context) | AUTO/HUMAN lanes, floor-clearing, downgrade-only classifier (GATE-4 heritage) |
| docs/prds/review-feedback-capture/ + pr-size study (session memory) | the adjacent human-feedback ledger; the pain evidence: 525-PR study found zero review at all sizes |

**Overlaps surfaced:**

- `brainstorm:authoring-reviewers` — the program's protocol rulings are decided and shared by citation (D-1/D-2/D-3/D-5 invariants; D-4's one-class-per-feature reserved this slot). This session inherits, never re-decides them.
- `prd:implementation-verification` WR-2 — judgment rungs inside the ladder are a decided deferral with a named trigger; this feature is *complementary*: it produces the calibration corpus that trigger names. Not covering — WR-2 is about rungs, this is a boundary stage.
- vision §4 **stage 9** — the human implementation review. This feature runs *under* it, exactly as prd-review runs under the stage-2 human gate: the machine reviews first, the human reads findings+dispositions. Complementary, never a substitute (the sibling programs' O-rulings: verdict-substitutes-the-gate parks for the autonomy-policy era).
- `corpus:VNEXT-7` — "multi-agent review for risky code (agent A implements, agent B critiques)" is this feature's corpus anchor; VNEXT-9's "risk affects whether human review is required" is the tiering hook.
- `prd:implement-now` (merged) — the host whose worker-exit path gains the invocation; integration lands as an additive amendment there (the same queue as codegate's VerificationGate seam — one integration wave, two consumers).
- merge-lane / GATE-4 — the downgrade-only relation is precedented: the lane's LLM classifier may only force HUMAN; this feature's verdict gets the same one-way authority.
- `prd:review-feedback-capture` (merged) — captures *human* review feedback; adjacent: impl-review's dispositions/overlays are machine-side data that could feed the same ledger. Boundary to keep crisp.
- `/code-review` (session tooling, not a pyxis feature) — the interim manual form at the same boundary; this feature is its governed, unmanned, factory-integrated replacement.

No source covers the idea; the proposal-class reviewer slot is open by explicit prior ruling.

## Framings

- **The third unmanned reviewer (the program instance)** — the feature is prd-review/tech-design-review's sibling for the **proposal class**: the proven verdict protocol (independent lenses in fresh contexts, closable findings with calibration rows, two-round scope-locked cap, dispositions bind, declined blockers escalate, report rides the PR) instantiated over a new subject — the diff + its charter + codegate's verdict. Thesis: *the protocol is proven twice; the third class inherits it by citation and varies only its lens pack and subject.* Opens: the WR-2 calibration corpus; a stage-9 read-to-rule surface later; the remote-check twin at WR-5.
- **The judgment half of the proposal gate** — the feature is the completion of stage-8 verification: the checks the deterministic ladder structurally cannot do (spec fidelity vs charter, design conformance, security/abuse lenses, code quality against repo conventions), packaged as the worker's exit step; the reviewer protocol is the mechanism, not the identity. Thesis: *codegate proves the code runs; this proves it's the right code — one gate, two halves.* Opens: charter↔diff traceability (VNEXT-3) as the review's spine; risk-tiered depth as a first-class dial.
- **The stage-9 attention compressor** — the feature is making the human implementation review cheap: the machine review's *product* is the ranked judgment-call surface the stage-9 human reads (read-to-rule for proposals); findings and dispositions are means. Thesis: *human attention is the scarce resource; the reviewer exists to spend less of it per proposal.* Opens: the gate-attention lineage applied to code; capture-line metrics for stage 9.

**Destination:** **the third unmanned reviewer** — the proposal-class instance of the reviewer program: the proven verdict protocol inherited by citation (fresh-context lenses, closable findings with calibration rows, two-round scope-locked cap, binding dispositions, escalation briefs), instantiated over a new subject — the diff + task charter + codegate verdict — at the **stage-8 tail** (proposal boundary, warm worker context, before PR-raise), feeding **stage 9** a pre-reviewed proposal whose report rides the PR as evidence. Operator's rationale: placement confirmed as stage-8-resident/stage-9-feeding (t2 clarification); the program slot was reserved by prior ruling and the protocol varies only its subject and lens pack.

## Decisions

### D-1 — the identity: the third unmanned reviewer, at the stage-8 tail

- **Question**: which framing — the program instance, the judgment half of the proposal gate, or the stage-9 attention compressor?
- **Answer**: the **program instance** — prd-review/tech-design-review's sibling for the proposal class; runs once at the proposal boundary (stage-8 tail, warm worker context), feeds stage 9; verdict downgrade-only toward the merge lane; advisory before blocking. The gate-completion framing's traceability spine and the compressor's stage-9 surface arrive as consequences, not identities.
- **Reasoning**: authoring-reviewers#D-4 reserved the slot; the protocol is proven twice — this session decides only class-specific ground (lens pack, subject inputs, tiering, seam, ramp); the findings corpus is codegate WR-2's stated trigger. Placement clarified with the operator at t2: stage 8 owns it, stage 9 consumes it. Recommendation adopted.
- **Anchor**: 2026-07-16 · t1–t2
- **Status**: live

### D-2 — the lens pack: four code-shaped lenses, floor-excluded, calibration-dialed

- **Question**: what does impl-review judge a proposal against — four code-shaped lenses, a lean pair, or four-plus-style?
- **Answer**: **four lenses** — (1) spec fidelity (charter↔diff: every AC addressed, nothing material beyond scope; the independent-reader move), (2) design & convention conformance (the governing design's binding elements via Traces; AGENTS.md/Go rules; duplication vs existing helpers), (3) security & abuse (risk-tier lens: touched-surface patterns + the served PRD's ABUSE rows re-checked against the diff), (4) correctness critic (edge/error cases, concurrency, leaks, would-stage-9-bounce). **Floor exclusion**: nothing codegate enforces is ever a finding. Calibration as the anti-nitpick dial: fidelity+conformance default blocking, security blocking on tier, critic advisory unless load-bearing. **Style/readability is not a lens** (the program's refusal stands; stated conventions ride lens 2).
- **Reasoning**: mirrors the proven sibling pack shape with code-class substance; each lens maps to a failure class the deterministic ladder structurally cannot see; the lean pair starves security/design coverage and the WR-2 corpus; the style lens re-opens a paid-for refusal. Recommendation adopted.
- **Anchor**: 2026-07-16 · t3
- **Status**: live

### D-3 — authority: downgrade-only forever; advisory ramp to a ten-review checkpoint

- **Question**: what authority does the verdict carry, and how does it ramp — downgrade-only with the sibling advisory ramp, or blocking from day one?
- **Answer**: **downgrade-only, permanently** — the verdict can force HUMAN or send the proposal back to the worker; it never grants AUTO and never substitutes for stage 9 (that substitution stays parked for the autonomy-policy era, program-wide). **Advisory first**: findings accumulate as measurement with dispositions recorded; adoption (verdicts gating PR-raise) at a ten-review checkpoint on measured precision. During the advisory period the worker still applies blocking-class findings in the warm context — advisory bounds the verdict's authority, not the findings' use.
- **Reasoning**: both halves are frozen house patterns (GATE-4's one-way classifier; prd-review's ramp); an uncalibrated judgment gate blocking the hottest path is the red-blindness recipe, and stage 9 reviews everything meanwhile so blocking-now buys little. Recommendation adopted.
- **Anchor**: 2026-07-16 · t4
- **Status**: live

### D-4 — depth: the merge lane's risk classes, three bands, skips recorded

- **Question**: how does review depth scale with proposal risk — reuse the merge lane's classes in three bands, or uniform depth?
- **Answer**: **reuse the lane's risk classification** (one classifier, versioned policy data — never a second review-side model) with three bands: **skip** (mechanical/docs-only/rename-only; the report records `skipped:<class>` — no silent caps), **standard** (the four-lens pass, two-round cap), **deep** (high/critical surfaces: lenses run with the incident registry + served ABUSE rows as mandatory context; the critic's calibration escalates).
- **Reasoning**: depth-as-data mirrors the factory; the skip band keeps cost proportional to risk rather than proposal count; AP-11 — one risk predicate tunes both routing and review. Uniform depth pays full fare on renames and erodes disposition discipline with noise. Recommendation adopted.
- **Anchor**: 2026-07-16 · t5
- **Status**: live

### D-5 — segments: the stage-9 human P1, the worker P2

- **Question**: who is this for — human-gate-first or worker-first segmentation?
- **Answer**: **P1 = the stage-9 human (operator)** — the gate this feeds; today's reality is raw diffs with zero machine review at any size; their dispositions are the calibration ground truth. **P2 = the implementing worker/host** — consumes compact closable findings warm, before PR-raise. **Anti-segments**: human-authored/bypass PRs (the v1 form is charter-fed; the charter-free remote twin is WR-5-era); repos without calibration data (advisory-only there).
- **Reasoning**: deliberately inverse of codegate's machine-P1 — that loop runs hundreds of times and needed efficiency to win trade-offs unambiguously; the reviewer runs once per proposal and its trade-offs are precision-vs-noise, adjudicated by the human whose dispositions tune it. Worker-first optimizes fix ergonomics over precision — the nit engine's road. Recommendation adopted.
- **Anchor**: 2026-07-16 · t6
- **Status**: live

### D-6 — the four jobs: substance-not-diffs · risk-always-scrutinized · warm bounce-learning · verdicts-route-risk

- **Question**: are these the jobs — two operator, one worker, one factory?
- **Answer**: confirmed. J1 (P1): findings + dispositions + verdict instead of a raw diff at the gate (today: zero machine review at any size). J2 (P1): risky surfaces always get the security/design lenses with ABUSE rows + incident registry in context (today: only if the human happens to look). J3 (P2): the worker learns what would bounce while warm, fixes at the cheapest moment, arrives pre-reviewed (today: cold bounce at stage 9, a re-claim). J4 (factory): the verdict forces the HUMAN lane mechanically when risk demands it (today: the lane reads only the deterministic floor).
- **Reasoning**: each traces to a live ruling and named pain; J3 anchors the boundary placement, J4 anchors downgrade-only — trimming either orphans a ruling. Recommendation adopted.
- **Anchor**: 2026-07-16 · t7
- **Status**: live

### D-7 — the bet and its observables: coverage + precision + catch + minutes; nit rate as the guardrail

- **Question**: what is the bet, its observable proof, and the guardrail?
- **Answer**: bet — a proposal-boundary reviewer calibrated against the operator's own dispositions ends zero-review debt: stage 9 judges substance in minutes, risky surfaces always get scrutiny, and the findings corpus becomes WR-2's calibration ground. Observables: (1) **coverage** — 100% of factory proposals carry a report or a recorded `skipped:<class>`; (2) **precision** — operator dispositions accept ≥ a threshold of blocking-class findings at the ten-review checkpoint (the adoption gate's own metric; number fixed in the PRD); (3) **the catch test** — seeded spec-infidelity, DINV-violation, and ABUSE-row defects caught in evals; (4) **stage-9 minutes** — capture-line time on reviewed proposals, directional vs unreviewed baseline. Guardrail: **nit rate bounded** — rejected dispositions per review ≤ threshold; breach pauses adoption progress (stays advisory), never silently retunes calibration.
- **Reasoning**: each observable maps to a job; precision-vs-nit is the pair that makes a judgment gate trustable; without disposition-measured precision the D-3 ramp has no principled gate. Recommendation adopted.
- **Anchor**: 2026-07-16 · t8
- **Status**: live

### D-8 — appetite: reviewer + seam, bounded by the codegate dogfood

- **Question**: what bounds the build — reviewer + seam dogfood-bound, or both surfaces including the remote twin?
- **Answer**: **reviewer + seam, dogfood-bound**. In: the `/impl-review` skill (sibling shape, four lenses, fresh contexts); the report template + artifacts contract (`impl-review-report/v1`, riding the PR); calibration as data + depth bands read from the lane's risk classes; the catch evals + disposition/precision measurement; the worker-seam invocation landing in the **same additive-amendment wave on prd:implement-now already queued for the VerificationGate** (one integration, two consumers); dogfood = the codegate epic's own proposals (T1–T11 when implementation resumes). Out: the WR-5 remote twin; verdict-substitutes-stage-9; self-tuning; the stage-9 presentation surface; non-code classes. Bound: advisory reviews end-to-end on real factory proposals with measured dispositions — the ten-review corpus started, not finished.
- **Reasoning**: the remote twin would ship untested against its real subject and drags WR-5's whole deferred complex in; the phase alignment with codegate keeps one integration wave. The verifier's build becoming the reviewer's first workload closes the session's loop a second time. Recommendation adopted.
- **Anchor**: 2026-07-16 · t9
- **Status**: live

### D-9 — findings are closed-schema data, PR-resident, and a durable corpus

- **Question**: (operator ruling at the gate, t10) what form do findings take and where do they live?
- **Answer**: three rulings, operator's words: (1) **schema-based output, never free-form** — a closed, versioned finding schema (severity · lens · location file:line · explanation · acceptance condition · disposition) "so that schema can be reused for analysis later" and "helps the system narrow down where to find the issue and fix it"; (2) **stored into the PR itself** — medium-appropriate (GitHub: a PR comment carrying the human rendering + a machine payload block, the spec-marker pattern; Carina: a typed decision-record) — "we have to preserve that info"; (3) **never thrown away** — unlike disposable implementation context, findings + dispositions are the durable corpus that measures and improves the review process (precision metrics, calibration tuning, the WR-2 trigger). Kinship: a shared finding-grammar core with codegate's finding schema + review-only fields, one grammar family for the host's fixer.
- **Reasoning**: operator's — the findings' analytical value outlives the proposal; the schema is what makes measurement and mechanization possible. Composes with D-7's observables (precision/nit metrics read this data) and resolves G-1's direction (the data home is the proposal).
- **Anchor**: 2026-07-16 · t10
- **Status**: live

## Facts

- **F-1**: the corpus names the idea: VNEXT-7 requires "multi-agent review for risky code (agent A implements, agent B critiques)"; VNEXT-9 makes human-review-required a risk-driven decision — source: docs/prds/vnext-requirements.md
- **F-2**: the reviewer program decided its invariants once, program-wide: no-human-assumption, verdict protocol (closable findings, calibration citations, two-round cap, escalate = brief + fail-closed park), one class per feature, independent evolution mirrored by pattern — source: brainstorm:authoring-reviewers#D-1..D-6; prd:prd-review NG-6/WR-3
- **F-3**: judgment inside codegate's ladder is contractually ruled out for v1 — WR-2 defers judgment rungs with the trigger "a judgment-check calibration corpus exists and deterministic-rung trust is established"; NFR-2 (determinism) and NFR-5 (zero tokens) are Must rows a review rung would break — source: docs/prds/implementation-verification/effective.md
- **F-4**: implement-now's worker exit path (ladder green → PR) has no reviewer seam; adding one is the additive-amendment integration class already queued for codegate's VerificationGate port — source: docs/tech-design/implement-now.md §3.7; design-brainstorm:implementation-verification-tech-design#D-2/#P-1
- **F-5**: the merge lane's LLM classifier is downgrade-only by frozen precedent (may force HUMAN, never grant AUTO) — source: glossary floor-clearing; prd:argus-unattended-drive FR-2/FR-3 lineage (GATE-4)
- **F-6**: the pain is measured: the predecessor's 525-PR study found effectively zero code review at all PR sizes — source: session memory (pr-size-analysis 2026-07)
- **F-7**: the sibling reviewers ship advisory-first with a ten-review adoption checkpoint; their reports are the introducing PR's evidence package — source: prd:prd-review (ship rule / advisory period)

## Fog

- **G-1** (non-blocking — blocks: the PRD's instrumentation detail, resolvable at synthesis/design): narrowed by D-9 — findings + dispositions live on the proposal itself (the durable corpus); the residual fog is only the harvest path: whether the precision/measurement reads mirror into the review-feedback-capture ledger or read the PR-resident data directly — the boundary between the two features' data stays to be drawn at PRD/design time

## Out of scope

- **O-1**: the remote twin — the charter-free PR-check form for human/bypass PRs — deferred to the WR-5-era remote phase, aligned with codegate's remote surface (from D-8)
- **O-2**: verdict-substitutes-stage-9 — the machine verdict never signs or replaces the human gate; parked program-wide for the autonomy-policy era (inherited, from D-3)
- **O-3**: reviewer self-tuning from acceptance/decline telemetry — the program's Optimizer-era park; calibration changes are ordinary data PRs (from D-8)
- **O-4**: the stage-9 presentation surface (read-to-rule for proposals) — its own feature in the gate-attention lineage; this feature produces the findings it would rank (from D-8)
- **O-5**: reviewing any class beyond factory code proposals — one class per feature, by program ruling (from D-1)
- **O-6**: a style/readability lens — the program's paid-for refusal stands; stated conventions ride the conformance lens (from D-2)

## Candidate glossary terms

- **T-1**: **proposal boundary** — the seam where a task's work stops being the worker's private tree and becomes a factory-owned proposal: ladder green, tree clean, before PR-raise; the stage-8 tail · avoid: *pre-commit* (git-hook connotation), *post-implementation* (the boundary is inside stage 8)
- **T-2**: **impl-review** — the proposal-class unmanned reviewer, the reviewer program's third instance · avoid: bare *code review* (the human act and session tooling), *PR review* (the remote twin's future surface — v1 reviews before the PR exists)

## Parked technical constraints

- **P-1**: the review consumes the charter (scope/spec fidelity is charter-relative), so the factory-proposal form is charter-fed; the remote/bypass-PR form (no charter) degrades to charter-free lenses — the same author-independence split codegate's FR-8/FR-11 already navigate — context: session pre-thinking, 2026-07-16
- **P-2**: reviewer independence is structural: fresh contexts, never the implementing agent's session; lens prompts + calibration live as data (the sibling reviewers' shape) — context: authoring-reviewers#D-5 pattern, 2026-07-16
- **P-3**: the warm-context placement (before PR-raise) is an efficiency choice, not a correctness one — findings apply cheapest while the worker lives; the identical review can run colder on the PR (the WR-5-era remote twin) — context: session pre-thinking, 2026-07-16

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-16 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered, zero blocking fog |
