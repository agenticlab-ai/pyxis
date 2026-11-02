# Review: prd:existing-repo-onboarding/000 — review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories (round 2) — two blocking coherence gaps (undo's undefined failure space; Must UB rows ungated by DoD) and seven advisories, all applied in one revision: undo semantics defined (FR-9/UB-6, operator ruling D-11), Must-UB folded into DOD-3, epoch-commit disclosure and config-surface failures specified, NFR-1 fit realigned. |
| **Relevant-when** | auditing or consuming the review of prd:existing-repo-onboarding; asking why the 000-base bounced; tuning prd-review calibration against real findings |
| **Subject** | prd:existing-repo-onboarding/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · .pyxis/authoring/prd-review.md (2026-07-23) |
| **Date(s)** | 2026-07-23 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — |
| **Template version** | review-report/v1 |

## Round 1

Four blind lenses generated over the shared context pack; candidates survived independent cross-scrutiny before landing. The docgate floor (citation resolution, immutability, ledger structure, fold) and §0 (read-to-rule, human-only) are excluded by protocol. One upstream-alignment candidate was killed on scrutiny (an incident-I3 CLI-misparse-refusal row: the PRD defers all CLI-verb mechanics to the design and does not contradict I3, and the registry routes incidents to the design stage — a design-review concern, not a PRD-level conflict). Basis fidelity is clean: all live D-1..D-10 carry, O-1..O-6 land in §3.4, P-1..P-5 pass to design via §7.2.

### FND-1 — Undo's failure space is undefined, and §8's flow-coverage ☑ falsely claims F3 is covered

- **Severity**: blocking (CAL-1)
- **Lens**: coherence + completeness-critic (independent convergence)
- **Evidence**: §8 row 7 — *"Every key flow (§4.3) has functional, failure, and non-regression coverage (§5.1–5.2) | ☑ | F1/F2/F3 → FR + UB-1..5 + NR-1..3"* — is not satisfied for F3: **no §5.2 UB row traces to F3** (UB-1→F1, UB-2→F2, UB-3→F1, UB-4→F1, UB-5→F1), and FR-9's fit covers only the happy path (*"after adopt → undo on the fixture, the whole tree is byte-identical"*). Worse, §4.4 sets the realistic case itself — *"After a week they run undo"* — so FR-9's *"restore the repository to its pre-adoption state"* admits three incompatible destructive semantics on a diverged tree (hard-restore the snapshot → clobbers a week of the operator's own commits; delete pyxis additive artifacts → but what of one the operator edited; detect-divergence-and-refuse). Undo is the explicit trust lever (§0 Judge-these #4; FR-9 rationale) yet is a destructive path with no fail-closed stance, and DOD-1's fixture exercises only immediate-clean undo. — vs — prd-template §5.2 (the failure space must be covered for each flow) · §5.8 charter (*"everything that must be true before complete"*) · the completeness what-would-stage-6-bounce check (a planner cannot write the undo charter without a product-level data-safety decision).
- **Acceptance condition**: define undo when the post-adoption tree has diverged — distinguish "remove the installation, preserve post-adoption work" from "revert to the pre-adoption snapshot," and specify that a modified pyxis additive artifact (chained hook / added file) or post-epoch work makes undo **fail-closed and name it, never silently clobber operator content**; add a §5.2 UB row for F3 with a fixture-assertable fit (adopt → modify a chained hook → undo refuses-and-names); flip §8 row 7 until the trace is true.
- **Disposition**: applied (round 2)

### FND-2 — Must-priority UB rows are gated by no blocking DoD row

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §5.8 gates functional behavior via DOD-3 (*"Every Must FR passes its fit criterion | FR-1..FR-10"*) and the happy-path round trip via DOD-1, but **no DoD row cites any UB row**. Yet UB-1, UB-2, UB-3, and UB-5 are all **Must** (e.g. UB-2, high risk: *"apply is run with no approved plan, or a plan stale… → refuse by name and change nothing"*). Per the priority bar (*"Must = the release literally does not ship without it"*) and §5.8's own charter (*"what the factory verifies against"*), a Must safety-refusal that no blocking DoD requires can be entirely absent while all six DoD rows go green — the fixture round trip exercises no empty-tree routing, stale-plan refusal, unknown-stack, or non-git path. — vs — prd-template §5.8 (the completion checklist is what the factory verifies) · the §5.1 Must bar.
- **Acceptance condition**: add a blocking DoD row rolling up the Must UB rows (UB-1/2/3/5), or broaden DOD-3 to "every Must FR **and Must UB** passes its fit criterion."
- **Disposition**: applied (round 2)

### FND-3 — Whether apply creates the epoch-anchoring commit (a mutation of the adopter's history) is unspecified

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.4 calls the install epoch *"the adoption baseline"* stored in the repo config surface and *"set once at apply"*; §5.5 lists *"commit for the epoch"* as the git interface — both presuppose a commit — yet nothing states whether apply **creates** one. If it does, adoption introduces a commit into the adopter's history, a user-visible change J1 (*"know exactly what will change before it changes"*) says must be disclosed; if it does not, FR-8's *"content changed after it"* has no committed anchor until the operator commits. The choice propagates to FR-9 (undo removes/reverts a commit vs cleans the working tree) and M1 (byte-identity over working-tree vs committed bytes). This is not a Q1–Q5 mechanism — Q1 presupposes the anchor exists. — vs — §2.3 J1 (full disclosure of what changes) · FR-8 · M1.
- **Acceptance condition**: state in §5.4/FR-8 (and F2's response) whether apply creates a commit to anchor the epoch or records existing HEAD, and whether adoption introduces a commit into the adopter's history; thread the choice into FR-9 and M1.
- **Disposition**: applied (round 2)

### FND-4 — The repo config surface (the one persistent write target) has its failure-modes column blank

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.5 lists *"the repo config surface"* — where the epoch and resolved surfaces are written — with **failure modes = "—"** (an explicit claim of none), for a surface adopt must create, apply must write, and undo must modify. AGENTS.md's own rule is that config *"parses closed: unknown keys are errors"*; UB-4 (*"adopt against an already-adopted repo → re-plan idempotently"*) and §2.6's named workaround (*"hand-copy pieces of the factory"* — drifts) make reaching a pre-existing, partial, or malformed config surface a reachable state with no defined behavior. — vs — §5.2's own refuse-by-name pattern (UB-2, UB-5) which this write target lacks · AGENTS.md closed-parse rule.
- **Acceptance condition**: fill §5.5's failure-modes column for the repo config surface, and add a §5.2 UB row (following UB-2/UB-5) covering an absent-but-required, partial, or malformed config surface at (re-)adopt/apply, so UB-4's idempotent re-adopt is well-defined.
- **Disposition**: applied (round 2)

### FND-5 — NFR-1's fit criterion verifies a weaker quantity than the requirement states

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: NFR-1's requirement — *"keep adoption cost **constant in the size of the pre-existing corpus**"* (total cost O(1) in corpus size N) — is measured by a fit that asserts only *"gate cost **per unchanged file** does not grow with corpus size"* (per-file O(1), which permits total O(N)). A design that visits every unchanged file at O(1) each would pass the fit yet violate the requirement as worded; R1 confirms *"a per-file 'unchanged since epoch' test with a real cost."* Load-bearing: DOD-6 gates on NFR-1. — vs — prd-template Appendix A (a fit criterion must be able to fail when the requirement is violated).
- **Acceptance condition**: align requirement and fit on one quantity — narrow NFR-1 to the per-file/changed-file scope actually achievable (gate cost bounded by the number of *changed* files, independent of corpus size), or strengthen the fit to assert the gate never visits the unchanged corpus.
- **Disposition**: applied (round 2)

### FND-6 — FR-8's rationale reference "the Q-4 shape" collides with the PRD's own §7.2 Q-series

- **Severity**: advisory (CAL-3)
- **Lens**: coherence (precision)
- **Evidence**: FR-8's rationale — *"the epoch keeps a real corpus from failing every gate (D-5; the Q-4 shape)"* — intends `prd:installer/Q-4` (the install-epoch/grandfathering shape), but the bare token "Q-4" collides with this PRD's own §7.2 open-question series, which §5.4 already references in the same style (*"home is design-owned, Q3"*). A reader (and a lens) reasonably resolves "Q-4" to a §7.2 question that does not exist. — vs — §7.2 Q1–Q5 (the PRD's own Q namespace) · Appendix A unambiguity bar.
- **Acceptance condition**: disambiguate the reference — write `prd:installer/Q-4` (or "installer's Q-4") wherever the bare "Q-4" means the sibling's open question, so it never reads as a §7.2 item.
- **Disposition**: applied (round 2)

### FND-7 — Several Must FR rows bundle separable behaviors under one ID (singular rule)

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-3 (*"apply exactly that plan **and, on re-run, land only drifted content**"* — two triggers, one ID), FR-6 (*"write nothing to it **and shall emit in the plan** the exact content"* — two `shall` clauses), FR-7 (*"detect and name the … stack … **and name the gate-command action**"*), and FR-9 (*"restore … **unchaining** any chained hook **and naming** any manual addition"*) each fold separable behaviors into one requirement. — vs — prd-template Appendix A quality bar: *"singular (no conjunctions)"*.
- **Acceptance condition**: split each compound into atomic FR IDs (e.g. an apply-exactly FR and a separate re-run-idempotence FR), or record in-row why each is a single atomic behavior. (FR-3's re-run idempotence in particular deserves its own ID — it is the basis for UB-4.)
- **Disposition**: applied (round 2)

### FND-8 — Banned requirement wording: a vague pronoun in FR-7 and a negative "shall not" in NFR-2

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: FR-7 — *"**When it inspects the repository**, the system shall…"* — puts the banned vague pronoun "it" as the actor in the trigger clause, the subject named only afterward; NFR-2 — *"The system **shall not** write, read into, or configure any path in the never-touch set"* — is a negative requirement. — vs — prd-template Appendix A blacklist (*"vague pronouns ('it', 'this') — reviewers strike on sight"*) and keywords rule (*"avoid … negative requirements ('shall not') — state the positive behavior instead"*).
- **Acceptance condition**: name the subject in FR-7's trigger ("When the system inspects the repository, …"); restate NFR-2 positively ("shall confine every write, read, and configuration action to paths outside the never-touch set").
- **Disposition**: applied (round 2)

### FND-9 — ABUSE-1's "Mitigated by" list omits FR-5, which protects a never-touch path

- **Severity**: advisory (CAL-3)
- **Lens**: coherence (traceability)
- **Evidence**: ABUSE-1 (*"apply touches a path in the never-touch set"*) lists *"Mitigated by | FR-2, FR-4, FR-6, FR-10"* — omitting FR-5, which is exactly the requirement that keeps apply from overwriting the adopter's existing workflow (*"the adopter's existing workflow file is untouched"*), a never-touch/CI path per FR-10 and §4.4. — vs — the abuse→requirement traceability convention (§5.3).
- **Acceptance condition**: add FR-5 to ABUSE-1's "Mitigated by" cell.
- **Disposition**: applied (round 2)

## Round 2

Scope-locked re-review (protocol step 8): only the round-1 findings' acceptance conditions and regressions within the changed spans were examined — the full context pack was not rebuilt. In this pre-engine bootstrap the author and reviewer are the same principal; all nine dispositions were applied in one revision, and the single blocking product decision — undo's divergence semantics (FND-1) — was ruled by the operator and recorded as a review-anchored decision (brainstorm:existing-repo-onboarding#D-11).

**Acceptance conditions met (9/9):**

- **FND-1** (blocking) → FR-9 redefined to *remove the installation, preserving post-adoption work*; new **UB-6** fails closed (preserve-and-name) on an operator-edited added artifact; F3's states and §8 row 7 corrected (F3 func→FR-9, fail→UB-6). D-11 records the operator ruling.
- **FND-2** (blocking) → DOD-3 broadened to *every Must FR and every Must UB* (FR-1..FR-11, UB-1/2/3/5/6/7).
- **FND-3** → §5.4 now states apply records the repository's commit as the epoch and creates no commit of its own; the operator commits the adoption (no undisclosed history change, J1).
- **FND-4** → §5.5 failure-modes filled; new **UB-7** refuses an absent/partial/malformed prior config by name.
- **FND-5** → NFR-1 requirement and fit realigned to *gate cost scales with the changed set, independent of corpus size*; DOD-6, §3.3, §0, §6.4 threaded.
- **FND-6** → FR-8's reference disambiguated to `prd:installer/Q-4`.
- **FND-7** → FR-3 split into FR-3 + **FR-11** (re-run idempotence, the load-bearing case for UB-4); FR-6/FR-7/FR-9 recorded as single atomic behaviors (partial application — an advisory's prerogative).
- **FND-8** → FR-7's trigger names the subject; NFR-2 restated positively.
- **FND-9** → ABUSE-1 gains FR-5.

**Regressions in changed spans:** none — the added FR-11/UB-6/UB-7 are traced and folded into DOD-3 and §8 row 7; the realigned NFR-1 carries no stale "constant in corpus size" claim (decision-log D-5 keeps its historical framing); `prd:installer/Q-4` resolves; the fold gates green.

Two-round cap reached with both blocking findings resolved.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-23 |
| 2 | approve-with-advisories | 2026-07-23 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 7 advisory
- **Dispositions**: 9 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
