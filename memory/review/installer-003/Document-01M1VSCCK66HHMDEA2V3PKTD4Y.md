# Review: prd:installer/003 — review report

| Field | Value |
|---|---|
| **In one line** | Approve at round 2 — round 1 bounced the amendment for obligating a merged sibling contract it never named, for leaving §5.3 promising a credential-free offline path, and for a readiness row still citing the requirement this amendment superseded; all three, plus both advisories, were applied and the re-fold verifies clean. |
| **Relevant-when** | auditing or consuming the review of installer amendment 003 (the distribution door as default channel); asking why it was bounced; tuning prd-review calibration against real findings |
| **Subject** | `prd:installer/003` |
| **Moment** | amendment |
| **Reviewer** | prd-review · `.pyxis/authoring/prd-review.md` (run inline by the authoring session, bootstrap-era) |
| **Date(s)** | 2026-08-19 |
| **Verdict** | approve |
| **Tracking** | — *(the amendment's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — FR-31 obligates a merged sibling contract that the amendment never names

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: subject FR-31 — "Where an installed repository's hosted CI is materialized, the workflow shall carry the release pin and a reference to the door credential, so the bootstrap resolves the engine with no hand-edit of the materialized file" — vs — `prd:config-driven-portability` FR-4, which scopes the rendering of that same file to "the installed CI workflow's branch, remote, and path references from config", and its UB-3, which has converge regenerate or `doctor` name any drifted generated surface. The workflow is a generated-class file owned by that sibling; adding pin and credential content to it changes what that sibling's Must row says gets rendered, and a hand-added credential line is drift by that sibling's own rule. The amendment's `Blast radius` names only installer rows and sections, and §7.2 carries no new cross-contract obligation.
- **Acceptance condition**: either record the obligation as a carried cross-contract open question in §7.2 with a named owner — the pattern §8 already uses for Q-1 — so the sibling amendment is visible and traceable, or rescope FR-31 so the pin and credential reach the bootstrap without changing the generated workflow's rendered content.
- **Disposition**: applied — `op: add Q` records Q-5, naming `prd:config-driven-portability` FR-4/UB-3, the grammar extension, the owner, and the needed-by; §8's open-questions row now names it alongside Q-1.

### FND-2 — §5.3 still tells the reader that the offline path needs no credential

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: standing §5.3 access paragraph — "GitHub operations use their ambient credential for the declared operation set only; the offline path needs no credential at all" — vs — subject FR-28 ("Where no artifact-source override is configured, the system shall resolve every artifact from the distribution door") and FR-29 (a missing or rejected door credential refuses with zero execution). After this amendment the default channel is credentialed for every install shape, the local-only path included: FR-13's offline floor is "zero network after fetch", and the fetch it depends on now cannot happen without the credential. A reader ruling from §5.3 would conclude a local-only install needs nothing, which the same document now contradicts.
- **Acceptance condition**: supersede §5.3's access paragraph (or the section) so the door credential is stated as a prerequisite of every path including the offline one, reconciled with FR-13's offline-floor claim.
- **Disposition**: applied — `op: supersede §5.3` restates the access paragraph: the door credential is required on every install shape including the local-only one, whose zero-network floor begins after that fetch; the GitHub-credential claim is narrowed to GitHub operations.

### FND-3 — §8's readiness row and DOD-1's source range still point at the superseded FR-22, and no §8 row covers the new rows

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: folded §8 — "Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ☑ | … F4 → FR-14, FR-22" — and DOD-1's source "FR-1..FR-22" — vs — the same projection's superseded ledger, "**FR-22** *(superseded by FR-27 (003))*". The executability gate's coverage claim and the blocking DoD row both anchor on an ID this amendment retired, while FR-27..FR-31 and UB-13 — five Must rows and one new failure mode — appear in no §8 coverage mapping at all. The ☑ marks therefore assert readiness the contract no longer demonstrates.
- **Acceptance condition**: supersede the affected §8 rows (flow coverage; blocking-DoD traceability) and DOD-1's source so they name FR-27..FR-31 and UB-13 and drop the retired anchor — or record, per row, why the standing rows already cover them.
- **Disposition**: applied — `op: supersede DOD-1` → DOD-9 sources "every live Must FR (FR-1..FR-31)"; `op: supersede §8` re-maps F4 → FR-14, FR-27 and places FR-28..FR-31 and UB-13 in F1/F2 coverage.

### FND-4 — the machine-recorded channel and the repo-configured override have no stated precedence

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: subject FR-30 — "`init` and `upgrade` shall resolve that channel flag → environment → recorded record" — vs — subject FR-27, whose effective source is "the distribution door by default, or a configured override". Where the machine's recorded channel and an initialized repository's configured override disagree, neither row says which governs, and FR-30's fit criterion exercises only flag-over-environment-over-record. A tester handed a door-recording machine and a GitHub-override repository cannot derive the expected result from the contract.
- **Acceptance condition**: state the precedence in FR-27 or FR-30 — for instance, the repo configuration governs inside an initialized repository and the machine record governs only where no configuration exists — with a fit criterion that exercises the disagreement.
- **Disposition**: applied — FR-30 now states that a configured artifact source governs over the recorded channel inside an initialized repository, with a fit criterion exercising the disagreement.

### FND-5 — the trade the amendment is made for is recorded nowhere as a cost

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: subject §6.2 adds "the door credential on consumer machines and in operand CI | user prerequisite | every default-path fetch | user-provided" — vs — standing §6.3 (risks, assumptions, rabbit holes), §3.4 (non-goals), and the waiting room, none of which record what moving the default channel gives up: access control passes from per-identity grants, which are revoked one person at a time, to a credential whose withdrawal from one consumer withdraws it from all of them. `Learned-from` names seat cost as the motivation, so the countervailing cost belongs in the contract rather than only in the session log.
- **Acceptance condition**: add a §6.3 risk row (or a waiting-room entry) naming the loss of per-consumer revocation and the rotation path it implies, with an owner.
- **Disposition**: applied — `op: supersede §6.3` adds R7, naming the loss of per-consumer revocation and the rotation reach, owner product.

### Examined and cleared, for the record

- **Blast-radius honesty** — declared `contract-changing` equals computed (`docgate diff`: contract-changing; affected §5.5 · §6.2 · §0 · FR-22 · FR-28 · UB-13). No understatement.
- **Learned-from resolution** — `operator-decision:logs/2026-08-19-1523-pyxis-init-404-diagnosis.md#P-1` is within the closed class set and resolves to a committed entry recording the decision, its date, and the alternative rejected.
- **Chain integrity** — the fold marks FR-22 → FR-27 with both entries retained; FR-28..FR-31 and UB-13 are fresh in the global namespace (prior maxima FR-26, UB-12); the §0, §5.5, and §6.2 supersessions are wholesale and byte-faithful except the intended changes.
- **Sibling scope, `prd:channel-inbox`** — its bulletin source is contracted host-agnostically ("signed, closed-schema file over TLS", §5.5) with its own integrity and authenticity rule (FR-9), so the artifact-source default does not reach it. No conflict.
- **Sibling scope, `prd:init-hardening`** — FR-29's refusal shape (name the variable and the host, zero execution) composes with the artifact-source preflight check rather than re-contracting it.
- **Upstream fidelity** — the trust root is untouched: FR-27 keeps manifest-hash verification on every byte whatever the channel, so the mirror-serves-bytes-never-chooses-them property that made an overridable source safe carries the default flip unchanged.
- **Incident registry** — no registry lesson is repeated; the change moves a host, not a verification boundary.

## Round 2

Scope-locked to the five round-1 findings and regressions within the spans they changed. The revised subject re-validates, re-folds, and re-verifies clean; computed blast radius remains `contract-changing`, matching the declaration, and the declared `touches` grew to name every newly touched row and section (projection `sha256:7486c701… → 95cd3198…`).

- **FND-1 → closed.** Q-5 exists with the sibling contract, the grammar constraint, an owner, and a needed-by; §8's open-questions note carries it. The obligation is now visible to anyone reading the contract, which is what the finding asked for — it does not, and was not asked to, resolve the collision.
- **FND-2 → closed.** §5.3's access paragraph no longer claims a credential-free offline path; it separates the door credential (every shape) from the GitHub credential (GitHub operations only) and anchors both to FR-28/FR-29 and FR-13.
- **FND-3 → closed.** DOD-9 sources the live Must set rather than a fixed range, so amendment-added rows are inside it by construction; §8's flow row maps F4 onto FR-27 and places the new rows and UB-13 in F1/F2.
- **FND-4 → closed.** FR-30 states the precedence and its fit criterion exercises the disagreement case that was previously underivable.
- **FND-5 → closed.** §6.3 R7 records the revocation and rotation cost with an owner.

**Regressions examined within the changed spans:** the §5.3 supersede carries ABUSE-1..4 forward with one deliberate change — ABUSE-1's mitigation cell now cites FR-27's effective-source reporting instead of resting on "the override is explicit config", a claim the default flip would otherwise have hollowed out; the row stays true post-flip. §6.3 carries R1..R6 verbatim. §8 carries every check row, changing only the three notes the findings named plus J1's mapping gaining FR-30. DOD-9 and Q-5 are fresh in their namespaces (prior maxima DOD-8, Q-4). No round-2 finding is raised against unchanged content.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-08-19 |
| 2 | approve | 2026-08-19 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 3 blocking · 2 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

Human overlay on PR #1090, @mdarabi, 2026-08-19 — five corrections the unmanned rounds did not catch. Rounds 1 and 2 and the verdict history are untouched; the subject was still an unmerged draft, so each correction was applied to amendment 003 directly rather than opening a 004.

- **P1 — UB-13 fell outside the blocking verification ranges.** NFR-4's fit criterion and DOD-3's source both enumerated `UB-1..UB-10`, so a release could pass without ever exercising an absent, malformed, or rejected credential — and UB-11 and UB-12, added by earlier amendments, were outside them too. Round 1's FND-3 caught the identical defect in DOD-1 and stopped at the FR side; the UB side went unexamined. *Applied:* `supersede NFR-4` → NFR-7 and `supersede DOD-3` → DOD-10, both sourcing the live set rather than a fixed range.
- **P1 — the machine record contradicted the retained storage contract.** FR-30 requires storing the channel beside the pin outside the repository, while §5.3 still ended "nothing is stored beyond the repo's own files" and §5.4 claimed every write is in the target repo and no sensitive data is read. *Applied:* §5.3's closing sentence now names the one outside write and the credential's never-persisted handling; `supersede §5.4` adds a data row each for the machine release record and the door credential (privacy class secret), and restates rollback, the privacy check, and egress.
- **P1 — Q-5 was marked as blocking nothing while blocking a Must row.** FR-31 is Must and DOD-9 blocks release on every live Must FR, so an unsatisfiable FR-31 is solution-blocking by construction; §7.2's closing prose and §0 also still called Q-1 the only cross-contract obligation. *Applied:* `supersede §7.2` states Q-5 as solution-blocking for FR-31 and names both obligations; §8's readiness row moves to ☐ with the condition that clears it; §0 records two obligations and which one blocks.
- **P2 — FR-30 defined only a partial precedence.** Flag → environment → record and configuration → record were stated; configuration versus flag or environment was not, leaving a conflict with no derivable result. *Applied:* FR-30 now states one total order — flag, environment, repository configuration, record — with the environment above configuration for parity with the shims' existing redirect behavior, and a fit criterion exercising every conflicting pair.
- **P2 — the session log contradicted itself.** "Nothing escalated this session" stood immediately above a human-decided escalation. *Applied:* the placeholder line is removed; P-1 stands as the record.

Human overlay, second pass on PR #1090, @mdarabi, 2026-08-19 — four corrections on the first overlay's own work. The entries above are left exactly as written; this block records what they got wrong.

- **P1 — Q-5 vanished from the source map.** Replacing the typed `add Q` with a whole-section supersession rendered the row in `effective.md` but registered no unit, so `effective.source.json` carried only Q-1..Q-4 and no citation or invalidation could resolve Q-5. *Applied:* the section supersession keeps the prose and Q-1..Q-4; a typed `op: add Q` follows it for Q-5. Verified — the row renders once and the unit is in the source map.
- **P1 — the amendment was Approved while its own executability gate failed.** Marking §8's solution-blocking row ☐ recorded the truth but produced an approved contract that cannot pass its gate, and a frozen projection cannot be cleared by a sibling amendment landing later — installer would have needed yet another amendment to answer Q-5. *Applied:* the operator chose deferral. The hosted-CI credential carriage (the row previously numbered FR-31) is removed from this amendment; Q-5 becomes a carried obligation blocking nothing, §8's row returns to ☑, and DOD-9's live Must set ends at FR-30. A later installer amendment contracts the carriage once `prd:config-driven-portability` is amended.
- **P2 — the PR description's fold evidence was stale.** It showed the first commit's fingerprint and affected set. *Applied:* the description now carries the fold for the head commit.
- **P2 — the log correction itself rewrote an append-only record.** Deleting the "nothing escalated" line was the same class of defect the first overlay was fixing. *Applied:* the line is restored where it was written, with a dated correction appended beneath it saying it is wrong and why it stays. The same discipline is why the first overlay block above was reverted to its original wording rather than edited.

Overlay, third pass — cross-check against in-flight tooling, 2026-08-19. The blocks above stand as written.

- **R7 had the defect the second overlay had just fixed elsewhere.** Introducing the risk row only inside the §6.3 restatement gave it no source-map lineage — the identical defect the second overlay caught on Q-5, missed on the row added in the same pass. It surfaced by running an unmerged `projection-congruence` check another session is currently building in `internal/docgate/fold`, against this projection: `projected unit R7 has no source-map lineage`. `origin/main`'s docgate does not yet carry that check, so this PR's CI would have passed and the defect would have surfaced only once the check landed. *Applied:* `op: add R` introduces R7 as a typed unit and the §6.3 restatement carries R1..R6; both docgate builds — merged and in-flight — now verify clean. The general lesson for this ledger: a new ID introduced by a section restatement needs a typed op beside it, whatever the kind.

Overlay, fourth pass, @mdarabi, 2026-08-19. The blocks above stand as written, including round 2's sentence approving the ABUSE-1 mutation — this block records that it was wrong.

- **P1 — a same-ID mutation carries no lineage either.** The §5.3 restatement changed ABUSE-1's mitigation cell while keeping its ID, so `effective.source.json` still attributed the changed row to `000-base.md`: neither the affected-unit set nor downstream invalidation saw the amendment. Round 2 examined that change and explicitly cleared it as a deliberate in-span edit, which was the miss — the row was mutated, not restated. Same regression class as Q-5 and R7, with a changed ID rather than a missing one. *Applied:* `op: supersede ABUSE-1` introduces **ABUSE-5** carrying the new mitigation, and §5.3 restates that resulting row.
- **Consequential re-anchoring, same pass.** Superseding ABUSE-1 left two live rows naming it in executable cells: NFR-3's fit criterion ("the UB-3/ABUSE-1 drill set") and DOD-5's source, which enumerates the drills that discharge it. Both are now `op: supersede` → **NFR-8** and **DOD-11**, the latter also taking UB-13 into the supply-chain evidence set so the credential refusal has a blocking home. DOD-11 in turn left ABUSE-4's verify cell naming DOD-5, so `op: supersede ABUSE-4` → **ABUSE-6** closes it. A sweep of the whole live projection confirms no executable reference introduced by this amendment points at a superseded row.
- **Where the re-anchoring deliberately stops.** The ledger's standing references from live rows to rows superseded by amendments 001 and 002 — NR-3 and UB-6 to FR-3, UB-9 and ABUSE-3 to FR-19, §5.4's migration bullet to UB-1, §6.5's release gate to DOD-1..6 — are left alone. `review:installer-001` ruled that shape valid provenance under supersession mechanics, and those rows belong to amendments this one has no business rewriting. The line drawn here: **003 re-anchors exactly the executable cells its own supersessions broke, and nothing else.**
- **A verbatim check accompanied the fix.** Every other row this amendment restates inside a section supersession was diffed against the pre-003 projection: ABUSE-2, ABUSE-3, R1..R6, and Q-1..Q-4 are byte-identical. ABUSE-1 was the only mutation.

Overlay, fifth pass, @mdarabi, 2026-08-19. The blocks above stand as written, including the fourth's claim that no executable reference introduced by this amendment still pointed at a superseded row — release gates were outside the sweep it ran, and this block records that miss.

- **P1 — the release gates omitted the replacement DoD rows.** DOD-8's blocking cell named the release condition as "DOD-1..7" and §6.5 advanced the release candidate on "DOD-1..6 green". Both ranges point at DOD-1, DOD-3, and DOD-5 — all superseded here — and omit DOD-9, DOD-10, and DOD-11, so a release could have shipped without ever proving the door-credential refusal that this amendment makes a Must. *Applied:* `op: supersede DOD-8` → **DOD-12** states the release ships on *every live blocking DoD row*; `op: supersede §6.5` advances the release candidate on the same live set and names DOD-12 at the dogfood stage; `op: supersede §6.1` re-anchors the dogfood phase's exit criterion. Neither section supersession introduces a unit, so neither repeats the lineage defect. A live set replaces a numeric range in both places, so no future supersession can drop a blocking row out of a gate again.
- **Where the fourth pass's sweep was wrong.** It swept requirement-shaped cells — fit criteria, verify cells, DoD sources — and concluded the amendment left nothing dangling. It never looked at §6.1 and §6.5, where the phase and rollout tables carry the conditions that actually decide whether something ships. The rule the two passes together arrive at: **a cell that decides — a gate, an exit criterion, a release condition — must name live rows or a live set, wherever in the document it lives.**
- **One reference deliberately left.** §3.2's M3 row still reads "measurement plan: the DOD-8 demo record". It is descriptive, not deciding: no gate consults it, and the row it names sits in the superseded ledger with identical criterion text. Re-anchoring it would mean superseding M3 into a fresh metric ID, which cascades into §6.1's scope column, §6.5's evidence column, DOD-12's own source, and §8 — disproportionate to a note. Flagged rather than silently kept; the operator's call if total ID hygiene is wanted.

