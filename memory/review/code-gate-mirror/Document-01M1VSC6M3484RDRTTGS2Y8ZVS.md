# Review: code-gate-mirror — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories — the single-source mechanism and the correctness-vs-measurement category rule are sound and faithful to the record; one blocking finding (the gate claims hermeticity a transitively-run non-hermetic test breaks, with the fix deferred out of scope) applied, one advisory applied, one advisory accepted. |
| **Relevant-when** | auditing or consuming the review of code-gate-mirror; asking why it was approved; tuning tech-design-review calibration |
| **Subject** | design:code-gate-mirror · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · .pyxis/authoring/tech-design-review.md (2026-07) |
| **Date(s)** | 2026-07-21 |
| **Verdict** | approve-with-advisories |
| **Tracking** | the design's introducing PR, once filed |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — the gate claims hermeticity that a transitively-run non-hermetic test breaks, while deferring the fix out of scope

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:#DINV-2 asserts "every check in the blocking gate is a pure function of the declared input… not admissible" and design:A-4 offers "the known latency assertion no longer blocks" as *this design's* acceptance — vs — design:§3.2/`#O-2` "the actual repair of the known non-hermetic check … Owned by a future design" (#380) and prd:implement-now/FR-8's DoD "a change failing a required CI check **also fails the local gate on the same input**." The repo's mirror necessarily lists `go test -race ./...` as a check (it is the required-CI check the #600 strand named); that one check **transitively executes** the non-hermetic `TestRung0Latency`. So the gate is *not* hermetic, and FR-8's "same input → same verdict" is *not* met, until the deferred `#O-2` neutralization lands — yet A-4 claims it already is, contradicting `#O-2`. The invariant and its acceptance overclaim relative to the design's own (correctly ruled) scope.
- **Acceptance condition**: sequence the dependency honestly rather than dispute the D-5 ruling — reword A-4 to test what this design delivers (the admissibility rule + a config carrying no *directly* non-hermetic check), and state in §3.7/§3.3 that a suite check inherits its members' hermeticity, so DINV-2's full realization for such a repo is gated on the offender's neutralization as a **named precondition** (`#O-2`/#380), not claimed at ship.
- **Disposition**: applied — A-4 reworded to the admissibility rule + directly-configured checks; §3.7 gained the suite-inheritance/named-precondition paragraph; DINV-2 cross-references it.

### FND-2 — DINV-2/DINV-3 are stated generally but verified only for the one known instance

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:#DINV-2/#DINV-3 state a general property ("every check… measurements never block"), but design:§3.7 rests enforcement on "reviewer discipline; an optional conformance guard later" and design:A-4 exercises only the single known offender. A *future* non-hermetic check re-enters the gate via `go test` with no mechanical catch — the invariant's verification is narrower than its statement.
- **Acceptance condition**: state plainly in §3.7 that enforcement for arbitrary future checks is admission-rule + review, with the general mechanical guard deferred to Q1 — so the invariant's verification scope is not read as stronger than it is.
- **Disposition**: declined — the design already discloses exactly this (§3.7 "structural, not conventional… reviewer discipline"; Q1 defers the conformance guard). FND-1's applied §3.7 edit makes the admission-rule framing explicit; no further change needed. Advisory, non-escalating (CAL-3).

### FND-3 — the full mirror on every local surface imposes an unaddressed runtime cost, in tension with the "faster local" motivation

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:#DINV-1 + §3.6 bind *every* surface — including pre-push and the executor's drive loop — to the identical full list, i.e. `go test -race ./...` + the whole set, on every `git push` and every drive iteration — vs — design:§1's own motivation ("cheaper and faster … local") and the template's performance/cost concern, which §3 does not answer. Race-instrumented full tests on every push are materially slower than today's `go build/vet/test`; DINV-1 forbids a faster *divergent* subset, so the cost is structural, and an unacknowledged one is a plausible `--no-verify` bypass driver that would defeat the desk gate.
- **Acceptance condition**: name the cost as a risk with its accepted tradeoff (net-cheaper than a failed CI round-trip; speed comes only from caching / verdict-preserving incremental selection, never a reduced list) — so the tension is a recorded decision, not a silent gap.
- **Disposition**: applied — added risk R6; then, on the operator's review-anchored ruling (record D-7), upgraded from an accepted cost to a real mitigation: a required/advisory check `class` (§3.1) keeps the required set lean, and verdict reuse (#DINV-6, §3.10) runs the required gate once per tree — an executor-gated push is a no-op — so a change is gated once locally (cached) + once remotely (authoritative), not three times.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-07-21 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 1 blocking · 2 advisory
- **Dispositions**: 2 applied · 1 declined · 0 withheld-as-violation

## Overlay corrections

none
