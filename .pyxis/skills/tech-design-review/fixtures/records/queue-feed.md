# Review: queue-feed — design-review report

| Field | Value |
|---|---|
| **In one line** | Approved with one open advisory after two rounds — six blocking substance defects (untestable DINV, partition leak, dangling A-trace, strawman §4, Serves-substance gap, cited-vs-honored AP-15 breach) found and closed. |
| **Relevant-when** | auditing the queue-feed design review (fixture); tuning tech-design-review calibration |
| **Subject** | design:queue-feed · shape: present · supersedes: — *(fixture subject)* |
| **Moment** | new-design |
| **Reviewer** | tech-design-review · guide 2026-07-10 |
| **Date(s)** | 2026-07-10 |
| **Verdict** | approve-with-advisories |
| **Tracking** | — *(fixture)* |
| **Template version** | design-review-report/v1 |

## Round 1

### FND-1 — a binding invariant nothing could test

- **Severity**: blocking (CAL-3 — a binding element goes untestable)
- **Lens**: contract-quality
- **Evidence**: design: §3.3 "DINV-1 — the feed is reliable" — vs — the template's invariants rule (property-test material, binding)
- **Acceptance condition**: DINV-1 restated as a falsifiable property (e.g. a feed read never returns a torn or stale-beyond-bound snapshot)
- **Disposition**: applied

### FND-2 — durable content leaked into the shape

- **Severity**: blocking (CAL-3 — the partition makes the invariant's home ambiguous, untestable-by-location)
- **Lens**: contract-quality
- **Evidence**: shape: "Durable invariants carried by the design: DINV-1…" — vs — the solution-shape partition (durable in the design, concrete in the shape; solution-shape-v1)
- **Acceptance condition**: the invariant list removed from the shape; the shape carries schema/verbs only
- **Disposition**: applied

### FND-3 — an acceptance row tracing to a ghost

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: A-2 "Traces to #DINV-2" — vs — §3.3, which defines only DINV-1
- **Acceptance condition**: DINV-2 defined (the outage-marking property) or A-2 re-traced to a real anchor
- **Disposition**: applied

### FND-4 — alternatives nobody could have chosen

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design: §4's two rows (do-nothing; a per-keystroke recompute) — vs — the conventions' alternatives rule (a design with one real option is an announcement)
- **Acceptance condition**: ≥ 1 genuinely choosable alternative weighed with honest loss reasons (e.g. event-derived state; on-demand query)
- **Disposition**: applied

### FND-5 — a Serves anchor the content never implements

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: header Serves `prd:queue-widget/FR-3` (outage → marked stale state) vs §3.1, which decides staleness from the snapshot's *age* only — outage-vs-age is never designed — vs — the anchor's fit criterion
- **Acceptance condition**: §3 decides the outage case (detection + the mark's semantics), or the Serves ref is dropped
- **Disposition**: applied

### FND-6 — a principle cited as honored while the shape breaches it

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design: Principles "AP-15 — honored… exact reads" — vs — shape: `"source": "cache"` feeding the staleness decision — the cited-vs-honored breach, both halves quoted
- **Acceptance condition**: the staleness path reads exactly (and the shape says so), or the Principles row becomes a reasoned deviation
- **Disposition**: applied

### FND-7 — an adapter surface with no testing strategy

- **Severity**: advisory (CAL-4 — not acceptance-load-bearing for this fixture's A-rows)
- **Lens**: feasibility-completeness
- **Evidence**: design: no testing-strategy subsection while the feed is consumed across a seam — vs — the template's §3.x menu (dependency bins; one adapter = hypothetical seam, two = real)
- **Acceptance condition**: a testing-strategy subsection binning the widget/feed dependency, or a reasoned skip
- **Disposition**: open (advisory)

## Round 2

none — closures of FND-1..FND-6 confirmed against their stated acceptance conditions; no regressions in the changed spans; FND-7 remains open by the author's choice (advisory).

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-10 |
| 2 | approve-with-advisories | 2026-07-10 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 6 blocking · 1 advisory
- **Dispositions**: 6 applied · 0 declined · 0 withheld-as-violation · 1 advisory open

## Overlay corrections

none
