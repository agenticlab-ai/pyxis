# Feature draft — brainstorm record

| Field | Value |
|---|---|
| **In one line** | A memory-native "feature draft": one revisable workspace per new feature in which the PRD, the technical design, and the plan co-evolve through product↔engineering feedback rounds and lock together once — replacing the per-stage PR-and-freeze document pipeline and relaxing the gates that enforce it. |
| **Relevant-when** | brainstorming or contracting how a new feature's PRD, design, and plan are authored, revised, reviewed, and locked; retiring or relaxing the document pipeline (per-stage PRs, merge-as-freeze, `/prd-amend` bounces, docgate FR-27/FR-30, contract-gate and plangate precheck) in favor of `memory/` records; checking prior rulings on joint PRD+design+plan approval, feature lifecycle states before planning, or intake lanes (bug fix, performance) that converge on a plan |
| **Idea** | "As we are moving away from using the documents as a source of the memory for Pixis to the new memory system, which actually has a better scalability and accuracy, we need to create some sort of a new state in the Pixis, which let's call it feature draft. Right now, we are going through this process of brainstorm, PRD, PRD review, and take a spec, take a, uh, take a spec, brainstorm, take a spec, review, and then planning. And any change to any, uh, and it happens in a kind of a sequential way without any feedback loop. And if it requires any feedback, meaning that, oh, in the design process, we learn something needs to be updated on the PRD, we have to raise another PR. In the GitHub to update it because after each step we are raising PR and we assume that oh it's done but the reality is that it's a kind of a back and forth process in the real world product managers write a PRD then when they talk to the engineers they learn oops there are certain uh, assumptions they made that are not incorrect or they're missing pieces so they go back and update the PRD and meanwhile engineering team also As the PRD gets updated, update the design, and this loop goes on until everyone agrees that, oh, this is the right version, and we capture everything properly. So we should have the same mindset. Nothing should be locked until this full loop of back and forth is done, and then we lock the PRD, take a spec, and the plan. And then we pass on everything for the development process to the execution plan. So. And also we need to uh, relax the code gate system to avoid checking the previous PRD and take aspect given that we are just uh, deprecating that process fully and replacing it with a new memory system. So focusing on that feature draft as a new process within the Pixis, tell me how far we are from that world and what are the main changes to me need to be done. And to be clear, this information later, this pack of information that we just created as a PRD take a spec, will be passed for the development along with the plans and uh, tasks that created. So for now, don't worry about the execution, how we are going to pass this information to the implementation and all this. So just focus on how we can create this feature draft process. And just to give you some more information, in the long run, We also might have a different process for fixing bugs. Not everything in a software engineering for a project is going to be a new feature or updating a feature. It just simply sometimes there's a bug, we need to update it. So that process rather than creating a PR to take a spec would be something else. So there will be multiple phases, steps, and the plan is the only place they are gonna all wrap into. So you can think of it, there is a input, what we are doing. Then there's a branching, oh, it's a feature development, debug, uh, uh, bug, uh, fixing bugs or performance, or whatever. Then everything will be done and then everything will be converted into a plan, which then passed on to the implementation process. So for now, we are just only focused on the feature draft process. There is a new feature, we want to build it. How to do so." |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-09-22 · 2026-09-23 (review: D-24 to D-26) |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | docs/prds/feature-draft/ |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/product-vision.md` (§1, §4, §7, §13) | governing ground: the feature contract as three inseparable obligations (PRD, design, definition of done); the stage grammar and its stage-3 input; document mutability; repository memory, "one pipeline grammar, elastic cost", no gate-bypass lane |
| `docs/architecture-principles.md` (AP-7, AP-21) | mutability ("immutable once the change that adds them is approved") and judge-once-at-the-richest-context |
| `.pyxis/pipeline.md` | the operational stage map: human gates at stages 2, 5, 9; merge as the freezing event for PRD and design |
| `docs/domain-model.md` (§3.1, §3.6, §5) | the Feature entity and its canonical lifecycle (PRD immutable at stage 2, before design) |
| `docs/wiki/glossary.md` | ledger, amendment, and gate vocabulary; "draft phase" already names a PRD's pre-merge phase; no memory-system vocabulary present |
| `docs/prds/README.md` + the 38 feature PRD headers | routing scan for overlapping capability |
| `cmd/memory/README.md` | the memory system's contract: typed immutable records, supersession, Documents, packet, render, pack, reconcile, closeout |
| `docs/tech-design/memory-staged-contracts.md` | the latest ruling on how a PRD and its design stage through memory; rejected bundling them in one PR |
| `docs/prds/docgate/effective.md` (FR-27, FR-30) · `docs/prds/plan-new/effective.md` (FR-1, FR-17, FR-27) · `docs/prds/tech-design-brainstorm/effective.md` (J3, FR-14, NG-6) | the merged rows that encode one PR and one freeze per stage, and the design-to-PRD bounce |
| `docs/brainstorms/` (63 records: one-liners, gates, out-of-scope rows) | prior refusals — none refuses a joint lock or a memory-native contract |
| `internal/memory/{packet,reconcile,record}`, `internal/docgate/gate`, scratch-clone probe of `memory-gate` at `9f2fbe98` | as-built gate behavior for a memory-native PRD and design (F-1, F-2) |

**Overlaps surfaced:**

- `prd:docgate` FR-27 / FR-30 — **direct conflict**: merged Must rows fail a diff that adds more than one ledger entry, or that bundles a PRD entry, a design, and implementation paths; a joint lock needs them superseded or confined to the legacy corpus.
- `prd:plan-new` FR-1 / FR-17 / FR-27 — **conflict**: planning accepts only an approved PRD projection plus an approved design, freezes the plan at its own approval, and requires a clean docs-only contract gate first.
- `prd:tech-design-brainstorm` J3 / FR-14 / NG-6 — **conflict in mechanism, agreement in purpose**: a product hole found during design must bounce formally to `/prd-amend` (a silent patch is a sev-1 defect) so "the PRD and the design tell one story"; the idea keeps one story by revising inside the draft, so the no-silent-patch guarantee must survive in a new form.
- `design:memory-staged-contracts` §4 — **prior rejection of bundling as an alternative**, on the ground that it contradicts the standalone-contract rule (FR-30) — it names the contract that must change rather than refusing the idea.
- `prd:contract-gate` + `plangate precheck` — **docs-only stage-6 readiness**: a memory-native feature cannot enter planning today.
- `cmd/memory/README.md` + `prd:parallel-memory` + the `memory-*` designs — **substrate, partially covered**: records, supersession, packet, render, and collision detection exist; no draft/lock lifecycle, no feature identity, and reconciliation subordinates memory to the documents (F-1).
- `docs/domain-model.md` §5 — **lifecycle divergence**: the canonical Feature lifecycle makes the PRD immutable at stage 2, before any design.
- `docs/product-vision.md` §4 — **possible tension, for the operator**: stage 3 takes "the approved requirement result" as input; a draft that designs before the PRD locks depends on how "approved" is read (§1 and AP-7 read compatibly).
- `prd:argus` — **driver adjacency**: Argus drives stage by stage and treats standing content as movable only by amendment, supersession, or reopen record.
- `prd:feature-implementation-drive` FR-29 + `prd:abandoned-feature-retirement` — **adjacent lifecycle**: feature states after planning (drive, abandonment, retirement) exist; the draft would be the lifecycle before the lock.

## Framings

- **1 — One contract, one approval** — the feature draft is a single revisable workspace per feature where the product side and the engineering side (human or agent) edit the PRD, design, and plan together, with reviewers as feedback rounds, and approve once; the approval locks all three together. · Private to its workspace until the lock (other work sees it only as in flight); one approval per feature contract; the smallest new state to build.
- **2 — A feature lifecycle the factory tracks** — the feature draft is a durable state on the mainline (draft → locked → reopened → retired) that every session and agent can see; draft edits land continuously but bind nothing until a lock seals a version. · Maximum visibility and shared editing; every consumer must learn to ignore drafts; each iteration still lands on the mainline.
- **3 — The feature lane of a general intake** — the feature draft is the first lane of an intake step that classifies incoming work (feature, bug, performance) and routes it to a lane; every lane ends by locking a plan pack for execution. · Builds the intake shape and the lane-neutral handoff now; larger first release; aligns with vision §13's one grammar with elastic instances.

**Destination:** framing 1, **one contract, one approval** — one revisable workspace per feature in which PRD, design, and plan change together through feedback rounds and lock together at one approval; the draft state is visible while the workspace is open and becomes locked at approval (D-1).

## Decisions

### D-1 — what the feature draft is

- **Question**: Which thesis to build toward — one workspace with one approval (1), a lifecycle state recorded on the mainline where every edit lands but binds nothing until a lock (2), or a general intake with the feature draft as its first lane (3)?
- **Answer**: Framing 1 — one contract, one approval.
- **Reasoning**: Recommended: it removes the per-stage PRs directly (on the `planner-driven-model-routing` case, 1 contract PR instead of 12) with the smallest new state, and the draft/locked state stays visible without landing drafts on the mainline; the intake of framing 3 is not the first build, matching the idea's "for now, we are just only focused on the feature draft process".
- **Anchor**: 2026-09-22 · turn 1
- **Status**: live

### D-2 — where brainstorm and review outputs live inside a draft

- **Question**: Besides the PRD, design, and plan, do brainstorm rulings and review findings become Q&A pinned to the requirement or decision they changed (A), stay as separate documents inside the draft (B), or disappear at the lock, leaving only the final result (C)?
- **Answer**: B — brainstorm records and review reports stay separate documents; all of them live inside the one draft and lock with it.
- **Reasoning**: Chosen over the recommended A; no reason stated in the turn. On the `planner-driven-model-routing` example the draft would carry its 5 brainstorm records and 6 review reports beside the PRD, designs, and plans, all in one proposal.
- **Anchor**: 2026-09-22 · turn 2
- **Status**: live

### D-3 — what must be true before a draft can lock

- **Question**: Is the lock refused until checks pass and then approved by a human (A), left to the approver's judgment with advisory checks (B), or automatic once checks pass (C)?
- **Answer**: A — the lock is refused until every open question is answered, every must-have requirement has a design decision and a pass/fail test, every requirement is covered by a plan task or explicitly deferred, and the final review of the version being locked has no unresolved blocking finding; only then is a human asked to approve.
- **Reasoning**: Recommended: it moves today's late bounces (e.g. `review-feedback-capture` re-cut by amendment 001 after the plan bounce) inside the draft, keeping the vision's human approval over new product and design judgment.
- **Anchor**: 2026-09-22 · turn 3
- **Status**: superseded by D-25 — plan coverage leaves the lock conditions and is checked when a plan is generated; the rest of D-3 carries into D-25

### D-4 — who the first version is for

- **Question**: Is the first version built for a solo operator who holds both the product and engineering sides and works with agents (A), for teams with separate product and engineering owners who must both approve (B), or for Argus running the loop by itself (C)?
- **Answer**: A — one human takes part in the rounds, agent sessions do the writing, Argus can drive when asked; one human approval locks the draft and records both roles held by that person.
- **Reasoning**: Recommended: it matches this repository's reality — one human, whose PR review is "the single real gate at this team size" (`argus-unattended-drive` status) — without shutting out Argus as a driver.
- **Anchor**: 2026-09-22 · turn 4
- **Status**: live

### D-5 — how a locked feature changes

- **Question**: After the lock, does a change reopen the feature as a new draft (A), use today's amendments, superseding designs, and new plans (B), or become a separate new feature that takes over the affected requirements (C)?
- **Answer**: A — any later change opens a new draft on the same feature, starting from the locked PRD, design, and plan, under the same rounds and lock rule; the old locked version stays in force until the new draft locks and replaces it; draft-born features use no amendments or superseding designs.
- **Reasoning**: Recommended: one loop serves both creation and change (e.g. `implement-now` amendment 002 becomes one reopened draft rather than three per-stage PRs), and the locked version keeps binding while the change is drafted.
- **Anchor**: 2026-09-22 · turn 5
- **Status**: superseded by D-25 — a reopen starts from the locked PRD and tech spec; a plan is not part of a locked version

### D-6 — switching from the document pipeline to drafts

- **Question**: At switch-over, cut over for new work and migrate existing features when they next change (A), run both processes side by side for as long as old features live (B), or convert all 38 existing features and remove the document pipeline at once (C)?
- **Answer**: A — from switch-over every new feature is a draft and the gates refuse new document PRDs and designs; a feature already in the old process moves into a draft the next time it changes, starting from its requirements already in memory; old documents stay frozen, readable history, and the old checks only guard them against edits.
- **Reasoning**: Recommended: it deprecates the old process fully without a big-bang conversion — the migration cost is paid per feature, only when one changes (e.g. `parallel-memory`'s pending contract review would be its first draft).
- **Anchor**: 2026-09-22 · turn 6
- **Status**: superseded by D-26 — existing features are not migrated in this release; the cut-over for new work stands

### D-7 — the bet and its success signal

- **Question**: Is the draft judged first by contract PRs from idea to a locked plan with a quality guardrail (A), by calendar time to a locked plan (B), or by how often a locked feature must be reopened (C)?
- **Answer**: A — primary: pull requests from idea to a locked plan, target one per draft or reopen; guardrail: changes needed after a lock because of a gap the lock checks should have caught, target none.
- **Reasoning**: Recommended: churn, not speed, is the measured pain — `planner-driven-model-routing` reached its first plan in 3 days but through 9 PRs, then needed 2 more plans (F-11); the guardrail keeps fewer PRs from meaning a weaker contract.
- **Anchor**: 2026-09-22 · turn 7
- **Status**: superseded by D-25 — the primary signal counts proposals from a draft's opening to its lock

### D-8 — what stays out, and the guarantees that stay in

- **Question**: Are the execution handoff, the intake step with other lanes, and governing-document edits out while execution keeps working and the locked result stays lane-neutral (A), out with a feature-only locked result (B), or is the execution handoff built here too (C)?
- **Answer**: A — out: changing how work is handed to implementation, the intake step and other lanes (bug fix, performance), and any edit to the product vision or principles; in, as guarantees: a locked draft's plan feeds today's execution unchanged, and the locked result carries no feature-only parts, so a later bug-fix lane could produce the same shape.
- **Reasoning**: Recommended: it follows the idea's "don't worry about the execution" while keeping the plan as the one place later lanes converge, and nothing downstream breaks at switch-over.
- **Anchor**: 2026-09-22 · turn 8
- **Status**: superseded by D-25 — a plan generated from a locked draft feeds today's execution unchanged; the rest of D-8 carries into D-25

### D-9 — who may change a requirement while a draft is open

- **Question**: When design finds a requirement wrong inside a draft, does the design session flag it for a product-side ruling (A), edit it directly and leave acceptance to the lock review (B), or may anyone edit anything with only the final version reviewed (C)?
- **Answer**: A — the design session records the problem; the requirement changes only when the product side rules (the operator, or Argus when its authority allows), with who and why recorded; the lock review lists every requirement changed during the draft.
- **Reasoning**: Recommended: it keeps the purpose of today's bounce rule — the PRD and design never quietly disagree, and new intent stays the product side's call — while replacing the separate amendment PR (e.g. #737 for routing FR-2) with a ruling inside the same draft.
- **Anchor**: 2026-09-22 · turn 9
- **Status**: live

### D-10 — open drafts that touch the same requirements

- **Question**: When two open drafts touch the same requirements, do they see each other and re-check on lock (A), does the second wait until the first locks (B), or is there no coordination (C)?
- **Answer**: A — both may be open; each shows the other open drafts touching the same requirements or areas; when one locks, every other draft touching the same things re-runs its readiness checks against the newly locked version before it can lock.
- **Reasoning**: Recommended: parallel sessions keep working, and conflicts like `implement-now` amendment 002 (forced by e2e-verification's design) are caught before the second lock rather than during execution.
- **Anchor**: 2026-09-22 · turn 10
- **Status**: superseded by D-22 — the lock-time readiness check against the landing mainline already shows the second draft what the first locked; tracking overlap between open drafts adds only early warning

### D-11 — appetite

- **Question**: Does the first release deliver the complete process for new features (A), switch the gates first and rework the skills in a second release (B), or pilot one hand-driven feature before sizing the build (C)?
- **Answer**: A — one release delivers the whole process for new features (draft workspace and rounds, lock checks and approval, reopen, drafts seeing each other, the gate switch-over, and the stage skills reworked), built on the existing memory records, reviewers, and plan projection; no new service or dependency; operator attention limited to rulings in rounds and one approval per lock; no token or money cap; the old pipeline closes in the same release.
- **Reasoning**: Recommended: the requirements are clear enough to build whole, and a split release would leave drafts hand-driven while the old pipeline stays open.
- **Anchor**: 2026-09-22 · turn 11
- **Status**: live

### D-12 — the contract's vocabulary

- **Question**: Use feature draft / round / lock / locked version / reopen and call the per-feature design "design", never "tech spec" (A), or the same terms keeping "tech spec" for the per-feature design (B)?
- **Answer**: B — the same terms, with "tech spec" naming the per-feature design; the living rollup `docs/techspec.md` then needs a rename or a clear note so "tech spec" has one meaning.
- **Reasoning**: Chosen over the recommended A, which avoided the clash with the `docs/techspec.md` rollup; the operator keeps their working term and accepts the follow-up on the rollup (G-1).
- **Anchor**: 2026-09-22 · turn 12
- **Status**: live

### D-13 — the draft against the vision's stage order

- **Question**: Vision §4 has design start from "the approved requirement result" (stage 2's result: "a ruling on new or changed intent"); inside a draft, design starts before the lock. Does this fit as written (A), fit with a note the operator appends to the vision (B), or require a mid-draft "ready for design" approval (C)?
- **Answer**: B — design starts from the product side's rulings inside the draft, and the lock is the single formal approval; the operator will separately append a note to vision §4 saying the stages may repeat inside one draft and are approved together at the lock.
- **Reasoning**: Chosen over the recommended A after a clarification turn; the note removes any strict reading of §4 by a reviewer. The vision edit is the operator's own, outside this feature (O-3) — this session writes nothing to it.
- **Anchor**: 2026-09-22 · turn 13 (after the operator's clarification request)
- **Status**: live

### D-14 — which repositories the first release switches

- **Question**: Does the first release switch only Pyxis's own repository (A), every installed repository with the memory system shipped to them (B), or let each installed repository choose by configuration (C)?
- **Answer**: A — this release switches Pyxis's own repository; installed repositories keep today's document pipeline, skills, and checks until a later feature ships the memory system to them, and Pyxis keeps shipping the document-pipeline skills for them meanwhile.
- **Reasoning**: Recommended: installed repositories do not receive the memory system today (F-12), which the draft depends on; distributing memory is its own capability.
- **Anchor**: 2026-09-22 · turn 14
- **Status**: superseded by D-26 — Pyxis's own repository is the only one using Pyxis

### D-15 — how the other features' conflicting rules change

- **Question**: Should the conflicting rows in other features' frozen PRDs (plan-new FR-1/FR-27, contract-gate FR-1, prd-review FR-1, tech-design-review FR-1, tech-design-brainstorm FR-14/NG-6, plus a new docgate refusal) change by one amendment each in the same release (A), be overridden by this feature's PRD with the others left as written (B), or by one governing-level ruling from the operator (C)?
- **Answer**: B — the feature-draft PRD itself states that those rows do not apply to draft-born features; the other PRDs stay exactly as written.
- **Reasoning**: Chosen over the recommended A, with its shown consequence accepted: read alone, the other PRDs still state the old rules, the two contracts disagree, and an automated reviewer will flag it (P-5); no extra PRs.
- **Anchor**: 2026-09-22 · turn 15
- **Status**: live

### D-16 — where a draft's parts are stored

- **Question**: Are all of a draft's parts — PRD, tech spec, plan, brainstorm records, review reports — memory records (A), or do brainstorm records and review reports stay markdown files under `docs/` while the contract lives in memory (B)?
- **Answer**: A — everything is stored in memory; brainstorm records and review reports are memory Documents with their full text; nothing new is written under `docs/` for draft-born features, and people read the parts rendered from memory.
- **Reasoning**: Recommended: one home for the whole draft, completing the idea's move from documents to memory; D-2's separate documents keep their form, only their storage moves (F-10).
- **Anchor**: 2026-09-22 · turn 16
- **Status**: superseded by D-24 — the plan is not stored in memory; every other part still is

### D-17 — how the switch-over rolls out

- **Question**: Does Pyxis's own repository stop accepting document PRDs and designs only after one real feature completes a draft, with an operator-only setting to reopen the document pipeline (A); at release with that setting (B); or at release with no way back (C)?
- **Answer**: C — the document pipeline closes for new features the moment the release lands and cannot be turned back on; problems in the draft process are fixed forward.
- **Reasoning**: Chosen over the recommended A after a clarification turn, accepting the shown consequence: a defect in the draft process blocks every new feature until it is fixed. Asked during PRD synthesis as a gap the gated record did not decide.
- **Anchor**: 2026-09-22 · review (prd-new synthesis gap)
- **Status**: live

### D-18 — when success is judged

- **Question**: Are D-7's signals judged after the first three draft-born features (PRs per draft as each locks; late gaps once all three are implemented) (A), after the first feature alone (B), or after the first five (C)?
- **Answer**: A — PRs per draft is checked as each of the first three draft-born features locks; the late-gap signal is judged once all three have finished implementation; the verdict is recorded then.
- **Reasoning**: Recommended: three features give more than one data point while late gaps get time to surface during implementation. Asked during PRD synthesis as a gap the gated record did not decide.
- **Anchor**: 2026-09-22 · review (prd-new synthesis gap)
- **Status**: live

### D-19 — reopening while the locked plan's tasks are unfinished

- **Question**: When a feature is reopened while tasks of its current locked plan are unfinished, must the draft mark each unfinished task keep/replace/cancel before the new version locks (A), does the new version lock with those tasks flagged stale (B), or is the reopen refused until the tasks finish or are cancelled (C)?
- **Answer**: C — a feature cannot be reopened while its current locked plan has unfinished tasks; they are finished or cancelled first, then the reopen proceeds; finished tasks stay history tracing the version they delivered.
- **Reasoning**: Chosen over the recommended A after a clarification turn, on the `argus-boundary-enforcement` example (plan 001's attribution gate, T3, superseded within a day by amendment 001), accepting the shown consequence: a task known to be wrong is built or cancelled by hand before the reopen. Closes review:feature-draft-000#FND-4 (incident I9).
- **Anchor**: 2026-09-22 · review (prd-review FND-4)
- **Status**: live

### D-20 — the planning gate's other checks at the lock

- **Question**: Does readiness also run the contract gate's currency, link-resolution, structural, and compilability checks adapted to drafts (A), only the cross-feature ones — currency and link resolution (B), or none beyond D-3's list (C)?
- **Answer**: A — readiness adds all four: nothing depends on another feature's superseded or retired content; every citation resolves; every requirement has a pass/fail test and the definition of done is present; requirement-to-decision links are consistent with no cycles.
- **Reasoning**: Recommended: replacing the planning gate for draft-born features must not drop its checks — incident I6's stale dependency would otherwise lock silently. Closes review:feature-draft-000#FND-5.
- **Anchor**: 2026-09-22 · review (prd-review FND-5)
- **Status**: live

### D-21 — the shape of a draft-born PRD and the lock's capture line

- **Question**: Does a draft-born PRD follow the PRD template including §0 "Read to rule", with the lock approval recording the capture line (A); the template without §0 or a capture line (B); or no fixed template (C)?
- **Answer**: A — draft-born PRDs follow the PRD template, §0 included, and the lock approval records the capture line (minutes spent; each feedback item marked in or outside §0).
- **Reasoning**: Recommended: keeps read-to-rule's routing aid and gate-time measurement for new features instead of silently dropping them. Closes review:feature-draft-000#FND-6.
- **Anchor**: 2026-09-22 · review (prd-review FND-6)
- **Status**: live

### D-22 — replacing overlap tracking with a check against the landing mainline

- **Question**: The operator asked why overlap matters when the second draft to land meets the first at merge time. Replace D-10's overlap tracking with one rule — a lock's readiness is computed against the mainline it lands on, with memory's existing in-flight warning as a free non-blocking heads-up (A) — or keep D-10 (B)?
- **Answer**: A — no overlap tracking between open drafts; a lock's readiness checks run against the mainline the lock actually lands on; memory's existing report of other open proposals on the same claims stays as a non-blocking heads-up.
- **Reasoning**: The operator's point holds for correctness once readiness runs against the landing mainline (currency and link checks per D-20 then refuse any conflict with what locked first); overlap tracking adds only early warning. The rule must be explicit because main's protection admits merges on checks computed against an older main (F-15). Supersedes D-10; closes review:feature-draft-000#FND-2.
- **Anchor**: 2026-09-22 · review (operator question on prd-review FND-2)
- **Status**: live

### D-23 — migration under the unfinished-tasks rule

- **Question**: Does D-19's rule — no reopen while the current plan has unfinished tasks — also apply when an existing feature moves into a draft (A), or may it migrate at any time while its old plan's tasks keep running (B)?
- **Answer**: A — an existing feature cannot move into a draft while its current plan has unfinished tasks; they finish or are cancelled first, the same rule as a reopen.
- **Reasoning**: Recommended: one rule for reopen and migration, so a draft's new plan never races tasks still building the old contract.
- **Anchor**: 2026-09-22 · review (confirmation of the FR-43 migration clause)
- **Status**: superseded by D-26 — no migration in this release


### D-24 — the plan stays out of memory

- **Question**: In the tech-spec brainstorm the operator ruled the plan is not stored in memory, against D-16 and FR-3. Which stands?
- **Answer**: The plan stays out of memory: "plan is the only artifact that should not be stored in the memory system given there is no learning or information. it is just a way to connect the idea (prd/techspec) to the execution engine." Every other part of a draft is stored in memory as D-16 ruled.
- **Reasoning**: The plan carries no learning of its own; it only bridges the locked contract to execution, so it stays today's plan document. Applied to the PRD in the same draft (FR-3, FR-29). From design-brainstorm:feature-draft-tech-design#D-4 and #D-5.
- **Anchor**: 2026-09-23 · review (tech-spec brainstorm, DQ-3)
- **Status**: live

### D-25 — the draft is the PRD and tech spec; the plan follows the lock

- **Question**: In the tech-spec brainstorm, asked how FR-3 reads when records are compiled only at the lock, the operator restated what a draft is and when it reaches memory.
- **Answer**: "PRD tech spec is a bundle as a draft... And then plan comes after that." At the lock everything goes into memory with each file's exact text, and every check runs. A plan is generated afterwards only when wanted and stays out of memory (D-24). D-3's other lock conditions, D-8's out-of-scope set and lane-neutral handoff, and D-5's reopen rule for the PRD and tech spec stand; D-7 counts proposals to the lock.
- **Reasoning**: The plan only connects a locked contract to execution, so it is neither drafted nor locked with the contract. From design-brainstorm:feature-draft-tech-design#D-12 and #D-13.
- **Anchor**: 2026-09-23 · review (tech-spec brainstorm, FR-3 wording turn)
- **Status**: live

### D-26 — day one: no migration, no switch, no other repositories

- **Question**: In the tech-spec brainstorm, delegating the remaining mechanics, the operator scoped the release.
- **Answer**: "do not worry about migration or feature control flag. assume it is day one on the pyxis with the new draft process and no other existing repo uses pyxis." New features start as drafts at the switch-over; existing features keep their frozen documents and are not migrated in this release; no setting switches the process; no installed repository needs the document pipeline kept.
- **Reasoning**: A clean start keeps this release to the draft process itself. Migration waits for a later feature (WR-5). From design-brainstorm:feature-draft-tech-design#D-14.
- **Anchor**: 2026-09-23 · review (tech-spec brainstorm, mechanics turn)
- **Status**: live
## Facts

- **F-1**: Today's `memory-gate` (a required code-gate check in CI, pre-push, and the executor) fails a memory-native PRD claim served by a memory-native design Decision: reconciliation reports `unauthorized-prd-serves` ("serves a PRD record whose address is not a live mapped source unit"), a regression beyond the baseline. Record-level `memory check` passes; only the rule tying design coverage to `docs/prds` ledgers and the design index blocks it — source: scratch-clone probe at `9f2fbe98`, 2026-09-22; `internal/memory/reconcile/analyze.go` (`authorizedPRDServes`).
- **F-2**: Memory immutability is judged against the branch's merge base: a record added on an unmerged branch and then edited in place passes `memory check` — source: same probe; `internal/memory/packet/packet.go` (`Compute`).
- **F-3**: No pipeline skill or authoring guide invokes a memory verb (0 of 17), and nothing outside `cmd/memory` and `cmd/memory-gate` imports `internal/memory` — source: repository search, 2026-09-22.
- **F-4**: `memory/` holds 4,004 records, 3,865 of them (97%) mirrors of documents (reconciliation, migration, or import provenance); the `Plan` record kind exists with zero records; plans live in `docs/plans/` — source: scan of `memory/`, 2026-09-22.
- **F-5**: Reconciliation captures every edit of an authored document as a new literal Document record (13 captures exist for `cmd/memory/README.md` alone) — source: `memory/tool/memory/`; `internal/memory/reconcile/source.go`.
- **F-6**: 22 of the 59 PRD amendments cite later-stage evidence (design brainstorms, design reviews, a plan bounce, build results) in their Learned-from; `planner-driven-model-routing` took 12 contract-side PRs — 3 amendments, 3 design-brainstorm reopenings, 3 plans — source: ledger Learned-from rows and git history, 2026-09-22.
- **F-7**: The memory CLI already renders a proposal's joint sign-off — the promise delta the product side signs, the design delta the engineering side signs, the narrative diff, undesigned claims, and other branches' in-flight work on the same families — and renders any document as of a commit (`memory render --at`) — source: `cmd/memory/README.md` (flow steps 2 and "Documents").
- **F-8**: A brainstorm record already rides its stage's introducing PR (stage 1 with the PRD base, stage 3 with the design), so bundling exists within a stage but never across stages — source: `.pyxis/authoring/feature-brainstorm.md` §8, `.pyxis/authoring/tech-design-brainstorm.md` §10.
- **F-9**: Memory subjects are open `namespace/name` pairs, so a `feature/<slug>` subject is valid today; feature ownership for closeout is derived from document headers — source: `internal/memory/record/record.go` (`subjectRE`), `internal/memory/reconcile/ownership.go`.
- **F-10**: Memory already addresses brainstorm records and review reports as their own Document subjects (`brainstorm/<slug>`, `review/<slug>`), separate from `prd/<slug>` and `design/<slug>` — source: `internal/memory/reconcile/source.go` (`sourceAddress`).
- **F-11**: `planner-driven-model-routing`'s contract side ran 2026-07-26 → 2026-07-29 through 9 PRs to its first plan (#708 base, #733 product gaps, #737 amendment 001, #741, #744 amendment 002, #780 amendment 003, #802 design bounce, #811 design, #814 plan), then 2 more plans (#922 on 07-31, #1109 on 08-21) — source: git history, 2026-09-22.
- **F-12**: Repositories that install Pyxis receive the document pipeline — the `docs-gate` workflow and desk hooks running `pyxis exec docgate -- gate`, the templates, and the stage skills — but not the memory system or its gate — source: `.pyxis/dist/github-workflows/docs-gate.yml`, `.pyxis/dist/operand-hooks/`, `internal/installer/`.
- **F-13**: At least six other features' merged rows bind the document pipeline: `prd:plan-new` FR-1/FR-27 (planning input is an approved PRD projection plus an approved design, behind a clean docs-based gate), `prd:contract-gate` FR-1 (planning calls that gate), `prd:prd-review` FR-1 and `prd:tech-design-review` FR-1 (each refuses any subject other than a PRD base/amendment or a design document), `prd:tech-design-brainstorm` FR-14/NG-6 (product holes bounce to `/prd-amend`); docgate has no rule refusing new document PRDs — source: those ledgers' `effective.md`.
- **F-14**: docgate's "requirement precedence" (amendment 004) orders requirements before designs; no existing mechanism lets one PRD override another PRD's rows — source: `docs/prds/docgate/004-requirement-precedence.md`.
- **F-15**: Main's branch protection requires the `docs-gate` check with `strict: false` and there is no merge queue, so a proposal can merge on checks computed against an older main — source: GitHub branch-protection and merge-queue queries, 2026-09-22.
- **F-16**: A memory record's author is self-declared — taken from `--actor`, then `$MEMORY_ACTOR`, then git `user.email` — so record authorship does not authenticate a person — source: `cmd/memory/README.md` (Verbs).

## Fog

- **G-1** (non-blocking — blocks: technical design, and the draft PRD's open questions): whether the living rollup `docs/techspec.md` is renamed or annotated so "tech spec" names only the per-feature design (D-12) — cannot be put precisely until it is known whether the living rollups (`docs/prd.md`, `docs/techspec.md`) survive the move to memory or become rendered views.

## Out of scope

- **O-1**: Changing how locked work is handed to implementation — today's plan projection and executor keep working unchanged; a new handoff of the whole locked pack is a later feature (from D-8).
- **O-2**: The intake step that classifies incoming work, and non-feature lanes (bug fix, performance) — later features; the locked result stays lane-neutral so they can join without reshaping it (from D-1, D-8).
- **O-3**: Edits to the product vision or the architecture principles — governing changes remain the operator's separate call (from D-8).
- **O-4**: Locking automatically when checks pass, or locking with known gaps — refused; every lock needs green readiness and a human approval (from D-3).
- **O-5**: Requiring two different people to approve a lock — not in the first version; both roles are recorded and one person may hold both (from D-4).
- **O-6**: Typed PRD amendments and superseding designs for draft-born features — refused; a change reopens the feature as a new draft (from D-5).
- **O-7**: Converting all existing features at switch-over, or keeping the document pipeline alive for old features indefinitely — refused; old features migrate when they next change (from D-6).
- **O-8**: Switching repositories that install Pyxis, or shipping the memory system to them — a later feature; they keep the document pipeline until then (from D-14).
- **O-9**: Amending the other features' ledgers, or a governing-level ruling, to change their conflicting rows — not used; this feature's PRD states the override itself (from D-15).
- **O-10**: A pilot period before the switch-over, or any setting that reopens the document pipeline after it — refused; the pipeline closes at release and defects are fixed forward (from D-17).
- **O-11**: Tracking overlap between open drafts — lists of drafts touching the same requirements or areas, and forced re-checks when one locks — refused; readiness against the landing mainline catches conflicts, and memory's in-flight warning stays advisory (from D-22).
- **O-12**: Marking the previous plan's unfinished tasks keep/replace/cancel inside a reopened draft, or locking with them flagged stale — refused; unfinished tasks finish or are cancelled before a reopen (from D-19).

## Candidate glossary terms

- **T-1**: feature draft — the open, revisable workspace for one feature's PRD, tech spec, plan, brainstorm records, and review reports, from opening until its lock (D-1, D-2) · avoid: bare "draft" for this (the legacy PRD template's `Draft` status and a code host's draft PR are different things)
- **T-2**: round — one pass of feedback inside a feature draft, in which any of its parts may be revised · avoid: "stage" (the per-stage pipeline), "amendment"
- **T-3**: lock — the single human approval, given only after readiness passes, that ends a feature draft and makes its PRD, tech spec, and plan immutable together (D-3) · avoid: "freeze" (the per-stage merge event of the legacy pipeline)
- **T-4**: locked version — what a lock produces, numbered v1, v2, …; in force until the next lock of the same feature replaces it (D-5) · avoid: "effective view" / "projection" (the legacy PRD fold's outputs)
- **T-5**: reopen — starting a new feature draft from a feature's locked version to change it (D-5) · avoid: "amendment", "supersession" (the legacy evolution channels)
- **T-6**: tech spec — the per-feature technical design inside a feature draft (D-12) · avoid: confusing it with `docs/techspec.md`, the living system rollup (G-1)

## Parked technical constraints

- **P-1**: Relaxing reconciliation for memory-native features must land on the mainline before the work that relies on it — the memory gate refuses candidate-selected enforcement (no branch flag, path whitelist, or blanket advisory switch) — context: `design:memory-staged-contracts` §3.5; `cmd/memory/README.md` (trusted base).
- **P-2**: A long-lived draft meets the gate's ancestry and evidence rules (the mainline must be an ancestor; file- and code-citing records carry content manifests checked at the landing tree); unlanded records can be re-evidenced with `memory evidence --write` — context: `docs/tech-design/memory-durable-evidence-v1.md`; `cmd/memory/README.md` ("Durable evidence").
- **P-3**: Plans are consumed through `plangate` fold and tracker projection from `docs/plans/<slug>/NNN-plan.{md,json}`; plan traces cite `prd:<slug>/FR-n` and `design:<slug>#…` document anchors, not memory addresses — context: `.pyxis/templates/plan-template.md`, `.pyxis/authoring/plan-new.md`.
- **P-4**: The documents that describe the per-stage freeze need updating when the switch-over lands: `AGENTS.md` (document-system rules 2, 7, 8 and the layout table — operator-owned instructions), `.pyxis/pipeline.md`, `docs/domain-model.md` §5, the stage authoring guides, and the templates — while installed repositories keep the document-pipeline versions (D-14) — context: Orientation overlaps.
- **P-5**: The override chosen in D-15 has no existing mechanism (F-14): the PRD must name each overridden row exactly (F-13) and state its scope (draft-born features in Pyxis's own repository); PRD review's upstream-alignment lens is expected to flag the disagreement with the unamended sibling ledgers, and every tool that reads those rows (planner, contract gate, reviewers) needs its override behavior specified in design — context: D-15; `docs/prds/prd-review/effective.md`.
- **P-6**: Argus drives stage by stage against `.pyxis/pipeline.md` and keeps its problem and delivery ledgers under `docs/problems/<slug>/`; its drive over a feature draft (rounds, product rulings under its grant per D-9, the lock request per D-4) and where its ledgers live for draft-born features are design matters — context: `prd:argus`; `.pyxis/authoring/argus.md`.
- **P-7**: The unmanned reviewers' protocols assume a fixed subject (one report per subject per moment, scope-locked two-round convergence); inside a draft whose parts keep changing, their mapping onto rounds and onto the required clean review of the version being locked (D-3) must be designed — context: `docs/tech-design/prd-review-artifacts.md`, `docs/tech-design/tech-design-review-artifacts.md`.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-09-22 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered, zero blocking fog (problem D-1 · segments D-4, anti-segment D-8/D-14 · jobs D-9 with F-6/F-11, D-1/D-3, D-5, D-10 · goals D-7 · non-goals O-1–O-9 · appetite D-11/D-14) |
