# Review: wake-watch-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Round 1 request-changes on five findings — no loop exit on a completed plan, a DINV-8-vs-§3.9 contradiction that silently killed dead-checker detection, an unbounded immediate-sweep hot loop on a stuck ready frontier, a Serves omission (UB-1), and a silently-omitted AP-16 — all applied same-day; round 2 approve. |
| **Relevant-when** | auditing or consuming the review of design:wake-watch-v1; asking why it was approved and what changed; tuning tech-design-review calibration |
| **Subject** | design:wake-watch-v1 · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide 2026-07 (`.pyxis/authoring/tech-design-review.md`) |
| **Date(s)** | 2026-07-30 (both rounds) |
| **Verdict** | approve |
| **Tracking** | — *(the subject's introducing PR, once filed)* |
| **Template version** | design-review-report/v1 |

## Round 1

*Subject post-lint. Context: the governing corpus, the basis `prd:self-drive-wake` (effective), the record `design-brainstorm:self-drive-wake-tech-design`, the Builds-on targets (`state-reader-v1` draft · `wake-channel-v1` live · `implement-now` live) — all read in full. Four lenses; killed candidates: a partition-honesty challenge (loop + verb in one document — the record's D-4 ruled they revise together; carried faithfully); a §4 strawman check (all seven alternatives are the record's scored ALTs with real verdicts); an FR-12 fitness challenge (the two-surface staleness split satisfies the fit's one-cadence drill while keeping the 3× defect bound — coherent once §3.6 states it). Five survivors.*

### FND-1 — The loop has no exit: a completed plan idles the watch forever

- **Severity**: blocking (CAL-4 — load-bearing for acceptance: the proof run ends with a completed feature, and the verb as drafted never ends)
- **Lens**: feasibility-completeness
- **Evidence**: design: §3.1 step 4 "**Idle** — awaited set empty ⇒ no reads, no events, block on the next derivation trigger" with no completion path anywhere in §3/§6 — vs — basis DOD-4's proof run ("one real 6-task feature driven **end-to-end**") and the A-row set, which contained no completion criterion.
- **Acceptance condition**: a defined completion behavior (empty awaited set ⇒ plan complete ⇒ summary + exit 0), the empty-but-incomplete case argued away or handled, and an acceptance row covering it.
- **Disposition**: applied — §3.1 step 1 now derives three await kinds and argues empty ⇒ complete (an incomplete DAG has a ready root or an in-flight/claimed blocker); step 5 defines the completion summary + exit 0; A-13 added.

### FND-2 — DINV-8 contradicts §3.9, and the contradiction silently disables dead-checker detection

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §3.3 DINV-8 "appends a typed event **before its effect proceeds**" — vs — design: §3.9 "Wake-events file unwritable → loop continues (events are diagnostics, **never load-bearing**)". Both cannot hold; and the chain is worse than the wording: §3.6's last-check age — FR-12's detection surface — reads from that same file, so the "benign" unwritable-file row quietly blinds the dead-checker drill the basis makes blocking.
- **Acceptance condition**: one consistent posture — effects never block on the sink, an unwritable sink is *loud* (a named cannot-verify-liveness defect on the status surface), and §3.6/§3.9/DINV-8 all say the same thing.
- **Disposition**: applied — DINV-8 rephrased (attempts; effects proceed; unwritable ⇒ the named defect); §3.6 names the events file as the liveness surface with the cannot-verify-liveness rendering; §3.9's row updated to match.

### FND-3 — A permanently-failing ready frontier hot-loops immediate sweeps

- **Severity**: blocking (CAL-4 — load-bearing: falsifies A-2's idle-cost posture and the basis's AP-19 ground in a state the operator has already hit live)
- **Lens**: feasibility-completeness
- **Evidence**: design: §3.1 step 1 "ready-but-unclaimed tasks (awaiting nothing — they trigger an immediate sweep)" with no re-trigger bound — vs — AP-16 ("nothing loops or skips silently… bounds its rounds") and the operator's recorded live case (a source repo with no worker backend: every ready task fails pre-claim, the frontier never changes, so derive→sweep→derive spins hot).
- **Acceptance condition**: sweep invocation is edge-triggered on derived-state change; a no-change sweep drops re-invocation to cadence pace with the wait surfacing at the worker-⚠ bound; a drill proves no hot loop.
- **Disposition**: applied — DINV-9 added (state-edge-triggered invocation); §3.1 step 3 states the stuck-frontier behavior; A-12's no-backend drill added.

### FND-4 — Serves omits UB-1 while the design implements its backoff and degraded-rendering halves

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.9 "Medium unreachable / rate-limited → … loop keeps cadence with **bounded backoff** … status shows unknown-with-reason" — implementing exactly basis UB-1's "keep cadence with bounded backoff … render the degraded state" — vs — the header's Serves, which listed UB-3/4/5 but not UB-1 (the reader's citation covers only the report-unknown half; the basis row's remaining substance was cited by no design).
- **Acceptance condition**: UB-1 in this design's Serves (overlapping Serves with state-reader-v1 is the conventions' expected shape).
- **Disposition**: applied — Serves row updated.

### FND-5 — AP-16 is this design's ground and was silently absent from Principles

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: §3.9's bounded backoff, the staleness alarms, and (post-FND-3) the edge-triggered re-invocation are AP-16's exact substance ("a safety skip that can repeat must count, alarm, or self-heal; every self-healing loop bounds its rounds") — vs — the Principles section, which cited fourteen principles but not AP-16.
- **Acceptance condition**: an AP-16 bullet consistent with §3.9/DINV-8/DINV-9.
- **Disposition**: applied — AP-16 bullet added.

## Round 2

*Scope-locked to the five named defects and regressions within their changed spans (§3.1 steps 1/3/5, §3.3 DINV-8/DINV-9, §3.6's liveness source, §3.9's unwritable row, the Serves row, Principles, A-12/A-13, §0's sanity line). All five closed as accepted: the completion argument is sound (the three await kinds make empty ⇒ complete a theorem, not a hope); the liveness posture is now stated identically in three places; DINV-9 is property-testable and A-12's drill pins it; the Serves and Principles additions agree with the body. Regression sweep within the spans: §0 stays ≤ 40 lines; the new A-rows carry Verify + evidence + traces; DINV numbering stays dense. Nothing new raised.*

**Verdict: approve.**

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-30 |
| 2 | approve | 2026-07-30 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 5 blocking · 0 advisory
- **Dispositions**: 5 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
