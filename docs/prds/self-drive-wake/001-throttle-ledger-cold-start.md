# Amendment 001 — throttle-ledger-cold-start

| Field | Value |
|---|---|
| **Amends** | self-drive-wake *(chain through 000)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-08-03 |
| **Learned-from** | build-result:implement-now-cold-start-content-budget-stall-2026-08-03 · operator-decision:2026-08-03-throttle-ledger-cold-start |
| **Blast radius** | tier: contract-changing; touches: §5.4, §7.3; reason: the throttle ledger's integrity rule and its home both change — an absent bucket becomes empty rather than spent, and the bucket moves from a repository checkout to the machine, keyed so it is never partitioned finer than the platform's limit |
| **Tracking** | — *(the state-reader-v2 supersession + its implementation, once filed)* |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede §5.4 Data

Reason: the throttle-ledger row contradicted itself — declaring the bucket "safe to delete" while declaring a missing bucket fully spent, so every ephemeral driver checkout, having never held a bucket, blocked its first content write for a full hour-window.

| Entity / field | Meaning | Source of truth | Retention | Integrity rules | Privacy class |
|---|---|---|---|---|---|
| throttle ledger | the write-budget bucket's state — one bucket per (medium host, authenticated identity), covering every repository that identity writes to, shared by every pyxis process on the machine | the machine's pyxis state dir — never a repository checkout, whose lifetime is unrelated to the window the platform meters | ephemeral; safe to delete — an absent bucket costs no writes and no delay | absent ⇒ start empty (no prior holder, so no unobserved spend); present but unreadable ⇒ start conservative (full budget assumed spent within the window); never partitioned finer than the platform documents its limit — an undocumented scope is assumed coarser, never narrower | internal |
| wake/check events | typed events per check, wake, unknown, throttle deferral, stale flag | the repo's event stream (existing events surface) | per the event surface's standing policy | append-only; additive schema evolution | internal |
| read cursors | per-consumer "comments newer than" positions | the consumer's durable state (drive ledger); the reader is stateless | consumer-owned | reader never persists medium content | internal |

- **Migration** — none: no existing data changes home. A bucket written by an earlier build inside a repository checkout is not migrated, read, or counted: it is ignored, superseded by the machine-level bucket, and safe to delete at any time.
- **Parallel-run / cutover** — none: additive capability; manual sweeps keep working unchanged (NR-1).
- **Rollback** — delete/ignore the state above; the factory returns byte-identically to operator-clocked sweeps.
- **Decommission** — n/a for v1 (nothing replaces it).
- **Privacy check** — no DPIA trigger: no new personal data, no profiling, no monitoring of people.
- **Egress** — none new: reads travel over the existing authenticated medium APIs; no content leaves the environment to any new destination (and NG-3 forbids the class permanently).

### op: supersede §7.3 Decision log

Reason: the two rulings behind the §5.4 change — the absent/unreadable split and the fail-safe bucket key — are load-bearing and were nowhere recorded.

| Date | Decision | Alternatives considered | Why | Who |
|---|---|---|---|---|
| 2026-07-29 | New capability feature, not covered ground | gate `covered` → design under existing contracts | the every-user, both-media capability layer is owned by no contract; the parked loops stay parked (brainstorm:self-drive-wake#D-1) | @mdarabi |
| 2026-07-29 | "Eyes + alarm" in one feature; background driver deferred | eyes-only; alarm-only; the full standing driver | both contracted consumers need exactly these two; the driver is the predecessor's most defect-dense ground (brainstorm:self-drive-wake#D-2) | @mdarabi |
| 2026-07-29 | Both backends day one | GitHub-first | the offline floor promise; the port shape stays honest (brainstorm:self-drive-wake#D-3) | @mdarabi |
| 2026-07-29 | Status verb with wait ages in v1 | machine-only; plain status | quiet-vs-stuck visible in one command (brainstorm:self-drive-wake#D-4) | @mdarabi |
| 2026-07-29 | Timer-only v1; push = reserved slots | push in v1; push-first | ~10 s buys nothing pre-30-minute steps; push adds silent-failure surface (brainstorm:self-drive-wake#D-5) | @mdarabi |
| 2026-07-29 | Success = 0 re-invocations · latency bounds · 0 rate-limit trips, on a real run | wall-clock speedup; demo-only | measures the feature, not the operator's calendar; numbers from logs (brainstorm:self-drive-wake#D-6) | @mdarabi |
| 2026-07-29 | The six refusals (NG-1..NG-6), incl. never-host-a-relay | keep the relay door open; ship a cadence knob | the middleman/custody line is permanent; knobs on demonstrated need only (brainstorm:self-drive-wake#D-7) | @mdarabi |
| 2026-07-29 | Everything-or-nothing appetite | cut-line dropping comment reads; GitHub-sequenced landing | complete-and-verified over partial-and-fast (brainstorm:self-drive-wake#D-8) | @mdarabi |
| 2026-08-03 | An absent throttle bucket starts empty; only a present-but-unreadable one starts spent | keep fail-closed for both (000-base's rule); drop the conservative start altogether | an unreadable bucket may hide spend the reader cannot see, so fail-closed is right; an absent one had no prior holder, so there is no spend to hide and the conservative start buys nothing but an idle hour — separating the cases is also what makes the row's own "safe to delete" promise true | @mdarabi |
| 2026-08-03 | One bucket per (medium host, authenticated identity) in the machine state dir — never per checkout, never per repository | per-checkout (as first built); per-repository; per (identity, repository) | the platform publishes the content-write caps but not the entity they are scoped to, and reserves the right to change them without notice, so the bucket is never partitioned finer than the platform documents: per-repository would let N repositories each spend a full window against what may be one identity's quota, while per-identity can only ever over-throttle — which UB-6 permits and a rate-limit trip (M3) does not | @mdarabi |
