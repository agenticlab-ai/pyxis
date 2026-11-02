# Prototype an idea before its contract — delivery ledger

| Field | Value |
|---|---|
| **Slug** | prototype |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | Contract drafting, version 2: reopened from `lock/prototype/v1` under problem:prototype#A-6; both contract reviews approve at digest `sha256:2599c039b74afda07f42732844014b1fddb13af935da306582dd280f95c0e2be`. The draft waits on local branch `draft/prototype-v2` until this journal lands on mainline, then it is published and its lock prepared. Of plan 001, T2 is merged; T1, T3, T4 and T5 are cancelled and will be planned again from version 2 |
| **Open items** | (1) The operator's review and merge of the journal pull request from `docs/prototype-drive-journal-2` under problem:prototype#A-1; then the version-2 draft's publication, lock preparation, and the operator's review and merge of the prepared lock. (2) Plan 002 from version 2 for T1, T3, T4 and T5, after the lock lands. (3) A known defect locked as written in design decision D-1: it checks that the local draft branch exists, which can outlive a locked or abandoned draft; version 2 leaves it as written (A-6). (4) Glossary terms T-1 and T-2 of brainstorm record 001 are not routed to the glossary. (5) An intermittent test failure on mainline, outside this drive's scope, refused one publication (E-20); reported to the operator. (6) The engine has no stop for a task that outgrows its planned size band or touch list during fix rounds (E-35); reported to the operator. Pull request 1568 now reports a fix round's changes away from its findings as scope drift, without stopping. (7) Draft readiness cannot resolve a reopened draft's citations of its own design brainstorm by the `<feature>-tech-design` name (E-39); reported to the operator |
| **Park state** | — |
| **Cumulative spend** | unavailable (the CLI exposes no structured spend signal; best-effort: one drive session, four independent contract-reviewer sessions, and five implementation sweeps with two workers and nine review launches; the T1 worker session reported 21.9 million tokens at the end of its last turn) |

## Journal

### E-1 — stage-entered — front door and feature definition

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `docs/problems/prototype/problem.md`; `docs/problems/prototype/delivery.md`
- **Body**: New idea handed over by the operator: prototype an idea before its contract. Slug `prototype` minted; no collision under `docs/problems/`, no PRD ledger, no draft state (`pyxis draft status prototype` reported no contract). The problem ledger holds the idea verbatim and the intake pre-ruling problem:prototype#A-1: Argus drives, the operator reviews and merges. Effective envelope: repo default, no override. Entered stage 1 by role against `.pyxis/pipeline.md` on the new-feature draft route.

### E-2 — reconciliation — scaffold publication refused by the push-time readiness check

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: local branch `draft/prototype`; scaffold commit `bea51741`; `drafts/prototype/`. No remote branch and no pull request exist.
- **Body**: `pyxis draft open prototype` refused first on missing agent identity. The existing authenticated agent account was named in the ignored local config, with its credential supplied only in the command environment. The second run committed the attributed scaffold, then its own push was refused by the pre-push code gate: the `draft-ready` check requires a complete, reviewed contract and the scaffold holds only template placeholders. The command restored the prior checkout and kept the local draft branch; this drive resumed it. Publication waits until the contract roles and both reviews exist. The refusing gate is preserved.

### E-3 — assumption-recorded — reading of the dictated idea

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: —
- **Body**: The idea arrived as dictated speech. Read "pixies" and "pixie" as Pyxis, "work tree" as a Git worktree, "future draft" as the feature draft, and "Rhonda test version 1 … accept and raise the pr" as instructions about this session, outside the feature. "the comments" could not be resolved and carries no requirement. Legality: tier — contradicts no governance content and no `A-n`; channel — a new feature draft is the sanctioned channel; regression — no contract is touched. Recorded at `brainstorm:prototype#F-1`.

### E-4 — assumption-recorded — destination: an optional prototype that feeds the draft

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `brainstorm:prototype#D-1`
- **Body**: Three framings considered: the prototype as evidence for the draft, as the first commit of the implementation, or as a standalone tool outside the pipeline. The idea names "the real implementation" and "the actual implementation" as separate from the prototype and asks for learning to be applied in the feature draft, which determines the first. Legality: tier — the pipeline grammar keeps its stages and the rule that implementation follows a locked contract stands, because prototype code never lands; channel — a new feature draft; regression — the draft, review and implementation contracts are unchanged for a feature that does not ask for a prototype. Recorded at D-1.

### E-5 — assumption-recorded — who it serves, the bet, and the appetite

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `brainstorm:prototype#D-2` · #D-3 · #D-4
- **Body**: Underdetermined gaps the stage puts to its principal. Segment: whoever hands an idea to Pyxis, with Argus driving on their behalf; anti-segment: changes whose solution is already understood. Bet: building a small throwaway version first surfaces obstacles and conflicts that otherwise appear after the contract locks; observable on the first prototyped feature as every finding answered in its locked contract. Appetite: tier T1 and a budget of four plan tasks, with the operator's one review of the contract. Legality: tier — none contradicts governance content or A-1; channel — draft content; regression — none. Each is the operator's to change in the lock review.

### E-6 — assumption-recorded — how the learning travels, and what an unfinished prototype means

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `brainstorm:prototype#D-8` · #D-9
- **Body**: The idea asks for the learning to be applied in the feature draft and the actual implementation and names no channel. Assumed the narrowest one: each finding is answered inside the draft's own contract parts, and implementation receives it through the locked contract it already receives, with no new channel to workers. A prototype that does not reach a working state still yields its findings; that is a result. Legality: tier — no governance content touched; channel — draft content; regression — implementation inputs are unchanged.

### E-7 — ruling-recorded — brainstorm gate signed ready-for-prd

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `drafts/prototype/brainstorm/001.md` Gate
- **Body**: Read the authority contract at the act: tier-2 grants `sign-gate-verdict`. Walked the record's structure and coverage checklists: all six decision-shaped inputs covered, zero blocking fog. Signed the stage-1 gate `ready-for-prd` as Argus. Two non-blocking fog items travel to the design stage. The operator's approval of the exact contract lock remains theirs under A-1.

### E-8 — stage-entered — requirement definition: the PRD

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `drafts/prototype/prd.md`
- **Body**: Synthesized the PRD from brainstorm record 001 at tier T1: five functional rows, two non-regression rows, the default failure row, and seven definition-of-done rows. Every functional row quotes the operator's words or cites the record decision that carries them. Four things the author believes useful and nobody asked for wait in the waiting room with their cost: Argus starting a prototype unasked, a mechanical lock check on findings, a separate budgeted worker, and an eval set. Task estimate three against a budget of four.

### E-9 — stage-entered — technical design brainstorming

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `drafts/prototype/brainstorm/002.md`
- **Body**: Basis: the draft PRD. Three design questions framed from the stage-1 fog and parked constraints: who builds the prototype, where its findings live, and what owns its worktree. Two questions open a seam and were panelled under the sequential fallback; one extends the draft verb set.

### E-10 — assumption-recorded — design direction

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `design-brainstorm:prototype-tech-design#D-1` · #D-2 · #D-3
- **Body**: Ruled as stage-3 authority: the asking session builds the prototype from a guide; findings are a numbered brainstorm working file; one draft verb creates and removes the worktree, which is kept locally until the draft locks or is abandoned. Each choice is the shape that adds least: no executor mode, no new working-file class, no readiness condition. Legality: tier — no governance content touched, and prototype code still never lands; channel — draft content; regression — the compiler, readiness, executor and memory store are unchanged.

### E-11 — ruling-recorded — design-brainstorm gate signed ready-for-design

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `drafts/prototype/brainstorm/002.md` Gate
- **Body**: Read the authority contract at the act: tier-2 grants `sign-gate-verdict`. Walked the structure and coverage checklists: the direction, the alternatives, the seam map and the out-of-scope set are present, with zero fog. Signed `ready-for-design` as Argus.

### E-12 — stage-entered — technical design: the tech spec

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `drafts/prototype/tech-spec.md`
- **Body**: Assembled four decisions from record 002: the worktree verb, the prototype guide, the findings record, and the passages that answer and check findings. Every Must row of the PRD is served. The two PRD open questions carried from the stage-1 fog are answered by decisions D-1 and D-2. Task estimate three.

### E-13 — stage-entered — independent contract reviews

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: committed subject `ffc1bb4319979178058e8c66dc36ef8d97ab4752`; contract digest `sha256:43fe81c871bdb5d207a4bfb41829e2b813024d97c4f6566d196d558433837945`; `drafts/prototype/reviews/001.md`; `drafts/prototype/reviews/002.md`
- **Body**: Entered requirement review and technical review together, at one digest. Both subjects are low risk (a T1 PRD; a design with no security or factory-control surface), so one cold reviewer session reviews them in sequence. The reviewer is a fresh session on this machine's configured review backend with no authoring context; it reads the subjects from the committed head and writes only the two reports. No verdict is claimed at this entry.

### E-14 — reconciliation — round-1 review verdicts and the one disposition

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `drafts/prototype/reviews/001.md` (approve, no findings); `drafts/prototype/reviews/002.md` (request-changes, one blocking finding); `drafts/prototype/tech-spec.md` revised
- **Body**: Read the authority contract at the act: tier-2 grants `dispose-blocking-finding`. The PRD review approved with no findings. The tech-spec review raised review FND-1 in report 002: the design promised removal of the prototype when the draft locks or is abandoned and assigned that step to nobody. Applied by narrowing to a guide passage: the session that ends the draft runs the removal, the Argus guide gains the same step, and the verification is stated. No mechanism was added and the PRD is unchanged. The contract digest moves, so both reviews run again on the new head; the technical re-review is scope-locked to the changed spans.

### E-15 — reconciliation — round-2 review verdicts

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: committed subject `b06f09f066d5adb1ea007524cd4609e5e237af02`; contract digest `sha256:563ed1172c201226f4d8f89f702f61f3a9a61df22792b44d5b163160a275524c`; `drafts/prototype/reviews/003.md` (PRD: approve); `drafts/prototype/reviews/004.md` (tech spec: approve)
- **Body**: A fresh reviewer session ran the scope-locked second round from the computed delta. The PRD blob is identical at both heads and its approval is renewed at the new digest. The technical re-review judged review FND-1 of report 002 closed against its acceptance condition and found no regression in the changed spans. No finding was declined and nothing escalates. Zero blocking findings remain open.

### E-16 — stage-entered — contract compilation and first publication

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: —
- **Body**: Entered contract compilation by role: the draft's mechanical readiness verdict, then publication of the reviewed draft, then lock preparation. The push-time memory check requires this problem's two ledgers to be captured as native records under this problem's own memory subtree before the branch can be published; that capture follows this entry and is repeated whenever a ledger changes. Approval of the prepared head is the operator's under problem:prototype#A-1 and the tier-2 contract; Argus will request it and perform none.

### E-17 — reconciliation — first publication refused again; ledger citations repaired; publish the reviewed draft

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: remote branch `draft/prototype`, first published at the commit that carries this entry; the draft pull request opened from that branch (identified by its head branch); contract digest `sha256:563ed1172c201226f4d8f89f702f61f3a9a61df22792b44d5b163160a275524c`
- **Body**: Mechanical readiness passed at the reviewed digest and the memory check was exact after the two ledgers were captured. The first push of the complete draft was still refused: the repository's own-corpus citation test resolves a typed brainstorm citation only against a merged record or locked memory, and five such citations in this ledger pointed at the draft's unlocked brainstorm records. Set those five references in code spans, with their text unchanged, so they read as plain references until the lock compiles the records; no entry's content changed and nothing had been published. Replaced the unpublished delivery capture so it matches these bytes. This entry is written ahead of the push it describes because any later ledger commit would need its own full publication cycle: the branch is pushed at this commit and the draft pull request is opened from it immediately after. Lock preparation follows; its prepared head will be journaled in the next proposal, since an entry on this branch would move the head the operator is asked to approve.

### E-18 — reconciliation — lock prepared and published; operator review requested

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: [PR #1547](https://github.com/agenticlab-ai/pyxis/pull/1547); prepared head `a69068c7637dca57b4a13546005d020acaf31626`; contract digest `sha256:563ed1172c201226f4d8f89f702f61f3a9a61df22792b44d5b163160a275524c`; the approval request in the pull request body; review requested from the operator
- **Body**: Done but not journaled at the time, because an entry on the branch would have moved the prepared head; recorded now that a new round has reopened the branch. The draft was published at the commit carrying E-17 and the draft pull request opened from it. Lock preparation rebased the draft, compiled the contract into records, passed its memory check and the publication gate, published the prepared head, marked the pull request ready, and requested the operator's review. All six hosted checks passed on that head. Authority at the act: tier-2, without `approve-merge`; Argus approved nothing and merged nothing.

### E-19 — answer-received — the operator's rulings in review of the prepared lock

- **Date**: 2026-10-10
- **Principal**: the human · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `docs/problems/prototype/problem.md` A-2 and A-3; `drafts/prototype/problems.md` P-1
- **Body**: Cites problem:prototype#A-2 and problem:prototype#A-3. Reviewing the prepared lock, the operator ruled that a Pyxis command must control the harness and model that build a prototype: from configuration when it names them, otherwise the current session's own, always as a separate headless run, and not through the implement-now flow. Prototypes stay inside feature drafts for this version. Recorded as requirement problem P-1 with the operator's ruling, and FR-6 was added under it. The approval request of E-18 is withdrawn by this change.

### E-20 — reconciliation — the draft round reopened

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: [PR #1547](https://github.com/agenticlab-ai/pyxis/pull/1547) returned to draft; restored head `c16c5f88bd425b990c002e8c49924b9fa0992dc5`, the revert of the preparation commit, with the contract digest unchanged at that head
- **Body**: `pyxis draft round prototype` restores the working files by reverting the preparation commit and republishing. The first attempt was refused by the publication gate on an intermittent failure of one test outside this drive's scope, `TestConcurrentCreatesDoNotLoseUpdates` in the maintenance jobs package ("open jobs lock: no such file or directory"); the same test failed again in a local five-run repeat at this head. The command was run a second time and passed. The failing test was left as it is and reported to the operator.

### E-21 — assumption-recorded — the details the rulings leave open

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `brainstorm:prototype#D-11`; `design-brainstorm:prototype-tech-design#D-4`
- **Body**: Ruled by assumption within A-2 and A-3. The configuration names a harness and a profile from that harness's existing catalog, not a raw model identifier, because a launch takes a model and an effort the catalog holds. With nothing configured, the session's own model is matched to a catalog profile of its harness, the cheapest when several share the model; a missing or unknown model refuses and names the configuration block. The build is bounded by the engine's default worker time bound. A failed or timed-out build yields no findings and never switches route. The budget rises from four to five plan tasks and the tier stays T1; the operator was told the price before the change. Legality: tier — no governance content is touched and prototype code still never lands; channel — draft content under the operator's ruling on P-1; regression — the implement-now worker cycle, the draft compiler and readiness are unchanged, and the harness port gains a launch purpose by addition.

### E-22 — stage-entered — contract reviews for the operator revision

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `drafts/prototype/reviews/005.md`; `drafts/prototype/reviews/006.md`
- **Body**: The contract changed under an operator ruling after both reviews had approved, so a new review episode covers the revised PRD and tech spec at their new digest. One cold reviewer session reviews both in sequence, as before, reading the committed head. No verdict is claimed at this entry.

### E-23 — reconciliation — new-episode round-1 verdicts and three dispositions

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: committed subject `fed23f47431e65bad545a82ebfc9e57ed82ae79a`; contract digest `sha256:1a30cdc6d10fc60060ec11f0209355e4d56a74b83bfb9905c2fab51e9dfd46b8`; `drafts/prototype/reviews/005.md` (PRD: approve, no findings); `drafts/prototype/reviews/006.md` (tech spec: request-changes, three blocking findings); `drafts/prototype/tech-spec.md` revised
- **Body**: Read the authority contract at the act: tier-2 grants `dispose-blocking-finding`. The PRD review approved the revised requirements, FR-6 included. The tech-spec review checked the launched build against the engine's code and raised three blocking findings in report 006: the launch needs prepared runtime state and a login snapshot; the adapter must be constructed with the same catalog the route was resolved from, local overlay included; and a commit must wait for the runtime to acknowledge that the process tree stopped. Each was confirmed against the code and applied by naming the engine's existing helper or rule; no mechanism beyond what FR-6 needs was added, and the PRD is unchanged. The digest moves, so both reviews run again; the technical re-review is scope-locked to the changed spans.

### E-24 — reconciliation — round-2 verdicts; publish the revised draft

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: committed subject `709082562dedd718e57001195f5856b4eb621065`; contract digest `sha256:81a8b3621e83dd810cbe31b2c15171f6b8ecc3ea13346b25953920acb4e64dcb`; `drafts/prototype/reviews/007.md` (PRD: approve); `drafts/prototype/reviews/008.md` (tech spec: approve); remote branch `draft/prototype`, published at the commit that carries this entry and its refreshed ledger captures, on [PR #1547](https://github.com/agenticlab-ai/pyxis/pull/1547)
- **Body**: A fresh reviewer session ran the scope-locked second round. The PRD blob is unchanged and its approval is renewed at the new digest. The technical re-review judged the three findings of report 006 closed against their acceptance conditions, checking each against the code, and found no regression in the changed spans. No finding was declined and nothing escalates. This entry is written ahead of the push it describes, as E-17 was. Lock preparation follows; its prepared head is journaled in the next proposal or the next round, since an entry on this branch would move the head the operator is asked to approve. Approval and merge remain the operator's under problem:prototype#A-1.

### E-25 — reconciliation — the draft pull request was merged before the lock was prepared

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: merged [PR #1547](https://github.com/agenticlab-ai/pyxis/pull/1547); merged head `a2d66071a81f`; mainline commit `ddc2eb28e8b870c3a6cb8ae4b2b93a9538aaac0b`; no lock tag; a new `draft/prototype` branch from that mainline commit and the draft pull request opened from it (identified by its head branch)
- **Body**: The operator reported the merge. Verified at the goal object and not at the badge: pull request 1547 was merged by the agent account at the head published in E-24, which carried the reviewed working files and on which all six hosted checks had passed. There was no operator review on that head. Lock preparation was still inside its publication gate; it was stopped and its temporary worktree removed. Mainline's tree equals the merged head: it holds the draft's working files, both ledgers and their captures, with no compiled contract records and no lock tag, and `pyxis draft status prototype` reports no contract state. The contract text on mainline is the reviewed one at digest `sha256:81a8b3621e83dd810cbe31b2c15171f6b8ecc3ea13346b25953920acb4e64dcb`, so nothing is authored again and both reviews stand. The lock is re-proposed: this entry is the first commit of a new draft branch from mainline, a draft pull request is opened from it, and lock preparation runs there so the operator can approve the exact prepared head. Planning waits for that lock.

### E-26 — reconciliation — the operator-approved contract lock verified on mainline

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: merged [PR #1548](https://github.com/agenticlab-ai/pyxis/pull/1548); prepared and approved head `00d1bc4c6b29a9b15900fcc21e71130b5e59a202`; mainline commit `919615751d8b79e09b2d44258986e7cae964199f`; tag `lock/prototype/v1`; [issue #1549](https://github.com/agenticlab-ai/pyxis/issues/1549)
- **Body**: Done but not journaled at the time: the follow-up pull request of E-25 was opened, lock preparation published the prepared head named above at the reviewed digest, and all six hosted checks passed on it. After the operator reported the merge, verified at the goal objects: the pull request was merged by the operator at that exact head, with the operator's approving review on that commit; the landed tree is byte-identical to the approved head; the lock recorder published the annotated v1 tag naming the merge, the approved head and the digest; and `pyxis draft status prototype` reports locked at version 1. The automated pull-request reviewer left two comments on the prepared head. One is the hole already priced as WR-2: a finding's answer can change after the reviews without making them stale. The other is a defect in design decision D-1, whose draft check reads the local branch and not the draft's lifecycle state. Both were put to the operator with a recommendation to apply them; the operator merged the lock as prepared, so the first stays in the waiting room and the second is carried as an open item. At the operator's instruction, issue 1549 records that an unprepared draft can merge.

### E-27 — stage-entered — implementation planning

- **Date**: 2026-10-10
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `docs/plans/prototype/001-plan.md`; `docs/plans/prototype/001-plan.json`; branch `plan/prototype-001`, published at the commit that carries this entry, and the plan pull request opened from it (identified by its head branch)
- **Body**: The planning-entry precheck returned `lock/prototype/v1`. Decomposed the locked contract into five tasks, the contract's budget: the worktree verb; the shared-surface additions (config block, launch purpose, shared runtime constructor); route resolution; the launched build; and the guide, skill, template and guide passages. The plan folds green and covers every Must row. Derived mix: three tasks at standard capability and two at strong, for implementation and for review; none at frontier. This entry is written ahead of the push it describes, as E-17 and E-24 were. The operator's merge of the plan pull request is the approval and the freeze under problem:prototype#A-1; projection onto the tracker follows that merge under the tier-2 `trigger-materialization` grant.

### E-28 — reconciliation — plan 001 frozen on mainline

- **Date**: 2026-10-11
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: merged [PR #1550](https://github.com/agenticlab-ai/pyxis/pull/1550); reviewed head `53c2bb93db1c`; mainline commit `a6fcc816d337`
- **Body**: The plan pull request announced in E-27 was opened from `plan/prototype-001`, and all six hosted checks passed on it. After the operator reported the merge, verified at the goal object: merged by the operator at the reviewed head, and the plan document, its folded projection, this ledger and its capture on mainline are identical to that head. The merge is the plan's approval and its freeze.

### E-29 — item-materialized — epic and five tasks projected onto the tracker

- **Date**: 2026-10-11
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: epic [#1551](https://github.com/agenticlab-ai/pyxis/issues/1551); tasks [#1552](https://github.com/agenticlab-ai/pyxis/issues/1552) (T1), [#1553](https://github.com/agenticlab-ai/pyxis/issues/1553) (T2), [#1554](https://github.com/agenticlab-ai/pyxis/issues/1554) (T3), [#1555](https://github.com/agenticlab-ai/pyxis/issues/1555) (T4), [#1556](https://github.com/agenticlab-ai/pyxis/issues/1556) (T5)
- **Body**: Read the authority contract at the act: tier-2 grants `trigger-materialization`. Triggered materialization through the planning stage's own contract, staged as it prescribes: a dry run to read the write set, a projection without the routing signal, then the full projection with routing applied last. Verified each item at the goal object: the epic and all five tasks exist and are open, every task carries the repository's automated-implementation label `local-agent`, each is linked under the epic with its spec posted, and the blockers match the plan (T3 on T1 and T2, T4 on T2 and T3, T5 on T4). No discrepancy. Argus implements none of these items.

### E-30 — stage-entered — implementation under the self-drive watcher

- **Date**: 2026-10-11
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: the watcher process for plan `prototype/001` on this machine, started after the commit that carries this entry; its claims, task branches and proposals are identified by the plan-task markers `prototype/001/T1` to `prototype/001/T5`
- **Body**: Entered implementation by role. The plan report shows every task projected. The stage's own driver, the self-drive watcher, runs the ready frontier (T1 and T2 first), starts the next frontier when a merge unlocks it, and exits when every task has closed. Workers run on the harness this machine's configuration names, with the configured independent review before publication. Argus starts and observes the watcher and implements nothing itself. Each task proposal needs the operator's review and merge under problem:prototype#A-1; Argus holds no `approve-merge` grant and performs neither. This entry is written ahead of the act it describes, as E-17, E-24 and E-27 were.

### E-31 — brief-emitted — the implementation review cannot complete on the configured review backend

- **Date**: 2026-10-11
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: watcher run `run-e7821a377a3db30c`, finished with 0 succeeded, 2 failed, 3 blocked; local branch `feat/t1-create-and-remove-a-prototype-worktree-for-an-1552` at `6e7871f8`; local branch `feat/t2-admit-the-prototype-config-block-and-launch-p-1553` at `fd7cc8fc`; nothing pushed; issues 1552 and 1553 open and unclaimed
- **Class**: override
- **Body**: The decision needed: which implementation-review route may the engine use for this plan's tasks on this machine. What happened: the watcher dispatched T1 and T2. Both workers implemented their task and passed the full local gate, T1 with no recovery attempt and T2 after one. Both then failed closed at the engine's pre-publish review, with their work committed locally and nothing published. T2's reviewer launched, could not obtain a thread identifier from its first lens subagent, and returned an escalation handoff saying the review is absent, so the required lens evidence does not exist. The same handoff, for the same cause, ended the two previous reviewer runs on this machine, for pull requests 1545 and 1546 on 2026-10-09. T1 failed one step earlier, on a name-resolution error while the review was being admitted. The configured review backend therefore cannot currently produce a review here, and every task of this plan will stop at the same point. Why it is the operator's: the review backend is machine policy the operator set, and the stage's own guide forbids changing review policy to make work pass. Options: (a) the operator names another installed harness as this machine's review backend, and the drive is run again; the finished work is kept and only the gate and the review repeat; the risk is that the other backend's review is unproven here since lens evidence became required. (b) Keep the backend and repair the reviewer's subagent evidence path first; the drive waits on an engine fix of unknown size. (c) The operator reviews T1 and T2 directly and directs their publication without the engine's review; this bypasses a fail-closed gate and is not recommended. Evidence: the engine log for the run; the reviewer process logs for the three handoffs; the two local branches. Citations: problem:prototype#A-1; delivery:prototype#E-30. Parked; the session ends here and resumes on the operator's answer.

### E-32 — answer-received — keep the configured reviewer and repair it

- **Date**: 2026-10-11
- **Principal**: the human · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `docs/problems/prototype/problem.md` A-4; merged [PR #1557](https://github.com/agenticlab-ai/pyxis/pull/1557), mainline commit `11d63e816d9a`
- **Body**: Cites problem:prototype#A-4, the operator's answer to brief E-31: option (b), keep the configured review backend and repair the reviewer's subagent evidence path. The repair was made outside this drive, on the operator's direct instruction, and merged: the review launch now keeps its session records and the host reads the lens children from them. The operator also considered moving the lenses into four separate launches and kept the existing approach, knowing that the records are written in the reviewer's own state. The problem is no longer parked.

### E-33 — stage-entered — implementation resumed under the self-drive watcher

- **Date**: 2026-10-11
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: the watcher process for plan `prototype/001` on this machine, started on mainline commit `11d63e816d9a` after the commit that carries this entry; issues 1552 and 1553 open and unclaimed at the start
- **Body**: Resumed at the reconstructed frontier. Verified at the goal objects before acting: both tasks are open and unclaimed on the tracker, neither task branch is published, and the finished work of E-31 is still committed on the two local task branches. The watcher is run again on current mainline, which carries the repair; for T1 and T2 the gate and the pre-publish review repeat. Argus implements nothing and performs no approval or merge. Written ahead of the act it describes, as E-30 was.

### E-34 — reconciliation — three resumed sweeps: T2 published, T1 stopped at the review time bound

- **Date**: 2026-10-11
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: [PR #1559](https://github.com/agenticlab-ai/pyxis/pull/1559) (T2, opened by the engine as a draft); the orphaned claim ref for issue 1552 released; local branch `feat/t1-create-and-remove-a-prototype-worktree-for-an-1552` unpublished; watcher runs `run-1ab5520c62e43458`, `run-ae548b154f37a65e` and `run-cdfb12144c799296`
- **Body**: First resumed sweep: T2 passed the gate and its pre-publish review completed with four lens children accepted from the harness's session records, verdict approve; the first review to complete on the configured backend since the repair. Publication was then refused because mainline had advanced during verification, by the unrelated merge of pull request 1546, and the watcher exited. T1 was skipped: the first run's claim had not been released, because its release failed during the name-resolution outage of E-31 and no local reaper exists. Verified the holder process was gone and released that exact claim with the guarded delete the engine uses for its own claims. Second sweep, on the newer mainline: T2 passed gate and review again, was published as pull request 1559, and its hosted checks are green. T1's review ran its four lenses and gathered candidates but exceeded the reviewer's fifteen-minute bound before reporting. Third sweep, T1 alone: the same, with nine candidates gathered when the bound expired. The bound is the operator's machine setting; a longer one was recommended to the operator and not applied. Argus implemented nothing and approved nothing.

### E-35 — reconciliation — T2 merged; T1 reviewed to a verdict four times and escalated

- **Date**: 2026-10-11
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: merged [PR #1559](https://github.com/agenticlab-ai/pyxis/pull/1559) (T2), mainline commit `473d2030e6e8`, issue 1553 closed; the review time bound on this machine set to thirty minutes; watcher run `run-26303f853035ac50`, finished with 0 succeeded, 1 failed, 3 blocked; local branch `feat/t1-create-and-remove-a-prototype-worktree-for-an-1552` at `710e614a`, unpublished; issue 1552 open and unclaimed
- **Body**: Cites problem:prototype#A-5: the operator raised the review time bound to thirty minutes, applied in this machine's local configuration; nothing else about the review changed. Fourth resumed sweep, T1 alone. While it ran, the operator merged pull request 1559; verified at the goal object: merged by the operator, the mainline commit named above carries T2, and its issue is closed. T1's next gate and review ran on that newer mainline, so publication met no mainline refusal. T1's review now reaches a verdict every round. Round one, eleven minutes: request changes, two blocking findings. Round two, twenty-one minutes: request changes, four. Round three, twenty-six minutes: request changes, seven. Round four, twenty-two minutes: escalate, five blocking findings, one of them a conflict the reviewer says the contract must settle. Rounds two to four would each have been cut off at the former bound. The engine allows three fix rounds; all three were used, the task ended failed with a fail-closed escalation, the claim was released, and nothing was published. Across the four reviewed heads the task's code outside tests grew from 335 lines to 404, 592 and 950, against the plan's size band of 160 to 260; its tests grew from 579 lines to 1712; the last head adds a direct module dependency and changes eighteen files outside the plan's touch list. The worker session reported 21.9 million tokens at the end of its last turn. Argus implemented nothing and approved nothing.

### E-36 — brief-emitted — T1 cannot pass its review as the contract is written

- **Date**: 2026-10-11
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: local branch `feat/t1-create-and-remove-a-prototype-worktree-for-an-1552` at `710e614a`, kept and unpublished; issue 1552 open and unclaimed; T3, T4 and T5 remain blocked behind T1
- **Class**: deadlock
- **Body**: The decision needed: how task T1 proceeds. What happened: E-35. Why the rounds did not converge, in two parts. First, a defect in the locked contract. Design decision D-1 places the prototype directory beside the main checkout and says the main checkout is resolved from the Git common directory, the same from any linked worktree. In a repository created with a separate Git directory, Git keeps no link from a linked worktree back to the main checkout; verified with Git itself on this machine. D-1 cannot be met there, and any refusal or required setting is behaviour the contract does not define. The reviewer raised this in round one, the worker's answers to it grew each round, and round four escalated it as a conflict. Second, the contract states no limit on what the command defends against. It says a failed creation removes whatever it created and that the invoking checkout is unchanged on every path, with no bound. The reviewer's security and correctness lenses hold those words against another process changing the same branch or directory between two steps, symlinked parents, and path names with trailing spaces. Each fix added code, and each round found new cases in the added code: ownership records, guarded ref deletion, a platform-specific rename. The engine has no stop for a task that outgrows its size band or its touch list. Why it is the operator's: the three fix rounds are exhausted, and the remaining gap is the wording of a locked contract, which moves only by reopening the contract as a new version that the operator approves. Options: (a) Reopen the contract as version 2 with two additions to D-1: the command supports the ordinary layout, with the Git directory inside the main checkout, and refuses any other by name; and the command relies on Git's own commands, locks and refusals, with no guarantee when another process changes the prototype's branch or directory while it runs. Lock it, then run T1 again from a fresh branch. Recommended: it removes the cause and returns the task to its planned size. Risks: one more draft round and lock pull request for the operator; the plan names lock version 1 as its basis and whether it needs a refresh is not yet verified; the three hours of work on the current branch are set aside. (b) Reopen the contract with the layout sentence only and run T1 again on the current branch. Risks: four other blocking findings stay open on a branch already more than three times its size band, the finding count has gone two, four, seven, five, and nothing indicates the next rounds end differently. (c) The operator reviews the current branch directly and directs its publication without the engine's review. This bypasses a fail-closed gate and ships the oversized change; not recommended. Evidence: the engine log and the retained review evidence for the four rounds of run `run-26303f853035ac50`; the local task branch. Citations: problem:prototype#A-1; problem:prototype#A-5; delivery:prototype#E-35. Parked; the session ends here and resumes on the operator's answer.

### E-37 — answer-received — reopen the contract as version 2

- **Date**: 2026-10-15
- **Principal**: the human · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `docs/problems/prototype/problem.md` A-6
- **Body**: Cites problem:prototype#A-6, the operator's answer to brief E-36: option (a). Before answering, the operator merged two engine changes prompted by E-35. Both were made outside this drive on the operator's direct instruction. [PR #1566](https://github.com/agenticlab-ai/pyxis/pull/1566), mainline commit `705a4155`, has reviewers escalate a conflict with the task charter or governing design instead of prescribing a different design. [PR #1568](https://github.com/agenticlab-ai/pyxis/pull/1568), mainline commit `56a0f6c9`, lets the worker dispute a blocking finding with a reason, which escalates to the operator, and reports a fix round's changes away from its findings as scope drift. T1's next run is their first on a real task. One risk E-36 left unverified is now settled: `pyxis draft reopen` refuses while any task of the current plan is unfinished, so the plan must be refreshed. The operator ruled on that in the same answer: Argus cancels plan 001's open tasks, and plan 002 is drafted from version 2. The problem is no longer parked.

### E-38 — stage-entered — contract version 2 draft

- **Date**: 2026-10-15
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: issues [#1552](https://github.com/agenticlab-ai/pyxis/issues/1552) (T1), [#1554](https://github.com/agenticlab-ai/pyxis/issues/1554) (T3), [#1555](https://github.com/agenticlab-ai/pyxis/issues/1555) (T4) and [#1556](https://github.com/agenticlab-ai/pyxis/issues/1556) (T5) closed as not planned, each with a comment citing E-36 and A-6; branch `draft/prototype-v2` and its draft pull request, opened by `pyxis draft reopen` as the configured agent account after the commit that carries this entry
- **Body**: Read the authority contract at the act: tier-2, without `approve-merge`. Closing the four tasks is the operator's act, delegated in A-6; it needs no grant. T1's local branch at `710e614a` is set aside, unpublished. T2's issue 1553 stays closed: its merged change keeps tracing version 1. Entered contract drafting by role on the reopen route, with cause `gap-lock-should-have-caught`: the D-1 defect of E-36 should have been caught before lock v1. Version 2 starts from version 1's exact text. Its only intended change is the two D-1 sentences of A-6; the round adds nothing else. Both contract reviews run at the version-2 digest before the lock is prepared, and the operator approves and merges the lock under problem:prototype#A-1. Written ahead of the acts it describes, as E-27 and E-30 were.

### E-39 — reconciliation — contract reopened locally as version 2; the round applies A-6

- **Date**: 2026-10-15
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: issues [#1552](https://github.com/agenticlab-ai/pyxis/issues/1552), [#1554](https://github.com/agenticlab-ai/pyxis/issues/1554), [#1555](https://github.com/agenticlab-ai/pyxis/issues/1555) and [#1556](https://github.com/agenticlab-ai/pyxis/issues/1556) closed as not planned, with their comments; local branch `draft/prototype-v2` at `2253a135`, unpublished; no draft pull request yet
- **Body**: Verified at the goal objects: the four issues are closed as not planned, each with the comment citing E-36 and A-6; issue 1553 stays closed as completed and epic 1551 stays open. The first `pyxis draft reopen` run was refused at its own commit. Its evidence reference named E-36, which the commit hook's ledger-citation check could not resolve on a branch cut from mainline, where the journal ends at E-27. The command rolled back and queued a step-failure event, which the next run wrote into the draft's event log. The second run cites issue 1552 instead: the task whose reviews exposed the defect, which now carries the cancellation comment. It committed version 1's text as the version-2 seed. Its own push was refused by the pre-push readiness check, as the first scaffold's was in E-2: the two contract reviews do not exist yet, and four citations in the tech spec do not resolve. The branch was kept and nothing was published. The four citations name the design brainstorm `prototype-tech-design`. That name resolves while a draft carries its brainstorm files, but a reopened draft carries none, and lock v1 stored both brainstorm records in memory under the feature's own name. This is an engine defect in the readiness check, reported to the operator. The round renames them `design-brainstorm:prototype#D-n`, which resolves against the locked records. It also replaces the tech spec's path to the draft's brainstorm file, which no longer exists, with the locked record's place. No decision changes with either repair. The round then applies A-6 to D-1. The layout sentence goes beside the directory rule, with its refusal folded into step 5 so no step is renumbered. A second sentence limits the verb to Git's own commands, locks and refusals, and the verification gains the separate-Git-directory refusal. The PRD is unchanged: UB-1 asks only that an injected creation failure leave the draft branch unchanged.

### E-40 — stage-entered — independent contract reviews of version 2

- **Date**: 2026-10-15
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: committed subject `a2cda8f0a486db9dd4da57a781cc8f78ee369871`; contract digest `sha256:ba5a041dfd27a7ba4145c10dc2ffcfe1d7daa5a61647bd70d34a4332b9f81a95`; `drafts/prototype/reviews/009.md`; `drafts/prototype/reviews/010.md`
- **Body**: Entered requirement review and technical review together at one digest, as E-13 did. Both subjects remain low risk: the PRD is unchanged from version 1 at tier T1, and the design's change narrows D-1 and adds no security or factory-control surface. One cold reviewer session on this machine's configured review backend reviews both in sequence, with no authoring context. It reads the subjects at the committed head and writes only the two reports. This is version 2's first review episode, so the whole subject is in scope, with the change since lock v1 supplied as a computed diff. No verdict is claimed at this entry.

### E-41 — reconciliation — version 2 round-1 verdicts and the one disposition

- **Date**: 2026-10-15
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `drafts/prototype/reviews/009.md`; `drafts/prototype/reviews/010.md`; the revised tech spec in the commit that carries this entry
- **Body**: Read the authority contract at the act: tier-2 grants `dispose-blocking-finding`. The PRD review approved with no findings. The tech-spec review requested changes with one blocking finding, FND-1 in report 010: D-1's numbered create steps fetch mainline at step 4 before the new layout check at step 5, so an unsupported layout would be fetched into before it is refused, against A-6's "before changing anything". Applied: the layout check and its named refusal move into step 1, ahead of the fetch; step 5 again checks only the existing branch and directory, and no step is renumbered. The layout paragraph states where each form refuses, and the verification observes the refusal by both forms before any fetch or other Git change. No locking or layout support is added and the PRD is unchanged. The digest moves, so both reviews run again; the second round is scope-locked to the changed spans.

### E-42 — reconciliation — version 2 round-2 verdicts; publish the reviewed draft

- **Date**: 2026-10-15
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: `drafts/prototype/reviews/011.md`; `drafts/prototype/reviews/012.md`
- **Body**: A fresh reviewer session ran the scope-locked second round from the computed delta. The PRD blob is identical at both heads, and its approval holds at the new digest. The tech-spec review judged FND-1 of report 010 closed against its acceptance condition and found no regression in the changed spans; it checked the fetch's effect against Git and the existing draft verb. No finding was declined and nothing escalates. Zero blocking findings remain open, and both reviews name digest `sha256:2599c039b74afda07f42732844014b1fddb13af935da306582dd280f95c0e2be`. The readiness check then required version 2's reports to continue after the numbers lock v1 used, so the four reports were renumbered 009 to 012 with their cross-references, and E-40 to this entry name the new files; no report's content changed.

### E-43 — reconciliation — the draft's publication refused at the attribution boundary; the journal is published first

- **Date**: 2026-10-15
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1, tier-2
- **External effect**: branch `docs/prototype-drive-journal-2`, published at the commit that carries this ledger's capture, and the pull request opened from it (identified by its head branch); local branch `draft/prototype-v2` at `dee4d2a6`, unpublished
- **Body**: The draft passed the readiness check, and its push was refused by the pre-push attribution check. After a problem's first pull request, a drive may write only the draft paths that a merged journal entry names. The merged journal ends at E-27, whose entries name version 1's reports 001 to 008. Version 2's reports 009 to 012 are named only in E-40 to E-42, which are unpublished. The refusal stands. The journal from E-28, rulings A-4 to A-6 and both ledger captures are therefore published first, in their own pull request, which touches only this problem's ledgers and captures. The operator reviews and merges it under problem:prototype#A-1. The draft branch is then rebuilt on that mainline with the same contract bytes and reports, so the digest both reviews name is unchanged, and published with its draft pull request. The reviewed heads the reports name stay on the local branch `draft/prototype-v2-reviewed`. Lock preparation follows the draft's publication. Its prepared head will be journaled in the next proposal, since an entry on the draft branch would move the head the operator is asked to approve. This entry is written ahead of the push and the pull request it describes.
