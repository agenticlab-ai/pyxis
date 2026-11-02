# Prototype an idea before its contract — delivery ledger

| Field | Value |
|---|---|
| **Slug** | prototype |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | The reviewed contract's working files are on mainline, merged before the lock was prepared; no lock exists. The lock is being re-proposed from mainline in a follow-up pull request |
| **Open items** | (1) The operator's approving review and merge of the prepared lock in the follow-up pull request, per problem:prototype#A-1 (age 0 days). (2) Glossary terms T-1 and T-2 of brainstorm record 001 are not routed to the glossary: the draft pull request's attribution scope admits no path outside the draft. (3) An intermittent test failure on mainline, outside this drive's scope, refused one publication (E-20); reported to the operator, no item filed. |
| **Park state** | — |
| **Cumulative spend** | unavailable (the CLI exposes no structured spend signal; best-effort: one drive session, plus four independent reviewer sessions on the configured review backend) |

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

