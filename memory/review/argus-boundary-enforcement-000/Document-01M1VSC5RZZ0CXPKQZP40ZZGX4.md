# Review: prd:argus-boundary-enforcement/000 — review report

| Field | Value |
|---|---|
| **In one line** | request-changes on one blocking coherence defect — DOD-4 (blocking) must prove ABUSE-3's forgery mitigation, which the contract carries only in FR-11 at **Should** priority (excluded from DOD-1's Must set) — plus one advisory on an unspecified write-hook runtime-error fail-closed path. |
| **Relevant-when** | auditing or consuming the review of the Argus boundary-enforcement PRD; asking why it was bounced at round 1; tuning prd-review calibration against a security-PRD's finding set |
| **Subject** | prd:argus-boundary-enforcement/000 |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide 2026-07-12 (`.pyxis/authoring/prd-review.md`) |
| **Date(s)** | 2026-07-12 |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

*Floor excluded per protocol step 2: the docgate gate suite is green on this tree both base-free (`docgate gate`, exit 0, zero defects) and `--merge-base origin/main` (exit 0). `argusledger.ClassForPath` was confirmed to match only `docs/problems/*/problem.md` and `docs/problems/*/delivery.md`, so the authority contract has no docgate parse today — the record's F-5 is faithful and the part-2 gap is real. Nothing the suite enforces is raised below. Four lenses run in parallel; each survivor cleared independent scrutiny. The record's live decisions D-1..D-6 are all carried (D-1→§4.1 Option A / decision-log r1; D-2→NG-1 / R4 / decision-log r2; D-3→§3.4 / §4.2; D-4→§1 R1 / §4.1 C·D / §6.3 R1 / ABUSE-4 / decision-log r3; D-5→§2.2 anti-segment / NG-4 / NR-4 / decision-log r5; D-6→header Appetite / decision-log r6), citations resolve, and the record's "no fog" holds — no basis-fidelity finding. The honest layer split (D-4 / §6.3 R1) is held consistently: every write-hook row (FR-1..3, UB-1..2, F1, ABUSE-4 sev **med**, §1 R1, decision-log r3) scopes the hook to its own surface and never over-claims it as the guarantee — a strength, not a finding. UB-1..5 are each specified (UB-1 unclassifiable→deny; UB-2 CLI default fail-open→adapter fail-closed-or-backstop; UB-3 malformed contract→named parse defect; UB-4 underivable attribution→fail closed; UB-5 I1 bootstrap→operator review). The glossary-term routing (DOD-8 / T-1..4) and the I1 bootstrap-exception PR note (DOD-7 / UB-5) are the known PR-time handoffs, not findings; the four terms were confirmed absent from `docs/wiki/glossary.md` today.*

### FND-1 — A blocking DoD row proves a high-severity abuse mitigation the contract carries only at Should priority

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: DOD-4 (§5.8, **Blocking: yes**) — "Each abuse case (ABUSE-1..4) has its mitigation proven" — combined with ABUSE-3 (§5.3, Severity **high**) whose "Mitigated by" column is "FR-11 (forgery-resistant, badge-independent derivation) + FR-8/FR-4..6 (the structure arm …)", while FR-11 (§5.1) is Priority **Should** and DOD-1's source is explicitly "FR-1..FR-10" (only the Must set; §8's "Every Must FR …" row likewise scopes to "FR-1..FR-10 (Must)"). — vs — the coherence lens rule "every Must FR covered by a blocking DoD row" and, more sharply, the reciprocal: a *blocking* DoD obligation must not rest on a *Should* requirement for a **high**-severity security mitigation. The "+ FR-8/FR-4..6" half of ABUSE-3's mitigation closes only the **authority-contract** evade path (structure legality is author-independent, F2/NR-4); it does not cover the forgery / false-refusal vector for the **governance-doc** or **out-of-declared-scope** protected-path classes, which have no structure arm — for those classes the *sole* merge-boundary guarantee is the attribution gate (FR-9) resting on FR-11's forgery-resistance and FR-10's fail-closed. If FR-11 is deferred (legitimate for a Should under DOD-1), DOD-4 becomes unsatisfiable for ABUSE-3 on exactly the classes the feature exists to protect. This is a self-contained cross-reference defect inside the contract, independent of the operator-accepted mechanism-deferral (R4): the *property* forgery-resistance is observable behavior the PRD already states in FR-11's fit criterion ("a spoofed-badge fixture does not flip a human diff to Argus or vice-versa"); only its **priority** is misaligned with the blocking obligation that consumes it.
- **Acceptance condition**: Reconcile the priority against the obligation, one of: (a) raise FR-11 to **Must** and extend DOD-1's source to "FR-1..FR-11" and §8's Must row accordingly (the forgery-resistance property, not the mechanism, is then a required-at-DoD behavior — consistent with R4, which defers only *how* attribution is derived, not *whether* it must be forgery-resistant); or (b) if FR-11 is to stay **Should**, narrow DOD-4's ABUSE-3 obligation and ABUSE-3's own "Mitigated by" so the *proven-at-DoD* mitigation is limited to what FR-1/FR-9/FR-10 + the structure arm actually guarantee without FR-11, and record the residual forgery/false-refusal exposure on the governance-doc and out-of-scope classes explicitly (e.g. as a stated R-row residual or a WR entry), so no blocking row silently depends on a deferred one.
- **Disposition**: applied — Argus, tier-2 (authority-contract/v1), **dispose-blocking-finding** exercised (delivery:argus-boundary-enforcement#E-11). The finding is correct: the attribution gate (FR-9 + FR-11 forgery-resistance + FR-10 fail-closed) is the sole merge-boundary fail-closed guarantee for the governance-doc and out-of-scope classes (the structure arm FR-4..8 covers only the authority-contract path), so a *blocking* DoD proof (DOD-4→ABUSE-3, high severity) resting on a *Should* row is a live coherence defect — if FR-11 deferred under DOD-1, DOD-4 is unsatisfiable on exactly the classes the feature protects. Took option (a): raised **FR-11 Should→Must** (and Risk high→critical, since it now carries the sole fail-closed backstop for two protected-path classes — honest-risk-tier rule; added `G1, J3` to its traces to match FR-9/FR-10; rationale rewritten to state the property-required-at-DoD / mechanism-design-owned split so it stays consistent with R4). Reconciled the DoD so every blocking proof rests on Must rows: DOD-1 source `FR-1..FR-10`→`FR-1..FR-11`, DOD-3 source likewise, and §8's "Every Must FR" row `FR-1..FR-10 (Must)`→`FR-1..FR-11 (Must)`. DOD-4's source is §5.3, and ABUSE-3's "Mitigated by" now points at FR-11 (Must) — DOD-4 no longer depends on a deferred row. R4's mechanism-deferral is preserved: Q2 (how "Argus-authored" is derived) stays a design question; only the forgery-resistance *behavior* is now required-at-DoD. Re-folded + re-gated green (base-free exit 0; `--merge-base origin/main` exit 0).

### FND-2 — The write hook's fail-closed contract omits a runtime hook-error path, distinct from UB-2's default-config fail-open

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: FR-2 (§5.1) covers "a drive's declared scope cannot be resolved"; UB-1 (§5.2) covers "cannot classify a target (unresolvable path or scope)"; UB-2 (§5.2) covers a CLI "hook surface fails open **by default**" (a configuration-time posture, remedied by the adapter setting the fail-closed option). — vs — the T3 security framing (§5.3, §5.9 EVAL-1) and the operator's fail-closed posture (§3.3 guardrail row "Desk write-hook fail-open on a supported CLI"), under which the canonical fail-closed obligation is that a **write-time hook that is wired fail-closed but itself errors, crashes, or times out at evaluation** must still deny the write. No row specifies that behavior: UB-1/FR-2 handle an unresolvable *input*; UB-2 handles a fail-open *default*; none handles the hook adapter *failing to produce a decision*. On a self-constraining security surface whose adversary is a mis-behaving or injected Argus (and where a wedged/crashing hook is a plausible bypass), leaving the hook-runtime-error verdict unstated is a completeness gap parallel to the UB-1 pattern the PRD already established. It is not load-bearing for §8 (which passes on the UB-1..4 safety plan), hence advisory — but it is a first-class fail-closed behavior a T3 write-hook contract should pin as observable behavior, not a mechanism detail.
- **Acceptance condition**: Add (or extend UB-1/UB-2 with) an unwanted-behavior row of the shape "If the write hook errors, crashes, or times out at evaluation for a candidate write, then the system shall treat the write as denied (fail closed) and name the reason," with a fixture asserting a forced-hook-error case is denied — mirroring UB-1's "never allows" fit criterion.
- **Disposition**: applied — Argus, tier-2 (authority-contract/v1) (delivery:argus-boundary-enforcement#E-12). The finding is correct: the runtime hook-error/crash/timeout verdict is genuinely distinct from UB-1's unresolvable-*input* and UB-2's fail-open-by-*default*, and is a first-class fail-closed behavior a T3 write-hook surface owes (a wedged/crashing hook is a plausible bypass on a self-constraining security surface). Added a new row **UB-6** to §5.2 (kept distinct from UB-1/UB-2 rather than overloading them): "If the write hook process errors, crashes, or times out at evaluation for a candidate write on a supported CLI → deny the write (fail closed) and name the reason" · Priority Must · Risk critical · fit "a forced-hook-error fixture on a supported CLI is denied, never allowed" · Verify Test · Traces FR-1, F1, §3.3. Reconciled the trace consumers: §8 safety-plan row and §6.4 Reliability now read "UB-1..4, UB-6", and §8 flow-coverage F1 now reads "FR-1..3/UB-1..2,UB-6/NR-3". Re-folded + re-gated green (both paths).

## Round 2 (scope-locked)

Examined only the two round-1 dispositions and regressions within the changed spans (FR-11's row + its priority/risk/trace propagation; the DOD-1/DOD-3/§8 source reconciliation; the new UB-6 and its trace consumers). Gate green both base-free and `--merge-base origin/main` (exit 0, defects JSON empty). No finding on unchanged content — scope lock honored.

**FND-1 — closed.** FR-11 is now Priority **Must**, Risk **critical**, its rationale stating the property-required-at-DoD vs mechanism-design-owned split (R4 preserved — the mechanism Q2 stays a design question). DOD-1 and DOD-3 sources now read `FR-1..FR-11`, and §8's "Every Must FR" row likewise; every FR-1..FR-11 is confirmed **Must** priority, so **every blocking DoD proof — including DOD-4 → ABUSE-3 → FR-11 — rests only on Must rows**. The newly-critical FR-11 raises no unmet §8 obligation: the "high/critical rows have a declared safety plan" row is satisfied by the feature's report-then-enforce rollout + revert-to-report-only kill switch + fail-closed posture (§6.5, now naming UB-6). Acceptance condition (option a) met.

**FND-2 — closed.** UB-6 added to §5.2 as a distinct Must/critical row (hook process error/crash/timeout → deny, fail closed), genuinely separate from UB-1 (unresolvable input) and UB-2 (default fail-open); its trace consumers are reconciled (§8 safety-plan row, §6.4 Reliability, §8 flow-coverage F1). It is desk-side hook-layer fail-closed — consistent with the honest-layer-split (§6.3 R1), not an over-claim of the unbypassable merge-boundary guarantee. Acceptance condition met.

**No round-2 findings on the changed spans.** No stale `FR-1..FR-10` range reference remains; the priority/risk/trace propagation is internally consistent. Terminal verdict at the two-round cap: **approve**.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-12 |
| 2 | approve | 2026-07-12 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 (terminal: approve)
- **Findings**: 1 blocking · 1 advisory
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation *(Argus, tier-2, disposed both — FND-1 blocking via dispose-blocking-finding, FND-2 advisory; nothing declined → nothing auto-escalated; round 2 confirmed both closed with no regression on the changed spans)*

## Overlay corrections

*Post-approve entries only — the M2 residual data; frozen rounds and history are never edited.*

none
