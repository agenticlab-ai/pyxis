# widget throttling — PRD *(effective view)*

---

**GENERATED — DO NOT EDIT.** Sole writer: `docgate fold`. As of: **001**. Content fingerprint: `sha256:cf96e6803c7614fda115e9334d4fc3b2833335eaa2ceb697b3b1d31b25d3ab06`. Tool version (recorded, never hashed): `docgate/v1`.

---


| Field | Value |
|---|---|
| **In one line** | Throttle widget writes so bursts never starve readers. |
| **Relevant-when** | planning, building, or reviewing widget write paths; triaging starvation defects; changing burst limits |
| **Status** | Problem review ✓ · Solution review ✓ |
| **Tier** | T2 |
| **Owner** | @owner |
| **Approver(s)** | @owner |
| **Contributors** | fixtures |
| **Informed** | — |
| **Date** | 2026-07-06 created · 2026-07-06 last updated |
| **Appetite** | small |
| **Target release** | v0 |
| **Tracking** | fixture |
| **Artifacts** | — |
| **Template version** | prd-template/v1 |

## 3. Goals and boundaries

### 3.4 Non-goals and waiting room

| ID | Non-goal | Why refused |
|---|---|---|

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Adaptive limits | after v0 evidence |

## 4. Scenarios

A burst arrives on one shard; that shard sheds politely; other shards and
all readers are untouched; the aggregate summary reports the burst.

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-3 *(001 ← FR-1)* | When a shard's write rate exceeds its limit, the throttle shall shed that shard's excess writes. | shard isolation | Must | med | per-shard shed in fixtures | Test | G1 |
| FR-4 *(001)* | When shedding ends, the throttle shall emit an aggregate shed summary. | operator view | Should | low | one summary per burst in fixtures | Test | G1 |
| FR-5 *(001)* | Where a tenant limit is configured, the throttle shall apply it before the shard limit. | cheap after sharding | Should | med | tenant fixture passes | Test | G1 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If the limiter state store is unavailable | fail open for reads, closed for writes | 503 on writes | Must | high | fixture drill: zero reader errors | Test | FR-1 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must FRs pass their fit criteria | FR-1..n | Test | CI run | yes |
| DOD-2 *(001)* | Shed summaries verified in the burst fixture | FR-4 | Test | fixture report | yes |

## 7. Alignment

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | Is the limit global or per-shard? | @owner | design | resolved (001) — Per-shard, with a global ceiling. |

## Superseded & retired

- **FR-1** *(superseded by FR-3 (001))* — `FR-1 | When write rate exceeds the limit, the throttle shall shed excess writes. | protect readers | Must | med | shed rate ≥ overload in fixtures | Test | G1`
- **FR-2** *(retired (001))* — `FR-2 | While shedding, the throttle shall emit a shed event per write. | observability | Should | low | one event per shed write | Test | G1`
- **NG-1** *(rescoped (001))* — `NG-1 | Per-tenant custom limits | appetite`
