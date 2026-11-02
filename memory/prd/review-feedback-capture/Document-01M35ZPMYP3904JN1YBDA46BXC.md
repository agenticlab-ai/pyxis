# Amendment 001 — recut-to-buildable-slice

| Field | Value |
|---|---|
| **Amends** | review-feedback-capture *(000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-07-13 |
| **Learned-from** | operator-decision:review-feedback-capture re-cut after /plan-new fog-bounce — implementation substrate absent in pyxis (no engine / ports / adapters / event-sink / running consumer), 2026-07-13 |
| **Blast radius** | tier: contract-changing; touches: FR-2, FR-6, UB-2, UB-4, EVT-1, EVT-2, EVT-3, DOD-4, §3.1, §3.2, §3.3, §3.4; reason: defer the substrate-blocked requirements to the waiting room, keeping the operator review + ledger + fold + citation-check slice that is buildable on today's docgate machinery |
| **Template version** | amendment-template/v2 |

## Operations

### op: retire FR-2

Reason: code-host comment ingestion needs a code-host adapter framework + a webhook/poll runner — unbuilt engine substrate; deferred to WR-4.

### op: retire FR-6

Reason: wiring a reviewer-calibration consumer needs a running consumer loop — unbuilt; deferred to WR-5.

### op: retire UB-2

Reason: the code-host-unavailable failure path belongs with the deferred code-host capture (WR-4).

### op: retire UB-4

Reason: the duplicate-ingestion (webhook + poll) path belongs with the deferred code-host capture (WR-4).

### op: retire EVT-1

Reason: no event sink exists; the `feedback_captured` domain event is deferred (WR-6). In the slice, M3/M4 read from the ledger's folded projection.

### op: retire EVT-2

Reason: no event sink exists; the `feedback_disposed` domain event is deferred (WR-6). In the slice, M1/M4 read from the ledger's folded projection.

### op: retire EVT-3

Reason: no event sink and no running consumer; the `feedback_signal_read` event is deferred with the consumer (WR-5/WR-6).

### op: retire DOD-4

Reason: the consumer-proof completion criterion depends on the deferred FR-6 / M2; deferred to WR-5.

### op: supersede §3.1 Goals as falsifiable hypotheses

Reason: G1's measured-by cites M2 (consumer-proof), now deferred with FR-6.

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | capturing the human-review loop (comment + disposition + outcome) as a closable, computable ledger | the factory can learn from and trace human review instead of discarding it | M1 (closability), M4 (provenance) |
| G2 | making capture automatic and additive | no new human review toil and no dependency on human review being present | M3 (coverage) + the §3.3 guardrails |
| G3 *(qualitative)* | closing the loop visibly | reviewers experience their comments as remembered and resolved, not dropped | the operator's own observation at review that a comment's outcome is visible in the ledger (solo operator; Appendix-A rule 15 — no ceremonial interview) |

### op: supersede §3.2 Goals → signals → metrics

Reason: M2 (consumer-proof) is deferred with FR-6; the event stream (EVT-*) is deferred (WR-6), so live metrics read from the ledger's folded projection; M3 narrows to the single live source (operator review).

*Checkpoints are observable exposure conditions, never dates. In this slice the domain-event stream (EVT-*) is deferred (WR-6), so live metrics are read by inspecting the ledger's folded projection directly.*

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| **M1 (primary)** | G1 | captured entries that reach a typed disposition | 0 (no ledger) | 100% closable; zero entries unresolved past their subject's merge without a flagged pending state — at the checkpoint *(first cohort of human reviews captured at 100% enablement on the pyxis repo)* | ledger projection (inspection) |
| M3 | G2 | operator review touchpoints that produce an entry | 0 | ≥ 99% of operator review touchpoints produce an entry (capture failures — tracked via UB-1 — are the only expected shortfall); any gap is itself visible in the ledger — at the checkpoint cohort | ledger projection (inspection) |
| M4 | G1 | human-caused changes that resolve back to a ledger entry | 0 | a human-caused change resolves to its entry (`Learned-from: review-thread` points somewhere real) — same checkpoint | ledger projection (inspection) |

*Deferred with the substrate: M2 (consumer-proof — the wired consumer reads the ledger; WR-5). Explicitly not a metric here: "the reviewers' M2 residual falls / the reviewers measurably improve" — the optimizer's payoff (NG-1), downstream of this feature (`brainstorm:review-feedback-capture#D-5`).*

### op: supersede §3.3 Guardrails and decision rule

Reason: the pre-registered ship rule cites M2, now deferred.

| Guardrail metric | Threshold | Action if breached |
|---|---|---|
| Reviewers' no-human-dependency posture | any pipeline path becomes unable to complete without a human review present | stop — protocol violation (NR-2) |
| Added ongoing human review toil | any new required human step in the review flow | stop / redesign (the feature is capture, automatic) |
| Gate / merge-path behavior | any change to merge authorization, required checks, or the risk lanes | stop — out of scope (NR-3) |
| Capture cost | capture adds a per-review cost above a bounded ceiling set in the technical design | pause; move disposition determination to the cheaper mechanism (P-1) |

**Ship rule (pre-registered):** M1 = 100% closable AND M4 resolves (provenance) AND no guardrail breached → adopt the slice; M1 high but entries not resolving back → the provenance edge is broken, fix the resolution check before widening; any guardrail breached → roll back capture (additive, so review is unaffected). *(The M2 consumer-proof gate returns with WR-5.)*

### op: supersede §3.4 Non-goals and waiting room

Reason: add the waiting-room entries that hold the substrate-blocked capabilities deferred by this amendment (FR-2, FR-6/M2/DOD-4, EVT-1..3).

| ID | Non-goal | Why refused |
|---|---|---|
| NG-1 | The self-optimization *engine* — the measure→propose→trial→adopt loop and its cohort/A-B machinery | a separately-specified program; this feature only *produces* the signal it consumes (`#D-2`, `#O-1`) |
| NG-2 | Backend scorecards / routing intelligence | the routing instance of the optimization loop, not this feature (`#O-2`) |
| NG-3 | Any review-management or analytics UI / dashboard | the ledger is data; a human-facing surface is a separate concern (`#O-3`) |
| NG-4 | Any change to the reviewers' "no human dependency" posture | human review stays an optional overlay; this captures it, never makes the pipeline depend on it (`#O-4`) |
| NG-5 | Migrating or modifying `review-report/v1` or the Argus ledgers | they stay untouched; this feature only *adds* cross-links to them (`#D-6`, `#O-5`) |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | A consumer that runs the self-optimization loop on this ledger | when the ledger has accumulated enough captured reviews to be worth optimizing on, and NG-1's engine is scheduled |
| WR-2 | Broadening consumers beyond reviewer-calibration (e.g. code-side proposal signal, Argus authority calibration) | when a second consumer is ready to read the ledger |
| WR-3 | Graduating the ledger to a docgate-governed class (structure lint + CI freeze) | when the schema is stable across the first several real entries (the record/ledger conventions' own staging posture) |
| WR-4 | Code-host comment ingestion (the retired FR-2 and its UB-2 / UB-4 failure paths) | when the engine substrate exists — a code-host adapter framework + a webhook/poll runner |
| WR-5 | The reviewer-calibration consumer wiring (the retired FR-6, metric M2, and DOD-4) | when a running consumer/optimizer loop exists to read the ledger |
| WR-6 | The domain-event instrumentation (the retired EVT-1..3: `feedback_captured` / `feedback_disposed` / `feedback_signal_read`) | when an event sink exists; until then live metrics read from the ledger's folded projection |
