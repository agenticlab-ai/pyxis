# Feature draft — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Solution-space session for the feature-draft contract: where an open draft lives and what locks it, how its parts are stored and authored in memory, how the plan reaches today's projection, where readiness runs, how the lock approval is authenticated, how the memory gate accepts native contracts, and how the document pipeline's gates relax at the switch-over. |
| **Relevant-when** | designing or changing the feature draft — its storage, rounds, readiness, lock, approval, reopen, plan handoff, migration, or the switch-over from the document pipeline; checking prior rulings or refused shapes about memory-native contracts or one-PR contract landing |
| **Basis** | `prd:feature-draft` — unmerged draft ledger on the same branch, admitted by D-1 |
| **Authority** | @mdarabi (operator) |
| **Date(s)** | 2026-09-23 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | `docs/tech-design/feature-draft-v1.md` · `docs/tech-design/contract-gate-v2.md` |
| **Prior records** | `brainstorm:feature-draft` |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/feature-draft/effective.md` | the basis: FR-1..FR-47, NR-1..NR-5, open questions Q1–Q11 |
| `docs/brainstorms/feature-draft.md` | the stage-1 record feeding the basis; its design pointers P-1..P-7 and fog G-1 |
| `docs/tech-design/index.md` | routing surface: no live design serves `prd:feature-draft`; the matches below |
| `cmd/memory/README.md` | the memory model the basis stores everything in: records, Documents and render, packet, reconcile, closeout, evidence |
| `docs/tech-design/memory-staged-contracts.md` | Relevant-when match: a PRD merging before its design |
| `docs/tech-design/standalone-docs.md` | Relevant-when match: a bundled PRD-plus-design change rejected (`prd:docgate/FR-30`) |
| `docs/tech-design/contract-gate-v1.md` | Relevant-when match: the planning-entry precondition and its five mechanical checks |
| `docs/tech-design/memory-durable-evidence-v1.md` | Relevant-when match: evidence before and after squash merges |
| `docs/tech-design/memory-causal-admission-v1.md` | Relevant-when match: candidate/base comparison after an unrelated rebase |
| `docs/tech-design/plan-new-v1.md` | Relevant-when match: the plan document class, freeze-at-merge, projection |
| `docs/tech-design/solution-shape-v1.md` | Relevant-when match: the design/shape split, only the design freezes |
| `docs/architecture-principles.md` | AP-1, AP-7, AP-11, AP-21, AP-23, AP-24, cited by the matched designs |
| `docs/observability/incidents.md` | I1, I2, I4, I6, I9: failures of per-stage freezing and stacked trains |

**Overlaps surfaced:**
- `design:memory-staged-contracts` §4 rejected bundling a PRD and its design in one PR because it contradicts `prd:docgate/FR-30`. The basis overrides that rule for draft-born features (FR-27, FR-30), so the direction here replaces that rejection for draft-born features without re-litigating it for document-pipeline features.
- `design:standalone-docs` decides one governed class per PR. It stays in force for document-pipeline features (NR-3); draft-born contracts produce no governed document class.
- `design:contract-gate-v1` already decides five mechanical contract checks at planning entry (currency, inverse coverage, links, structure, compilability) over docgate's machine faces. Readiness (FR-11 to FR-16, FR-37 to FR-42) reuses them rather than re-deciding them.
- `design:plan-new-v1` decides the plan class, freeze-at-merge, and projection. FR-25 and NR-1 keep its projection; only where a draft's plan lives is open here.
- `design:memory-durable-evidence-v1` and `design:memory-causal-admission-v1` decide evidence and rebase admission. A long-lived draft must satisfy both.
- No live design decides a draft's storage, lock, approval, or switch-over, so the session is not `covered`.

## Design questions

- **DQ-1**: Where does an open feature draft live while it stays off the mainline, and what event makes it the locked version?
- **DQ-2**: How are the draft's PRD, tech spec, brainstorm records, and review reports represented and authored in memory?
- **DQ-3**: How is the plan represented inside the draft, and how does the locked plan reach today's plan projection unchanged?
- **DQ-4**: Where is readiness evaluated, over which contract model, and against which mainline?
- **DQ-5**: How is the lock approval authenticated as the human operator and bound to the exact draft version presented?
- **DQ-6**: How does the memory gate recognize a draft-born feature and authorize its memory-native requirement-to-decision links, and how does that authority land before any draft relies on it?
- **DQ-7**: How does a long-lived draft satisfy the memory gate's ancestry and evidence rules at the lock?
- **DQ-8**: How is the switch-over configured, and which document-pipeline checks relax, refuse, or stay?
- **DQ-9**: How does an existing document-pipeline feature move into a draft, including known capture gaps?
- **DQ-10**: How do the stage skills and the two reviewers operate on a draft, and how is a review bound to the version it reviewed?
- **DQ-11**: How does Argus drive a draft, and where do its problem and delivery ledgers live for a draft-born feature?
- **DQ-12**: What does abandoning an open draft do, and what record remains?
- **DQ-13**: How do several sessions edit one draft without losing each other's edits?
- **DQ-14**: How is each feature's contract state reported, and how does a replaced locked version stay readable?
- **DQ-15**: How is the living rollup `docs/techspec.md` kept from colliding with the per-feature term "tech spec"?
- **DQ-16**: How does the lock guarantee that readiness was computed against the mainline it lands on, when main lets a pull request merge on checks run against an older mainline (F-13)?
- **DQ-17**: How is each lock (its version number, approver in both roles, and capture line) recorded, when the approval arrives after the draft's last commit?

## Panel log

- **DQ-1**: opens — fired — a draft's home and its lock event are a new state and storage seam; no existing mechanism holds an unlocked multi-stage contract
- **DQ-2**: opens — fired — the authoring surface of a memory-native contract is new; no skill writes memory today (`brainstorm:feature-draft#F-3`)
- **DQ-3**: replaces — fired — where the locked plan lives and how projection reads it replaces the plan document as the handoff source
- **DQ-4**: replaces — fired — readiness replaces the document-fed planning gate for draft-born features; three structurally different homes exist
- **DQ-5**: opens — fired — authenticating the lock approval is a new trust seam; nothing enforces a human approval on the mainline today (F-17)
- **DQ-6**: opens — fired — authorizing memory-native requirement-to-decision links is a new classification seam in the reconciler (F-4)
- **DQ-7**: extends — fired — forced by D-3: compiled records must survive the removal of their working files, so the evidence rule's shape needs a choice
- **DQ-8**: extends — waived — the operator dropped the draft-mode setting (D-14); the refusals (FR-27, widened by D-15) are unconditional after the release; the Q3 part was put to the operator without a panel, as a pure extension of FR-27
- **DQ-9**: opens — waived — the operator dropped migration from this release (D-14)
- **DQ-10**: extends — waived — delegated to the tech spec (D-14): skills edit the draft's working files; each review records the version it read; the lock needs the last reviews to match the locked version
- **DQ-11**: extends — waived — delegated to the tech spec (D-14): Argus runs rounds and requests the lock; the operator's review stays the approval; its ledgers stay as today
- **DQ-12**: extends — skipped — abandoning closes an existing seam (the draft pull request, D-2); what remains is the operator's product question Q11
- **DQ-13**: extends — waived — delegated to the tech spec (D-14): plain git on the draft branch, one file per part
- **DQ-14**: extends — waived — delegated to the tech spec (D-14): state derived from open draft PRs, merged locks, and existing ledgers; old versions via `memory render --at`
- **DQ-15**: extends — skipped — a naming question on an existing document; no seam
- **DQ-16**: replaces — fired — the landing check replaces merging on possibly stale checks; three structurally different mechanisms exist
- **DQ-17**: opens — waived — delegated to the tech spec (D-14): an annotated tag per lock on the merge commit, carrying the version, approver, and capture line from the approval review

## Decisions

### D-1 — Design over the unmerged draft PRD, one PR for every stage

- **Question**: The stage-3 guide admits only a merged PRD. The feature-draft PRD is unmerged on its branch. Proceed, and how does the work land?
- **Answer**: The operator's direction: "given we are moving to the world of draft, let's keep going with the tech design brainstorm and tech design and everything else and we raise one PR for all of them." Stage 3, the tech spec, its review, and the plan continue on the PRD's branch and land with the PRD as one PR.
- **Reasoning**: This dogfoods the draft being designed: one contract, one approval (`brainstorm:feature-draft#D-1`). The PRD may still move under the design, which the draft model accepts (`prd:feature-draft/FR-2`, FR-8, FR-9). The single PR fails `check-standalone-docs` (F-8) until the switch-over relaxes it; how it merges is the operator's call at the PR.
- **Anchor**: 2026-09-23 · operator message after the main repair (#1226) merged
- **Status**: live

### D-2 — A draft is a pushed branch with a draft pull request; the approved squash merge locks it

- **Question**: Where does an open draft live off the mainline, and what event makes it the locked version (DQ-1)?
- **Answer**: ALT-1. One pushed branch per draft, with a code-host draft pull request from the first round; each round is a push. The operator's approval plus the squash merge is the lock, and the merge commit is locked version N. A later change is a new branch from the mainline while version N binds.
- **Reasoning**: It reuses merge-base immutability, the packet, the in-flight scan, CI, and freeze-at-merge, with no new storage. ALT-2 needs two landing mechanisms and hides rounds; ALT-3 contradicts FR-5.
- **Anchor**: 2026-09-23 · DQ-1 turn
- **Status**: live

### D-3 — Authors edit markdown working files; a compiler writes the records; only records land

- **Question**: How are the draft's PRD, tech spec, brainstorm records, and review reports represented and authored (DQ-2)?
- **Answer**: ALT-5. Authors edit markdown working files on the draft branch (`drafts/<feature>/prd.md`, `tech-spec.md`, and the brainstorm and review documents), close to today's templates. A deterministic compiler turns them into memory records at every round, and the lock commit removes the working files, so only records land.
- **Reasoning**: Familiar authoring and readable markdown diffs during rounds, and today's templates and skills adapt with little change. The operator chose this over the recommended ALT-4; the cost, two representations during a draft, is closed by an exact compile check.
- **Anchor**: 2026-09-23 · DQ-2 turn
- **Status**: live

### D-4 — The plan is not stored in memory; it is the bridge to execution

- **Question**: What lands for the plan at the lock, and where does projection read it (DQ-3, offered as ALT-7 to ALT-9)?
- **Answer**: The operator: "plan is the only artifact that should not be stored in the memory system given there is no learning or information. it is just a way to connect the idea (prd/techspec) to the execution engine." With D-2, the plan stays today's plan document on the draft branch and locks with the same merge (ALT-10).
- **Reasoning**: The plan carries no learning of its own and only connects the contract to the execution engine, so storing it adds work and no knowledge. Plangate and the executor stay unchanged (NR-1). It contradicts FR-3 as written, parked as P-1.
- **Anchor**: 2026-09-23 · DQ-3 turn (operator clarification)
- **Status**: superseded by D-12 — the plan leaves the draft and follows the lock; it still stays out of memory

### D-5 — The plan ruling stands over FR-3, applied in this draft

- **Question**: FR-3, from `brainstorm:feature-draft#D-16`, stores every part of a draft in memory, the plan included; D-4 keeps the plan out. Which stands (P-1)?
- **Answer**: A — D-4 stands. FR-3 becomes "every part except its plan" and its fit criterion drops the plan root; FR-29, M3, Q3, Q6, and the In one line, Direction, In-scope, Decommission, and data-dictionary lines follow; `brainstorm:feature-draft#D-24` supersedes D-16 for the plan.
- **Reasoning**: Recommended: the ruling is the product side's, and the PRD is still a draft in this same change, so the correction applies in place, as the draft model's FR-8 and FR-9 intend, rather than through an amendment.
- **Anchor**: 2026-09-23 · P-1 turn
- **Status**: live

### D-6 — Readiness runs in memory at the lock; plangate validates the plan

- **Question**: Where does the readiness check run, the check that must pass before a lock (DQ-4, offered as ALT-11 to ALT-13)?
- **Answer**: The operator: "i expect at the lock time there will be some check like plangate we have today that ensures every requirement has techspec association and vise versa. this can be done in memory. plangate would act as the validatoin for the tasks/plan." The requirement-to-tech-spec association is checked both ways in memory at the lock, and plangate validates the plan and its tasks (ALT-11).
- **Reasoning**: Memory holds both ends of every association, so the check runs where the data lives, once (AP-11), and plangate keeps its role as the plan's validator. "Vice versa" adds a condition the basis lacks, parked as P-2.
- **Anchor**: 2026-09-23 · DQ-4 turn (operator clarification)
- **Status**: superseded by D-7 — the reverse association check is withdrawn; the rest carries into D-7

### D-7 — Readiness keeps the current coverage rule; no reverse check

- **Question**: Does the lock check also require every tech-spec decision to serve a requirement, and how far does requirement coverage reach (P-2)?
- **Answer**: The operator: "no need to change the existing ruling. skip what i just said. stick to the current rules (every must requirements have a techspec ruling and that is it)." Readiness is computed in memory at the lock and plangate validates the plan (from D-6); coverage is FR-13 as written, with no reverse check.
- **Reasoning**: The operator withdrew the "vice versa" part of D-6, so the basis's coverage rule stands unchanged and no requirement is added.
- **Anchor**: 2026-09-23 · P-2 turn (operator clarification)
- **Status**: live

### D-8 — The lock approval is the operator's code-host review, enforced by branch protection

- **Question**: How is the lock approval authenticated as the operator and bound to the exact version (DQ-5)?
- **Answer**: ALT-14. Branch protection requires the operator's approving review as code owner, dismisses approvals when new commits arrive, requires approval of the latest push, and binds admins. The review body carries the capture line, and the lock reads the review through an approval port.
- **Reasoning**: Recommended: the operator's normal review becomes the enforced lock approval, bound to the exact head (FR-46, FR-47), with no key setup. It holds only while agents never hold the operator's credentials (F-16).
- **Anchor**: 2026-09-23 · DQ-5 turn
- **Status**: live

### D-9 — The landing race is set aside for now

- **Question**: How does the lock guarantee readiness ran against the mainline it lands on, when the mainline can move before the merge (DQ-16, offered as ALT-17 to ALT-19)?
- **Answer**: The operator: "let's not worry about this case for now." No landing mechanism is chosen; readiness runs against the mainline current at the lock request, as the draft PR's required check.
- **Reasoning**: Deferred by the operator. FR-24's fit criterion names this race, so the gap is parked as P-3 (proceed-anyway) and carried into the design as an open question (G-1).
- **Anchor**: 2026-09-23 · DQ-16 turn
- **Status**: live

### D-10 — A native decision may serve a natively written requirement; captured requirements keep document rules

- **Question**: The operator asked why the gate must tell draft-born features apart at all; after the explanation (every draft link exists only in memory, while document features keep their fake-coverage protection), how does the gate tell them apart (DQ-6)?
- **Answer**: A — judge each requirement (ALT-23). A natively written requirement may be served by a native decision; a requirement copied from a PRD document row stays covered only by its design document.
- **Reasoning**: Recommended: no marker, list, or label to declare or forge, and NR-4 holds for every document-pipeline feature. Because the gate refuses candidate-selected rules (F-7), this change lands before the first draft-born lock.
- **Anchor**: 2026-09-23 · DQ-6 turn (after the operator's clarification)
- **Status**: live

### D-11 — Records are compiled once, when the lock is prepared

- **Question**: The operator asked when records reach the mainline (only at the lock). Does the draft branch also hold compiled records during rounds (DQ-7)?
- **Answer**: B (ALT-27). Rounds commit markdown only, and readiness compiles on the fly without writing. Preparing the lock rebases the draft onto the mainline, compiles the records once, and removes the working files.
- **Reasoning**: The operator's choice over the recommended per-round compile: readable round diffs and one fresh compile. Accepted consequences: other drafts see this one only through on-the-fly compilation (FR-23, G-2), and the parts are in memory from the lock onward (FR-3, P-4).
- **Anchor**: 2026-09-23 · DQ-7 turn (after the operator's clarification)
- **Status**: live

### D-12 — The draft is the PRD and tech spec; it reaches memory at the lock; the plan follows

- **Question**: Asked how FR-3 should read when records are compiled only at the lock, the operator restated the model.
- **Answer**: "During the feature draft, we will just working on the PRD tech spec. After the log time, we write them into the memory, and then we decide to generate the plan out of it or not. So to be clear, PRD tech spec is a bundle as a draft. Let's call them draft. And then plan comes after that. So at the log time of the draft, everything will go to the memory, and a copy of the file will be stored. And that's it. At that time, we should check the memory and everything." ("log time" is the lock, in a voice transcript.)
- **Reasoning**: The draft is the PRD and tech spec bundle; at the lock it is written into memory with a stored copy of its files, and every check runs then. The plan is an optional later step that stays out of memory (carried from D-4). Where the file copy lives is asked next; the PRD revision is parked as P-5.
- **Anchor**: 2026-09-23 · FR-3 wording turn (operator restatement)
- **Status**: live

### D-13 — The stored copy of each file lives inside memory

- **Question**: At the lock the draft goes into memory and "a copy of the file will be stored". Where does the copy live: inside memory, or as frozen files in the repository?
- **Answer**: A — inside memory. At the lock memory stores each file's exact text as a literal copy, as it captures documents today, and the markdown working files leave the repository.
- **Reasoning**: Recommended: one home, with no second copy on the mainline that could drift or need its own immutability check; `memory render` shows the locked PRD exactly as approved. Consistent with D-3 (only records land) and FR-29.
- **Anchor**: 2026-09-23 · file-copy turn
- **Status**: live

### D-14 — Mechanics delegated to the tech spec; day one, with no migration and no switch

- **Question**: Delegate the remaining mechanics to the tech spec with the stated directions: switch-over setting, skills and reviewers, Argus, concurrency, state reporting, lock record, migration, overrides, plan traces?
- **Answer**: The operator: "agree with everything except do not worry about migration or feature control flag. assume it is day one on the pyxis with the new draft process and no other existing repo uses pyxis." Delegated: DQ-10, DQ-11, DQ-13, DQ-14, DQ-17, the overrides (Q8), and plan traces (Q6), with the stated directions. Dropped: migration (DQ-9) and any draft-mode setting (DQ-8's switch).
- **Reasoning**: A clean start keeps the release to the draft process; the tech-spec review and the lock approval check the delegated directions. The dropped scope changes the basis, parked as P-6.
- **Anchor**: 2026-09-23 · mechanics turn
- **Status**: live

### D-15 — New brainstorm and review documents are refused too

- **Question**: The PRD's Q3: after the release, do the gates also refuse new brainstorm and review documents, as they refuse new PRDs and tech-design documents (FR-27), or do the reworked skills (FR-34) suffice?
- **Answer**: A — refuse them too. Any new file under the brainstorm and review roots is refused with the feature draft named as the path; plan documents stay allowed.
- **Reasoning**: Recommended: the closed corpus (M3) then holds by rule, not by discipline; a stray document from an old skill is caught on its own pull request. Applied as FR-27 and UB-5 in the PRD.
- **Anchor**: 2026-09-23 · Q3 turn
- **Status**: live

### D-16 — An abandoned draft lands one reason record

- **Question**: The PRD's Q11: what does abandoning an open draft do, and what record remains, given that nothing reaches memory before a lock?
- **Answer**: A — the draft's pull request is reduced to a single memory Learning stating why the draft was abandoned; the operator approves and merges it, and the branch is deleted.
- **Reasoning**: Recommended: the reason is failure-and-judgment signal that AP-28 keeps in git, and memory search finds it later; it costs one approval and no new mechanism beyond the draft PR.
- **Anchor**: 2026-09-23 · Q11 turn
- **Status**: live

### D-17 — The architecture rollup keeps its name, with a note

- **Question**: The PRD's Q1: how is the living rollup `docs/techspec.md` kept apart from a feature's "tech spec", and do the rollups survive?
- **Answer**: The operator: "docs/techspec.md is the rollup of all the changes to the architecture/design of the system that can be indepdent of the memory; still it i snot a source of truth but we want to keep it around." Then A: the rollup keeps its name, and its header states that it is the system-wide architecture rollup, not a feature's tech spec.
- **Reasoning**: Recommended once the rollups stay: no churn across the 51 files that cite it, and the note redirects anyone who lands on it; a feature's tech spec lives in its draft, then in memory.
- **Anchor**: 2026-09-23 · Q1 turn (after the operator's clarification)
- **Status**: live

### D-18 — The contract gate is superseded, scoped to document-pipeline features

- **Question**: The live `design:contract-gate-v1` §3.10 binds plan-new to run the contract gate before any plan, which refuses a draft-born feature (no PRD ledger); FR-31 overrides that for draft-born features. Narrow it with a recorded deviation, supersede it, or have the gate forward to memory readiness?
- **Answer**: B — supersede it. The introducing change adds `contract-gate-v2`, which carries v1's decisions forward with the planning-entry precondition scoped to document-pipeline features; plan-new's precondition for a draft-born feature is the locked draft's memory readiness.
- **Reasoning**: The operator chose the conventions' clean path over the recommended recorded deviation: two live designs never disagree, at the cost of a second design in the same change.
- **Anchor**: 2026-09-23 · review (tech-design-new authoring gap)
- **Status**: live

### D-19 — Unruled requirement changes are surfaced at the lock, not refused

- **Question**: The planning dry run found no mechanism for UB-1 and FR-18: with markdown working files anyone can edit a requirement row. When may a requirement change, and how is an unruled edit caught?
- **Answer**: C — nothing refuses an edit mechanically. Engineering-stage skills record requirement problems and never edit rows, and the lock's approval request lists every requirement row changed during the draft, each with its ruling or marked unruled, for the operator to judge. UB-1, FR-18, ABUSE-1, and NFR-1 change in the PRD to match.
- **Reasoning**: The operator chose review at the lock over the recommended ruling-gated edits once the tech spec starts; the approval request makes a silent rewrite visible before anything locks.
- **Anchor**: 2026-09-23 · review (plan-new dry run)
- **Status**: live

### D-20 — The operator's required review covers the contract records only

- **Question**: The shape made the operator code owner of every path, which would stop the live merge lane's machine identity from ever merging (`design:merge-lane-v1`). Scope the required review to the contract records a lock adds, or keep it repository-wide?
- **Answer**: A — the operator owns the contract records only; a lock always needs the operator's review, and proposals that add no contract records keep today's merge policy, the merge lane included.
- **Reasoning**: Recommended: it keeps D-8's guarantee for every lock without contradicting a live design.
- **Anchor**: 2026-09-23 · review (plan-new dry run)
- **Status**: live

## Alternatives

### ALT-1 — Draft branch with a pull request; the approved merge is the lock

- **Shape**: One pushed branch per draft, with a code-host draft pull request from the first pushed round. Rounds are commits whose memory records stay unlanded, so they can be edited in place (F-3). The lock is the operator's approval plus the squash merge; the merge commit is locked version N. A reopen is a new branch from the mainline.
- **Trade-offs**: Reuses the merge-base immutability, the packet as the PR body (F-2), memory's in-flight scan of other branches, CI gates, and freeze-at-merge. Costs: a long-lived branch drifts and must rebase before the lock (FR-24; DQ-7), and the review surface is the code host's.
- **Principles**: AP-7 honored (immutable once the adding change is approved: the merge); AP-13 honored (the lock is a verifiable mainline commit); AP-22 honored (merge is atomic); AP-1 strained (the approval surface is a code-host feature; DQ-5 must keep it behind a port).
- **Incidents**: I2 avoided (one PR per draft, no stacked train); I4, I6, I9 addressed (the PRD, tech spec, and plan freeze together).
- **Verdict**: chosen — one mechanism for rounds, review, and lock; everything it needs exists today (D-2)
- **From**: DQ-1 · panel

### ALT-2 — Draft refs outside branches; a lock record lands the draft

- **Shape**: Rounds are commits on a dedicated ref namespace (`refs/drafts/<feature>/<n>`), with no branch or pull request while drafting. At the lock a tool checks readiness, obtains approval, and lands the draft plus a `Lock` record naming the draft's digest on the mainline.
- **Trade-offs**: Keeps branch and PR lists clean, and the lock is a first-class record. Costs: drafts are invisible in the code host's interface during rounds; memory's in-flight scan and CI must learn the namespace; a pull request is still needed at the lock for branch protection, so there are two mechanisms for one event.
- **Principles**: AP-1 honored (the lock is a record, not a medium event); AP-11 strained (two landing mechanisms); AP-18 strained (draft state lives in refs few tools observe).
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: rejected — two landing mechanisms for one event, and rounds invisible on the code host (D-2)
- **From**: DQ-1 · panel

### ALT-3 — Draft records on the mainline under a draft status

- **Shape**: Records land on the mainline as they are written, tagged with a draft identity. Every binding reader excludes drafts until a `Lock` record lands.
- **Trade-offs**: No drift and no rebase, and everyone sees in-flight work. Costs: unapproved content on the mainline, contradicting the basis's FR-5; a draft filter in every reader; abandoned drafts left in history.
- **Principles**: AP-7 strained (content lands before approval); AP-20 strained (every reader carries the blast radius).
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: rejected — contradicts FR-5, draft content stays off the mainline (D-2)
- **From**: DQ-1 · panel

### ALT-4 — Record-native authoring

- **Shape**: Stage skills write records directly. Requirement rows become typed claims on `prd/<feature>`, and the PRD's prose is one `prd` Document whose body embeds `{{FR-n}}` markers (F-1). The tech spec is a `design/<feature>` Document plus Decisions serving claims; brainstorm records and review reports are their own Documents. People read `memory render` and the packet.
- **Trade-offs**: One representation, and render, packet, pack, and collide work unchanged. Each record is one file, so concurrent edits rarely collide. Costs: pull-request diffs are JSON records, so review goes through the rendered document and the packet, and the templates need memory-native forms.
- **Principles**: AP-11 honored (one representation); AP-23 honored (one file per item); AP-10 honored (typed records, not prose conventions); AP-6 honored (contractual data between stages).
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: rejected — review would run through rendered output instead of readable markdown diffs; the operator preferred familiar working files (D-3)
- **From**: DQ-2 · panel

### ALT-5 — Markdown working files compiled into records

- **Shape**: Authors edit familiar markdown drafts on the draft branch (`drafts/<feature>/prd.md`, `tech-spec.md`), close to today's templates. A deterministic compiler turns them into the same records at every round. The lock commit removes the working files, so only records land.
- **Trade-offs**: Familiar authoring and readable markdown diffs during rounds, and today's templates and skills adapt easily. Costs: two representations during every draft, a new compiler to build and keep exact, and a lock step that must prove the records equal the files.
- **Principles**: AP-10 honored (the compiler enforces the grammar); AP-11 strained (two representations of one contract); AP-24 honored (records derive from human-authored files, not the reverse).
- **Incidents**: I9 pattern risk — a generated face drifting from its source; mitigated only by an exact compile check.
- **Verdict**: chosen — familiar authoring and readable diffs during rounds; only records land (D-3)
- **From**: DQ-2 · panel

### ALT-6 — Today's documents on the branch, captured by reconciliation

- **Shape**: Authors keep writing `docs/prds/<feature>/000-base.md` and `docs/tech-design/<slug>.md` on the draft branch; reconciliation captures them as literal Documents and typed claims, as it does today; the sources retire after the lock.
- **Trade-offs**: No new authoring surface at all. Costs: it contradicts FR-27 (no new ledger entries or designs) and FR-29 (no document source for a draft-born feature), keeps the document gates in force, and doubles every contract until closeout.
- **Principles**: AP-11 strained (source plus capture); AP-7 honored.
- **Incidents**: I4, I6 pattern — document-class rules fighting the joint lifecycle.
- **Verdict**: rejected — contradicts FR-27 and FR-29 and keeps the document gates (D-3)
- **From**: DQ-2 · panel

### ALT-7 — Plan records only; projection reads them from the locked version

- **Shape**: The plan is a working file (`drafts/<feature>/plan.md`, today's plan grammar) compiled into one `Plan` record per task plus a plan Document, like the other parts (D-3). Plangate's rules run on the compiled plan at every push (FR-37). Only records land; `plangate project` gains a memory-backed reader that assembles today's `plan/v1` machine face from the records at the locked commit.
- **Trade-offs**: One representation, consistent with D-3, and task identity is a record that later work can trace (FR-44). Costs: plangate needs a second plan source behind its reader; the tracker output is unchanged (NR-1).
- **Principles**: AP-11 honored (one plan representation); AP-6 honored (the same `plan/v1` contract crosses to projection); AP-24 honored.
- **Incidents**: I9 addressed — a plan locks with its requirements, so a later change reopens the draft instead of silently stranding traces.
- **Verdict**: rejected — stores the plan in memory, against D-4
- **From**: DQ-3 · panel

### ALT-8 — Plan records plus a generated plan document at the lock

- **Shape**: Drafting as in ALT-7. The lock commit also writes `docs/plans/<feature>/NNN-plan.{md,json}` generated from the records, and projection reads that pair exactly as today.
- **Trade-offs**: Zero change to plangate. Costs: two copies of the plan on the mainline, a check proving they match forever, and a committed generated projection, which memory's model avoids (F-1).
- **Principles**: AP-11 strained (two plan representations on the mainline); AP-10 honored (the match is checked in code).
- **Incidents**: I9 pattern risk — a generated face drifting from its source.
- **Verdict**: rejected — stores the plan in memory twice, against D-4
- **From**: DQ-3 · panel

### ALT-9 — Today's plan document, captured in memory

- **Shape**: The plan is authored directly as `docs/plans/<feature>/NNN-plan.{md,json}` on the draft branch, lands at the lock unchanged, and reconciliation captures it as a literal Document.
- **Trade-offs**: No plan compiler and no plangate change. Costs: the plan alone keeps a document source, unlike the PRD and tech spec (D-3); task identity stays a document anchor, not a record.
- **Principles**: AP-11 strained (the plan and the rest of the contract follow different models); AP-7 honored.
- **Incidents**: I9 pattern — document anchors stranded by later requirement changes.
- **Verdict**: rejected — captures the plan in memory, against D-4; its document half survives as ALT-10
- **From**: DQ-3 · panel

### ALT-10 — Today's plan document, outside memory

- **Shape**: The plan is today's `docs/plans/<feature>/NNN-plan.{md,json}` on the draft branch. It locks with the same merge (D-2), and plangate folds and projects it unchanged. Memory holds no plan record and no capture of it; readiness reads the plan file for coverage (FR-15) and plangate's rules (FR-37).
- **Trade-offs**: No plan compiler, no plangate change, and no capture: the plan stays a disposable bridge to execution. Costs: plan traces cite requirement IDs that now live in memory, so trace resolution must read memory, and the reconciler must stop capturing draft-born plans.
- **Principles**: AP-11 honored (one plan representation); AP-19 honored (no memory work for an artifact that carries no learning); AP-7 honored (frozen at the merge).
- **Incidents**: I9 addressed — a requirement change reopens the draft with its plan, so traces cannot strand silently.
- **Verdict**: chosen (D-4)
- **From**: DQ-3 · hand-authored from the operator's clarification

### ALT-11 — Memory-native readiness

- **Shape**: One readiness verdict computed in the memory tool from memory's own predicates: completion (a Must with no serving decision), unanswered questions, supersession and dangling relations. New memory-side checks add fit and verification presence, required PRD sections, citation resolution, and review dispositions. Plangate supplies plan coverage and plan rules from the plan document (FR-15, FR-37). The same verb runs locally, as the draft PR's required check, and at the lock.
- **Trade-offs**: Each condition over memory data has one implementation, reusing checks memory already runs for its gate. Costs: the currency, link, structural, and graph checks are written again over the memory model, while the document-era contract gate keeps serving document-pipeline features until they migrate.
- **Principles**: AP-11 honored (readiness is one implementation over memory data, shared by local, CI, and lock); AP-10 honored; AP-6 honored (one closed verdict).
- **Incidents**: I4 addressed (inverse coverage is checked on every round, before anything freezes); I6 addressed (currency runs before the lock).
- **Verdict**: chosen — readiness is computed once in memory, where the data lives; plangate validates the plan (D-6, restated by D-7)
- **From**: DQ-4 · panel

### ALT-12 — The contract gate reads memory

- **Shape**: `docgate contract-gate` gains a memory-backed contract source; its five checks run over the compiled records, draft-only conditions join its check registry, and plangate's precheck keeps its shape.
- **Trade-offs**: The contract gate's check logic is reused across both pipelines. Costs: the document tool learns the memory model, and coverage over memory data would then be computed twice, by the contract gate and by memory's own completion check.
- **Principles**: AP-11 strained (two components computing coverage over the same memory data); AP-10 honored.
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: rejected — the document gate would learn the memory model and compute coverage a second time (D-6)
- **From**: DQ-4 · panel

### ALT-13 — A temporary document projection feeds the unchanged contract gate

- **Shape**: At each readiness run, generate throwaway docgate machine faces (a PRD projection and design-index rows) from memory, run the unchanged contract gate, and add a small checker for the draft-only conditions.
- **Trade-offs**: No change to the contract gate. Costs: a generated projection that must be exact, two checkers stitched into one verdict, and memory's completion still computing coverage separately.
- **Principles**: AP-11 strained (coverage twice; two verdict sources); AP-19 strained (projection work on every run).
- **Incidents**: I9 pattern risk — a generated face drifting from its source.
- **Verdict**: rejected — a throwaway projection and two stitched checkers (D-6)
- **From**: DQ-4 · panel

### ALT-14 — The code host's review approval, enforced and read through a port

- **Shape**: Branch protection requires an approving review from a code owner (the operator) on the draft PR, dismisses approvals when new commits arrive, requires approval of the latest push, and binds admins. The approval review's body carries the capture line (FR-45). The lock check reads the review through an approval port whose first adapter is the code host.
- **Trade-offs**: The operator approves exactly as with any PR today, and the code host binds the approval to the exact head (FR-47). Costs: authenticity rests on agents never holding the operator's credentials (F-16), and branch protection must change: required reviews, stale dismissal, and admins bound.
- **Principles**: AP-1 honored through the port (the core sees an approval, not the code host); AP-26 honored (the lock checks that the approved commit is the version readiness passed); AP-10 honored (branch protection, not convention).
- **Incidents**: I2 pattern — a code-host badge certifies its own object; the lock must check the approved commit, not the badge.
- **Verdict**: chosen — the operator's normal review, bound to the exact head by the code host (D-8)
- **From**: DQ-5 · panel

### ALT-15 — A lock record signed with the operator's hardware key

- **Shape**: The operator runs an approve command and touches a hardware security key. A signed lock record naming the draft's content digest, the readiness digest, and the capture line is committed as the draft's last change; the gate verifies it against allowed signers on the mainline, excluding the record from its own digest. Any later change breaks the digest (FR-47).
- **Trade-offs**: Medium-independent, and agents cannot forge a signature that needs a physical touch. Costs: a hardware key and signing setup, a new approve command, and the merge button still separate from the approval.
- **Principles**: AP-1 honored (git plus signatures only); AP-13 honored (a verifiable artifact); AP-17 honored (a missing or wrong signature fails closed).
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: rejected — hardware-key setup and a second approve step outweigh medium independence while the code host's review can be bound through a port (D-8)
- **From**: DQ-5 · panel

### ALT-16 — Both the code-host approval and a signed record

- **Shape**: ALT-14's enforced review for the merge, plus ALT-15's signed lock record for the durable, medium-independent approval.
- **Trade-offs**: The strongest authentication and a portable record. Costs: two ceremonies on every lock, and two approval sources that can disagree.
- **Principles**: AP-11 strained (two approval predicates); AP-1 honored.
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: rejected — two ceremonies per lock and two approval sources (D-8)
- **From**: DQ-5 · panel

### ALT-17 — A merge queue checks the exact landing tree

- **Shape**: Enable the code host's merge queue on the mainline. Required checks, readiness and the memory gate included, run on the queue's merge group: the mainline plus the queued changes ahead plus this one. The lock lands only if they pass there.
- **Trade-offs**: FR-24 holds by construction, for drafts and every other pull request, and closes the stale-green merge that broke main after #1224. Costs: every merge waits for one CI run on the queue, and the workflows must also run on the queue's event.
- **Principles**: AP-13 honored (the landing tree itself is verified); AP-15 honored (a fresh read for a consequential decision); AP-26 honored (the queue's status certifies the tree that lands).
- **Incidents**: I2 pattern closed (a badge no longer certifies a different base than the one landing); the #1224 stale merge would have been refused.
- **Verdict**: deferred with the landing race (D-9, G-1); the recommended candidate when it is designed
- **From**: DQ-16 · panel

### ALT-18 — Require branches to be up to date before merging

- **Shape**: Turn on the code host's up-to-date requirement: a pull request merges only after it contains the latest mainline and its checks passed there.
- **Trade-offs**: FR-24 holds with no new workflow event. Costs: every mainline change forces every open pull request to rebase and re-run CI, a treadmill that grows with parallel work.
- **Principles**: AP-15 honored; AP-23 strained (parallel authors serialize on every mainline move); AP-19 strained (repeated CI on every rebase).
- **Incidents**: the #1224 stale merge would have been refused.
- **Verdict**: deferred with the landing race (D-9, G-1)
- **From**: DQ-16 · panel

### ALT-19 — A lock command re-checks on a trial merge

- **Shape**: `pyxis draft lock <feature>` fetches the mainline, builds a trial merge of the draft onto it, runs readiness and the memory gate there, verifies the approval, and merges through the code host only if all pass.
- **Trade-offs**: Only draft locks pay the cost, and nothing changes for other pull requests. Costs: a race window between the trial check and the merge; other pull requests can still merge on stale checks; the merge button must be blocked for drafts.
- **Principles**: AP-15 honored for drafts only; AP-11 strained (a second landing path beside the merge button).
- **Incidents**: the #1224 class stays open for every non-draft pull request.
- **Verdict**: deferred with the landing race (D-9, G-1)
- **From**: DQ-16 · panel

### ALT-20 — Recognize a draft-born feature from its observed lineage

- **Shape**: The reconciler treats a feature as draft-born when no PRD ledger exists or ever existed for its slug and its requirement claims carry no document provenance. For such a feature it authorizes a link from a tech-spec Decision to a requirement claim when both are active native records of that feature. Every other feature keeps today's document-derived authorization (NR-4).
- **Trade-offs**: Nothing to declare or forge, and a document-pipeline feature can never slip into native authority. Costs: migration (FR-28) needs its own explicit transition, because a migrated feature does have a ledger history.
- **Principles**: AP-18 honored (keys on observed lineage); AP-17 honored (anything ambiguous stays under document rules); AP-24 honored.
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: hybridized into ALT-23 — the same observed-lineage idea, judged per requirement instead of per feature (D-10)
- **From**: DQ-6 · panel

### ALT-21 — A draft-origin marker record

- **Shape**: Opening a draft writes a `feature/<slug>` record declaring the feature draft-born. The reconciler authorizes native links for features carrying an active marker, and refuses a marker on a feature that has a PRD ledger.
- **Trade-offs**: Explicit and easy to read. Costs: the proposal that adds the marker also benefits from it, the candidate-selected authority memory's gate forbids (F-7), unless a second rule re-derives what ALT-20 observes anyway.
- **Principles**: AP-18 strained (declared, not observed); AP-17 strained (a forged marker widens authority).
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: rejected — a candidate-added marker grants its own authority (D-10)
- **From**: DQ-6 · panel

### ALT-22 — A list of draft-born features in the repository configuration

- **Shape**: `.pyxis/config.yaml` lists draft-born features, and the reconciler reads the list at the trusted base, so a feature must be listed before its draft can lock.
- **Trade-offs**: A single visible list. Costs: an extra proposal per feature, against one proposal per draft (M1), and a shared append surface every feature edits.
- **Principles**: AP-23 strained (a shared list as a conflict magnet); AP-24 honored.
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: rejected — an extra proposal per feature and a shared list (D-10)
- **From**: DQ-6 · panel

### ALT-23 — Judge each requirement by its own provenance

- **Shape**: A native tech-spec Decision may serve a requirement claim that was itself written natively in memory. A claim that mirrors a PRD document row stays covered only by what a design document declares. No feature-level label, marker, or list exists.
- **Trade-offs**: Nothing to declare or forge, and document-pipeline features keep their fake-coverage protection (NR-4) with no special case. Costs: the reconciler must tell native claims from captured ones, which their provenance tags already record.
- **Principles**: AP-18 honored (keys on each record's observed provenance); AP-17 honored (a captured requirement fails closed); AP-11 honored (one authorization rule).
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: chosen (D-10)
- **From**: DQ-6 · hand-authored in the clarification turn

### ALT-24 — Compiled records carry no working-file evidence; code fingerprints refresh by recompiling

- **Shape**: Records compiled from working files carry no file evidence, since memory is their source and the files are scaffolding removed at the lock. A decision that names code carries that code unit's fingerprint. Before the lock the draft rebases onto the current mainline and recompiles, refreshing every fingerprint; the rebase also satisfies the gate's ancestry rule.
- **Trade-offs**: The lock passes the evidence check with no manual repair, and drafts can live for days. Costs: the compiler must be deterministic, so that recompiling unchanged files after a rebase changes nothing except the refreshed fingerprints.
- **Principles**: AP-13 honored (evidence matches the landing tree); AP-22 honored (recompile converges); AP-17 honored (a stale fingerprint fails closed until recompiled).
- **Incidents**: the #1224 aftermath — records whose evidence no longer matched the landing tree blocked every PR.
- **Verdict**: rejected — the operator preferred markdown-only rounds and a single compile (D-11)
- **From**: DQ-7 · panel

### ALT-25 — Compiled records cite their working files as evidence

- **Shape**: Each compiled record's provenance names its working file, as `--source` does today, and the lock keeps the working files so the evidence resolves.
- **Trade-offs**: Each record points back at the exact text it came from. Costs: the working files must land on the mainline, contradicting D-3 and FR-29, and every round's edit invalidates the evidence of every record from that file.
- **Principles**: AP-11 strained (two sources on the mainline); AP-7 honored.
- **Incidents**: the #1224 aftermath — evidence tied to files that later change.
- **Verdict**: rejected — contradicts D-3: the working files would have to land (D-11)
- **From**: DQ-7 · panel

### ALT-26 — Records are written only once, at the lock

- **Shape**: Rounds edit only working files; the compiler writes the records once, in the lock commit.
- **Trade-offs**: No stale evidence at all. Costs: the draft's parts are not in memory during rounds, against FR-3, and readiness cannot run in memory before the lock (D-6).
- **Principles**: AP-13 strained (readiness has nothing to read until the end).
- **Incidents**: none matched — checked `docs/observability/incidents.md` 2026-09-23.
- **Verdict**: hybridized into ALT-27 — first presented as leaving readiness nothing to read; corrected, readiness compiles on the fly (D-11)
- **From**: DQ-7 · panel

### ALT-27 — Compile once, when preparing the lock; readers compile on the fly

- **Shape**: Rounds commit markdown working files only. Readiness, and any other reader of an open draft, compiles the working files in process and writes nothing. Preparing the lock rebases the draft onto the mainline, compiles the records once, and removes the working files, so only freshly compiled records land.
- **Trade-offs**: Rounds show readable markdown diffs, and the records that land are compiled against the landing mainline, so their evidence is fresh. Costs: other drafts see this one only through on-the-fly compilation (FR-23), and the parts are in memory from the lock onward (FR-3).
- **Principles**: AP-13 honored (evidence matches the landing tree); AP-11 honored (one deterministic compiler for every reader); AP-19 honored (no record churn during rounds).
- **Incidents**: the #1224 aftermath — records whose evidence no longer matched the landing tree; avoided by compiling at the landing moment.
- **Verdict**: chosen (D-11)
- **From**: DQ-7 · reframed after the operator asked when records reach the mainline

## Seam map

- **S-1**: draft storage and the lock event — extends — behind it: one branch and one draft pull request per draft; the approved squash merge — feeds: Scope (draft lifecycle); Serves FR-1, FR-2, FR-5, FR-7, FR-19, FR-20, FR-21, FR-22
- **S-2**: draft authoring surface — opens — behind it: markdown working files under `drafts/<feature>/`, a deterministic compiler to memory records, and an exact compile check at every push; the lock removes the files — feeds: Scope (authoring and compile); Serves FR-2, FR-3, FR-4, FR-29
- **S-3**: plan handoff — extends — behind it: today's plan document on the draft branch, folded and projected by plangate unchanged; traces resolve requirement IDs against memory — feeds: Scope (plan); Serves FR-15, FR-25, FR-26, FR-37, FR-44
- **S-4**: readiness — extends — behind it: one memory-side readiness verdict built on memory's completion and question checks, plus plangate for plan coverage and rules; run locally, as the draft PR's required check, and at the lock — feeds: Scope (readiness); Serves FR-11 to FR-16, FR-24, FR-37 to FR-42
- **S-5**: lock approval — opens — behind it: an approval port whose first adapter reads the code host's enforced review (approver, approved commit, capture line); branch protection requires the operator's review, dismisses stale approvals, binds admins — feeds: Scope (lock); Serves FR-17, FR-19, FR-45, FR-46, FR-47
- **S-6**: native coverage authorization — replaces — behind it: the reconciler authorizes a native Decision serving a natively written requirement claim, while captured claims keep document-declared coverage (NR-4); the evaluator change lands before the first draft-born lock (F-7) — feeds: Scope (memory gate); Serves FR-29, NR-4
- **S-7**: lock preparation — opens — behind it: one step that rebases the draft onto the mainline, compiles its records once, and removes the working files; readers of an open draft compile on the fly — feeds: Scope (lock preparation); Serves FR-3, FR-5, FR-24, FR-29
- **S-8**: document-gate refusals — extends — behind it: after the release docgate refuses new PRD ledger entries, technical designs, brainstorm records, and review reports, with the feature draft named as the path, and no setting; plan documents stay allowed — feeds: Scope (switch-over); Serves FR-27, FR-30, FR-36, UB-5
- **S-9**: abandonment — extends — behind it: the draft's pull request reduced to one memory Learning with the reason, approved and merged, and its branch deleted — feeds: Scope (abandonment); answers the basis's Q11
- **S-10**: planning entry for draft-born features — replaces — behind it: `contract-gate-v2` superseding `design:contract-gate-v1`, its planning-entry precondition scoped to document-pipeline features; a draft-born feature's planning precondition is its locked version's memory readiness — feeds: Scope (plan handoff); Serves FR-31, FR-6

## Facts

- **F-1**: A PRD or design narrative is a `Document` record on its subject with family `prd` or `design`. Its markdown body embeds claims by reference (`{{FR-3}}`); `memory render` produces the human document, `memory revise` writes the next version, and `render --at <ref>` renders it as of a commit — source: `cmd/memory/README.md` (Documents).
- **F-2**: `memory packet --body` renders a proposal's body from its records: the promise delta, the design delta, supersessions, undesigned claims, and in-flight work on the same families — source: `cmd/memory/README.md` (The flow, step 2); `brainstorm:feature-draft#F-7`.
- **F-3**: Record immutability is judged against the branch's merge base, so a record added on an unmerged branch may be edited in place — source: `brainstorm:feature-draft#F-2`.
- **F-4**: The reconciler authorizes a requirement-to-decision link only through document ledgers and the design index; a native claim served by a native Decision fails `unauthorized-prd-serves` — source: `brainstorm:feature-draft#F-1`; `internal/memory/reconcile`.
- **F-5**: Before merge, `record-introduction` evidence must match the exact landing tree; after a squash, it is verified at the record's first mainline introduction — source: `design:memory-durable-evidence-v1` §3.2.
- **F-6**: Draft evidence repair re-binds an unlanded record only when its original evidence bytes still match; changed originals refuse repair — source: `cmd/memory/README.md` (Durable evidence); `design:memory-durable-evidence-v1` §3.6.
- **F-7**: The memory gate requires the trusted base's own policy to reproduce exactly, so a candidate cannot select weaker enforcement for itself — source: `internal/memory/cli/reconcile.go`; `design:memory-staged-contracts` §3.5.
- **F-8**: `check-standalone-docs` fails a diff holding two governed classes (a PRD ledger entry and a technical design) or one class plus capability paths (`cmd/`, `internal/`, `.pyxis/`, `.github/workflows/`). Plans, brainstorm records, reviews, and `memory/` are not governed classes — source: `internal/docgate/gate/reg_check_standalone_docs.go`; `internal/docgate/config/config.go`.
- **F-9**: The contract gate reads the working tree's machine faces; its "not merged" refusal checks only that the feature's ledger directory exists — source: `internal/docgate/contractgate/resolve.go`.
- **F-10**: `plan-new`'s "design merged with Approved status" precondition is a guide rule; `plangate project` refuses any plan not reachable as merged content — source: `.pyxis/authoring/plan-new.md` step 1; `design:plan-new-v1` §3.1.
- **F-11**: In this repository, docgate runs from source in CI and in the desk hooks, so a pull request's own gate-code change applies to that pull request — source: `.pyxis/scripts/docgate-resolve.sh`.
- **F-12**: The `Plan` record kind exists with zero records; plans are `docs/plans/<slug>/NNN-plan.{md,json}` — source: `brainstorm:feature-draft#F-4`; `design:plan-new-v1` §3.1.
- **F-13**: Main is squash-merged with branch protection `strict: false` and no merge queue — source: `brainstorm:feature-draft#F-15`; main's history.
- **F-14**: A record's author is self-declared (`--actor`, `$MEMORY_ACTOR`, git `user.email`) — source: `brainstorm:feature-draft#F-16`.
- **F-15**: Incidents I1, I4, I6, and I9 each trace to per-stage freezing: a design landing before its requirements, coverage found only after a design froze, a lineage remedy blocked by immutability, and a frozen plan invalidated by a later amendment — source: `docs/observability/incidents.md`.
- **F-16**: Agent sessions on the operator's machine use the GitHub account `mdarabiassistant` (scopes `repo`, `workflow`); both it and the operator's `mdarabi` are repository admins — source: `gh auth status`; the repository collaborators API, 2026-09-23.
- **F-17**: Main's branch protection requires only the `docs-gate` status check: no pull-request review is required, stale approvals are not dismissed, and admins are not bound — source: the branch-protection API for `main`, 2026-09-23.

## Fog

- **G-1** (non-blocking — blocks: the design's open question on FR-24): how a lock is refused when the mainline moves between the readiness run and the merge; the candidates are ALT-17 (merge queue), ALT-18 (up-to-date branches), and ALT-19 (lock command), deferred by D-9
- **G-2** (non-blocking — blocks: the design's handling of FR-23 and of the draft PR's body): how readers that need an open draft's content (the in-flight notice, the pull-request packet, readiness) compile its working files on the fly, including other open drafts' branches

## Out of scope

- **O-1**: Migrating existing document-pipeline features into drafts — not in this release; existing features keep their frozen documents (from D-14)
- **O-2**: A setting that turns the draft process on or off — none; the draft process is how Pyxis contracts features (from D-14)
- **O-3**: Repositories other than Pyxis's own — none use Pyxis (from D-14)
- **O-4**: Drafting, locking, or storing the plan with the contract — the plan follows the lock as today's plan document, outside memory (from D-4, D-12)
- **O-5**: A check that every tech-spec decision serves a requirement — not added; coverage stays FR-13 (from D-7)
- **O-6**: Refusing a lock whose mainline moved between the readiness run and the merge — deferred; carried as G-1 (from D-9)
- **O-7**: Renaming or retiring the architecture rollup `docs/techspec.md` (from D-17)

## Candidate glossary terms

- **T-1**: working files — the markdown files of an open draft on its branch (`drafts/<feature>/…`), compiled into memory records when the lock is prepared and then removed · avoid: *draft documents* (reads as the document pipeline's files)
- **T-2**: lock preparation — the step that rebases a draft onto the mainline, compiles its records once, and removes its working files before the approval · avoid: *finalize*, *freeze*
- **T-3**: architecture rollup — the living `docs/techspec.md`, a non-authoritative summary of the system's design · avoid: *tech spec* for it (the term names a feature's design)

## Parked product gaps

- **P-1**: The basis stores every part of a draft in memory, the plan included; D-4 keeps the plan out of memory — anchors: `prd:feature-draft/FR-3` (statement, and a fit criterion allowing no new files under the plan root), the Direction and In-scope lines, the data dictionary's "locked version" row, the §7.3 row for `brainstorm:feature-draft#D-16`, and `brainstorm:feature-draft#D-16` — disposition: ruled in-draft per D-5; applied to the PRD, the glossary, and `brainstorm:feature-draft#D-24`
- **P-2**: D-6 checks the association both ways, but the basis requires only that every Must requirement is served (FR-13) and never refuses a tech-spec decision that serves no requirement (FR-42 checks only that links resolve) — anchors: `prd:feature-draft/FR-13`, FR-42, and the readiness ranges in FR-11 and FR-31 — disposition: withdrawn per D-7; FR-13 stands and no requirement changes
- **P-3**: FR-24's fit criterion refuses a lock whose readiness passed before another draft locked a conflicting change; D-9 defers that race — anchors: `prd:feature-draft/FR-24`, UB-8 — disposition: proceed-anyway per D-9; the design serves FR-24 at the lock request and carries the race as G-1
- **P-4**: FR-3 requires every part of a draft except its plan to be retrievable from the repository memory, but under D-11 an open draft's parts are working files and reach memory only at the lock — anchors: `prd:feature-draft/FR-3` — disposition: resolved by D-12 (the draft is written to memory at the lock); FR-3 is restated with P-5
- **P-5**: The basis puts the plan inside the draft and the lock, which D-12 removes: the plan follows the lock as an optional step — anchors: `prd:feature-draft/FR-1`, FR-3, FR-6, FR-15, FR-19, FR-25, FR-26, FR-31, FR-37, FR-43, FR-44, M1, the glossary's "feature draft", "lock", and "locked version", and `brainstorm:feature-draft#D-1` — disposition: ruled in-draft per D-12; applied to the PRD (FR-1, FR-3, FR-15, FR-19, FR-25, FR-26, FR-37, UB-10, DOD-4, DOD-9, the readiness ranges, narrative, flows, data dictionary), the glossary, and `brainstorm:feature-draft#D-25`, which supersedes D-3, D-5, D-7, and D-8 there
- **P-6**: The basis migrates existing features on their next change and keeps installed repositories on the document pipeline; D-14 drops both — anchors: `prd:feature-draft/FR-28`, NR-2, FR-43 (its migration clause), NR-5, Q2, R3, F6, and `brainstorm:feature-draft#D-6`, #D-14, #D-23 — disposition: ruled in-draft per D-14; FR-28 and NR-2 removed, migration moved to WR-5, the rest restated, and `brainstorm:feature-draft#D-26` supersedes D-6, D-14, and D-23 there

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-09-23 |
| **Authority** | @mdarabi (operator) |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog |
| **Amend route** | — |
