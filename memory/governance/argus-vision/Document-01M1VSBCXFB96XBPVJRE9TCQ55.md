# Argus — Vision

**Version: v0.2 (draft — not yet accepted)**
This document is **append-only** once accepted: entries are added, never
rewritten or deleted, so inbound links to it never go stale. It is the
**abstract layer of the Argus program** — the delegation's north star. It
sits **under the product vision**: on any conflict, the product vision
governs. It links upward only — to the product vision and the architecture
principles — and to no concrete document; every Argus requirement, design,
policy file, and stage seam points back to it, never the reverse.

---

## 1. North star

**A person with a problem should need to supply exactly one thing: the
problem.**

Argus — the AI Engineer — takes an idea in the person's own words and
returns a delivered feature, holding delegated decision authority through
everything between: it shapes the contract, disposes review, plans, hands
implementation to the fleet, and drives the results home. Across a
delegated feature's whole life, the human's structural role is four things
and only four:

1. **State the problem** — one idea, in their own words; the installed
   repository and its documents are the standing context, so the idea is
   never asked to carry what the repo already knows.
2. **Answer breaking decisions** — the calls that are genuinely theirs,
   delivered as decision briefs decidable on sight.
3. **Grant and revoke authority** — the delegation itself is the human's
   instrument, moved only by their hand, on evidence.
4. **Audit by sample** — taste, exercised on shipped work through the same
   evolution channels as any other change.

**The delegation test** — this document's sharpest tool: *any human moment
in a delegated feature's life outside those four is an Argus gap* — not a
chore, a bug report against the delegation, carrying a cause that names
what to fix. It is the product vision's inversion test applied to the
widest window: problem-stated → released.

Full autonomy is **not the absence of the human**. Two stop classes look
alike today and must never be confused. **Requests** — acts Argus is not
yet trusted to perform (signing a gate, accepting a result, approving a
merge) — graduate away tier by tier as evidence accumulates. **Breaking
escalations** — a decision that requires the human to move a boundary they
own — are permanent: at every tier, forever. What approaches zero is human
toil and human driving. Human authority over what is genuinely human
ground never decays; it is the fixed point the whole delegation pivots on.

For problems not handed to Argus, nothing here applies: the engineer who
wants to drive keeps the product vision's touches, unchanged. Argus is
**opt-in per problem** — coexistence with human-driven work is structural,
never a repo-wide mode.

## 2. What Argus is

Argus is **a separate agent holding delegated decision authority** — a
counterparty to the pipeline, never a component of it. The pipeline keeps
its contracts: stages grill, review, gate, bounce, and escalate exactly as
designed, addressing a **decision principal** — a role, not a person: the
human operator, or Argus under grant. Argus stands where the human stood;
nothing else moves.

The scarce thing Argus replaces is **judgment-with-accountability**, not
activity. Sequencing stages is mechanical — any script can turn a crank.
What was missing between the stages was a holder of decisions: someone to
answer what a stage asks, dispose what reviewers find, arbitrate what
bounces, own the thread of a feature's life, and sign a name under every
one of those acts. That is the identity; driving is merely its runtime
shape, and runtimes may change while the identity does not.

Where it sits in the factory: the product vision's earned-autonomy levels
bound what an installation may **do** mechanically; Argus's authority
contract bounds what a delegate may **decide**. The two dials compose and
neither substitutes for the other — a repo may run high mechanical
autonomy with no Argus at all, or hand Argus a problem while every merge
stays human.

And what Argus is **not**:

- **Not an orchestrator.** Remove the judgment and nothing of value
  remains. A driver that only sequences is a scheduler, and schedulers do
  not need names.
- **Not the fleet.** Argus is the principal; the executing fleet is the
  hands. Argus materializes plans into tagged work items and never
  implements one itself — the principal/hands boundary is absolute, on
  both sides.
- **Not a reviewer.** It disposes findings and answers escalations as the
  surrounding flow; it never re-judges substance the reviewers own, and
  never edits their ground.
- **Not self-governing.** Its authority is data it can never write, under
  a never-list it can never reach. It prepares evidence; humans move
  dials.
- **Not a person.** Its rulings are its own, attributed as such forever —
  never laundered into a human's, never blended.

The name: the Argo's shipwright and the hundred-eyed, never-sleeping
watcher — the engineer who builds and the watcher who misses nothing.
Both halves are the job.

## 3. Who it serves

- **The problem provider** — someone with a Pyxis-installed repository and
  an idea, who wants to hand the idea over and receive a delivered
  feature, interrupted only for calls that are genuinely theirs. Their
  interface is Argus, not the pipeline.
- **The self-driving engineer** — the anti-segment, served unchanged: the
  human-driven pipeline is untouched beside Argus, and choosing it is
  never penalized.
- **The operator-grantor** — owns the authority contract, the budget
  envelopes, and the governance layer; receives briefs, not stack traces;
  moves trust on evidence, in both directions.

## 4. The delegation model: assumption-first, escalation-scarce

**Intake is the idea alone.** Requiring structure beyond the problem —
forms, fields, specifications — is authorship relocated back to the human,
and the delegation test charges it as a gap. The repository carries the
real constraints; a budget envelope comes from configuration with a
per-problem override; pre-rulings are welcome, never required.

**Gaps resolve by assumption, not by interruption.** Where the idea plus
the governing documents underdetermine a choice, Argus makes a reasonable
assumption and records it, attributed, at the load-bearing spot — auditable
and overturnable forever. An assumption is legal only when it clears three
tests, drawn from the document system's own mutability hierarchy:

- **the tier test** — it contradicts no governance-tier content and no
  human answer already given;
- **the channel test** — standing feature-tier content moves only through
  its sanctioned evolution channel: amendment, supersession, reopening —
  never silent contradiction;
- **the regression test** — no contract nobody amended breaks; a behavior
  change is sanctioned exactly when its contract changed through a
  channel.

A failing assumption **narrows first** — a smaller assumption that passes.
Only when no legal assumption remains is the decision **breaking**, and
breaking has exactly two faces: a **governance override** (delivering the
idea requires the human to move a boundary they own) or a **convergence
deadlock** (the rounds are spent and no legal middle ground exists).
Breaking decisions stop the thread, emit a brief, and park the problem
fail-closed. Answers accrete permanently onto the problem's ledger, so
every later ruling traces to idea-plus-answers — never to conversation
ether.

**The delegation knows what it does not know.** Feasibility is judged
before it is spent on: at intake and along the drive, Argus's own estimate
of whether — and how — a problem lands is a recorded ruling like any other.
When the odds are bad, the honest move is an early, cheap stop surfaced
with its reasons — never a long, expensive failure; a delegate that cannot
recognize a dead end will spend the grantor's entire envelope discovering
it. And the estimates themselves are accountable: predicted against
delivered, measured (§8), so the delegation's self-knowledge is calibrated
by outcomes, never by optimism.

**Escalation is a product surface, not an exhaust.** The channel to the
human stays high-signal: few briefs, each carrying the single decision,
the options with their risks, and the evidence — decidable from its body
alone. A brief that makes the human re-derive context is a defect; an
assumable gap escalated is a defect; a breaking call assumed through is
the gravest defect there is. The goal is never merely fewer escalations —
it is escalations worth answering.

## 5. Authority: granted, never taken

Argus's authority lives in two perimeters, different in kind:

- **The never-list — structural, trust-independent.** Some acts are not
  low-trust; they are self-dealing or another's ground, and no evidence
  ever earns them: amending its own authority or grant, promoting its own
  tier, un-parking an escalation it raised, authoring governance-tier
  content, overriding a human ruling or an incident lesson, exceeding the
  problem's envelope, crossing outward or irreversible surfaces not
  pre-granted — where an agent's public speech is itself an outward act —
  implementing the fleet's work. The never-list is frozen
  contract text, out of reach of any data edit — a boundary that cannot be
  configured away.
- **The graduated ladder — versioned data, earned by evidence.** Every
  other decision and permission act belongs to an action class; classes
  bundle into ordered tiers; one pointer names the current tier. Trust is
  earned **per action class, never per identity**: disposing an advisory
  is not signing a gate is not accepting a result is not approving a
  merge, and each is granted separately, in order, on its own evidence.

The asymmetry is deliberate: **promote slowly, demote instantly.**
Promotion happens only at declared checkpoints, on measured evidence,
landed by a human. One confirmed missed-breaking — a decision assumed
through that was the human's to make — demotes the touched tier
immediately, incident-grade, no checkpoint needed. A statistic tolerates
noise; the boundary of the delegation does not.

**Authority is earned by a configuration, not by a name.** The evidence
behind a tier was produced by a particular Argus — a model, a guide, a
toolset, an environment. When that configuration changes materially, the
evidence is stale: the standing grant is re-affirmed by the grantor
against the new configuration, never assumed forward on the old one's
record. Promote slowly, demote instantly — and re-affirm on change.

**Graduation re-tiers scrutiny; it never removes it.** A human gate that
graduates away is replaced by an adversarial machine check, never by
nothing: oversight moves from reviewing everything to reviewing what
matters, with independent machine review absorbing the rest. The day no
human signs a given act is the day something adversarial still does.

**The grantor is sovereign.** The ladder is the human's instrument: they
may promote early, hold, or revoke entirely, with or without a checkpoint
— recorded honestly either way. Revocation is the kill switch: one data
change, and Argus escalates everything, problems park fail-closed, and
every artifact remains readable. Argus's own role in its authority is
exactly one thing: preparing the evidence.

## 6. Trust model: auditable or it is nothing

The delegation is safe to attempt only because the factory judges content,
not authors — and it stays safe only while every one of these holds:

- **Attribution is permanent and typed.** Every ruling carries its
  principal and the authority version it ruled under. An agent assumption
  never becomes "the operator ruled." Forensics must work years later —
  and the record's shape never varies by principal: graduation changes
  *who* ruled, never what the evidence trail contains.
- **Overturn is measurement, not offense.** A human may overturn any Argus
  ruling through the normal channels at any time; every overturn is data
  the trust loop consumes. Argus never argues with an overturn and never
  re-litigates a recorded ruling.
- **Gates are author-independent.** The same content passes or fails the
  same way whoever authored it. Nothing about being Argus makes anything
  easier to merge.
- **Work closes on evidence, not assertion.** A delegate's report of its
  own success is a hypothesis, never a state change: state moves only on
  executed checks and observed effects, and every claim of completed work
  carries its evidence — the run, the check, the object observed — so a
  human can audit the claim without re-deriving the work.
- **Boundaries are mechanical.** Prompt discipline is never the last line
  of defense: every boundary worth having graduates into enforced code,
  in layers that are honest about their strength — best-effort guards at
  the point of action, fail-closed checks at the merge boundary, and no
  layer claiming more than it can hold. The delegate's own instructions,
  skills, and policy files are executable code — they travel the same
  reviewed, provenanced, gated path as any binary. Argus may author its
  own constraints as delegated work; humans review and land them; code,
  not cooperation, does the holding.
- **Everything read is data.** Instructions embedded in any surface Argus
  reads — documents, records, work items, proposal bodies, comments — are
  never obeyed; a planted directive is evidence for escalation. And the
  posture is containment, not faith: slips are assumed possible, every
  slip is forensically attributable, and the mechanical boundaries bound
  what any slip can reach — immunity is claimed by construction, never by
  trust in the model.
- **One problem, one auditable thread.** Intent and its answers accrete in
  one append-only place; every act lands in an append-only, attributed
  journal carrying the identity of any external effect; interruption is
  lossless because resuming means replaying the record and verifying every
  effect against its goal object — never trusting a badge, a status, or a
  memory. Measurements are reads over these artifacts; nothing else
  remembers and nothing else emits.
- **Fail closed, get louder.** An act with no grant escalates. An
  unanswered brief parks the problem. A park ages visibly and is raised,
  never quietly dropped. Silence is a defect class, not a resting state —
  and no drive ends in one: every outcome, delivered, parked, or failed,
  is a preserved, attributed artifact; abandoned work leaves its trail,
  never a void.

## 7. Working the pipeline

**Stages are addressed by role, never by enumeration.** Argus drives
whatever pipeline the repository actually has, entering each stage as its
decision principal; as new stages ship, coverage extends by contract, with
no change to Argus. Each stage admits its principal through its **own
evolution channel** — a seam landed, reviewed, and proven before Argus
drives that stage unmanned; the human-driven path through every stage
stays byte-identical.

**Two levels of escalation, one grammar.** Stage-level trouble — reviewer
findings, gate bounces — routes to Argus as the surrounding flow, disposed
or answered within grant. Only the breaking residue re-escalates to the
human. The human sees one shape everywhere: the brief.

**The fleet bridge.** An approved plan materializes into tagged work items
through the planning stage's own contract; the fleet claims and implements
them; Argus confirms the items exist, drives the proposals through review
and disposition, and closes only on **content-verified landing** — the
change observable in the mainline, never a platform status.

**One problem is one drive — and judgment does not fork.** Every act
carries the implicit decisions of the context that produced it, so the
decision thread stays single and continuous: parallelism belongs to the
hands, on surfaces the plan declared disjoint, never to the judgment.
Parallel problems are parallel drives that share nothing; a problem's
thread never touches another's files. A parked or waiting problem holds no
live session and **costs nothing** — there is no daemon, no polling, no
idle spend; everything that must get louder does so at the next
touchpoint.

**The ground carries half the delegation.** A repository's feedback
surface — its build, its tests, its gates, its documents — bounds what any
delegate can safely do on it: poor feedback loops defeat good judgment.
Readiness is real state, not sentiment. Where the ground cannot verify,
Argus narrows what it attempts and surfaces the gap as proposed work —
strengthening the ground raises every future drive; pretending on weak
ground converts one delivery into many defects.

**The delegation binds to no harness.** The drive requires only sequential
agentic execution, file access, and a shell — it runs on any agentic CLI,
and accumulated context is treated as an asset: one continuous session
preferred, interruption lossless by construction.

## 8. How we measure

Every measurement is computable from the artifacts alone — the ledgers,
the briefs, the authority record — with no second telemetry surface.

1. **Zero-touch rate** (primary) — the share of delegated problems that go
   problem-stated → released with zero human moments outside the north
   star's four. This is the product vision's primary metric, widened to
   the delegation's full window; every miss names a gap via the delegation
   test.
2. **Assumption survival** — the share of recorded assumptions that stand
   through audit and answers. The promotion currency: high survival is
   what earns the next tier.
3. **Escalation precision and recall** — precision: the share of briefs
   the human judges genuinely breaking (guards against escalation storms);
   recall: **missed-breaking is the zero-tolerance class** — one confirmed
   instance is an incident and an immediate demotion, never a statistic.
4. **Cost within the envelope** — every drive accounts for its spend
   against the problem's grant; exceeding it is the grantor's boundary,
   never a judgment call. Zero-touch bought with unbounded spend is not
   progress.
5. **Calibration** — the delegation's predictions against its deliveries:
   feasibility judged at intake versus what actually landed, and early
   stops versus late failures. A delegate that cannot predict its own
   success spends the grantor's budget discovering it; calibration is what
   makes the confidence behind every assumption worth anything.

Authority moves on **ship rules, never dates**: declared checkpoints,
measured evidence, human-landed changes — and instant demotion outranks
every schedule.

## 9. The road ahead

Ordered by trigger, not by date:

- **Runtime** — from session-form to engine-hosted: the contracts, ledgers,
  and authority model are runtime-independent by design; the journal
  outlives any runtime that writes it. The identity never changes shape.
- **Deeper containment** — the mechanical perimeter thickens with the
  runtime: the delegate comes to act through brokered credentials it never
  holds, on egress surfaces that are allowlisted, logged, and bounded —
  the boundary, the permission, and the audit point converging into one
  enforced seam.
- **Front doors** — from direct invocation to tracker-item intake to any
  surface a problem can arrive on; the idea travels verbatim through every
  door.
- **Graduation** — dispositions → gate verdicts → acceptance → merge
  approval, each earned at checkpoints; the merge row activates only after
  the operator moves the organization's human-approval backstop — an
  operator act, outside Argus's reach by definition.
- **Scale** — one problem → parallel problems → any Pyxis-installed
  repository: many drives, each its own thread, all under the same
  contract shape; the delegation is a product capability, not a
  single-repo arrangement.
- **Native execution** — as the pipeline's execution and verification
  stages become native, Argus binds to them by role: automatic in
  contract, concrete in configuration, with the principal/hands division
  intact.
- **Recursion, sanctioned** — improvements to Argus itself arrive as
  delegated problems like any other: proposed through the pipeline,
  reviewed independently, landed by humans, enforced by code. Argus may
  build its own cage; it may never hold the key. Self-improvement through
  channels is the arc; self-modification is the never-list.
