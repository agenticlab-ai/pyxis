# Review: implement-now amendment 005 — review report

| Field | Value |
|---|---|
| **In one line** | Approve — authenticated review now has coherent setup, routing, exact-revision handoff, producer proof, isolation, recovery, and live release evidence. |
| **Relevant-when** | auditing implement-now amendment 005; asking how PR #1034's forged-artifact, auth, workspace, head-drift, and recovery defects changed the contract |
| **Subject** | `prd:implement-now/005` |
| **Moment** | amendment |
| **Reviewer** | prd-review · 2026-07-10 guide |
| **Date(s)** | 2026-08-06 (rounds 1–2) |
| **Verdict** | approve |
| **Tracking** | PR #1034 |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — Live scope and recovery rows contradicted the replacement review contract

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: the first fold left §4.2 declaring review-role resolution out, §4.3 naming only `artifact_unverified`/`infrastructure`, UB-19 permitting proposal-head re-derivation and citing FR-33..FR-39 plus EVT-13/EVT-14, and DOD-23 citing the retired EVT-14 — vs — FR-40..FR-43 and EVT-15/EVT-16 require exact-green capture, authenticated attribution, three failure classes, and no re-derivation.
- **Acceptance condition**: replace the executive solution/scope/flow/scenario spans, supersede UB-19 and DOD-23 with live three-class exact-obligation rows, and update every release/readiness join to the replacements.
- **Disposition**: applied — §1 and §4.1..§4.4 now describe the complete seam; UB-21 and DOD-32 replace the stale rows; all gates cite the live IDs.

### FND-2 — Auth preflight's trigger made its own fit criterion impossible

- **Severity**: blocking (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-34 triggered only "when a pending review obligation is eligible" but required reviewer auth "before avoidable implementation spend," while FR-42 unconditionally retained "the obligation" even for a factory/auth failure before any task could create one — vs — a critical Must fit criterion must identify one observable trigger and terminal for absent config, setup failure, and per-obligation routing.
- **Acceptance condition**: separate optional config/factory/auth admission from obligation routing; make absent review uninstalled, setup failure run-level/not-created/exit-1 with no task failure, and route each admitted obligation through its assurance-band catalog.
- **Disposition**: applied — FR-44 owns absent/config/factory/auth admission and FR-45 owns band-aware route fidelity; FR-42 now distinguishes `not_created` setup failure from pending-attempt failure.

### FND-3 — The attribution requirement could silently rewrite a frozen sibling artifact contract

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: FR-41 required an attempt proof in the accepted proposal artifact, but the first dependency table treated impl-review amendment 003 and its artifact schema as already sufficient and named no compatibility decision — vs — AP-6 makes cross-boundary data contractual and RW-5 forbids implementation from inventing a schema change that its owning design has not made.
- **Acceptance condition**: make the superseding design prove that attribution composes with the frozen impl-review artifact contract, or require a sibling impl-review amendment/design before code; expose that decision at the design gate.
- **Disposition**: applied — §6.2 and Q-7 now make artifact-contract compatibility an explicit dependency and fail closed to a sibling amendment if no conformant wrapper exists.

## Round 2

No new findings. Scope-locked re-review confirmed that the live projection contains only UB-21/DOD-32 and EVT-15/EVT-16 for review recovery, FR-44's absent/empty/setup matrix is disjoint from FR-45's obligation routing, inherited setup names and authenticates every distinct effective backend before dispatch, FR-42 has a testable run-level `not_created` terminal, and §6.2/Q-7 prevent silent mutation of impl-review's artifact contract. The blocking DoD and §8 joins include FR-44/FR-45 and DOD-32.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-06 |
| 2 | approve | 2026-08-06 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 3 blocking · 0 advisory
- **Dispositions**: 3 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- 2026-08-06 · operator `@mdarabi` (`review-thread:pyxis#1054`) — Removed code-host-author/principal separation from the completion predicate. FR-41/FR-42, UB-20, ABUSE-6, DOD-28, and their folded spans now require the exact broker receipt plus fresh review-attempt proof and accept the same or a different code-host author identically. A scope-locked check found no new finding: replay, prewriting, and receipt substitution remain fail-closed, while a distinct code-host reviewer principal is explicitly deferred to a later amendment. Verdict remains `approve`.
- 2026-08-06 · operator review (`review-thread:pyxis#1054`) — Closed the publication-ownership and trust-anchor gaps surfaced after approval. FR-41 now defines the review artifact broker as impl-review's host-side persistence adapter executing frozen `prd:impl-review/FR-6`; the reviewer child returns one completed report, implement-now supplies attempt binding and consumes the native receipt, and neither becomes a second publisher. §5.3..§5.5, ABUSE-6, DOD-28/DOD-29, §6.2, and Q-7 protect canonical obligation/attempt/receipt state from both child roles and make any future publisher transfer require a sibling amendment. WR-4 records the deliberate distinct-principal deferral in the live waiting room. Scope-locked comparison against impl-review FR-6 found the correction preserves rather than changes its artifact contract; verdict remains `approve`.
- 2026-08-19 · operator review (`review-thread:pyxis#1054`) — Closed the remaining sibling-capability assumption. FR-41 and DOD-28 now block implementation and activation unless a sibling impl-review amendment and design explicitly authorize the host persistence adapter to perform FR-6 for the reviewer program and expose exactly-one publication returning a native receipt. §5.5, §6.2, Q-7, and §0 treat missing or ambiguous performer authority, receipt capability, or exactly-once semantics—not only an ownership transfer—as a dependency failure that design cannot absorb. Scope-locked comparison confirms the current sibling contract lacks these explicit capabilities, so the dependency is honestly marked blocked while the amendment's verdict remains `approve`.

## Post-verdict scope-change addendum — 2026-08-06

The approved subject was subsequently narrowed to compose with the stacked self-drive recovery contract. This addendum is append-only: it does not alter the two recorded rounds or their verdict history.

### FND-A1 — Provider-wide recovery contradicted the consumer's bounded selected frontier

- **Severity**: blocking (CAL-1)
- **Lens**: coherence + upstream-alignment
- **Evidence**: provider FR-38 reconciled every in-scope pending obligation, while the consumer contract required one invocation per round over only obligations whose durable recovery count remained below bound. FR-42/FR-44 also treated every preflight failure as run-level `not_created`, which is false when selected persisted obligations already exist.
- **Acceptance condition**: give recovery requests a closed non-empty selected obligation set plus round identity; exact-read and attempt only selected members; distinguish new-work not-created setup failure from selected-pending setup failure; repair dependent empty/non-regression/DoD/event joins.
- **Disposition**: applied — FR-46 replaces provider-wide recovery; FR-47 and NR-10 close empty/selected semantics; FR-42/FR-44 distinguish both setup cases; EVT-15 carries optional `drive_round_id`; DOD-33 replaces the aggregate NR proof; DOD-31/DOD-32 and all ship/readiness ranges cite only live rows.

### Addendum verification

Scope-locked verification found no remaining live citation to FR-38, FR-39, NR-9, or DOD-26 outside the generated superseded ledger. Invalid empty/duplicate/unknown/verified/out-of-scope selections fail before launch; selected persisted setup failure remains obligation-scoped/pending; unselected or exhausted obligations are not read or attempted; consumer retry accounting and exhaustion remain outside the provider.

**Addendum verdict:** approve the post-verdict selected-frontier scope change.
