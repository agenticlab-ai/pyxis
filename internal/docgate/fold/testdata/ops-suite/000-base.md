# widget throttling — PRD

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
| NG-1 | Per-tenant custom limits | appetite |

| ID | Waiting room | Revisit when / intended version |
|---|---|---|
| WR-1 | Adaptive limits | after v0 evidence |

## 4. Scenarios

A burst of writes arrives; the throttle sheds politely; readers never
starve.

## 5. System requirements

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When write rate exceeds the limit, the throttle shall shed excess writes. | protect readers | Must | med | shed rate ≥ overload in fixtures | Test | G1 |
| FR-2 | While shedding, the throttle shall emit a shed event per write. | observability | Should | low | one event per shed write | Test | G1 |

### 5.2 Unwanted behavior and failure handling

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | If the limiter state store is unavailable | fail open for reads, closed for writes | 503 on writes | Must | high | fixture drill: zero reader errors | Test | FR-1 |

### 5.8 Definition of Done

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | All Must FRs pass their fit criteria | FR-1..n | Test | CI run | yes |

## 7. Alignment

### 7.2 Open questions

| # | Question | Owner | Needed by | Status |
|---|---|---|---|---|
| Q1 | Is the limit global or per-shard? | @owner | design | open |
