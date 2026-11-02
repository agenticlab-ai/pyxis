# prototype — PRD

| Field | Value |
|---|---|
| **In one line** | Lets whoever hands Pyxis an idea ask for a throwaway prototype first, and carries what the prototype taught into the feature's draft. |
| **Relevant-when** | Planning, building, or reviewing the prototype step or its findings record; triaging a prototype run that failed or findings that never reached a contract; changing the feature draft's brainstorm working files or the authoring guides that read them. |
| **Status** | Draft — exact-head lock approval pending |
| **Tier** | T1 — one session of model work spent on findings nobody uses |
| **Owner** | Argus (drive `prototype`) for the operator @mdarabi |
| **Approver(s)** | @mdarabi — the exact-head contract lock (problem:prototype#A-1) |
| **Contributors** | — |
| **Informed** | — |
| **Date** | 2026-10-10 created · 2026-10-10 last updated |
| **Appetite** | The operator's review of this contract; a budget of 5 plan tasks, current estimate 5 (raised from 4 when the operator's ruling added FR-6); no rollout burden, because the step is optional |
| **Target release** | The first Pyxis release after its tasks land; nothing else waits on it |
| **Tracking** | `docs/problems/prototype/delivery.md` |
| **Artifacts** | `drafts/prototype/brainstorm/001.md` |
| **Template version** | prd-template/v2 |
| **Design coverage** | requirements-first/v1 |

## 0. Read to rule — the human reviewer's section (~3 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | Prototype code is thrown away: never merged, never the starting commit of the implementation | Start the implementation from the prototype | A veto puts code with no contract and no review under the feature → NG-1, NR-1 |
| 2 | A prototype happens only when the idea's provider asks | Argus decides when an idea is uncertain enough | A veto pulls WR-1 into §5 → FR-1 |
| 3 | The two reviews check that every finding was answered; the lock has no mechanical check for it | The lock refuses while a finding is unanswered | A veto pulls WR-2 into §5, about one more task → FR-5 |
| 4 | Tier T1, and a five-task budget, raised from four when FR-6 was added | T2 with metrics, phases, and an eval set; or four tasks by cutting scope | A veto on the tier restores the skipped sections and WR-4; a veto on the budget means choosing what to cut → header, §3.4 |

**Sanity-check the numbers** — task budget 5, estimate 5 (header Appetite); the post-launch review fires after three prototyped features (§6.7).

**The author's open questions** — none — Q1 and Q2 are design questions, answered by the tech spec (§7.2).

**Skip the rest** — structure → draft readiness; requirement quality and coherence → the PRD review; the build itself → the tech spec, its review, the code gate, and implementation review.

## 1. Summary — the one-pager

- **Problem** — Pyxis writes and locks a feature's contract before any of the feature is built. What is hard to build, and where the idea collides with what already exists, shows up afterwards: in design review, mid-implementation, or after the lock.
- **For whom** — Whoever hands Pyxis an idea whose difficulty they cannot judge in advance. Today that is the operator, with Argus driving for them.
- **Today** — One operator on one machine, working in this repository and in repositories that install Pyxis. Each idea goes through one feature draft, driven by Argus or by hand.
- **Why now** — Argus now drives an idea to a locked contract with less human reading than before, and the last contract written with nothing built was cut by hand after its plan froze (incident I13).
- **Direction** — On request, Pyxis builds a throwaway version of the idea in a worktree of its own before the PRD is written. A Pyxis command launches the build as a separate headless run, on the harness and model the repository configures for prototypes, or on those of the session that asked when none is configured. Pyxis records what the build learned as findings: technical challenges and product conflicts. The draft then answers each finding, and the prototype's code is discarded.
- **Appetite** — Five plan tasks and the operator's review. It excludes Argus choosing to prototype on its own, any mechanical lock check on findings, running prototypes through the implement-now flow, and prototypes outside a feature draft.
- **Success** — On the first feature drafted with a prototype, every finding is answered in the locked contract or set aside with a reason (DOD-2).
- **Guardrail** — No prototype change reaches mainline, and a draft without a prototype behaves as it does today (NR-1, NR-2).
- **Top risks** — The prototype grows into a second implementation (R1); findings are written and then ignored (R2); the bet itself is wrong, and a prototype surfaces nothing a paper review would miss (R3).

## 2. Problem space

### 2.1 Problem statement

The person handing over an idea, and Argus driving it, decide requirements and design on paper. Technical obstacles and conflicts with existing behavior are found later, when a design session cannot build a requirement, a reviewer finds a contradiction, or a worker is already implementing. Each late discovery costs a ruling, a revised contract, or rework, and the last draft written this way was cut by hand after its plan had frozen (brainstorm:prototype#F-4, brainstorm:prototype#F-5).

### 2.2 Users and segments

Skipped: T1 — the segment and the anti-segment are stated in §1 and in brainstorm:prototype#D-2.

### 2.3 Jobs and desired outcomes

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | I hand Pyxis an idea and cannot judge how hard it is or what it collides with | Have Pyxis try building it first | See the technical challenges and product conflicts before the contract is written | Find them during design review, implementation, or after the lock | brainstorm:prototype#D-3 |

### 2.4 Evidence

Skipped: T1 — the two supporting facts are cited in §2.1.

### 2.5 The journey today

Skipped: T1 — the journey is the existing pipeline map.

### 2.6 Opportunity and why now

Skipped: T1 — stated in §1.

**Decision:** Problem gate — signed via the operator's exact-head lock review.

## 3. Goals, metrics, and guardrails

### 3.1 Goals as falsifiable hypotheses

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | Building a small throwaway version of an idea before its contract is written | The idea's technical challenges and product conflicts being answered in the contract instead of found after it locks | DOD-2 on the first prototyped feature; the §6.7 review after three |

### 3.2 Goals → signals → metrics

Skipped: T1 — success is the acceptance criterion DOD-2.

### 3.3 Guardrails and decision rule

Skipped: T1 — the guardrails are NR-1 and NR-2, each a blocking DoD row.

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | Merging prototype code, proposing it for merge, or starting the implementation from it | Landed code follows a locked contract, and a prototype has no acceptance criteria and no review (brainstorm:prototype#O-1) |
| NG-2 | Requiring a prototype, or failing any gate because a draft has none | The idea's provider chooses it (brainstorm:prototype#O-2) |
| NG-3 | Settling a product question inside the prototype | A prototype raises questions; rulings stay with the operator, or with Argus under its grant (brainstorm:prototype#O-3) |
| NG-4 | Running prototypes through the implement-now flow | Its workers must pass the full code gate and publish a proposal, and neither fits throwaway code (brainstorm:prototype#O-5) |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Argus starts a prototype nobody asked for when it judges an idea uncertain (brainstorm:prototype#O-4) | At the §6.7 review |
| WR-2 | The lock refuses while a finding is unanswered. Without it an unanswered finding is caught only by the two reviews. About one task | When a contract locks with an unanswered finding |
| WR-3 | Choosing the model level for each prototype on the request, and metering its spend. Without it one configured profile serves every prototype and the only bound on a build is time. About two tasks | When one configured profile proves wrong for some prototypes |
| WR-4 | An eval set for the prototype step. Without it the step's quality is judged by demonstration | At the §6.7 review |
| WR-5 | Running a prototype without an open feature draft, with its findings stored outside a draft (brainstorm:prototype#O-6) | When something other than a feature draft needs a prototype's findings |

## 4. Solution space

### 4.1 Approach and alternatives considered

When the provider of an idea asks for a prototype, Pyxis builds the smallest version that exercises the idea, in a worktree of its own, before the PRD is written. A Pyxis command launches that build as a separate headless run: on the harness and model the repository's configuration names for prototypes, or, where it names none, on those of the session that asked. Pyxis then writes a findings record into the feature's draft. Each finding is a technical challenge or a product conflict, with what was tried and what was observed. The later authoring stages answer every finding, in a PRD row, a tech-spec decision, or a ruled requirement problem, or set it aside with a reason. The feature is then locked, planned, and implemented as usual, from the contract and not from the prototype's code.

| Option | Sketch | Why not chosen |
|---|---|---|
| A (chosen) | The prototype is evidence for the draft; its code is discarded | — |
| B | The prototype's code becomes the first commit of the implementation | Puts code with no acceptance criteria and no review under the feature (brainstorm:prototype#D-1) |
| C | A standalone prototyping command tied to no draft | The learning has no route into the contract (brainstorm:prototype#D-1) |
| D | Build the prototype with the implement-now flow | Its workers must pass the full code gate and publish a proposal; the operator agreed to keep prototypes out of it (brainstorm:prototype#D-10) |
| E | The session that asked builds the prototype itself | The operator wants Pyxis to control which harness and model execute a prototype (brainstorm:prototype#D-10) |
| Do nothing | — | Obstacles and conflicts keep surfacing after the contract is written (§2.1) |

### 4.2 Scope perimeter

- **In (plan of record)** — a prototype on request, built in its own worktree by a headless run a Pyxis command launches on the configured harness and model, or on the asking session's when none is configured; a findings record in the draft; every finding answered or set aside before the lock.
- **Future (believed next, not committed)** — WR-1 to WR-5.
- **Out** — NG-1 to NG-4.

### 4.3 Experience: flows and states

Skipped: T1 — the one flow is the paragraph in §4.1.

### 4.4 Scenarios

Skipped: T1.

**Decision:** Solution gate — signed via the operator's exact-head lock review.

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When the provider of an idea asks for a prototype, Pyxis shall build a prototype of the idea before the feature's PRD is authored. | "when we give an idea we can also prototype it first" (brainstorm:prototype#D-5) | Must | low | On a feature whose provider asked for a prototype, the draft's history shows the findings record committed before the first authored PRD content; on a feature whose provider did not ask, no prototype is built | Demo | G1, J1 |
| FR-2 | When Pyxis builds a prototype, Pyxis shall build it in a Git worktree of its own, on a branch that belongs to neither the feature's draft nor mainline. | "pyxis will go implement it in a work tree" (brainstorm:prototype#D-6) | Must | medium | After a prototype run, every file the prototype changed is on the prototype's own branch, and the draft branch and mainline contain none of those changes | Test | G1, J1 |
| FR-3 | When a prototype run ends, Pyxis shall record each thing the run taught as a finding of kind technical challenge or product conflict, stating what was tried and what was observed. | "try to understand what are the challenges technical challenges where the product conflicts that we need to think through" (brainstorm:prototype#D-7) | Must | low | The run's findings record holds at least one finding, or the explicit statement that the run found none; every finding carries its kind, what was tried, and what was observed | Inspection | G1, J1 |
| FR-4 | If a prototype does not reach a working state, then Pyxis shall record what stopped the build as a finding. | The purpose is to understand the challenges, and the hardest is often what stopped the build (brainstorm:prototype#D-9) | Must | low | A run on an idea that cannot be built in the repository ends with a findings record whose findings name the obstacle | Demo | G1, J1 |
| FR-5 | While a feature's draft holds prototype findings, Pyxis shall answer each finding with a PRD row, a tech-spec decision, or a ruled requirement problem, or mark the finding set aside with a reason, before the contract locks. | "you apply that learning when you're going … through the feature draft and the actual implementation" (brainstorm:prototype#D-8); implementation receives the learning through the locked contract | Must | low | At the prepared lock head, every finding names the row, decision, or problem that answers it, or carries a set-aside reason; zero findings are unanswered | Inspection | G1, J1 |
| FR-6 | When a prototype is requested, a Pyxis command shall launch the prototype's build as a separate headless run on the harness and model that the repository's configuration names for prototypes, or, where the configuration names none, on the harness and model of the session that asked. | "it should be a cmd in the pyxis ecosystem that based on the config value (if it exists) decides on the harness and the model; if there is no configured value, then it falls back to whather the ambient (current running harness and model)" (problem P-1 in this draft, ruled by the operator; brainstorm:prototype#D-10) | Must | medium | With a prototype configuration naming a harness and a model, the command launches the build on exactly that harness and model and reports both; with no prototype configuration, the command launches the build on the harness and model the asking session passed and reports both; in neither case does the asking session build the prototype itself | Test | G1, J1 |

**Non-regression (NR-n)**

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Code reaches mainline only through a change implemented from a locked contract and approved at its exact head | A prototype has no acceptance criteria and no review; this rule is what makes landed code trustworthy | medium | No prototype commit is an ancestor of mainline; no pull request is opened from a prototype branch; the draft pull request holds no path outside the feature's draft, problem, and memory records | Inspection | G1 |
| NR-2 | A feature drafted without a prototype passes through its stages, readiness, and lock as before | The prototype is optional (brainstorm:prototype#D-5) | low | The existing draft, readiness, and review test suites pass unchanged, and a draft with no findings record meets readiness with no prototype-related condition | Test | G1 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | A prototype run fails for a reason no other row names | Stop the run, record the cause, and report the failure, without retrying, repairing, or guessing | The failure and its cause; the draft continues without that run's findings | Must | low | An injected failure to create the prototype's worktree ends the run with a reported cause and leaves the draft branch unchanged | Test | J1 |

### 5.3 Permissions and abuse cases

Skipped: T1 — no new role; a build runs with the login of the harness that launches it, as implementation runs do, confined to the prototype's worktree.

### 5.4 Data

Skipped: T1 — the only new datum is the findings record, a draft working file kept and compiled like the draft's other records. A build sends the idea and the code it reads to its harness's provider, as implementation runs already do.

### 5.5 Interfaces and integrations

Skipped: T1 — the one boundary crossed is the harness that runs the build, through the engine's existing harness adapters; prototype branches stay on the local machine.

### 5.6 Non-functional requirements

Skipped: T1 — none asked for.

### 5.7 Instrumentation — the tracking plan

Skipped: T1 — no metric in §3.2.

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | A prototype asked for with an idea is built in its own worktree and yields a findings record before the PRD is authored | FR-1, FR-2, FR-3 | Demo | On the first feature drafted with a prototype request: its findings record, the prototype's branch and head commit, and the draft history showing the record ahead of PRD content | yes |
| DOD-2 | Every finding of that feature is answered or set aside at its locked contract | FR-5 | Inspection | That feature's findings record at the locked head, with each finding's answer or set-aside reason, and both review reports at the locked digest | yes |
| DOD-3 | A prototype that cannot be made to work reports what stopped it | FR-4 | Demo | One run on an idea chosen because it cannot be built in the repository, and its findings record | yes |
| DOD-4 | No prototype change reaches mainline or the draft pull request | NR-1 | Inspection | For the first prototyped feature: mainline ancestry holds no prototype commit, and the draft pull request's file list holds only the feature's draft, problem, and memory records | yes |
| DOD-5 | Drafts without a prototype are unaffected | NR-2 | Test | The existing draft, readiness, and review suites passing in the implementing pull requests' required checks | yes |
| DOD-6 | A failed run stops and reports its cause | UB-1 | Test | A test that injects a worktree-creation failure and observes the reported cause and an unchanged draft branch | yes |
| DOD-7 | The prototype's own branch holds every prototype change | FR-2 | Test | A test that creates a prototype worktree for a draft and observes its branch, its base on mainline, and an unchanged draft branch | yes |
| DOD-8 | The command launches the build on the configured harness and model, and on the asking session's when none is configured | FR-6 | Test | Tests that resolve the route with and without a prototype configuration and that launch a stub harness on the resolved model; on the first prototyped feature, the command's report naming the route used | yes |

### 5.9 AI behavior and evals

Skipped: T1 — the step's model-driven behavior is verified by demonstration (DOD-1, DOD-3); an eval set waits in WR-4.

## 6. Delivery

### 6.1 Phasing and milestones

Skipped: T1 — one release, no phases.

### 6.2 Dependencies

Skipped: T1 — none.

### 6.3 Risks, assumptions, and rabbit holes

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R1 | The prototype grows into a second implementation and costs more than it teaches | M | M | The step builds the smallest version that exercises the idea and stops at the first working path or the first obstacle understood; the build is bounded in time; its findings record states what was left out | Argus |
| R2 | Findings are recorded and then ignored by the contract | M | H | FR-5, checked by both reviews; WR-2 adds a mechanical check if a contract locks with an unanswered finding | Argus |
| R3 | Assumption: a prototype built without a contract surfaces conflicts a paper review would miss | M | M | DOD-1 and DOD-2 on the first prototyped feature, then the §6.7 verdict | @mdarabi |

### 6.4 Release criteria

Skipped: T1 — folded into §5.8's blocking rows.

### 6.5 Rollout and reversibility

Skipped: T1 — the step is optional; not asking for a prototype is the off switch (NR-2).

### 6.6 Launch readiness

Skipped: T1.

### 6.7 Post-launch review

- **Review trigger** — three features drafted with a prototype · **Owner** — @mdarabi
- **Verdict recorded** — adopt / iterate / kill, on the header's Tracking ref, with how many findings each prototype raised and how many changed its contract.

## 7. Alignment

### 7.1 FAQ

Skipped: T1.

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | How long are a prototype's worktree and branch kept after its findings are written, and who removes them? (brainstorm:prototype#G-1) | Argus | Solution gate | answered — kept on the local machine, unpublished, until the feature's draft locks or is abandoned, then removed with the same command that created them (design:prototype#D-1) |
| Q2 | Does the session that asked for the prototype build it, or does a separate worker? (brainstorm:prototype#G-2) | Argus | Solution gate | answered — a separate headless process always builds it, launched by a Pyxis command on the configured harness and model, or on the asking session's when none is configured (design:prototype#D-2) |

### 7.3 Decision log

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-10-10 | The prototype is evidence for the draft and its code is discarded | Start the implementation from it; a standalone tool | The idea separates the prototype from the real implementation | Argus for the operator (brainstorm:prototype#D-1) |
| 2026-10-10 | A prototype happens only on request and before the PRD | Always; when Argus judges it useful | "we can also prototype it first" | Operator (brainstorm:prototype#D-5) |
| 2026-10-10 | Findings are of two kinds and state what was tried and observed | Free-form notes | The idea names technical challenges and product conflicts | Operator; Argus for the observation clause (brainstorm:prototype#D-7) |
| 2026-10-10 | Each finding is answered in the draft or set aside with a reason; implementation receives it through the contract | A separate channel to workers | The idea names no channel, so the existing one serves | Argus for the operator (brainstorm:prototype#D-8) |
| 2026-10-10 | Tier T1, four-task budget; the budget is superseded two rows below | T2 with metrics and phases | A simple, optional, reversible step | Argus for the operator (brainstorm:prototype#D-4) |
| 2026-10-10 | A Pyxis command launches every prototype as a headless run, on the configured harness and model or on the asking session's; prototypes stay out of the implement-now flow and inside a feature draft | The asking session builds; reuse implement-now with no reviewer; standalone prototypes | The operator wants control of what executes a prototype | Operator (problem:prototype#A-2, problem:prototype#A-3; brainstorm:prototype#D-10, brainstorm:prototype#D-12) |
| 2026-10-10 | Tier T1 kept; budget raised to five tasks for FR-6 | Keep four tasks by cutting scope | The ruling added two tasks | Argus for the operator (brainstorm:prototype#D-11) |

### 7.4 Changelog

| Date | Change | Author |
|---|---|---|
| 2026-10-10 | First draft from brainstorm record 001 | Argus |
| 2026-10-10 | Operator review of the prepared lock: FR-6, NG-4, WR-5 and DOD-8 added, WR-3 rescoped, budget raised to five | Argus, on the operator's rulings |

## 8. Contract readiness — the executability gate

| Check | Pass? | Notes |
|---|---|---|
| Problem basis accepted — §1 problem accepted (T1); §2 problem gate signed (T2/T3) | ☐ | Awaits the operator's exact-head lock review |
| Zero solution-blocking TBDs or open questions (§7.2) | ☑ | Q1 and Q2 answered by the tech spec |
| Every Must FR has a fit criterion and a verification method (§5.1) | ☑ | |
| Every requirement row carries a risk tier; high/critical rows have a declared safety plan — staged rollout, rollback rehearsal, kill switch, migration fallback, manual recovery, or an explicit non-applicability rationale (§5.1, §6.5) | ☑ | No high or critical row |
| Non-goals drawn (§3.4); non-regression rows named (§5.1) | ☑ | |
| Every primary job (§2.3) is covered by a requirement, a non-goal, or a waiting-room entry | ☑ | J1 by FR-1 to FR-6 |
| Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ☑ | Vacuous: §4.3 skipped at T1 |
| Every primary metric (§3.2) has instrumentation (§5.7) or an explicit measurement plan | ☑ | Vacuous: §3.2 skipped at T1 |
| Every blocking DoD row traces to a requirement, guardrail, release criterion, or human-judgment rubric (§5.8) | ☑ | |
| Risks and assumptions have owners (§6.3) | ☑ | |
| Human-judgment criteria name an owner and a rubric (§5.8) | ☑ | Vacuous: none |
| DoD complete: every blocking row sourced and evidenced; every n/a explained (§5.8) | ☑ | |
| Every FR, UB, and NFR row names its origin (§5.2's default failure row aside); rows nobody asked for wait in §3.4, not in §5 | ☑ | |
| The author's task estimate fits the appetite's task budget (header) | ☑ | 5 of 5 |
