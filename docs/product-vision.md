# Pyxis — Product Vision

**Version: v0.5 ** 
This document is **append-only** once accepted: entries are added, never rewritten or deleted, so inbound links
to it never go stale. It is the **abstract layer** of the document system: it
links to no concrete document — PRDs and technical designs point back to it,
never the reverse.

---

## 1. North star

**A product engineer's only job is to define the right solution to the business
problem — and what "done" means. Everything else is the factory's job.**

The normal path has two product-engineer touchpoints:

1. **Author and evolve the feature contract** — three inseparable obligations:
   the **PRD** (the business problem, who it's for, why now), the **design**
   (the decisions and the shape of the solution), and the **definition of
   done** (structured acceptance criteria, test cases, eval cases). Their
   artifact forms are proportional to the new information being introduced;
   approved memory may satisfy an obligation only through an explicit,
   checkable reuse result. The third obligation is what makes "done" decidable
   by the factory instead of by a human's recollection of intent — a design
   without a testable definition of done is an incomplete input: the factory
   can *build* from it, but it cannot *finish* from it.
2. **Accept the result** — review what shipped. Because the contract made
   "done" machine-checkable, this moment is taste and judgment (*is this the
   right solution?*), not verification (*does it work?*) — verification already
   ran in the pipeline against the engineer's own criteria.

An additional interaction is legitimate only when the factory reaches an
irreducible product-judgment boundary that could not reasonably have been
resolved in the contract. It arrives as a decision-ready brief, and the answer
evolves the contract and becomes durable memory. A preventable recurrence is a
factory gap. Policy, authority, ethics, and material-risk rulings belong to the
operator role (§3), not to implementation toil for the product engineer.

**The inversion test** — the sharpest tool in this document: *any minute an
engineer spends on machinery rather than authoring or evolving the contract,
resolving irreducible product judgment, or accepting the result is a factory
gap*, attributable to a missing or broken capability. "I had to fix CI," "I
had to unblock a worker," "I had to remember how releases work" are not
chores; they are bug reports against the factory.

Prioritization — *what to design next* — stays human, upstream of the factory,
not inside it.

## 2. What Pyxis is

Pyxis is an **open-source software factory**: engineers simply install it on
their own machines or servers. It orchestrates agent workers to turn raw
product ideas into merged, verified software, under governance.

Pyxis provides **only the core logic — the engine**. The user of each
installation decides which technologies it integrates with: where work items
live (GitHub Issues, Linear, a local file), where code is hosted (GitHub,
GitLab, Bitbucket, local bare repos), which CI runs checks, which agentic CLI
implements tasks (Claude Code, Codex, Cursor, OpenCode), and how the engine is
invoked (pull via a scheduler, push via webhooks). In its simplest form, Pyxis
runs against a local git repository and a plain task file.

Two commitments define the product's posture:

- **No privileged substrate.** Every medium is an adapter behind a documented
  interface; GitHub is one binding among many. Swapping any integration
  touches adapters only — never the engine.
- **CLI plurality everywhere — and plurality becomes intelligence.** Workers,
  skills, hooks, and guards are portable across all supported agentic CLIs,
  and the factory *learns* which backends are good at which work — routing
  tasks by measured capability and cost, not just by configuration (§11).

The required capability surface and the engine's load-bearing operational
invariants are specified in the concrete requirement and design documents —
each of which points back to this vision.

## 3. Who it serves

- **The product engineer** — authors and evolves contracts, resolves only
  irreducible product judgment, accepts results, and otherwise never leaves
  design (§1).
- **The operator** — owns policies, trust decisions, and the fleet; the factory
  escalates to them only when judgment or authority is required, never for
  toil. Those interventions are required governance, not implementation work.
- **Teams of one to a few (2–5)** — the factory also owns what happens *between*
  engineers: trust boundaries, review routing, onboarding. A new engineer's
  path to productivity is one install command plus a design doc — not a wiki
  crawl. As an open-source product, every such team runs its own Pyxis on its
  own infrastructure.

## 4. The pipeline grammar: governed intent → verified change

The product has one integrity grammar for converting governed intent into a
fully implemented, verified repository state. The table shows its maximal
raw-idea → feature instance. A fix, refactor, maintenance action, or other
change may enter with approved intent and decisions already in memory, but it
still resolves every delivery obligation. Stages 1–10 form that delivery path;
stages 11–12 are continuous lifecycle loops that emit results when triggered.

Every delivery stage emits a durable, positive stage result. Depending on what
the change teaches the repository, that result may be a new artifact, a typed
evolution of an approved artifact, or a mechanically validated reuse record
identifying the approved memory that already satisfies the obligation. Missing
artifacts, unchecked citations, and labels such as “small” never count as
evidence that a stage passed. The form, factory effort, and scrutiny are
elastic; the integrity obligations are not.

The **Judgment owner / execution** column identifies who owns any irreducible
judgment, while governing policy and earned autonomy (§6) select the permitted
human or automated lane. “Human judgment” does not require manual production
of mechanical work, and the factory never grants itself authority. The
definition of done resolved in stages 1–5 and compiled executable in stage 6
is exactly what stage 10 executes.

| # | Stage | Judgment owner / execution | Input (from) | Stage result |
|---|---|---|---|---|
| 1 | **Intent / requirement definition** | Factory drafts · product engineer owns new intent | A raw idea / prioritized problem, or approved intent already in memory | A requirement result: a new or evolved **PRD**, or a validated-reuse record identifying the approved intent that governs the change |
| 2 | **Requirement review** | Policy-routed · product engineer owns new product judgment | The requirement result (stage 1) | An approved requirement result: a ruling on new or changed intent, or validated reuse of prior approval. Multiple design tracks may follow; for now only the technical track is assumed |
| 3 | **Technical design brainstorming** | Factory-facilitated · product engineer owns irreducible solution judgment | The approved requirement result + retrieved governing decisions (stage 2) | A design-direction result: alternatives and a ruling proportional to uncertainty, or validated reuse of an approved direction |
| 4 | **Technical design** | Factory within the approved direction | The approved requirement result + design-direction result (stages 2–3) | A design result: one or more new or superseding **technical designs**, or validated reuse of approved design memory |
| 5 | **Technical review** | Policy-routed · product engineer owns irreducible design judgment | The design result (stage 4) | An approved design result: a ruling on new or changed design, or validated reuse of prior approval |
| 6 | **Contract compilation** | Factory | The approved requirement and design results + structured DoD (stages 2 + 5) | The contract **compiled into an executable form**: requirement graph, verification graph, risk assessment, missing-information and ambiguity report, suggested tests/evals, required human decisions, estimated cost, and an implementation-readiness verdict. A contract classified ambiguous, incomplete, contradictory, or unverifiable **bounces back with named defects** instead of entering planning |
| 7 | **Implementation planning** | Factory | The compiled executable contract (stage 6) | An executable plan proportional to work topology: a single-task charter or **milestones, epics, and tasks** with their dependency graph |
| 8 | **Implementation** | Factory | The executable plan + frozen task charters (stage 7) | Per task: governed outputs + testing + memory updates + raising the proposal — each proposal carrying its **evidence package** (§6) |
| 9 | **Implementation review** | Policy-routed · human owns required judgment | The proposal + its evidence package (stage 8) | An evidence-backed review verdict from a policy-permitted lane; unresolved judgment produces a decision-ready brief rather than an implicit approval |
| 10 | **E2E verification** | Factory | The merged change + the compiled DoD / verification graph (stages 6, 8–9) | A verification verdict at the scope demanded by the contract and risk: every applicable requirement works as expected and relevant regressions are ruled out |
| 11 | **Periodic maintenance** | Factory | The live codebase + living documents + policies (factory state, continuous) | Review of the code base and live documents for stale information, dead code, refactoring opportunities — and **drift**: places where reality has diverged from documents or policy (§11) |
| 12 | **Incident management** | Factory · operator owns required governance | A detected failure or regression + its trace (factory observability, or a human report) | What went wrong and what we learn — the memory of the system: a change later turned out to cause an issue; the incident captures why we missed it and what prevents it in the future |

Stage 6 exists because **bad contracts produce bad autonomous work**. Validating
the contract itself — before any agent burns tokens on it — may be the most
load-bearing capability in the product.

## 5. The capability map

The factory's surface, in ten areas:

| Area | In one line |
|---|---|
| Scaffolding & onboarding | A repo becomes factory-managed in one command; new projects in under a minute |
| Design & planning | Governed intent → elastic requirement and design results → compiled contract → dependency-ordered executable plan |
| Autonomous implementation | Leased, isolated, resumable agent workers on any supported CLI; idle costs ~0 |
| Quality & merge | Nothing merges unauthorized, ungated, or unrecoverable; risk-tiered human/auto lanes |
| Fleet operations | Containerized, kind-blind nodes that join, work, heartbeat, self-heal, and retire cleanly |
| Identity & security | Bot and human identities never confusable; least-privilege short-lived credentials; fail-closed guards |
| Governance & compliance | The factory keeps *itself* true: manifest-declared primitives, drift repair, org policy |
| Knowledge system | Decisions immutable, current-state alive, failures remembered, principles distilled |
| Observability & spend | First-class events, traces, and spend attribution; silent failures become loud |
| Engineer workbench | One-command human environment: any CLI, same gates, rendered review surfaces |

**The litmus test** for admitting a new capability: does every repo (of its
type) need it; must it be uniform for agents to operate without per-repo
special-casing; does its absence pull an engineer away from design? Three
yeses → it becomes a governed primitive with enforcement and a doctor check.
The next wave of capabilities is summarized in §11.

## 6. Trust model: earned autonomy

Not every team will trust the factory equally on day one — and none should have
to. A Pyxis installation sets an explicit **autonomy level**, per repo and
composable with risk tiers (a repo can run Level 3 for documentation changes
while auth-touching changes stay at Level 2):

| Level | The factory may… | Engages (pipeline stages · capabilities) |
|---|---|---|
| 0 | Draft documents only; no code changes | Stages 1–6 (idea → compiled contract) · design & planning, knowledge system, the workbench's review surfaces |
| 1 | Produce implementation plans | L0 + stage 7 · planning: a single-task or dependency-graph plan with frozen task charters |
| 2 | Create branches and proposals; never merge | L1 + stages 8–9 and 11–12 · autonomous implementation, quality gates + evidence packages (every merge stays human), fleet operations, identity & security, observability |
| 3 | Auto-merge low-risk changes once gates pass | L2 + stage 10 · the risk-tiered AUTO merge lane (low-risk classes only; everything else stays human) and the full e2e-verification loop |
| 4 | Auto-release low-risk changes | L3 + release automation: versioned, reproducible releases ship without a human once stage-10 verification passes |
| 5 | Auto-remediate known production issues | L4 + operations as first responder (§11): stage 12 closes the loop autonomously for known issue classes; humans only on escalation |

Autonomy and governing policy select the permitted execution and review lane at
each stage. The factory may act only inside that lane: it can recommend a route
and supply evidence, but it cannot lower scrutiny, reinterpret risk, or grant
itself authority in order to proceed.

Three mechanisms let a team climb the ladder with eyes open:

- **Evidence packages** — every factory proposal carries its own review kit:
  requirements addressed, design decisions followed, tests/evals run, risk
  tier, known limitations, rollback plan, cost, and the agent/backend used —
  with contents scaled to the risk tier, so trivial changes stay cheap.
- **End-to-end traceability** — every change can answer: which requirement
  caused this, which design decision authorized it, which test proves it,
  which policy allowed it to merge. Agent-written code demands explainability.
- **High-quality escalations** — when the factory is blocked, it delivers a
  decision brief (what happened, why blocked, the decision needed, options
  with risks, the evidence), never a stack trace; each escalation
  self-classifies as *factory gap* vs *required governance*, feeding §10's
  metrics. The goal is not merely fewer escalations — it is better ones.

## 7. The document system

Five governed document classes anchor authoritative contract and governance
memory, **each with a template**: `product_vision`,
`architecture_principles`, `prd`, `technical-design`, `incidents`. The feature
contract of §1 lives here: the PRD and technical design are its first two
artifacts, and the **structured** definition of done (machine-readable
acceptance criteria with verification methods) rides with them. These classes
are not an exhaustive list of repository memory: code, tests, schemas,
configuration, events, operational observations, and derived current-state
views also carry typed memory. One artifact may serve several memory roles, and
one role may span several artifact kinds.

Mutability, gating, and link rules are architecture principles: vision,
principles, and incidents are append-only; per-feature PRDs and designs are
immutable once merged (no automated author may change them — gated in CI and
in prompts); `prd.md` and `techspec.md` are living rollups updated on each PR.
**Links point from concrete to abstract** — this vision and the principles link
to no concrete document, and every concrete document cites what governs it.

## 8. Per-repo onboarding

Every repository that onboards into Pyxis carries a **`pyxis-config/`
directory** — that repo's customized extension point: repo-specific Pyxis
configuration, plus custom prompts for specific actions and skills used
throughout development in that repo. Beyond configuration, the factory builds
and maintains its own **repo intelligence** — a living, execution-verified
model of each onboarded repo's architecture, commands, conventions, risks, and
history (§11) — so no agent starts blind.

## 9. Distribution & the improvement loop

- **Auto-updaters** for both the **supervisor** and the **developer** installs:
  when a new release of either ships, the updater brings the user's account up
  to date.
- **Staged rollout / A/B testing**: an update can target only X% of accounts,
  or specific accounts, so the impact of a new release is watched before it
  widens.
- **Beta channel — self-pull from git**: beta users (including the developers
  of the supervisor and container themselves) track main directly. The
  **docker test container** auto-updates the core supervisor in place without
  rebuilding the image.
- **Global telemetry**: anonymous information from users' supervisor and
  developer nodes, collected to improve the system over time. Local operation
  never depends on it.

## 10. How we measure progress

Computable from the observability plane — which exists to measure these, not as
a side effect:

1. **Zero-touch rate** (primary) — % of features that go contract-merged →
   released with zero human interventions, where "released" requires the
   engineer-authored definition of done to have passed. The north star is this
   number approaching 100%. Every miss is classified as a *factory gap* or
   *required governance*; only the former is a factory defect, while both
   remain visible costs. (Only meaningful *because* §1 makes done
   machine-decidable.)
2. **Interventions per feature** (diagnostic) — the count and *cause taxonomy*
   of manual touches, reported separately for factory gaps and required
   governance. Preventable recurrence is the factory's backlog-priority
   function. Escalation self-classification (§6) feeds this taxonomy.
3. **Design-share of engineer time** (human-side check) — fraction of logged
   human activity that is design/acceptance vs machinery operation.
4. **Guardrails** — bypass rate, drift MTTR, fleet compliance %, and **cost per
   shipped feature** (tokens, runner minutes). Zero-touch gained by bypassing
   gates or burning unbounded tokens is not progress; token- and
   size-consciousness is a standing design constraint.

## 11. The road ahead

Where the vision demands more than the capability set specified so far — new
ground, ordered by trigger rather than date (a factory moves on demand, not on
quarters):

- **The upstream edge** — feature definition (stages 1–2): the factory drafts
  complete PRDs from raw ideas.
- **Contract compilation & quality gate** (stage 6) — the factory validates and
  compiles every contract before execution: executable / ambiguous /
  incomplete / contradictory / unverifiable is a verdict, not a discovery made
  mid-implementation.
- **End-to-end traceability** — the queryable chain from raw idea to incident:
  the substrate the next three items stand on.
- **Conformance & drift sentinel** — detect when reality diverges from
  documents or policy: approved design vs implemented architecture, claimed
  capability vs covering tests, incident lessons vs new changes, policy vs
  actual PR behavior. Drift detection is central to a trustworthy factory.
- **The downstream edge** — e2e verification (stage 10) and periodic
  maintenance (stage 11) as first-class pipeline stages with their own gates
  and reports.
- **Evals as a verification primitive** — for AI products, evals are the half
  of verification tests cannot cover; the engineer's eval cases (contract
  artifact 3) become gates the factory runs. Without them "verified" is hollow
  and zero-touch is unsafe.
- **Repo intelligence** — a living, execution-verified model of every onboarded
  repo, so agents never start partially blind.
- **Routing intelligence** — backend scorecards by task type (success, cost,
  latency, regression, review-rejection, violation rates) turn CLI plurality
  into learned routing, adversarial pairing, and fallback.
- **Unified policy engine** and a **specified worker-isolation contract** —
  governance and sandboxing as one declarative, enforceable model: for
  isolation, *the exact boundary is the product*.
- **Push triggers** — webhook invocation alongside the polling backstop.
- **The platform loop** — auto-update channels, A/B cohorts, and global
  telemetry (§9): what turns a personal factory into a distributable product.
- **Operations** — deploy, SLOs, and incident response wired to agents as first
  responders (runbooks are skills, incidents are issues, on-call is a
  supervisor loop with human escalation). Trigger: before the first hosted
  service ships — ops toil must never land on engineers.
- **Team readiness** — per-engineer identity, review routing, real code
  ownership. Trigger: before the first additional engineer joins.
- **Self-improvement** *(appended 2026-07-02)* — the factory adapts to each
  organization and codebase (change-size norms, code and review conventions,
  documentation style) and optimizes its own prompts, thresholds, and policies
  from observed outcomes. Every adaptation is a versioned, evidence-backed,
  gate-respecting proposal: the system tunes itself, and never silently
  drifts.

## 12. Parallel authorship *(appended 2026-07-10)*

The factory's throughput *is* parallelism: one plan fans out into many
sibling tasks, authored concurrently — code and documents alike — by workers
that share nothing but the mainline they all race to land on. Contention
between siblings is therefore a product concern, not an accident of tooling:
**every hand-resolved conflict between independent changes is a factory gap**
under §1's inversion test — an intervention in §10's taxonomy whose cause
names the surface to fix, charged to the design that created the shared
point. Two commitments follow:

- **Independent work merges independently.** When the plan declares two
  tasks independent, their changes compose without human resolution —
  guaranteed by the shape of the surfaces they touch, never by the luck of
  line placement. Every surface the factory owns — source, documents,
  indexes, registries, plans — is designed so that adding one unit of work
  is pure addition, not an edit to a center every sibling must also edit.
- **Serializing is mitigation, not repair.** The factory may sequence or
  re-merge its way around a known conflict magnet to keep shipping, but that
  spends the very parallelism it exists to deliver; the debt is repaid only
  by redesigning the surface so the contention cannot recur.

## 13. Repository memory — the substrate of autonomy *(appended 2026-07-31)*

Pyxis's deeper product is an **AI-ready software repository**: a durable,
queryable, self-correcting memory system from which an AI engineer can
understand, change, verify, and evolve the software without hidden human
context. The pipeline (§4), knowledge system (§5), repo intelligence (§8),
traceability, and improvement loop (§11) are different operations over that
memory. Code is the system's executable form; repository memory preserves the
meaning around it that code alone cannot reliably reconstruct.

**The fresh-agent test.** Given only a clean checkout, authorized tools, and a
stated goal, a fresh competent AI engineer can either finish within its granted
authority or stop only at an explicit human-judgment or permission boundary
with a decision-ready brief. It requires no prior chat, tribal knowledge, or
undocumented intervention. It can answer, within a bounded context budget:
what outcome is intended; what the system does now; which promises, decisions,
and invariants govern the area; why the system is this way; how to build, test,
operate, and recover it; what authority the agent has; and what evidence proves
completion. Failure to finish or reach such a boundary is a named factory gap
in memory, retrieval, observation, verification, or execution — not a reason
to turn a human into a search engine.

Repository memory has several complementary roles:

- **Semantic memory** preserves promises, concepts, invariants, and the
  current architecture — what the product and system mean now.
- **Episodic memory** preserves changes, decisions, reviews, outcomes, and
  incidents — how and why the system arrived here.
- **Procedural memory** preserves the safe, repeatable ways to build, test,
  operate, change, and recover the system.
- **Verification memory** preserves the tests, evals, observations, and
  evidence behind claims of correctness and quality.
- **Operational memory** preserves relevant facts about the running and
  deployable system that cannot be inferred safely from source alone.

Feature records and current-system views are therefore complementary. A
feature is an episode: its intent and decisions freeze into provenance. The
system is the integral of all those episodes: living or deterministically
compiled views answer what is true now. History is never erased to make a
current view, and an agent is never forced to replay all history to recover
current truth.

An AI-ready repository is:

- **decidable** — requirements and quality claims have machine-checkable
  definitions of done;
- **addressable** — every load-bearing fact has stable identity, authority,
  provenance, validity, and a bounded retrieval path;
- **enforceable** — permissions and invariants are held by machinery, never by
  faith that an agent will remember prose;
- **operable** — build, test, inspect, deploy, and recovery paths are
  discoverable and executable without tribal knowledge; and
- **learnable** — outcomes and failures become durable evidence that improves
  future memory, routing, verification, and action.

**One pipeline grammar, elastic cost.** The pipeline is the integrity grammar
for every change, not a requirement that every change pay the same ceremony.
The raw-idea flow in §4 is its maximal feature instance; a fix, refactor,
maintenance action, or other change may enter with intent and decisions already
recorded. Every change still resolves its intent or requirement, governing
decision, action plan, evidence, and verification.

Every delivery stage emits a durable, positive result. A node may resolve by
creating a substantial new artifact, evolving approved memory through its
governed channel, or recording a compact, mechanically validated claim that
identified approved memory already satisfies the obligation. It never resolves
as an unverifiable exemption or as evidence inferred from an absent artifact.
There is no artifact-free or gate-bypass lane for the factory — especially when
Pyxis changes itself.

Governing policy selects each node's permitted execution lane and scrutiny from
the change's new promises and decisions, risk and blast radius, uncertainty,
work topology, available verification, memory impact, and earned autonomy
(§6). Within that lane, the factory selects the proportional instance form and
performs the mechanical work; it never chooses or widens its own authority. The
initial route is made once at the richest pre-work context point and frozen as
the minimum for downstream use. If implementation discovers a requirement gap,
new decision, wider risk, or weaker proof, a recorded re-characterization may
move the workflow only toward a heavier form and gate; it never silently
downshifts to reclaim cost. Automation makes small work cheap; it does not make
integrity optional. Generated boilerplate that carries no retrievable or
verifiable information is ceremony, not memory.

**Every workflow is a memory transaction.** Before acting, the factory
retrieves relevant memory, observes executable reality, distinguishes facts
from assumptions and judgment, and validates that it has enough information
to proceed. After acting, it preserves the evidence and historical episode,
updates or rebuilds current-state projections, and checks for contradiction or
drift. Parallel workers add disjoint source records; shared indexes and views
are derived rather than hand-maintained contention points (§12). Governing
intent remains human-authored input: executable reality is reconciled toward
it, never used to silently regenerate or justify it.

A change is complete only when the code and other governed outputs conform to
authoritative intent, every authorized change of intent passed through its
governing evolution channel, verification evidence supports the resulting
claims, and the repository can retrieve both what is true now and why it
became true. Code quality is therefore never an assumption attached to AI
automation; it is a claim the factory must continuously prove.

**Ask once, remember durably.** Business intent, priority, and irreducible
product or design judgment belong to the product engineer's contract touch;
acceptance of the result is the normal second touch (§1). Ethics, grants of
authority, and acceptance of material risk belong to the operator's trust role
(§3). Inaccessible external facts may require an attributed human source.
Every other intervention is a factory gap.

When previously unavailable judgment surfaces mid-flight, the factory fails
closed and presents one decidable brief with the evidence, options,
consequences, recommendation, and reversibility. The answer becomes attributed
memory with explicit scope, authority, validity, and expiry or supersession
conditions, so later sessions reuse it rather than asking again. This is
legitimate required governance, but it remains measured and every preventable
recurrence becomes a factory improvement. A remembered answer may authorize an
act within policy; it never silently widens policy or becomes a self-grant.

**The self-improvement loop begins with memory failures.** A failed autonomous
run is classified: necessary knowledge was absent; it existed but retrieval
missed it; it was stale, ambiguous, contradictory, or out of scope; reality
had drifted; an observation or execution capability was missing; verification
could not prove the result; an agent violated a known constraint; or policy
correctly required human judgment. All but the last produce a specific factory
improvement. Each improvement remains a versioned, evidence-backed,
gate-respecting proposal (§11). Pyxis becomes more autonomous by making its
memory and proof better — never by weakening the boundary that detected the
failure.
