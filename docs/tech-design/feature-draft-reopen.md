# Feature draft reopen — a reopened draft starts from the locked text and its lock writes only what changed — technical design

| Field | Value |
|---|---|
| **Status** | Approved — operator review 2026-09-24 *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-09-24 |
| **Relevant-when** | designing or changing what a reopened feature draft starts from, what its lock writes to memory against the binding locked version, how brainstorm, review, and problem numbers and requirement IDs continue across versions, or how a plan keeps resolving a requirement that a later version retired; implementing the reopen command or the lock-preparation compile for a reopened draft; superseding this design |
| **Scope** | A reopened draft's starting content; what its lock writes relative to the binding version; per-feature numbering and ID reuse across versions; plan-trace resolution after a retirement; the lock's check that the draft started from the binding version's text. **Not**: when a reopen is refused and how a lock is approved (`design:feature-draft-v1`); the approver-identity guard, the readiness check on proposals that are not drafts, and counting a merged plan's unprojected tasks as unfinished (the implementation plan); where a draft-born feature's concrete layouts live (the plan's stage-skill task). |
| **Tracking** | agenticlab-ai/pyxis — the introducing PR (filed with this design) |
| **Builds-on** | docs/tech-design/feature-draft-v1.md · docs/tech-design/plan-new-routed-demand.md |
| **Supersedes** | — |
| **Serves** | prd:feature-draft/FR-20 · prd:feature-draft/FR-22 |
| **Security/privacy impact** | none — no identity, permission, or approval surface changes; a reopened draft locks only through `design:feature-draft-v1`'s approval |
| **Template version** | tech-design-template/v1 |

## 0. Read to rule — the human reviewer's section (~4 min)

*The body is the contract; this section only routes attention — on any conflict the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | **The next lock writes only what differs from the binding version** (§2, §3.6) | every unit re-issued at each lock (`design-brainstorm:feature-draft-tech-design-2#ALT-5`) | each lock writes the whole contract again → §3.6, #DINV-2 |
| 2 | **An add-on that builds on feature-draft-v1, not a successor** (§2) | a successor to v1, which forces one to contract-gate-v2 (`#ALT-1`); the rule in the plan (`#ALT-3`) | v1 and contract-gate-v2 are superseded, or the rule moves into a plan → §1, §4 |
| 3 | **A decision serving a reworded requirement is re-issued even when its own text is unchanged** (§3.6) | compare text only (§5 R1) | the reworded requirement loses its serving link and readiness refuses the lock → §3.6, A-3 |
| 4 | **A reopened draft with no change to its PRD or tech spec is refused, not locked** (§3.6) | lock an identical next version (`#ALT-7`) | a version number can mark no contract change → §3.6, A-8 |

**Sanity-check the numbers** — a reopened draft's digest equals the binding version's before any edit (A-1, #DINV-1); an unchanged unit writes zero records (A-2, #DINV-2); no number or ID is ever reused (A-4, #DINV-4); a first commit that differs from the binding text by even one byte refuses the lock (A-7, #DINV-6).

**The author's open questions** — none — every mechanic follows from the record's rulings and the facts it cites.

**Skip the rest** — header, anchors, and structure → docgate; substance and reuse → the unmanned `tech-design-review`; implementation defects → codegate and the memory gate; rejected shapes → §4 (explanatory).

## 1. Problem

`design:feature-draft-v1` fixes that a change to a locked feature reopens it as a new draft on its own branch while version N binds, and that the next lock replaces version N (§3.1, S-1). It leaves three things open (`design-brainstorm:feature-draft-tech-design-2#F-4`). What does the reopened draft start from? What does its lock write to memory relative to version N? How do its numbered files continue?

Each gap fails in a concrete way. Memory admits a record only if it names the active record it replaces, and it reports two active claims for one requirement as a blocking conflict (`#F-1`). A lock that writes every row afresh is therefore refused. A review numbered from 001 again lands in version N's review family (`#F-6`). A requirement retired by a later version can leave an earlier plan with unresolvable traces, which is incident I9 (`#F-10`). The fix belongs in a design before planning, while no reopen code exists.

## 2. Decision

**Decision:** A reopened draft starts from the binding version's stored PRD and tech-spec text. Its lock writes only what differs from the binding version:
- a changed unit supersedes the binding record;
- a removed row's records are revoked;
- an unchanged unit writes nothing.

Brainstorm, review, and problem numbers continue across versions, and IDs are never reused (`design-brainstorm:feature-draft-tech-design-2#D-2`). This design builds on `design:feature-draft-v1` without replacing it (`#D-1`).

The reasoning, stated once:
- A locked version becomes memory as of its lock commit, which feature-draft-v1 already renders (S-7).
- The records a lock writes are exactly the change its approval request lists (`design:feature-draft-v1` §3.9).
- Memory's rule that a record names the active record it replaces holds by construction.
- Nothing this design adds makes a binding statement of feature-draft-v1 false (`#F-3`). Both v1 and `design:contract-gate-v2`, which builds on it, stay live.

## 3. The design

### 3.1 The seed

**Decision:** The reopen command writes the binding version's stored `prd.md` and `tech-spec.md` text, byte for byte, as the draft's first commit. The problems and events files start empty, and no brainstorm record or review report is copied: they stay in memory as the binding version's history (`design-brainstorm:feature-draft-tech-design-2#D-2`; `brainstorm:feature-draft#D-25`).

The stored text is the exact text the binding lock wrote (`prd:feature-draft/FR-3`; `design:feature-draft-v1` §3.7). The draft digest is computed over the same paths and bytes, so right after the seed it equals the binding version's digest (feature-draft-v1 S-1). Every row first appears in the seed commit. So the approval request's list of requirement changes starts from the binding text, and a removed row counts as a change (`design:feature-draft-v1` #DINV-11).

### 3.2 Boundaries and non-goals

- **Not solved here** — when a reopen is refused, how a lock is approved, and the lock record (`design:feature-draft-v1` §3.1, §3.9); moving existing document-pipeline features into drafts (`prd:feature-draft/WR-5`).
- **Owned by the implementation plan** — the approver-identity guard on opening a draft, the readiness check passing on proposals that are not drafts, and counting a merged plan's unprojected tasks as unfinished (`design-brainstorm:feature-draft-tech-design-2#O-1`); where a draft-born feature's concrete layouts live (`#O-2`).
- **Implementation must not infer** — that a locked version is one batch of records: it is memory as of its lock commit. That a later version ever supersedes or revokes a problem, brainstorm record, review report, or event. That a decision keeps serving a reworded requirement without being re-issued.
- **Must remain unchanged** — feature-draft-v1's lifecycle, readiness conditions, approval, and lock record; `design:contract-gate-v2`; a first version's lock, where no binding version exists and every unit is new.

### 3.3 Invariants

- **DINV-1** — A reopened draft starts with exactly the binding version's PRD and tech-spec text, byte for byte, and with nothing else from that version (`prd:feature-draft/FR-20`).
- **DINV-2** — A reopened draft's lock writes a record only for a unit that is new or differs from the binding version's active record at its address; an unchanged unit writes nothing (`FR-22`).
- **DINV-3** — Every record compiled from a PRD or tech-spec row the draft no longer holds is revoked at the lock; a later version never supersedes or revokes a problem, brainstorm record, review report, or event.
- **DINV-4** — A per-feature number of a brainstorm record, review report, or problem, and the ID of any PRD or tech-spec row, is never reused across versions.
- **DINV-5** — A plan's traces resolve against memory as of the lock its Basis names, so a retirement at a later version never invalidates it.
- **DINV-6** — A reopened draft locks only when its branch's first commit carries exactly the binding version's stored PRD and tech-spec text.
- **DINV-7** — A reopened draft whose PRD and tech-spec text equal the binding version's never locks.

### 3.4 Observability

- **Events/logs** — no new events. The reopen emits feature-draft-v1's `draft_reopened` (EVT-6). A refused lock names each unmet condition in `lock_refused` (EVT-4), a reused number or ID and a changed binding version included.
- **Metrics** — none new; reopen causes stay feature-draft-v1's M2.
- **Alerts** — none; every refusal is reported where it happens, naming the item and the remedy.
- **Debugging surface** — the lock-preparation proposal's memory packet lists each new, superseding, and revoking record. Rendering the feature as of each lock tag shows each version in full.
- **Trace/correlation** — as `design:feature-draft-v1` §3.4: every event carries the feature and the draft version digest.

### 3.5 Security & privacy

none — the header row: no identity, permission, or approval surface changes. A reopened draft's records land only through the operator's approval of the exact head (`design:feature-draft-v1` #DINV-4). The revocations this design adds are part of that approved change.

### 3.6 What the next lock writes

**Decision:** Lock preparation compiles the working files as `design:feature-draft-v1` §3.6 defines. It then compares each compiled unit with the binding version's active record at the same address:
- a unit with no record at its address is written new;
- a unit whose content or related records differ supersedes the active record, which memory requires to be of the same kind (`design-brainstorm:feature-draft-tech-design-2#F-7`);
- an unchanged unit writes nothing;
- each record compiled from a PRD or tech-spec row absent from the draft is revoked, and a question row's answer is revoked with it;
- the literal PRD and tech-spec Documents are superseded when their text changed.

Units compiled from problems, brainstorm records, review reports, and events are history: a later version adds to them and never supersedes or revokes them (#DINV-3).

**Decision:** A unit is unchanged only when every field except identity and issuance time is equal and every record it relates to is itself unchanged. Memory authorizes a serves link only between active records (feature-draft-v1 S-8), so a tech-spec decision serving a reworded requirement is re-issued to serve the new record, even when its own text did not change. An answer to a reworded question is re-issued the same way. Readiness judges the result as feature-draft-v1 §3.8 defines, over the binding records with the draft's differences applied.

**Decision:** Lock preparation refuses a reopened draft unless the draft branch's first commit carries exactly the binding version's stored PRD and tech-spec text; the refusal names the version that text matches, if any (#DINV-6). The check observes the starting point from content and never trusts the branch name (AP-18). It catches a draft that started from an older version: comparing it against a newer binding version would silently undo that version's changes. It also catches a branch seeded by hand. Once it passes, the approval request's change list is every difference the lock writes, because that list is computed from the branch's history since its first commit (`design:feature-draft-v1` #DINV-11; `prd:feature-draft/UB-1`). This fails closed instead of guessing (AP-17).

**Decision:** Lock preparation refuses a reopened draft whose PRD and tech-spec text equal the binding version's, naming the remedy: abandon the draft (`design:feature-draft-v1` §3.14). There is no change to lock, and a version number marks a contract change. The binding version stays, no lock tag is written, and the abandonment keeps the reason (`design-brainstorm:feature-draft-tech-design-2#D-3`; #DINV-7).

### 3.7 Numbering and IDs across versions

**Decision:** Brainstorm records, review reports, and requirement problems continue the feature's numbering across versions (`design-brainstorm:feature-draft-tech-design-2#D-2`). Readiness refuses, as an invalid part (`prd:feature-draft/UB-7`), a working file whose number the feature already used, naming the next free number. It refuses any PRD or tech-spec row that reuses an ID an earlier version retired the same way: requirement IDs are stable and never reused (`#F-8`), and question, risk, and decision rows compile to per-feature families just as requirements do.

### 3.8 Earlier plans after a retirement

**Decision:** A plan's traces resolve against memory as of the lock its `Basis` header names (`design:feature-draft-v1` §3.11, S-10). So a requirement retired at a later version stays resolvable for plans generated from earlier versions, and their finished tasks keep tracing the version they delivered (`prd:feature-draft/FR-44`). A plan generated from the new version that traces the retired requirement is refused as an unresolvable trace (`FR-15`, `FR-37`). This closes incident I9's failure for draft-born features (`design-brainstorm:feature-draft-tech-design-2#F-10`, `#F-11`).

### 3.9 Failure modes and testing strategy

| Failure | System behavior | Detection | Recovery |
|---|---|---|---|
| A working file reuses a number, or a row reuses a retired ID | lock refused, naming the item and, for a number, the next free one | readiness (EVT-4) | renumber, or pick a new ID |
| The draft's first commit does not carry the binding version's text: it started from an older version or was seeded by hand | lock preparation refused, naming the version that text matches, if any | lock preparation (EVT-4) | reopen from the binding version and carry the edits over |
| A reopened draft's PRD and tech-spec text equal the binding version's | lock preparation refused with the abandon remedy; no tag is written | lock preparation (EVT-4) | abandon the draft, recording why |
| The binding version's stored text is unreadable | the reopen fails naming the step; nothing is written (`design:feature-draft-v1` #DINV-10) | EVT-2 | repair and re-run |
| A decision serving a reworded requirement is compared by text only | the requirement reads as unserved and the lock is refused | readiness (`FR-13`) | an implementation defect: fix the comparison (§3.6) |

Test seams: the comparison is in-process, tested with table-driven fixtures of a binding version and a draft, with golden memory packets. The seed and the lock run on real repositories in temporary directories. Plan-trace resolution is tested through plangate's fold over plans with two lock bases.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| A successor to feature-draft-v1 (`design-brainstorm:feature-draft-tech-design-2#ALT-1`) | forces a successor to `contract-gate-v2` too, whose Builds-on the planning gate would otherwise refuse, for a rule that contradicts nothing in v1 |
| The rule in the implementation plan's acceptance criteria (`#ALT-3`) | a design choice would live in a plan, which is not a design document and never enters memory |
| Re-issue every unit at each lock (`#ALT-5`) | a supersession per unchanged row, and a lock whose change hides among re-issued records |
| Lock an identical next version when nothing changed (`#ALT-7`) | a version number with no contract change, and later plans based on it (`#D-3`) |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | A decision serving a reworded requirement is compared by its own text only | the lock writes nothing for it; the requirement loses its serving link and readiness refuses the lock | "unchanged" includes every related record (§3.6) | A-3 |
| R2 | An author starts a reopened draft's numbering at 001 | a new review lands in the binding version's review family | readiness refuses a used number and names the next free one (#DINV-4) | A-4 |
| R3 | A plan tool resolves traces against the current version | a finished plan breaks when a later version retires a requirement (I9) | traces resolve as of the plan's Basis lock (#DINV-5) | A-5 |
| R4 | A draft locks from text other than the binding version's | changes relative to the binding version lock without appearing in the approval request (`prd:feature-draft/UB-1`) | lock preparation checks the first commit's text by content (#DINV-6) | A-7 |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Reopening a locked fixture feature commits the binding version's `prd.md` and `tech-spec.md` byte for byte, with empty problems and events files and no brainstorm or review files; the draft digest equals the binding version's | Test | real-repository fixture | `prd:feature-draft/FR-20` · #DINV-1 |
| A-2 | The fixture's binding version carries a problem, a brainstorm record, a review report, and an event. Locking a reopened draft that rewords one requirement, removes one, adds one, and leaves the rest writes exactly: a superseding record for the reworded row, a Revocation for each record of the removed row, a new record for the added row, and a superseding literal PRD Document. It writes nothing for unchanged rows and supersedes or revokes none of the binding version's history. The memory gate passes at the landing tree | Test | real-repository fixture with a golden memory packet | `prd:feature-draft/FR-22` · #DINV-2 · #DINV-3 |
| A-3 | A tech-spec decision whose text is unchanged but which serves a reworded requirement is re-issued to serve the new record, and readiness finds the requirement served | Test | compile fixture | `prd:feature-draft/FR-22` · #DINV-2 |
| A-4 | A reopened draft reusing a brainstorm, review, or problem number, or the retired ID of a requirement, question, risk, or decision row, is refused at readiness naming the item and, for a number, the next free one | Test | per-case fixtures | `prd:feature-draft/FR-20` · #DINV-4 |
| A-5 | After version 2 retires a requirement that a plan generated from version 1 traces, that plan still folds clean; a plan generated from version 2 that traces the retired requirement is refused | Test | two-version plan fixture | `prd:feature-draft/FR-22` · #DINV-5 |
| A-6 | Rendering the fixture feature as of each lock tag shows that version's full contract, unchanged rows included | Test | render fixture | `prd:feature-draft/FR-22` · #DINV-2 |
| A-7 | Lock preparation refuses a reopened draft whose first commit does not carry the binding version's stored text: one that started from an older version and one seeded by hand, each refused naming the version the text matches, if any | Test | older-start and hand-seeded fixtures | `prd:feature-draft/FR-20` · #DINV-6 |
| A-8 | Lock preparation on a reopened draft whose PRD and tech-spec text equal the binding version's is refused with the abandon remedy; the binding version stays and no lock tag is written | Test | no-change fixture | `prd:feature-draft/FR-22` · #DINV-7 |

## 7. Open questions

none — every mechanic follows from the record's rulings (`design-brainstorm:feature-draft-tech-design-2#D-1`, `#D-2`, `#D-3`) and the facts it cites.

## Principles

- **AP-7 — honored.** `design:feature-draft-v1` stays frozen; this design narrows it through Builds-on and makes none of its binding statements false.
- **AP-10 — honored.** Number and ID reuse, a changed binding version, and history retirement are refused by code, not left to authors (§3.6, §3.7).
- **AP-11 — honored.** The comparison reuses feature-draft-v1's one compiler and memory's existing supersession and revocation; there is no second compiler.
- **AP-17 — honored.** A draft that did not start from the binding text, or a reused number, refuses the lock instead of guessing (#DINV-4, #DINV-6).
- **AP-18 — honored.** A reopened draft's starting point is observed from its first commit's content, never read from its branch name (#DINV-6).
- **AP-21 — honored.** Downstream readers read exactly what was judged: memory as of the lock (§3.6).
- **AP-22 — honored.** Recompiling an unchanged reopened draft writes nothing new (#DINV-2).
- **AP-28 — honored.** Problems, brainstorm records, review reports, and events stay as history and are never retired by a later version (#DINV-3).
- **RW-5 — honored in part; see the deviation.**
- **Deviation:** RW-5 is honored only in part. The reopen rule narrows feature-draft-v1's lifecycle from a separate document, because superseding v1 would force superseding `design:contract-gate-v2` as well (`design-brainstorm:feature-draft-tech-design-2#D-1`, `#F-2`).
