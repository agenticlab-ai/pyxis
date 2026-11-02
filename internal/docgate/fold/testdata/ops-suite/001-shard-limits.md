# Amendment 001 — shard limits

| Field | Value |
|---|---|
| **Amends** | widget-throttling (chain: 000 only) |
| **Status** | Approved |
| **Owner** | @owner |
| **Date** | 2026-07-06 |
| **Learned-from** | build-result:fixture-run · operator-decision:review |
| **Blast radius** | tier: contract-changing; touches: FR-1, FR-2, NG-1, Q1, §4; reason: supersedes a Must row and flips a non-goal inward |
| **Template version** | amendment-template/v1 |

## Operations

### op: supersede FR-1

Reason: per-shard limits proved necessary in fixtures.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-3 | When a shard's write rate exceeds its limit, the throttle shall shed that shard's excess writes. | shard isolation | Must | med | per-shard shed in fixtures | Test | G1 |

### op: add FR

Reason: shed metrics need an aggregate view.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-4 | When shedding ends, the throttle shall emit an aggregate shed summary. | operator view | Should | low | one summary per burst in fixtures | Test | G1 |

### op: add DOD

Reason: the summary needs its own completion row.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-2 | Shed summaries verified in the burst fixture | FR-4 | Test | fixture report | yes |

### op: retire FR-2

Reason: per-write shed events flood at scale; the summary (FR-4) replaces the need.

### op: answer Q1

| Field | Value |
|---|---|
| **Answer** | Per-shard, with a global ceiling. |
| **Evidence** | build-result:fixture-run |
| **Decided-by** | @owner |
| **Effects** | FR-1 (superseded by FR-3) |

### op: rescope NG-1

Reason: per-tenant limits turned out to be one config key.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-5 | Where a tenant limit is configured, the throttle shall apply it before the shard limit. | cheap after sharding | Should | med | tenant fixture passes | Test | G1 |

### op: supersede §4

Reason: scenarios now cover shards.

A burst arrives on one shard; that shard sheds politely; other shards and
all readers are untouched; the aggregate summary reports the burst.
