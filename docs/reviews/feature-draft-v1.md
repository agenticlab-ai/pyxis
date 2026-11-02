# Review: feature-draft-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve with one advisory declined after round 2. Round 1 requested changes: DINV-1 contradicted abandonment, no path back after lock preparation, native records and plans conflicting with today's reconciler, an overclaimed staleness refusal, superseded rulings cited, a hybridized alternative omitted, principles omitted, and an event mechanism with no ruling. All nine blocking findings were applied. |
| **Relevant-when** | auditing or consuming the review of feature-draft-v1; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:feature-draft-v1 · shape: present · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · `.pyxis/authoring/tech-design-review.md` as of 2026-09-23, run in the authoring session |
| **Date(s)** | 2026-09-23 (round 1) · 2026-09-23 (round 2) |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(the feature-draft introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — DINV-1 contradicts abandonment

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: "the lock-preparation commit is the only change that adds a draft's records" (#DINV-1) — vs — design: "Abandoning an open draft reduces its pull request to one memory Learning stating why, which the operator approves and merges" (§3.14), a draft record landing without a lock
- **Acceptance condition**: DINV-1 names the abandonment record as its one exception, or §3.14 stops landing a record
- **Disposition**: applied — DINV-1 now excepts the single reason record of an abandoned draft

### FND-2 — No path back after lock preparation

- **Severity**: blocking (CAL-4 — load-bearing for `prd:feature-draft/FR-2` and A-17)
- **Lens**: feasibility-completeness
- **Evidence**: design: "compiles its working files once, writes the records ..., and removes the working files" (§3.7) and the state diagram's "Prepared --> Drafting: readiness refused / further round" (§3.1) — vs — `prd:feature-draft/FR-2` "accept revisions to any of its parts in any round"; nothing says how removed working files return for that round, or what version a review bound before preparation matches after it (`FR-16`)
- **Acceptance condition**: the design states how a round after preparation restores the working files, and how the draft digest stays comparable across preparation
- **Disposition**: applied — §3.7 adds a Decision: a later round reverts the preparation commit, restoring the files byte for byte, and the draft digest is recomputed from the stored exact texts (shape S-1 records both)

### FND-3 — Stored exact texts would read as orphaned source captures

- **Severity**: blocking (CAL-4 — load-bearing for A-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: "keeps each file's exact text as a literal Document" (§3.6); shape: "each file's exact text is also kept as a literal Document" (S-3) — vs — the reconciler's source-capture model (`cmd/memory/README.md`, The flow, step 6), under which a captured source whose path is absent from the landing tree is a finding unless sealed; lock preparation removes every working file (§3.7)
- **Acceptance condition**: compiled records, the exact texts included, are native memory carrying no source-capture or PRD-ledger provenance, stated in the design and the shape
- **Disposition**: applied — §3.6 states compiled records are native and carry none of the reconciler's source-capture or ledger provenance; shape S-3 names the excluded tags

### FND-4 — Draft-born plans would still be captured into memory

- **Severity**: blocking (CAL-4 — load-bearing for A-17 and #DINV-8)
- **Lens**: feasibility-completeness
- **Evidence**: design: "The plan is never stored in memory" (#DINV-8) and "the plan stays out of memory" (§3.11) — vs — the reconciler's source discovery, which captures every plan document under the plans root as an authored source (`design-brainstorm:feature-draft-tech-design#ALT-10` trade-offs: "the reconciler must stop capturing draft-born plans"); neither the design nor the shape changes it
- **Acceptance condition**: the design and shape exclude a draft-born feature's plan documents from source discovery
- **Disposition**: applied — §3.11 states that source discovery skips the plan documents of a feature with no PRD ledger; shape S-10 records the rule

### FND-5 — A-23 claims a staleness refusal the mechanism does not provide

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: "a lock refused for staleness names the mainline change" (A-23) and "at the lock request, against the mainline current at that moment" (§3.8) — vs — design: §7 Q1 and R2, which defer refusing a lock when the mainline moves after readiness ran; the only re-run is lock preparation's rebase
- **Acceptance condition**: §3.8 and A-23 name lock preparation as where readiness re-runs against the current mainline, and route a mainline moving after preparation to Q1
- **Disposition**: applied — §3.8, A-23, and R2 now say exactly that, with UB-8 routed to Q1 beside FR-24

### FND-6 — Superseded rulings cited as the basis

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §2 cites `#D-6`, §3.8 cites `#D-6`, and §3.11 cites `#D-4` — vs — the record: "superseded by D-7 — the reverse association check is withdrawn" (D-6) and "superseded by D-12 — the plan leaves the draft and follows the lock" (D-4)
- **Acceptance condition**: the design cites only live rulings: D-7 for readiness, D-12 for the plan
- **Disposition**: applied — §2 and §3.8 cite D-7, and §3.11 cites D-12

### FND-7 — A hybridized alternative missing from §4

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §4 carries `#ALT-21` and `#ALT-22` but no `#ALT-20` — vs — the record: "hybridized into ALT-23 — the same observed-lineage idea, judged per requirement instead of per feature" (ALT-20)
- **Acceptance condition**: §4 carries ALT-20 with its hybridized verdict
- **Disposition**: applied — §4 adds the per-feature observed-lineage row, hybridized into ALT-23

### FND-8 — Relevant principles silently omitted

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: Principles cite neither AP-5, AP-14, AP-20, AP-25, AP-27, nor AP-30; shape: "drafts/<feature>/prd.md" and "branch draft/<feature>" hardcode the root and branch prefix (S-1) — vs — AP-30 ("Every operand-owned value is configured, never hardcoded"), and the design's own events (§3.4), real-repository acceptance (§3.15), branch isolation (#DINV-1), reworked skills (§3.13), and self-registered check and adapter (§3.9, §3.12)
- **Acceptance condition**: Principles cite each relevant principle with how it is honored, and the shape configures the drafts root and branch prefix
- **Disposition**: applied — Principles add AP-5, AP-14, AP-20, AP-25, AP-27, and AP-30; shape S-1 configures the root and the branch prefix

### FND-9 — An event-retention mechanism no ruling decides

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: "each draft step appends to the draft's event log, a working file compiled into memory at lock preparation" (§3.4); shape: "events.md ... Learning tagged event:<name>" (S-1, S-3) — vs — the record, whose rulings and delegations (D-14) cover state reporting, the lock record, and concurrency but not where events live before a lock; the authoring guide forbids inventing a design decision
- **Acceptance condition**: event retention before the lock is parked as a §7 question with an owner, the working-file log kept only as its proposal
- **Disposition**: applied — §3.4 routes it to the new §7 Q4 (owner: implementation); the shape labels the log as Q4's proposal; §0 lists Q4

### FND-10 — FR-44 has no mechanism

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: "a finished task keeps tracing the version it delivered" (§3.1) — vs — `prd:feature-draft/FR-44`; neither the design nor the shape says how a task knows the locked version
- **Acceptance condition**: the plan names the locked version it was generated from
- **Disposition**: applied — §3.11 states that a plan names its lock; shape S-10 adds the plan header's Basis field

### FND-11 — Independently revisable choices in one document

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design: "These choices co-revise as one lifecycle" (§2) — vs — the approval mechanism behind its port (§3.9), the native-link rule (§3.10), and the document gate's closure (§3.12), each of which could change alone
- **Acceptance condition**: split the independently revisable choices into their own designs, or record why they stay
- **Disposition**: declined — the operator directed one tech spec for the draft (`design-brainstorm:feature-draft-tech-design#D-1`, `#D-12`). The approval port already isolates the medium, and the native-link rule and gate closure are the lifecycle's enforcement edges. The one independently revisable scope, the contract gate's planning entry, was split into `contract-gate-v2` (`#D-18`).

## Round 2

Scope-locked to the spans changed for FND-1 to FND-10.

- FND-1 — closed: DINV-1 names the abandonment exception, and §3.14 agrees.
- FND-2 — closed: §3.7's Decision gives the revert path and the digest recomputation, and shape S-1 matches.
- FND-3 — closed: §3.6 and shape S-3 state native provenance with the capture and ledger tags excluded.
- FND-4 — closed: §3.11 and shape S-10 exclude a draft-born feature's plans from source discovery, consistent with #DINV-8.
- FND-5 — closed: §3.8, A-23, and R2 agree on lock preparation and route the moving mainline to Q1.
- FND-6 — closed: §2, §3.8, and §3.11 cite live rulings only.
- FND-7 — closed: §4 carries ALT-20 with its verdict.
- FND-8 — closed: Principles cite the six principles; shape S-1 configures the root and prefix.
- FND-9 — closed: §7 Q4 owns event retention; §3.4, §0, and the shape defer to it.
- FND-10 — closed: §3.11 and shape S-10 carry the plan's Basis.

No new finding on the changed spans.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-09-23 |
| 2 | approve-with-advisories | 2026-09-23 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 9 blocking · 2 advisory
- **Dispositions**: 10 applied · 1 declined · 0 withheld-as-violation

## Overlay corrections

- 2026-09-23 · operator (plan-new dry run, after approval): the planning precheck found the design building on the superseded `plan-new-v1`; Builds-on now names `plan-new-routed-demand`, and §3.2 follows.
- 2026-09-23 · operator (plan-new dry run): DINV-11, §3.5, §3.6, §3.9, and A-10 now surface unruled requirement changes in the approval request instead of refusing them (`design-brainstorm:feature-draft-tech-design#D-19`).
- 2026-09-23 · operator (plan-new dry run): §3.2, §3.9, and A-5 scope the required code-owner review to the contract records, so the merge lane keeps its policy (`design-brainstorm:feature-draft-tech-design#D-20`).
- 2026-09-23 · operator (plan-new dry run): the solution shape gains the working-file grammar, the approval request's home, the lock-record writer, the scoped code owners, and a digest limited to the contract files.
