# Review: impl-review-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Round 1: request-changes — 8 blocking + 9 advisory (the GitHub medium hostile to the machine-payload model on three counts: attribution, size cap, marker-grammar; merge-lane and context-pack reuse asserted past their real interfaces; the AP-13 staleness class in the multi-round envelope; fresh-context isolation at the wrong boundary; AP-12 omitted); all applied in revision; round 2: approve. |
| **Relevant-when** | auditing or consuming the review of design impl-review-v1; asking why it was approved after a request-changes round; tuning tech-design-review calibration against real findings |
| **Subject** | design:impl-review-v1 (post-lint draft; shape: none; record-fed from design-brainstorm:impl-review-tech-design; basis prd:impl-review) |
| **Moment** | new design |
| **Reviewer** | tech-design-review · `.pyxis/authoring/tech-design-review.md` (2026-07-17) |
| **Date(s)** | 2026-07-17 (rounds 1 and 2) |
| **Verdict** | approve |
| **Tracking** | — |
| **Template version** | design-review-report/v1 |

## Round 1

*Process note: four lenses ran as independent parallel reviewers over the stage context pack, the served basis, the feeding record, and the composed live designs (codegate, merge-lane, context-pack) incl. their in-tree code. Independent convergence: the GitHub-attribution mechanism (feasibility + contract-quality); the AP-13 staleness class in the envelope (coherence) — its fifth appearance this session; the codegate-field composition wrinkle (upstream + feasibility). The two structural blockers (merge-lane's `Downgrader(Diff)` interface, context-pack's closed-kind enum) were read from the actual code.*

### FND-1 — the reviewed revision is per-envelope, but rounds evaluate different head shas; DINV-7 can't certify a multi-round review

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.1 carries one envelope-level `reviewed_revision`; DINV-1/§3.2 append rounds inside the one payload; round 2 exists *because* the worker pushed a fix (a new head sha) — so round 1 was judged against sha_A, round 2 against sha_B, and one envelope field can't bind both. `context_id`/provenance have the same envelope-vs-round mismatch. — vs — AP-13/UB-6 (a verdict certifies the exact revision).
- **Acceptance condition**: `reviewed_revision`, `context_id`, and provenance are per-round; DINV-7 evaluates the live verdict against the latest round's revision.
- **Disposition**: applied — §3.1 moves these fields into the round record; DINV-7 restated per-round; the fifth appearance of the AP-13 class, now closed at the envelope↔round seam.

### FND-2 — merge-lane exposes no consumable slot; "zero lane-side change" is false

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: NR-4/A-9/§3.6 claim the verdict "is emitted into the merge lane's existing GATE-4 slot with zero lane-side change" — vs — `internal/lane/classifier/classifier.go`'s only seam is `Downgrader.Downgrade(Diff) (bool, string, err)`: synchronous, **diff-only**, `nil`-by-default (WR-3-deferred). A persisted per-proposal verdict cannot enter a diff-only synchronous call without the lane growing a fetch — real lane-side change.
- **Acceptance condition**: this design produces the verdict signal; wiring it into the `Downgrader` seam is a named owed merge-lane amendment (a `Downgrader` adapter that reads the impl-review verdict), aligned with merge-lane's own WR-3 deferral; correct NR-4's "existing slot, zero change."
- **Disposition**: applied — §3.6/§5.5/NR-4 corrected: the design emits the verdict; consumption is an owed merge-lane amendment (§7 Q4, a parked cross-design gap — the PRD's NR-4 fit needs a corrective amendment, codegate-NR-2 style, noted).

### FND-3 — context-pack's admissible kinds are a closed enum; the review inputs' membership is asserted, not shown

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §3.4 "extends context-pack-v1 … with the review-class subject set" — vs — context-pack-v1 §3.4/DINV-3/R1: the admissible kinds are a **closed enum** (governing docs · the frozen subject under work · calibration), changed only by a reviewed code+design change, and **no authoring-trail kind** (prior dispositions) is admissible. The design never maps its five inputs onto admitted kinds.
- **Acceptance condition**: map the inputs onto existing admitted kinds (the proposal at its reviewed revision = "the frozen subject under work"; charter + design binding = governing peers; calibration = the stage calibration) and confirm no authoring-trail kind (no prior findings/dispositions — which DINV-5 already excludes) enters; no enum change needed.
- **Disposition**: applied — §3.4 states the kind-mapping explicitly; the "extends" reframed to "declares a review-class pack over context-pack's existing admitted kinds, reusing the machinery"; the no-trail-kind confirmation ties to DINV-5's finding-blindness.

### FND-4 — the GitHub attribution mechanism ("the comment-edit actor") is not a harvestable field; the anti-forgery ground collapses on GitHub

- **Severity**: blocking (CAL-4: load-bearing for FR-16 (critical) and A-5)
- **Lens**: feasibility-completeness *(independently: contract-quality)*
- **Evidence**: §3.1/§3.2/DINV-4 make "the GitHub actor of the comment edit" the attribution source — vs — GitHub's comment object exposes one `user` (the bot) + `updated_at`, not clean per-edit authorship (that lives in a separate `userContentEdits` GraphQL connection, unreliable). An operator disposition made by editing the bot's comment cannot be attributed; ABUSE-5's whole defense fails on GitHub. No GitHub adapter exists in-tree — the mechanism is entirely on paper.
- **Acceptance condition**: GitHub operator dispositions are **separate operator-authored artifacts** (a PR review or comment, `user` = operator, natively attributable), not edits to the bot comment; A-5 exercises the real surface.
- **Disposition**: applied — §3.2 restructured (below): dispositions are operator-authored artifacts per medium; the attribution source is the disposition artifact's own author (Carina decision-record author / GitHub review-or-comment author), never a comment-edit actor.

### FND-5 — GitHub's 64KB comment cap vs the "byte-identical round-trip" of an append-only-growing payload

- **Severity**: blocking (CAL-4: load-bearing for A-3 (Must) — the deep band is exactly where payloads bloat)
- **Lens**: feasibility-completeness
- **Evidence**: §3.2 "a re-review edits the same comment (appending a round)"; A-3 asserts unconditional byte-identical round-trip — vs — GitHub's 65536-char comment cap; a deep-band multi-round review (many findings + ABUSE/incident context + the human rendering in the same comment) grows unbounded in a fixed container. R1 names serializer *drift*, not *capacity*.
- **Acceptance condition**: a GitHub-medium size strategy (continuation blocks with a reassembling parser, or a size-bounded block with the human rendering separated) and an A-3 large-payload leg.
- **Disposition**: applied — the persistence port (below) makes Carina (no cap) the reference medium; the GitHub adapter's size handling (continuation comments, machine-block-only sizing) is a named residual with the product bar fixed; A-3 gains the oversize leg.

### FND-6 — fresh-context isolation is proven at the subagent boundary, not the host↔implementer boundary FR-2 protects

- **Severity**: blocking (CAL-4: load-bearing for A-6 (Must) and the §3.9 security claim)
- **Lens**: feasibility-completeness *(the §3.9-has-no-testable-binding half independently: contract-quality)*
- **Evidence**: §3.4/A-6 prove four *subagents* are distinct from each other and carry a `context_id`; FR-2 protects distinctness from the *implementing session* — but if `/impl-review` is invoked by implement-now's worker (same lineage), the subagents are children of the implementing session and FR-2 fails. §3.9's "a compromised implementing agent cannot steer the review" holds only if the reviewer's *root* is outside the implementer, unshown.
- **Acceptance condition**: specify the invocation topology — the reviewer spawns as a fresh top-level session *sibling* to (not child of) the implementing worker; `context_id` derived from that root; an A-6 leg that fails if the reviewer inherited the implementer's context.
- **Disposition**: applied — §3.4 states the sibling-session topology; DINV-5 extended to the host↔implementer boundary; A-6 gains the would-fail-if-inherited leg; Q1 resolved to the topology, not just the field shape.

### FND-7 — AP-12 silently omitted; the orphan-review reaper and its in-flight predicate are unowned

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment *(the reaper-ownership half independently: feasibility)*
- **Evidence**: §3.4 "an orphaned in-flight review is reaped … by the host's janitor" introduces a durable throwaway resource (report surface, context_id, partial payload); Principles lists no AP-12, "Deviation: none" — vs — AP-12 (a new throwaway-resource type ships with its janitor in the same change; acting on absence needs an explicit in-flight predicate). No in-flight predicate or absent-timeout is defined.
- **Acceptance condition**: an AP-12 Principles row (honored-or-deviation); the in-flight predicate + absent-timeout defined here even if the reaper executes host-side (NR-2).
- **Disposition**: applied — AP-12 row added as a reasoned deviation (the reaper is the host's via NR-2; this design defines the in-flight predicate — a review with no terminal verdict past the absent-timeout — and the timeout as data); §3.8 carries it.

### FND-8 — every `§5.9` anchor is broken (§5 is Risks; no eval subsection exists)

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.4 and A-7 cite "§5.9" for the eval protocol four times; §5 is "Risks" and no §5.9 exists — the protocol content is inline in §3.4, but the label resolves nowhere.
- **Acceptance condition**: give the eval suite its own §3.x subsection and repoint the citations.
- **Disposition**: applied — the eval suite + its protocol is now §3.5 (§3.x renumbered); the four citations repointed.

### FND-9 — the finding composition overloads codegate's `check_id`/`rung` and drags a dead `rung` key

- **Severity**: advisory (CAL-3)
- **Lens**: upstream-alignment *(the rung-serialization half independently: feasibility)*
- **Evidence**: §3.1 reuses codegate's `check_id` as "lens+rule identity" and carries `rung` (a required, non-omitempty field per `internal/verify/schema/types.go`) "as its zero value" — repurposing codegate's frozen field semantics and emitting a dead `"rung":""` in every finding.
- **Acceptance condition**: kinship at the *grammar* level (name/JSON-compatible overlapping fields: path, line, message, log_ref) with impl-review's own finding struct + review-only fields, not literal struct embedding; a compatibility test (R4) asserts the overlap matches codegate's grammar.
- **Disposition**: applied — §3.1 reframed: impl-review declares its own finding whose overlapping fields are byte-compatible with codegate's core (one grammar family for the host's fixer — D-1's actual intent), no struct embedding, no dead field, no `check_id` overload; a `rule_id` review field carries the lens+rule identity.

### FND-10 — the RW-3 principle governing GitHub comment-as-database grammar is silently omitted

- **Severity**: advisory (CAL-2)
- **Lens**: contract-quality
- **Evidence**: §3.2 binds a parsed fenced block in a bot comment on GitHub — vs — RW-3/INV-39 and local-work-tracker-v2 §3.1 ("decision-records replace GitHub's HTML-comment marker grammar … neither is a parsed comment") are retiring exactly that; Principles cites RW-5 but not RW-3.
- **Acceptance condition**: an RW-3 Principles row, honored-with-reason (GitHub lacks a native typed record; the fenced block is the port's honest realization, absorbed into the port contract RW-3 sanctions), or route GitHub through a structured surface.
- **Disposition**: applied — RW-3 row added (honored: the GitHub adapter's block is the port's medium-residual realization, Carina being the typed reference; RW-3 absorbs medium weakness into the port contract, which §3.2's adapter is).

### FND-11 — the persistence is a two-adapter port with no port/conformance statement or testing strategy

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: two media (GitHub + Carina) = a real port with two adapters (AGENTS.md: every port ships a conformance suite); the design has no testing-strategy subsection binning the seams (serializer hermetic · LLM backend true-external · harvest remote-but-owned).
- **Acceptance condition**: a persistence-port subsection with a conformance suite + a testing-strategy subsection binning the seams.
- **Disposition**: applied — §3.2 recast as the persistence port + two adapters with a conformance suite; a §3.11 testing-strategy subsection added.

### FND-12 — the harvest's per-medium enumeration mechanism and cost are unstated

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: §3.3 "enumerate a repo's proposals" — GitHub (which PRs? paging cost) vs Carina (a worktracker query) are two enumeration paths; M1 needs the *full* proposal universe (to see never-invoked ones); the O(corpus) note is parse-cost only.
- **Acceptance condition**: name the enumeration contract per medium and the universe M1 divides by; the GitHub enumeration cost noted; A-4 covers both media (or scopes GitHub to Phase B).
- **Disposition**: applied — §3.3 states per-medium enumeration (Carina reference, GitHub Phase-B) and M1's proposal universe.

### FND-13 — the two-grammar cross-class end-state (JSON impl-review vs markdown siblings) is an unnamed debt

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: impl-review graduates to JSON while the siblings stay markdown (NR-3); no statement of whether a future unified corpus/WR-2 must bridge two grammars.
- **Acceptance condition**: name the two-grammar divergence as an accepted debt (the harvests are per-class; WR-2 reads impl-review's JSON).
- **Disposition**: applied — §3.6 names it: the divergence is accepted (each class's harvest is its own; WR-2 consumes impl-review's JSON directly).

### FND-14 — DINV-4/FR-16 (critical) attribution has no test against the real medium

- **Severity**: advisory (CAL-3; folds into FND-4's mechanism fix)
- **Lens**: contract-quality
- **Evidence**: A-5 tests exclusion on a synthetic fixture; the real-medium derivation was deferred to Q3 (squash-merge) — a critical invariant whose real realization is unverified (AP-14).
- **Acceptance condition**: bind the attribution test to the real medium's authorship surface before the FR-16-gated metric is trusted.
- **Disposition**: applied — with FND-4's operator-authored-artifact mechanism, A-5 tests the real author field (Carina now; GitHub at Phase-B integration, Q4).

### FND-15 — Serves hygiene: FR-7 mechanized-but-unserved; UB-2/UB-5 asymmetric

- **Severity**: advisory (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.1 attributes the verdict enum to FR-7 but Serves omits FR-7; UB-2 is served but its mechanism is only in A-2/§3.1 validation (uncited); UB-5 is reasoned-from in §4 but unserved.
- **Acceptance condition**: add FR-7 and UB-5 to Serves (the verdict set + the persistence-write-failure path); cite UB-2's mechanism.
- **Disposition**: applied — Serves gains FR-7, UB-5; §3.2's persistence adapter cites UB-2/UB-5.

### FND-16 — §3.9's isolation security leg asserts a property no invariant tests

- **Severity**: advisory (CAL-3; folds into FND-6)
- **Lens**: contract-quality
- **Evidence**: §3.9 "(3) a compromised implementing agent cannot steer the review" — but A-6 tested only inter-lens blindness + subagent distinctness, not host↔implementer isolation.
- **Acceptance condition**: bind the property (folds into FND-6's topology + A-6 leg) or scope it to FR-2's inherited inspection.
- **Disposition**: applied — FND-6's sibling-session topology + the A-6 would-fail-if-inherited leg gives §3.9's claim its verifier.

### FND-17 — the `Migration: none` / coexistence statement is implicit

- **Severity**: advisory (CAL-4, low)
- **Lens**: feasibility-completeness
- **Evidence**: the design introduces new entities but never states migration/coexistence explicitly (the template prompts it; the basis §5.4 says none).
- **Acceptance condition**: one line — all entities new, no migration; the persistence port coexists with the siblings' markdown by class.
- **Disposition**: applied — §3.6 "Must remain unchanged" gains the coexistence line.

## Round 2

*Scope-locked re-review of the revision: FND-1..17's acceptance conditions verified met. The persistence-port recast (§3.2, Carina reference + GitHub residual adapter), the per-round revision (§3.1, DINV-7), the sibling-session topology (§3.4, DINV-5, A-6), the grammar-compatibility finding (§3.1), the merge-lane and context-pack reconciliations (§3.4/§3.6/§5.5/Q4), the eval subsection (§3.5), the AP-12 and RW-3 Principles rows, and the testing-strategy subsection (§3.11) are internally consistent and contradict no recorded ruling — FND-2's and FND-4's resolutions are honest realizations of the record's D-1/D-2 intent (kinship as grammar-compatibility; the verdict produced here, consumed by an owed lane amendment). No finding against unchanged content; no new findings on changed spans.*

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-17 |
| 2 | approve | 2026-07-17 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 8 blocking · 9 advisory
- **Dispositions**: 17 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

- **2026-07-17 · operator, at the design's operator review gate (build order):** the design's Carina-first / GitHub-Phase-B ordering (which FND-4's disposition leaned on — "Carina now; GitHub at integration") is **reversed**: GitHub ships first because the dogfood proposals are GitHub PRs (`#D-6`, review-anchored). The consequence — the GitHub attribution mechanism moves onto the v1 critical path — is handled by *specifying* it in §3.2 (an operator-authored PR review/comment as the disposition artifact) rather than deferring; only the squash-merge survival remains Q3. FND-4's exclusion mechanism is unchanged; only which medium proves it first flips. §0 row 6, §2, §3.2, §3.3, §3.6, A-5, Q3 revised.
- **2026-07-17 · operator, at the design's operator review gate:** FND-5's applied disposition (GitHub continuation-blocks / size-spill for the 64KB cap) is **superseded**. The operator ruled it the wrong answer on two grounds: (1) 64KB is ample for a compact code review — a large payload is a *verbosity defect* fixed in the skill prompts (compactness discipline + an eval bound), not a persistence case; (2) splitting a payload across continuation-comments *itself* violates one-source-of-truth. The design now carries `#D-5` (review-anchored in the record): one proposal → one medium → one self-contained artifact, never split or mirrored; §3.2/§3.5 revised, DINV-9 + a one-source-of-truth Principles row added, the size Failure-mode and A-3 oversize-continuation leg replaced with the compactness bound. FND-4's operator-authored-disposition-artifact mechanism stands (attribution), reframed as folded-into-the-payload-by-reference to keep the review one artifact.
