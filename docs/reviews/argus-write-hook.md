# Review: argus-write-hook — design-review report

| Field | Value |
|---|---|
| **In one line** | Round 2 (scope-locked): both round-1 dispositions land on their acceptance conditions and the changed spans carry no regression — the deny remedy is now the third element of the binding `{path, boundary, remedy}` reason in §3.1, threaded into DINV-1 and the §3.3 fail-closed rules and fixture-asserted in A-1/A-4 (all now tracing NFR-4), closing FND-1; DINV-6 is a single testable "No new surface" invariant with the honesty meta-claim moved to prose (§2/§3.4/§3.7 + R1), closing FND-2. The honest layer-split still holds — no row over-claims the hook as the unbypassable guarantee (that stays the merge-boundary layers'). No new findings; **approved** (advisory period — blocks nothing). |
| **Relevant-when** | auditing or consuming the review of the Argus write hook (part 1 of argus-boundary-enforcement); asking why it was request-changes in the advisory period; tuning tech-design-review calibration |
| **Subject** | design:argus-write-hook · shape: none · supersedes: — |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide as of 2026-07-12 |
| **Date(s)** | 2026-07-12 (R1), 2026-07-12 (R2) |
| **Verdict** | approve *(advisory period — blocks nothing; both round-1 findings closed on applied dispositions)* |
| **Tracking** | the subject's introducing PR (filed on approval) |
| **Template version** | design-review-report/v1 |

## Round 1

*Four lenses generated in parallel; the lint floor (header schema, Serves
resolution/liveness, mandatory sections) is excluded. The crux — the honest
layer split (the write hook is the fast, best-effort layer, never the
unbypassable guarantee, which parts 2+3 carry at the merge boundary) — was
verified and is **clean**: every occurrence of "unbypassable"/"guarantee"
(§1, §2, §3.2, §3.4, §3.5 DINV-6, §3.7 R1-note, §3.8 out-of-band row, R1,
AP-13/AP-26) attributes it to parts 2+3, and no row claims the hook as the
guarantee. All three fail-closed paths are specified (unresolvable scope ⇒
deny — FR-2/DINV-2/A-4; hook runtime error/crash/timeout ⇒ deny —
UB-6/DINV-3/A-4; a CLI with no fail-closable hook ⇒ listed + merge-boundary
backstop named — UB-2/DINV-4/A-3). AP-25 (canonical logic once + per-CLI
pointer adapters) is honored throughout; §3.7 confirms no privileged/OS/
outward surface. Established non-findings per the review brief (the R4
repo-internal judgment, RW-5 one-of-three decomposition, the glossary/I1
PR-time handoffs) are not re-litigated.*

### FND-1 — the denial "remedy" NFR-4 mandates is claimed in §3.6 but absent from the binding reason shape (§3.1) and every acceptance row

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: `design:` §3.1 defines the canonical, reused deny reason as path + boundary only — *"the reason names the target path and which boundary it hit (authority contract / frozen doc / other problem's ledger / out-of-declared-scope) … — the standard reason/remedy shape"* (no remedy element enumerated) — while `design:` §3.6 asserts a **three-part** reason: *"each denial returns a reason naming the target path, the boundary class hit, **and a remedy**"*; A-1's required evidence is only *"a reason naming the boundary"* and A-4's only names *"the reason"*, so no binding element or A-row carries the remedy — vs — the upstream requirement it Serves, prd:argus-boundary-enforcement/NFR-4 (*"Every denial/defect/refusal shall name the path, the boundary, **and a remedy** … each layer's fixtures assert a reason naming path + boundary + remedy"*, Verify: Test), which makes the remedy a Must element with a fixture obligation, not optional.
- **Acceptance condition**: reconcile the reason shape across §3.1, §3.6, and acceptance — add the **remedy** element to the canonical reason definition in §3.1 (the shape every per-CLI adapter reuses) and to the required evidence of A-1 and A-4 (a fixture asserting the denial names path + boundary + remedy), so the §3.6 claim and the served NFR-4 are backed by a binding element and an executed check rather than asserted only in Observability prose.
- **Disposition**: applied *(Argus, tier-2 — dispose-blocking-finding)* — added the **remedy** as the third element of the canonical deny reason in §3.1 (the `{path, boundary, remedy}` shape every per-CLI adapter reuses, with the actionable next step per boundary class); carried it into the binding invariant **DINV-1** (now `{path, boundary, remedy}`, Serves FR-1 + NFR-4) and into the §3.3 fail-closed rules (both the unresolvable-scope and hook-runtime-error denies name a remedy); and added the three-part assertion to the required evidence of **A-1** and **A-4** (both now Trace NFR-4). §3.6's claim and the served NFR-4 are now backed by a binding element and an executed fixture check, not Observability prose alone.

### FND-2 — DINV-6 couples a non-testable design-honesty meta-claim with a testable no-new-surface invariant

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: `design:` §3.5 DINV-6 — *"**Fast layer, not the guarantee; no new surface.** This layer denies through hook-honoring surfaces only and **is never claimed to stop an out-of-band write** (parts 2+3 carry that); it adds no new egress, identity, secret, network, or privileged OS/container surface. (Serves NFR-1; …)"* — bundles two properties of different kinds: the "never claimed to stop an out-of-band write" clause is a **meta-assertion about the design's own honesty** (verifiable only by reading the document's claims, not a runtime/inspectable system invariant), whereas the "no new … surface" clause is inspectable and does map to A-4 — vs — the contract-quality bar that a `DINV-n` be property-testable (guide §3 contract-quality; the honest-limit stance already lives non-invariantly in §3.4 "Implementation must not infer", §3.7 R1-note, and R1).
- **Acceptance condition**: split DINV-6 so the invariant states only the testable, inspectable property — no new egress/identity/secret/network/privileged-OS surface (Serves NFR-1; verified by A-4) — and let the "fast layer, not the guarantee / never over-claimed" honesty remain where it is already carried as prose and as risk R1, rather than as a DINV clause; nothing binding is lost because the crux is already stated in §1/§3.2/§3.4/§3.7 and fit-criterioned by A-1.
- **Disposition**: applied *(Argus, tier-2 — dispose-advisory-finding)* — split DINV-6 to the single testable property (**"No new surface"**: adds no new egress/identity/secret/network/privileged-OS surface; Serves NFR-1; maps to A-4). The design-honesty meta-claim ("this fast layer is never claimed to stop an out-of-band write; parts 2+3 carry that") is dropped from the invariant and noted as prose that already lives in §2, §3.4, and the §3.7 R1-note + risk R1. Nothing binding is lost.

## Round 2

*Scope-locked to the round-1 dispositions and regressions within their changed
spans (§3.1 reason shape, DINV-1, §3.3 fail-closed rules, DINV-6 split, A-1/A-4)
— per the protocol, a finding on unchanged content is violation-withheld and
round 2 is the cap. The gate is green on both paths (tree-local `docgate gate`
and `docgate gate --merge-base origin/main`). Both dispositions meet their
acceptance conditions and the changed spans introduce no regression; the crux —
the honest layer split — is re-confirmed held across every changed span.*

**FND-1 (blocking, coherence) — dispositioned `applied`; acceptance condition met → closed.**
The acceptance condition required the **remedy** element added to the canonical
reason definition in §3.1 and to the required evidence of A-1 and A-4. Verified:

- **§3.1** now defines the deny reason as the three-part `{path, boundary, remedy}`
  shape — *"the reason names the target path, which boundary it hit … **and a
  remedy** … the standard `{path, boundary, remedy}` reason shape (Serves NFR-4).
  The remedy is not optional prose: every deny across every per-CLI adapter reuses
  this three-part shape, and the fixtures assert it (A-1, A-4)."* The remedy is
  now a binding element of the shape every adapter reuses, not Observability prose.
- **DINV-1** carries it — *"with a reason naming the path, the boundary, **and a
  remedy** (the `{path, boundary, remedy}` shape NFR-4 mandates). (Serves FR-1,
  NFR-4.)"* — the invariant now Serves NFR-4.
- **§3.3 fail-closed rules** both name a remedy — unresolvable-scope denies *"with
  the standard `{path, boundary, remedy}` reason (the remedy: resolve the drive's
  declared scope)"*; hook-runtime error *"denied with the same three-part reason
  (the remedy: fix/restart the hook)."*
- **A-1** required evidence now asserts all three — *"a reason naming the **path,
  the boundary, and a remedy** (a fixture asserts all three elements of the
  `{path, boundary, remedy}` shape)"* — and Traces `/NFR-4`.
- **A-4** required evidence now asserts all three for both the unresolvable-scope
  and forced-hook-error denies — *"names the reason — **path, boundary, and
  remedy** … with the same three-part reason"* — and Traces `/NFR-4`.

The reason shape is uniform across §3.1, §3.3, §3.6, DINV-1, A-1, and A-4 (no
residual two-part occurrence remains), and the served NFR-4 is now backed by a
binding element and two executed fixture checks rather than an Observability
assertion. The original §3.6-vs-§3.1/A-rows coherence gap is fully reconciled
from the §3.1 side. **FND-1 closed.**

**FND-2 (advisory, contract-quality) — dispositioned `applied`; acceptance condition met → closed.**
The acceptance condition required DINV-6 split to state only the testable,
inspectable property, with the design-honesty stance left as prose and risk R1.
Verified:

- **DINV-6** now reads only the single testable property — *"**No new surface.**
  This layer denies through hook-honoring surfaces only and adds no new egress,
  identity, secret, network, or privileged OS/container surface — an inspectable
  property that maps to A-4. (Serves NFR-1.)"* — property-testable and mapped to
  A-4, satisfying the contract-quality bar.
- The design-honesty meta-claim is **moved to prose** and genuinely lives at every
  cited location: §2 (*"never claimed as the unbypassable one"*), §3.4
  (*"Implementation must not infer … that this layer is an absolute guarantee …
  it is the **fast** layer"*), the §3.7 R1-note (*"This layer **cannot** stop an
  out-of-band write … Over-claiming the hook as unbypassable is the R1 risk"*),
  and risk R1. Nothing binding is lost; the claim was moved, not dropped.

**FND-2 closed.**

**Regression sweep (changed spans) — clean; honest-layer split held.** Across the
five changed spans no row over-claims the hook as the unbypassable guarantee:
DINV-1 claims a write-time deny (*"denied before any bytes"*), not
unbypassability; DINV-6 is now surface-scoped only; the §3.3 denies name remedies
without a guarantee claim; A-1 is corpus/write-time-scoped (*"100% of
protected-path write attempts … denied"*); A-4 is scope + hook-error + no-new-
surface. The unbypassable claim remains correctly assigned to the merge-boundary
layers (DINV-4; the §3.8 out-of-band row — *"the merge-boundary layers (parts
2+3) are the unbypassable backstop"* — is unchanged and still honest). No round-2
findings.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-12 |
| 2 | approve | 2026-07-12 |

## Escalation brief

none

## Measurements

- **Rounds**: 2 *(round cap reached at convergence — verdict approve, not escalate)*
- **Findings**: 2 total — 1 blocking (FND-1, coherence, CAL-1) · 1 advisory (FND-2, contract-quality, CAL-3); **0 new in round 2**
- **Findings closed**: 2 of 2 (FND-1 + FND-2, both on applied dispositions verified against their acceptance conditions in round 2)
- **Dispositions**: 2 applied · 0 declined · 0 withheld-as-violation *(both applied by Argus, tier-2, during the advisory period — dispose-blocking-finding exercised on FND-1; dispose-advisory-finding on FND-2)*
- **Round-2 scope**: locked to the changed spans (§3.1 reason shape, DINV-1, §3.3 fail-closed rules, DINV-6 split, A-1/A-4); 0 findings withheld (no in-scope disposition or regression surfaced a finding on unchanged content)
- **Gate**: green both paths — `docgate gate` (tree-local) and `docgate gate --merge-base origin/main` (history/CI), both exit 0

## Overlay corrections

none
