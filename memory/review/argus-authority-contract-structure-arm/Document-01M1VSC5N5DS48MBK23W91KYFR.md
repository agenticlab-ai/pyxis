# Review: argus-authority-contract-structure-arm — design-review report

| Field | Value |
|---|---|
| **In one line** | **Approve (round 2).** Both round-1 dispositions land on their acceptance conditions: FND-1's history-mandatory DINV-4 genuinely reds a wiped/absent-history non-initial current-tier (zero rows at a non-born tier — the round-1 evade path is closed on the merits), and FND-2's anchors now resolve to `#D-2`. One security-adjacent observation — a *single* fabricated `move: none` seed row at a non-initial tier still passes via the unchanged latest-row test, so "the born contract is the sole zero-move legal state" slightly overclaims — is **violation-withheld** as a finding on unchanged content (not the disposition's acceptance condition, not a regression in the changed span); it is surfaced for the sibling attribution/write-hook layers, not held against this arm. |
| **Relevant-when** | auditing or consuming the review of argus-authority-contract-structure-arm; asking why it was request-changes; tuning tech-design-review calibration |
| **Subject** | design:argus-authority-contract-structure-arm · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide 2026-07-12 |
| **Date(s)** | 2026-07-12 (round 1) · 2026-07-12 (round 2) |
| **Verdict** | approve *(round 2; was request-changes at round 1)* |
| **Tracking** | the subject's introducing PR (filed on approval) |
| **Template version** | design-review-report/v1 |

## Round 1

*RECORD-FED subject — basis is `design-brainstorm:argus-boundary-enforcement-tech-design` (gated ready-for-design, riding this PR, authored by Argus at tier-2, legitimate); its `D-n`/`ALT-n` are treated as basis and record-fidelity checks apply. Advisory period: this verdict blocks nothing until the ten-review checkpoint promotes adoption. `shape: none` — no partition-honesty pair evidence; every quote is `design:`. Dispositions left `pending` — Argus (tier-2) disposes.*

*Excluded as floor (not findings): header schema, Serves resolution/liveness, mandatory §3.6/§3.7 sections — `lint-tech-designs` proves these and the gate is green base-free and `--merge-base origin/main`. Established (not findings): the operator accepted Argus's R4 judgment (mechanism defers to design); the three-design program per RW-5; the glossary landing and the I1-bootstrap note are known PR-time handoffs (basis DOD-7/DOD-8).*

### FND-1 — DINV-4 "no bare pointer flip" is undefined on an empty change-history table — the canonical bare-pointer-flip state is uncovered

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: `design:` §3.2 — the DINV-4 test is *"the latest change-history row's resulting tier equals the current-tier"*, defect when *"latest-history-tier ≠ current-tier"*; and §3.9's fixture list is *"a latest-history-tier ≠ current-tier"* only — both presuppose ≥ 1 history row exists. — vs — `design:argus-artifacts#DINV-4` (a Serves anchor): *"a bare pointer flip **with no history entry** is invalid"*, and the frozen `authority-contract/v1` template checklist row *"The latest history row's tier equals Current tier"*. A change-history table whose heading is present and delimitable but carries **zero data rows** is exactly the "pointer with no history entry" DINV-4 names, yet it is not a parse defect (§3.1 fails closed only when a *region heading is absent* or *a row cannot be split into columns*, neither of which fires on a present-but-empty region), token-membership passes, attribution-present passes vacuously (no rows to check), and the "latest row" the DINV-4 test references does not exist — so whether the arm reports a defect or silently passes is unspecified. The state is reachable off the born-state contract (one seed row) by deleting the row, so an attacker or a drift can leave `Current tier: tier-4` with an empty history and, on the "no latest row ⇒ no mismatch ⇒ pass" reading, merge a self-elevated pointer past a green structure arm.
- **Acceptance condition**: §3.2 states that an **absent** latest history row (an empty or heading-only change-history region) is itself a DINV-4 "bare pointer flip / history-mandatory" defect — the arm requires at least one history row and the latest to equal the current-tier — and §3.9 / A-1 add an **empty-change-history** (zero-rows) fixture red at exactly this arm, distinct from the `latest-history-tier ≠ current-tier` fixture.
- **Disposition**: applied *(Argus, tier-2 — dispose-blocking-finding)* — the finding is a real security hole: the born contract's seed row is deletable, leaving `Current tier: <non-initial>` under an empty history that §3.1's parse would not flag (the region is delimitable) and that the old "latest-history-tier = current-tier" test skipped ("no latest row ⇒ no mismatch ⇒ pass"), merging a self-elevated pointer past a green arm. **Fixed** by redefining the DINV-4 "no bare pointer flip" test as **history-mandatory**: the arm reds when the change-history table carries **zero data rows while the current-tier is not the born initial tier** (or when the latest row's tier ≠ current-tier) — the canonical `design:argus-artifacts#DINV-4` "pointer flip with no history entry" and the frozen `authority-contract/v1` template checklist row. §3.1 now models a present-but-empty region as an empty history-row list the evaluation reds (not a parse defect); §3.2 carries the load-bearing rule + the one legitimate exception (the born contract's single `move: none` seed row at the initial tier stays green); §3.8 adds the failure-mode row; §3.9 + **A-1** add the zero-rows fixture (red at exactly this arm, distinct from the latest-tier-mismatch fixture) **and** the born-contract fixture (green); §3.7 ABUSE-1 states the wiped-history self-elevation is now closed. **Repo-internal + additive — no new grant or outward surface, so no R4 escalation** (the arm already reads the file and reports defects; this adds one more defect condition on the same input). **Residual**: none for this class — the born contract is the sole zero-history-move legal state, and it is distinguished by carrying its seed row + the initial tier; every other zero/empty-history state under a non-initial pointer reds.

### FND-2 — §2/§4 cite record anchors ALT-1/ALT-2 for the package-boundary decision, but those anchors carry the decomposition (DQ-1) alternatives

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: `design:` §2 cites the sibling-parse decision as *"(design-brainstorm:argus-boundary-enforcement-tech-design#D-2, #ALT-2)"* and §4's first alternative is titled *"Reuse the `A-n`/`E-n` ledger parse's class registry ... (design-brainstorm:argus-boundary-enforcement-tech-design#ALT-1's package variant)"*. — vs — the record's `## Alternatives`: **ALT-1** = *"One cohesive technical design (all three layers in one document)"* and **ALT-2** = *"A program of three designs (one per layer)"* — both **DQ-1 decomposition** options; the record carries **no** distinct `ALT-n` for the DQ-2 package-boundary choice (its DQ-2 options are scored only inside D-2's reasoning, whose own line *"The panel (ALT-1/ALT-2) scored the two package boundaries"* is the record-internal mislabel the design inherited). So `#ALT-2` resolves to "program of three," not "sibling parse," and `#ALT-1's package variant` resolves to nothing the design's §4 row describes — the ALT-n ledger is not carried faithfully at the citation layer. The record's D-2 *decision* content (sibling parse is the honest fit) is sound and is **not** disputed here; the finding is the misresolving anchor citation.
- **Acceptance condition**: the §2 and §4 citations name the record locus that actually carries the package-boundary reasoning (`#D-2`, whose body scores reuse-`argusledger` vs sibling-`authoritycontract`) rather than `#ALT-1`/`#ALT-2`, or introduce the qualifier that the DQ-2 options live in D-2's reasoning, not as standalone ALT-n entries — so every anchor resolves to the content the design attributes to it.

- **Disposition**: applied *(Argus, tier-2 — dispose-advisory-finding)* — corrected the misresolving anchors: §2 now cites `#D-2` with the explicit qualifier that D-2's reasoning scores the two package boundaries (reuse-`argusledger` vs sibling-`authoritycontract`) and that `ALT-1`/`ALT-2` are the DQ-1 *decomposition* options, not the DQ-2 package-boundary options; §4's first alternative row likewise re-cites `#D-2` (the reuse-`argusledger` boundary scored in D-2's body) instead of `#ALT-1's package variant`. Every anchor now resolves to the content the design attributes to it. The D-2 *decision* content (sibling parse is the honest fit) was not disputed and is unchanged.

## Round 2

*Scope-locked re-review (the cap). Examined only (a) each round-1 disposition against its acceptance condition and (b) regressions within the changed spans — §3.1 (empty-region model), §3.2 (DINV-4 redefinition), §3.7 (ABUSE-1), §3.8/§3.9 (fixtures), A-1. A finding on any unchanged span is violation-withheld. Gate re-run green on both paths: `go run ./cmd/docgate gate` and `--merge-base origin/main` each exit 0, no defects (a doc-only change; the arm is a design, not yet code).*

### Disposition verification

**FND-1 (blocking, applied) — acceptance condition met; verified on the security merits.** The acceptance condition asked that §3.2 make an absent latest history row (an empty / heading-only, zero-row change history) itself a DINV-4 history-mandatory defect (requiring ≥ 1 row and latest == current-tier), and that §3.9 / A-1 add a zero-rows fixture red at exactly this arm, distinct from the latest-mismatch fixture. All four corners are present:

- **§3.2** (the DINV-4 "no bare pointer flip (history mandatory)" table row, `design:` §3.2) now reds when *"the change-history table has **zero data rows** (empty / heading-only region) while the current-tier is **not the born initial tier**, **or** the latest data row's tier ≠ current-tier."* The load-bearing paragraph names the wiped-history self-elevation and states the born-contract exception.
- **§3.1** models a present-but-empty region as *"an **empty history-row list** (not a parse defect — the region *is* delimitable) that the DINV-4 evaluation then reds when the current-tier is non-initial"* (`design:` §3.1) — the empty-region model the acceptance condition required.
- **§3.9 / A-1** add the zero-rows fixture *"an **empty / heading-only change history under a non-initial current-tier** ... red at the DINV-4 arm, **distinct from** the latest-history-tier ≠ current-tier fixture"* plus the born-contract green fixture (`design:` §3.9, A-1). **§3.7 ABUSE-1** and **§3.8** carry the closure and the failure-mode row.

**Security merits — the round-2 question, verified.** Trace the wiped/absent-history self-elevation the finding named: an attacker sets `Current tier: tier-4` and deletes the seed row, leaving **zero** data rows. The revised clause reds when zero rows **and** current-tier ≠ the born initial tier (`floor`, per `design:argus-artifacts#DINV-4` and the live contract's *"born at floor"* seed) — `tier-4 ≠ floor` → **RED**. The old *"no latest row ⇒ no mismatch ⇒ pass"* escape is closed. The **born-contract green exception** is coherent: the born contract carries **one** row (`move: none`, tier = `floor`) and so passes through the ordinary latest-row == current-tier clause, not through any zero-rows path — it is not a zero-rows contract at all. The one changed-span zero-rows *pass* (zero rows **at** the born initial tier `floor`) grants only `floor`'s single `dispose-advisory-finding` bundle — the minimum authority, no elevation — so it carries no security loss. **No new grant or outward surface:** the arm reads one repo-local file with a Go parse and adds one more defect condition on the same read (§3.7 shows no new identity/egress/credential path) — the round-1 "one more defect condition on the same file read is repo-internal" holds. **FND-1 is closed.**

**FND-2 (advisory, applied) — acceptance condition met.** Every anchor now resolves to the record locus that actually carries the package-boundary reasoning. §2 (`design:` §2) cites *"#D-2 — whose reasoning scores the two package boundaries, reuse-`argusledger` vs sibling-`authoritycontract`; the DQ-2 package-boundary options live inside D-2's body, not as standalone `ALT-n` entries — `ALT-1`/`ALT-2` are the DQ-1 decomposition options"* — exactly the "re-cite `#D-2` + carry the qualifier" the acceptance condition offered. §4's first alternative (`design:` §4) re-cites *"#D-2 — the reuse-`argusledger` package boundary scored in D-2's reasoning, not a standalone `ALT-n`"* in place of `#ALT-1's package variant`; §3.1/§3.2/§3.3 likewise cite `#D-2`. The record confirms D-2's body genuinely scores reuse-`argusledger` vs sibling-`authoritycontract` (`design-brainstorm:argus-boundary-enforcement-tech-design#D-2`) and that ALT-1/ALT-2 are the DQ-1 decomposition options (record `## Alternatives`). **FND-2 is closed.**

### Round-2 findings

**None admitted.** One security-adjacent observation surfaced and was tested for admissibility:

- **Withheld (unchanged content) — the single-fabricated-seed-row masquerade.** The revised §3.2 residual and §3.9 / A-1 assert the born contract is *"the **sole** zero-history-move legal state, ... distinguished by carrying its seed row + **the initial tier**."* On the merits this slightly overclaims: an attacker who writes a **single** fabricated row `move: none | tier: tier-4 | evidence: "born at tier-4"` under `Current tier: tier-4` is **not** a zero-rows case, so it never reaches the new zero-rows clause; it passes the **unchanged** latest-row clause (`latest row's tier == current-tier`: `tier-4 == tier-4`), with `none` in-enum (DINV-3), `tier-4` in the ladder (DINV-4 tier-in-ladder), and defined tokens (DINV-5). So a *non-initial* tier can masquerade as a born contract via one fabricated `move: none` seed row — a second zero-*move* legal-passing state under a non-initial pointer, which the "sole" wording does not cover. **Why withheld:** this masquerade routes through the **latest-row-equals-current-tier test, which is unchanged content** (it passed this input identically at round 1 — no regression), and it lies **outside FND-1's acceptance condition** (narrowly the zero-rows / absent-history case, which is met). Under the round-2 scope rule ("a finding on unchanged content is violation-withheld"; the protocol's Never-deliver-round-2-findings-on-unchanged-content), it cannot be a round-2 finding. It is recorded here so the signal is not lost: the "sole ... distinguished by the initial tier" claim is stronger than the operative test delivers, and the fabricated-single-seed self-elevation is a real (if lower-yield than the closed zero-rows path) evade vector — worth a fresh-subject look, most naturally where the same authority-contract surface is governed at write/attribution time (the sibling ABUSE-1 layers). It does **not** disturb this round's terminal verdict.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-12 |
| 2 | approve | 2026-07-12 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 (round 2 is the cap; scope-locked)
- **Findings**: 1 blocking · 1 advisory (both round 1) · 0 admitted at round 2 (1 security-adjacent observation raised → withheld as unchanged-content)
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation *(both applied by Argus, tier-2, during the advisory period — dispose-blocking-finding exercised on FND-1, the wiped-history security hole)*
- **Round-2 outcome**: both dispositions verified against their acceptance conditions (met); FND-1 confirmed on the security merits (the wiped/absent-history non-initial self-elevation reds; the born-contract green exception is coherent and grants no elevation; no new grant/outward surface); FND-1 and FND-2 **closed**
- **Verdict trajectory**: request-changes (round 1) → **approve** (round 2)
- **Gate**: `go run ./cmd/docgate gate` → exit 0; `--merge-base origin/main` → exit 0 (both green; doc-only change)
- **Advisory period**: this verdict blocks nothing until the ten-review checkpoint promotes adoption

## Overlay corrections

none
