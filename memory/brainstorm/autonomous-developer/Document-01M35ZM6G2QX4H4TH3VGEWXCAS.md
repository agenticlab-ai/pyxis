# Autonomous developer — brainstorm record

| Field | Value |
|---|---|
| **In one line** | The AI engineer that replaces the human engineer as the pipeline's driver: sole input "here is the problem I want to solve"; it holds delegated decision authority and the accesses to rule and permit changes at every formerly-human moment, driving a feature to done or to a named stop (escalation). |
| **Relevant-when** | brainstorming or contracting the autonomous developer / AI engineer, problem-statement intake, decision-authority delegation, unmanned driving of the authoring pipeline, or escalation-to-human boundaries; checking prior rulings on the human boundary, authority graduation, or ruling attribution |
| **Idea** | "We want to build an autonomous developer that replaces the engineers. Its sole input to our process would be: here is the problem I wanna solve — and then it will interact with the system to get it to a finish line or get the stop, depending on if one of the pieces in our pipeline realizes we need more information or there's something the autonomous developer cannot help us with. We are building the AI engineer, which can interact with our system; that engineer has all the accesses to drive an answer or permit the changes that our system — in other words, pyxis — needs to achieve." (operator, 2026-07-10, lightly normalized from voice) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-10 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/argus/ |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md | §1 the north star this program moves: the engineer's **two touches** (author the contract · accept the result) + the inversion test; §4 the stage table marking 2/5/9 **Human**; §10 zero-touch rate + the interventions cause-taxonomy — the AD's success metric and escalation log, pre-built |
| docs/prds/prd-review/effective.md + tech-design-review/effective.md + docs/brainstorms/authoring-reviewers.md | the shipped unmanned substitutes for stages 2/5; the escalation protocol (embedded brief + fail-closed park + staleness alarm) the AD's spine generalizes; the ruled no-human posture (reviewers' D-1) — the precedent this program extends from review to *authority* |
| docs/brainstorms/contract-compiler.md (gated needs-investigation) | stage 6 deferred with reopen triggers naming fan-out-Planner work — this program is the arriving consumer; D-1/D-2 stand ready for the reopen |
| .pyxis/authoring/feature-brainstorm.md · prd-new.md · tech-design-new.md · docs/prds/plan-new/effective.md | the stages the AD drives; feature-brainstorm's "every decision is the operator's, live-answer anchors only" is the grilling surface the AD inverts |
| docs/domain-model.md · docs/system-design.md | actor entities and planes — where a decision authority sits; no existing entity holds rulings |
| gen-1 (harbormaster) precedents *(operator memory)* | the execution half is proven (issue → worker → PR → bot-squash-merge as the App once approvals + checks are green); risk-tiered PR approval designed (bot-attributed AUTO lane over a deterministic floor, P11); multi-CLI worker backends — the "accesses" machinery exists in gen-1 form, pyxis-native equivalents unbuilt |
| factory-repo governance *(operator ruling, 2026-06)* | every factory repo requires one human approval to merge — the standing hard backstop any "permit the changes" authority collides with |

**Overlaps surfaced:** no covering capability — plan-new plans, the reviewers judge single documents, gen-1's supervisor sequences tasks mechanically; nothing holds decision authority over formerly-human rulings and nothing drives the authoring pipeline end-to-end. Adjacent boundaries: the reviewers' never-edit/never-re-judge perimeter ends exactly where the AD begins (someone must *dispose* their findings and answer their escalations). The parked vision-§1 autonomy append (operator's, on the tech-design-brainstorm ledger) is subsumed by this session's Q1.

## Framings

- **The driver** — a long-lived per-feature actor that walks the stage graph: invokes each skill, answers its questions, disposes reviewer findings, cuts PRs, reacts to bounces and escalations. Thesis: *the pipeline defines the stages; what's missing is the actor that turns the crank, with judgment at the wheel.*
- **The surrogate authority** — the identity is the **authority contract**: an explicit, versioned boundary of what the AD may rule (anything the problem statement + governing docs determine) versus what it must escalate (what they underdetermine; governing-doc changes; appetite/spend; irreversible surfaces; overriding a prior human ruling). Driving is mechanical policy on top. Thesis: *the scarce thing being replaced is judgment-with-accountability, not activity.*
- **The fleet-emergent engineer** — no single actor: make every stage self-ruling the way the reviewers already are, add a problem-intake artifact and a unified escalation spine; "the developer" is the pipeline running unmanned. Thesis: *don't build a homunculus — finish the per-stage pattern and add connective tissue.*

**Destination:** the **AI Engineer (Agent)** — a *separate agent* holding delegated decision authority (D-1): pyxis's stages keep their contracts and gain an automated counterparty; the authority contract is the agent's own policy; the driver is its runtime shape.

## Decisions

### D-1 — the identity: a separate agent holding delegated authority — the AI Engineer

- **Question**: which framing is the feature — the driver, the surrogate authority, or the fleet-emergent engineer?
- **Answer**: **B, sharpened**: "a separate agent that has its own set of logic, policy and requirements and can interact with our system in an automated way — so it is an AI Engineer (Agent)." A first-class actor **outside** the pipeline — a counterparty, not a component: the stages keep their contracts (they grill, review, bounce, escalate exactly as designed) and the AI Engineer stands where the human engineer stood, holding delegated decision authority bounded by an explicit authority contract (its policy). The fleet-emergent dissolution (C) is rejected as identity — cross-stage judgment needs a home; the driver (A) is the agent's runtime shape, tech-design ground.
- **Reasoning**: operator's ruling, with the separateness emphasis carrying the architecture: (1) pyxis's stage contracts change minimally — the pipeline gains an automated counterparty, not self-ruling stages; (2) the AI Engineer gets its **own requirements corpus** (logic, policy, requirements) rather than riding amendments into every stage's PRD; (3) the authority contract is the agent's policy object — versioned data, the graduation dial; (4) attribution follows — rulings are the agent's, recorded as such, never blended with human rulings.
- **Anchor**: 2026-07-10 · t3
- **Status**: live

### D-2 — the human boundary: problem provider, nothing else structural

- **Question**: in the end state, what remains of the human's role — problem-only, or problem plus a blocking accept-the-result touch?
- **Answer**: "The human is just a problem provider to the AI Engineer — for example: I will give it an input and ask it to work with pyxis to deliver it." The end state is **problem-only**: the human's structural role is providing the problem (and answering escalations when the agent hits its authority boundary — the stop semantics of the founding statement). The human's interface is the **agent**, not pyxis — in the end state the human never touches the pipeline. Acceptance is delegated: DoD-pass + the agent's acceptance ruling = shipped; "is this the right solution?" survives only as a **sampled, non-blocking audit** feeding the measurement plane (overrides-observed as the trust signal), never a gate. Rollout rides the graduation path: acceptance starts blocking and decays to sampling on evidence, via authority-contract checkpoints — one ruling: end state + path.
- **Reasoning**: a standing blocking end-gate contradicts "sole input = the problem" and permanently caps zero-touch below 100%; §1's own logic makes the delegation safe — acceptance is *taste, not verification*, because the DoD made done machine-decidable — and taste-misses route to the audit + escalation channels at far lower cost than gating every feature. **Vision gap, surfaced explicitly**: this amends §1's north star (two touches → one input); the §1 append is the operator's to author later — this session touches no governing document. The parked vision-§1 autonomy append ("human involvement ends at feature-brainstorm") is subsumed by this stronger posture.
- **Anchor**: 2026-07-10 · t4
- **Status**: live

### D-3 — the authority perimeter: a structural never-list over a graduated list

- **Question**: what may the AI Engineer never rule, and what starts escalated but can promote?
- **Answer**: adopted as recommended — two tiers. **Never (structural, trust-independent)**: (1) **its own grant** — never amends its authority contract, never self-promotes past a checkpoint, never un-parks an escalation it raised; authority changes are human-versioned data changes; (2) **governing documents** — vision and principles are human-authored by standing rule (incident entries stay *appendable*: observations, not governance); (3) **overriding a recorded human ruling** — a frozen D-n, an incident lesson, a prior escalation answer; it may propose a supersession, the escalation channel approves; (4) **appetite and spend beyond the problem's grant** — the problem statement carries a budget envelope; exceeding it escalates, never a judgment call; (5) **irreversible or outward surfaces not pre-granted** — publishing, external comms, production-facing acts, real spend; (6) **destination-shaping gaps the problem statement underdetermines** — the information-gap stop is definitionally the human's decision brief. **Graduated (escalate now, promotable by checkpoint)**: factory-repo merge approval (F-4's human-approval backstop stands until the operator changes org governance — an operator act outside the agent's reach), the acceptance ruling (D-2's path), brainstorm gate verdicts, disposing *blocking* reviewer findings (advisories agent-disposable from day one).
- **Reasoning**: every never row is one of: self-dealing (1), a standing governance rule (2, 3), a resource boundary only the grantor can move (4, 5), or the definition of an information gap (6). Everything else is trust-level territory — exactly what the versioned authority contract and its checkpoints exist to decide on evidence; hardcoding more into "never" would freeze judgment calls that measurement should settle.
- **Anchor**: 2026-07-10 · t5
- **Status**: live — **row 6 superseded by D-4** (underdetermination resolves by recorded assumption, not escalation; the escalation trigger narrows to breaking decisions); **row 3 refined by D-5** (the inviolable set is governance-tier content + incident lessons + human breaking-call answers; feature-tier human rulings evolve through sanctioned channels like any feature-tier content — silent contradiction stays forbidden); rows 1–2, 4–5 and the graduated list stand as written

### D-4 — the intake and the assumption regime: idea-only in; assumption-first; escalate only breaking decisions

- **Question**: what must the human's input carry, and what happens at the gaps it leaves?
- **Answer**: leaner than every offered option — "I just ask this AI Engineer: this is my idea, go figure it out and implement it end to end. I'm not providing much information beside the constraints of the repo itself. The AI Engineer is responsible to figure things out, make some assumptions if needed. Unless there is some breaking decision that must be made — [the report's web pages went red; now we must go black] — a breaking change for the whole system, no middle ground the AI Engineer and pyxis can agree upon: that's where the human comes in and makes the call. In other words, the AI Engineer should make reasonable assumptions." Recorded as: (1) required input = **the idea, in the human's words — nothing else**; the installed repo (its code, documents, rulings, and pyxis config) is the standing context; (2) the grant (D-3 row 4's envelope) defaults from **per-repo config**, overridable per problem — never a required intake field; optional pre-rulings stay available, never required; (3) gaps resolve **assumption-first**: the agent makes reasonable assumptions and records each one, attributed, at the load-bearing spot; (4) escalation is reserved for **breaking decisions** — the no-middle-ground class; (5) breaking-call answers **accrete onto the problem's ledger**, so every later ruling traces to idea-plus-answers, never to conversation ether.
- **Reasoning**: the human is a problem provider (D-2) — any *required* structure beyond the idea is authorship relocated back to the human, and the repo already carries most real constraints; assumption-first is what "figure it out" means operationally. **Supersedes D-3's row 6 only**: underdetermination no longer escalates by default — it resolves by recorded assumption; the escalation trigger narrows from "destination-shaping gap" to "breaking decision."
- **Anchor**: 2026-07-10 · t6
- **Status**: live

### D-5 — the tier line: governance inviolable; feature-tier evolvable through channels, regression-guarded

- **Question**: what makes an assumption *reasonable* and a decision *breaking* — where exactly is the line?
- **Answer**: the line is the document system's own tier hierarchy — "as long as it's not gonna go against the governance documents — the vision, the principles, the incidents — it's okay to change certain features we already have in place or some design decisions. Nothing from the north-star perspective; nothing the policies of the system defined. The color red will be defined in one of the governance documents — changing it to black is obviously a red flag. But a feature we designed in a PRD unit — 'if the user clicks here, we do A' — our new feature making that 'go through another step' is acceptable, as long as we do not cause any regression." Formalized:
  **A change/assumption is legal when all hold** — (1) **tier check**: it contradicts no governance-tier content (vision, principles, incident lessons, system policies) and no human breaking-call answer on the problem ledger; (2) **channel check**: where it touches standing feature-tier content (a merged PRD row, a frozen design decision, an existing behavior), it moves through that content's sanctioned evolution channel — typed amendment, supersession, reopen-record — never silent contradiction; (3) **regression check**: no *unamended* contract breaks — existing features' DoD/tests keep passing; a behavior change is sanctioned exactly when its contract was amended through (2); (4) **recorded**: attributed at the load-bearing spot (D-4).
  **A decision is breaking (escalate) when either holds** — (a) **governance override**: delivering the idea requires contradicting governance-tier content or a human breaking-call answer (red→black); (b) **convergence deadlock**: the agent↔pyxis loop cannot settle — rounds/bounces exhausted per the existing caps with no legal assumption remaining ("no middle ground they can agree upon," made mechanical). D-3's never rows 1–5 remain independent always-escalate triggers.
- **Reasoning**: operator's line, and it lands on infrastructure that already exists: the tier boundary is mechanically enforced today (governing docs require the operator trailer; frozen feature content evolves only by amendment/supersession under docgate), so the authority contract's hardest edge is already in CI (F-9). "No regression" reads precisely, not as behavior-frozen: adding a step *is* a behavior change and is legal *because* its contract gets amended; regression = violating a contract nobody amended — stage 10's charter verbatim. Dropped from the earlier proposal: the standalone two-way-door/reversibility test — the tier line carries it internally (git makes feature-tier work revertible; *outward* irreversibility stays covered by D-3 row 5). Refines **D-3 row 3**: feature-tier human rulings are evolvable by the agent through sanctioned channels; only governance-tier content, incident lessons, and breaking-call answers stay propose-only.
- **Anchor**: 2026-07-10 · t7
- **Status**: live

### D-6 — the principal model: stages address a role; attribution is permanent

- **Question**: how do agent rulings live in a document system whose stage contracts say "every decision is the operator's, anchored to a live answer"?
- **Answer**: adopted as recommended — the stages address a **decision principal**, a role, not a person: the human operator today, the AI Engineer under grant. (1) **Attribution permanent and typed** — every ruling records which principal made it (human vs agent) plus the authority-contract version it ruled under; agent D-ns anchor to the **problem ledger** (the idea + accreted breaking-call answers) the way human D-ns anchor to live session turns; the two are never blended or laundered — an agent assumption never becomes "the operator ruled." (2) **One uniform seam per stage, not self-ruling stages** — each stage guide gains the same small delta ("the principal answers"), preserving D-1's counterparty architecture; grilling, gates, and reviewer protocols otherwise untouched. (3) **Overturn is measurement** — a human may overturn any agent ruling at any time through the normal channels (amendment, supersession, audit annotation); every overturn is captured as data — the trust signal the graduation checkpoints consume.
- **Reasoning**: the minimal change that makes the agent a legal counterparty at every stage; forensics stay honest forever (incident analysis must know who decided what, under which grant); D-2's sampled audit becomes exactly the feedback loop the authority contract needs.
- **Anchor**: 2026-07-10 · t8
- **Status**: live

### D-7 — the bet and the graduation currency

- **Question**: what does the program's success measure, and what evidence promotes authority tiers?
- **Answer**: adopted as recommended. **The bet**: the AI Engineer is the capability that moves **zero-touch rate** (§10's primary) from ~0% toward 100% — window widened under D-2 to *problem-stated → released*; the interventions cause-taxonomy is its escalation/overturn log; measurements ride the artifacts, no new telemetry infrastructure. **Measurements**: M1 **assumption survival** — share of recorded agent assumptions not overturned by human audit or escalation answer per checkpoint window: the promotion currency. M2 **escalation precision** (share of escalations judged genuinely breaking — guards the storm regression) **and recall** (share of overturns saying "should have been escalated"); **missed-breaking is the zero-tolerance class** — one confirmed instance is incident-grade, not a statistic. M3 **cost per shipped feature** within the grant. **Graduation**: D-3's graduated tiers promote at N-feature checkpoints on M1/M2 evidence — ship rules, never dates; **demotion is symmetric and fail-closed** — a confirmed missed-breaking demotes the touched tier immediately, no checkpoint needed.
- **Reasoning**: zero-touch was designed as this program's metric before the program existed (F-5); M1/M2 are the two ways delegated judgment fails — bad assumptions, bad escalation boundary — and both are computable from artifacts D-4/D-6 already require; promote-slow/demote-fast matches every trust structure in the repo.
- **Anchor**: 2026-07-10 · t9
- **Status**: live

### D-8 — v1 scope: full pipeline by contract; execution through the existing fleet by tracker convention

- **Question**: what does "end to end" bind to now, given pyxis ships stages 1–7 as skills while implementation is not pyxis-native?
- **Answer**: full pipeline by contract, with execution bound to machinery that already exists — "the rest already exists in another repo: when the plan is generated and GitHub issues are created, we can tag them with local-agent and the automated system would implement those GitHub issues." The AI Engineer drives pyxis's authoring chain (stages 1–7) as principal; **materializes the plan as tracker work items tagged for automated implementation** (bootstrap binding: GitHub issues + the `local-agent` label; the gen-1 fleet's node workers claim, implement, and raise bot-authored PRs); then **drives the results home** — dispositions, approvals per D-3's graduated list, re-planning on bounces. The agent is the **first escalation authority everywhere**: stage escalations (reviewer briefs, gate bounces) route to the agent; only breaking ones re-escalate to the human — a two-level spine reusing the shipped brief + fail-closed park + staleness-alarm shape at both levels. Coverage is addressed **by role, not enumeration**, extending automatically as pyxis-native stages ship. **Bootstrap form**: a long-running agent session per problem on the vendor-neutral surface (any CLI), resuming across sessions via files-as-truth — the problem ledger plus a per-problem **delivery ledger** (stage position, open items, escalation states); no in-memory state survives, everything auditable. One agent per problem; parallel problems are parallel agents.
- **Reasoning**: operator's grounding — the execution half is F-3's proven machinery reached through a tracker convention, not new construction. The division of labor is clean and race-free: the agent is the principal and driver, the fleet is the hands — the agent never self-implements fleet-tagged work (a known gen-1 race class). "End to end" therefore binds today: idea → authored contract → plan → tagged work items → fleet PRs → landed, the agent driving dispositions and escalations throughout. The fleet seam stays adapter-neutral in the contract (work items tagged for automated implementation), with GitHub + `local-agent` + the gen-1 fleet as the concrete binding — per-repo config (F-8) carries the binding, preserving portability.
- **Anchor**: 2026-07-10 · t10
- **Status**: live

### D-9 — the name, the home, and the front door: Argus

- **Question**: what is this thing called, where do its requirements live, and how does the human hand it an idea?
- **Answer**: the skill/product name is **Argus** — the Argo's shipwright and the hundred-eyed, never-sleeping watcher — staying in the ship's family (pyxis the compass, carina the keel, argus the builder). Feature ledger at `docs/prds/argus/` (this record's Feeds); the **authority contract lives as versioned data at the agent's home** — policy is data, changed by PR, never prose-buried; exact placement is tech-design ground. **Front door**: bootstrap = invoke Argus with the idea — a session/skill invocation carrying the human's words verbatim into the problem ledger; a tracker-item intake (an item labeled for Argus) is the second door when wired — idea-verbatim either way. **Vocabulary split**: Argus is the persona/skill; **AI Engineer** stays the domain-model role it instantiates (the agent class of decision principal); "autonomous developer" remains the informal description.
- **Reasoning**: operator wanted a real-thing name (Jarvis-class); Argus doubles its mythology usefully — the engineer who built the ship, and the watcher who never sleeps; the persona-vs-role split keeps domain vocabulary stable independent of branding.
- **Anchor**: 2026-07-10 · t11–t12
- **Status**: live

### D-10 — one feature; bootstrap-first appetite with a dogfood prove

- **Question**: one feature or a program of PRDs — and what appetite?
- **Answer**: "one feature" — a single `argus` PRD binding the agent, its policy object, and its artifacts; the per-stage principal seams land as small **additive amendments to the stage documents at implementation time** (the reviewers' invocation-step pattern, proven). Appetite in checkpoints, never dates: the first increment is the **bootstrap Argus** — session-form, the full drive loop (idea → authored contract → plan → `local-agent` items → fleet PRs → landed) with the graduated list at its floor: the human still merges and still signs gates; Argus rules everything else and records every assumption. Prove phase = **dogfood**: Argus drives one real, small pyxis feature end to end as its own DoD evidence. Authority promotions ride D-7's N-feature checkpoints.
- **Reasoning**: D-1 gave Argus its own requirements corpus, *singular* — a program would scatter one identity across ledgers; the seam-amendment pattern is proven; bootstrap-first exercises the whole loop while the unproven agent stays under the graduated floor.
- **Anchor**: 2026-07-10 · t13
- **Status**: live

### D-11 — segments: the problem provider; the self-driving engineer is the anti-segment

- **Question**: who is Argus for, and who is it explicitly not for?
- **Answer**: confirmed as proposed at the gate — **primary segment**: the problem provider with a pyxis-installed repo ("I have an idea and I've installed pyxis on my repo" — the operator's own use case as archetype). **Anti-segment**: the engineer who wants to *remain* the driving principal — served unchanged, because **Argus is opt-in per problem** (D-9's front door): it exists only where an idea is handed to it; the pipeline without Argus is untouched.
- **Reasoning**: closes Appendix B §2.2; opt-in-per-problem means adopting Argus is never a repo-wide mode switch — coexistence with human-driven features is structural, not configured.
- **Anchor**: 2026-07-10 · t14
- **Status**: live

## Facts

- **F-1**: vision §1 gives the engineer exactly two touches — author the contract, accept the result — with the inversion test charging everything else to the factory; §4 marks stages 2/5/9 Human; prioritization stays human, upstream — source: product-vision §1 · §4
- **F-2**: the unmanned substitutes for stages 2/5 are shipped — prd-review and tech-design-review complete with zero human involvement, escalate = embedded brief + fail-closed park + staleness alarm, and the no-human posture is a frozen ruling (authoring-reviewers D-1) — source: those PRDs + record
- **F-3**: gen-1 proves the execution half — issue → worker → PR → bot-squash-merge as the App once one approval + green checks are met — and has a designed risk-tiered approval lane (bot-attributed approvals over a deterministic floor); pyxis-native equivalents unbuilt — source: gen-1 (harbormaster) shipped fleet + risk-tiered design, operator memory
- **F-4**: every factory repo requires one human approval to merge — the last hard backstop "permit the changes" collides with; the automated author cannot bypass it — source: org governance ruling, 2026-06
- **F-5**: zero-touch rate is defined contract-merged → released with zero human interventions, and interventions carry a cause taxonomy — the AD's success metric and escalation log exist in the vision already — source: product-vision §10
- **F-6**: feature-brainstorm's contract makes every D-n the operator's, anchored to a live answer from the session, never from corpus content — the exact surface the AD inverts; an AD-ruled session needs its own anchor semantics — source: .pyxis/authoring/feature-brainstorm.md (Never list) + its PRD
- **F-7**: the contract compiler is deferred with reopen triggers naming stale-flag pain at scale or stage-10 / fan-out-Planner work starting — this program is the arriving consumer — source: docs/brainstorms/contract-compiler.md D-3
- **F-8**: every pyxis-installed repo carries a per-repo config surface (`pyxis-config/`, an operator product guideline) — the natural home for the grant envelope and the authority-contract binding D-4 relocates out of the intake — source: operator product guidelines 2026-07-01
- **F-9**: D-5's tier line is already mechanically enforced — governing docs take an operator trailer and are pure-append-only; frozen feature-tier content evolves only by typed amendment / supersession under docgate — the authority contract's hardest edge is in CI today — source: docgate gate suite + parallel-authorship governance (#150)

## Fog

none *(session open)*

## Out of scope

- **O-1 — org/repo governance changes**: the human-approval merge backstop is an operator act, never the feature's; D-3's graduated merge row activates only after the operator changes org governance — originates: D-3 (with F-4)
- **O-2 — the vision §1 append**: the north-star amendment (two touches → problem provider) is operator-authored governing-doc work, surfaced as a vision gap, never performed by this feature or its sessions — originates: D-2
- **O-3 — gen-1 fleet internals**: the executing fleet is consumed through the tracker convention (`local-agent` items), not modified — originates: D-8
- **O-4 — pyxis-native execution stages**: nativizing stages 8–10 is separate future work; Argus binds to them by role when they ship — originates: D-8
- **O-5 — re-judging document substance**: the reviewers' ground stays theirs — Argus disposes findings and answers escalations, never re-runs review lenses — originates: D-1

## Candidate glossary terms

- **AI Engineer (Agent)** — the domain-model role: the separate agent holding the human engineer's delegated decision authority, interacting with pyxis as an automated counterparty (D-1)
- **Argus** — the persona/skill name of the AI Engineer: the Argo's shipwright, the hundred-eyed watcher (D-9)
- **authority contract** — the AI Engineer's policy object: the versioned, explicit boundary of what it may rule versus what it must escalate (D-1)
- **reasonable assumption** — the agent's default gap-filler: recorded, attributed, standing until overturned; legal iff it clears D-5's tier, channel, and regression checks (D-4, D-5)
- **breaking decision** — the escalation class: governance override or convergence deadlock — the call the agent must never assume through (D-4, D-5)
- **decision principal** — the role the stage contracts address: the human operator, or the AI Engineer under grant; every ruling records its principal and authority-contract version (D-6)
- **problem ledger** — the human's idea plus every accreted breaking-call answer: the root artifact agent rulings anchor to (D-4, D-6)
- **delivery ledger** — the per-problem driving state: stage position, open items, escalation states — files-as-truth, resumable across sessions (D-8)

## Parked technical constraints

- **runtime shape** — long-running session (bootstrap) vs engine-hosted runtime (engine era): tech-design ground per D-1's demotion of the driver framing; the PRD binds only the bootstrap form (D-8)
- **fleet-seam adapter boundary** — which tracker, which fleet, which label: per-repo config carries the concrete binding (F-8); the contract stays adapter-neutral (D-8)

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-10 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered, zero blocking fog |
