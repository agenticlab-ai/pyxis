# Review: prd:self-drive-wake/000 — review report

| Field | Value |
|---|---|
| **In one line** | Round 1 request-changes on two blocking defects — a latency promise arithmetically unachievable at the stated check cadence, and landing state read as a medium claim without saying so (the AP-26/I2 class) — plus four advisories; all six applied same-day; round 2 approve. |
| **Relevant-when** | auditing or consuming the review of prd:self-drive-wake/000; asking why it was approved and what changed between fold `91fc0c17…` and fold `0d2d047e…`; tuning prd-review calibration against real findings |
| **Subject** | `prd:self-drive-wake/000` |
| **Moment** | new-base |
| **Reviewer** | prd-review · guide 2026-07 (`.pyxis/authoring/prd-review.md`) |
| **Date(s)** | 2026-07-29 (both rounds) |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | review-report/v1 |

## Round 1

*Subject at fold `sha256:91fc0c17…` (as of 000). Context: the prd-review context pack (vision · principles · incidents · folded effect · calibration, digest `d77a4e3e…`) + the basis record `brainstorm:self-drive-wake` read directly. Floor excluded (docgate-enforced structure); §0 excluded (human-only). Four lenses run in parallel; nine candidates; three killed under adversarial scrutiny (an EARS-subject nitpick on FR-7; unranked Musts — moot under the record's D-8 no-cut-line appetite; an NR-1-vs-EVT-6 seam — no actual contradiction, the fit criterion is precise). Six survivors.*

### FND-1 — M2's latency promise is arithmetically unachievable at the document's own cadence

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: §3.2 M2 "p95 ≤ 15 s GitHub … at the proof run" and §4.1 "the adapter-advised cadence (GitHub ≈ 15 s …)" — vs — arithmetic: transitions arrive uniformly within a check window, so transition→wake latency ≈ U(0, cadence) + read time; at a 15 s cadence, p95 ≈ 14.25 s + ~1–2 s API ≈ 15.5–16 s > 15 s. The document promises a number its own stated parameter cannot meet; the basis ruling (brainstorm:self-drive-wake#D-6) recorded "~15 s" — the PRD hardened an approximate ruling into an infeasible exact bound.
- **Acceptance condition**: make the pair self-consistent without touching the ruling's substance — either size the cadence advice below the promise (advice ≈ 10 s ⇒ p95 ≈ 11 s, comfortably ≤ 15 s, ~7% of the read budget, still within NFR-1) or restate the bound as cadence-relative; state the latency measurement basis.
- **Disposition**: applied — §4.1 cadence advice re-sized to ≈ 10 s with its sizing rationale stated; NFR-2 fit pins the proof run to the shipped advice; §0 sanity line and the FAQ quota answer updated. (D-5's "~15 s" phrasing was the illustrative default in the ruling's answer text; the binding substance — timer-only, adapter-advised, no knob, idle-zero — is untouched, per FR-6.)

### FND-2 — Landing state is the medium's claim, and the contract never says so

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §5.1 FR-1 "landing state (open · landed · closed-unlanded)" and §4.3 F1 step 3 "the drive re-derives the frontier through fresh reads and starts the next ready task" — vs — AP-26 ("a medium's status certifies the medium's object, never yours… goal-state is accepted only by observing the goal object itself"), incident I2 (four green "merged" badges, none of the content on the mainline), and the consumers' own contracts (prd:feature-implementation-drive judge-row 6; prd:argus-unattended-drive FR-6) which require goal-object verification the reader's output must enable. As drafted, the reader can launder a platform "merged" flag into what a consumer treats as landed truth — the exact I2 recurrence path, through the new capability.
- **Acceptance condition**: the contract states that landing state is the medium-reported claim; landed reports carry the medium's landed revision identifier so consumers can verify ancestry at the goal object; the flow and interface rows say which side owns that verification.
- **Disposition**: applied — FR-13 added (Must, high; fit: revision id on both adapters + a consumer-side ancestry-check fixture from the reader's output alone + port docs marked medium-claimed); §4.1, §4.3 F1 step 3, §5.5 read-port row, DOD-1 range, and §0 judge-row 4 updated in the same pass.

### FND-3 — FR-1's "comments" never names which discussion surfaces count

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.1 FR-1 "the comments newer than the caller's cursor" — vs — the subject medium's plurality of comment surfaces (proposal discussion comments, inline review comments, review summary bodies): a fixture author must invent the enumeration, so the stated Verify could not disagree with a wrong choice; the stage-6 compiler would bounce the ambiguity.
- **Acceptance condition**: enumerate the surface classes at contract level (medium-neutral), leaving per-medium mapping to the design.
- **Disposition**: applied — FR-1 now reads "across the medium's proposal-discussion surfaces (discussion comments and review comments, review summaries included)"; fit criterion updated to "per enumerated surface".

### FND-4 — FR-1's risk tier understates the touched surface

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.1 FR-1 "Risk: med" — vs — the template's own rule ("risk is upgraded by the touched surface") and AP-15: this read feeds dispatch/continuation decisions in every consumer; a semantically-wrong-but-schema-valid read is not FR-9's failure class and lands directly on the drive's advance decision.
- **Acceptance condition**: FR-1 carries high.
- **Disposition**: applied — FR-1 risk raised to high.

### FND-5 — One staleness predicate, two named evaluators

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: §5.7 EVT-5 "Source: status verb / wake channel" — vs — AP-11 ("one predicate, one place"; the predecessor's twin-predicate loops): naming two sources for one threshold predicate plants the divergence seam the principle exists to prevent.
- **Acceptance condition**: the event row names a single shared predicate implementation surfaced by both consumers.
- **Disposition**: applied — EVT-5 source now "the staleness predicate — one shared implementation, rendered by the status verb and emitted with wake-channel events (AP-11)".

### FND-6 — Measurement-plan gaps: cross-clock latency and M1's attribution scope

- **Severity**: advisory (CAL-4)
- **Lens**: completeness-critic
- **Evidence**: §5.7 EVT-2 latency "medium timestamp → delivery" with no clock-basis statement (skew between the medium's clock and the local machine can silently distort M2's p95); §3.2 M1 "operator-initiated executor invocations during a driven feature" with EVT-6 carrying no feature scoping — an unrelated sweep on the same repo would count against the proof run.
- **Acceptance condition**: state the latency measurement basis (and record clock offset in the proof-run report); scope M1's signal to the driven feature and give EVT-6 the property that makes it computable.
- **Disposition**: applied — M2's signal column states the basis + offset recording; M1 re-scoped "targeting the driven feature's scope"; EVT-6 gains `scope: str`.

## Round 2

*Scope-locked to the six named defects and regressions within the changed spans (subject re-folded at `sha256:0d2d047e…`; changed spans: §0 rows 4 + sanity line · §1/§4.1 approach paragraph · §3.2 M1/M2 · §4.3 F1 step 3 · §5.1 FR-1/FR-13 · §5.5 read-port row · §5.7 EVT-5/EVT-6 · §5.8 DOD-1 · §7.1 quota answer · §7.4 changelog. No finding delivered or considered against unchanged content.*

- FND-1 closed: cadence advice (≈ 10 s) now sits below the 15 s promise with the sizing rationale stated; p95 arithmetic holds with headroom; NFR-2 pins the proof run to the shipped advice; no ruling content changed (FR-6's adapter-advised/no-knob substance intact).
- FND-2 closed: FR-13 carries the claim-vs-goal-state distinction, the revision id, a consumer-side ancestry fixture, and the port-doc marking; the flow and interface rows now say who verifies. Checked for regression: FR-13 is EARS-shaped, carries fit + Verify + risk, lands inside DOD-1's updated range, and disputes no recorded decision.
- FND-3..FND-6 closed as accepted: enumerated surfaces (fit updated), FR-1 high, single-sourced EVT-5, measurement bases stated with EVT-6's new property feeding M1.
- Regression sweep within changed spans only: §0 stays ≤ 40 lines with the reading budget intact; DOD-1's widened range keeps §8's "every blocking DoD row traces" true; the changelog row records the pass. Nothing new raised.

**Verdict: approve.**

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-29 |
| 2 | approve | 2026-07-29 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 2 blocking · 4 advisory
- **Dispositions**: 6 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
