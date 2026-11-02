# queue-widget — PRD *(seeded fixture subject — an abbreviated draft 000-base with five planted defects; not a real ledger entry)*

| Field | Value |
|---|---|
| **In one line** | A dashboard widget showing the pending-session queue. |
| **Status** | Draft — problem review pending |
| **Tier** | T2 |
| **Appetite** | small: one widget + its feed |
| **Template version** | prd-template/v1 |

## 3.1 Goals

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G1 | a queue widget | the operator stops polling | M1 |
| G2 *(qualitative)* | the widget feels current | trust in the display | *(planted defect: no metric, no qual method — coherence)* |

## 3.2 Metrics

| Metric | Goal | Signal | Target |
|---|---|---|---|
| M1 | G1 | widget opened instead of queue polled | ≥ 80% of checks at checkpoint "20 sessions" |

## 5.1 Functional requirements

| ID | Requirement (EARS) | Priority | Fit criterion | Verify |
|---|---|---|---|---|
| FR-1 | When the queue changes, the widget shall reflect the change. | Must | the widget reflects a seeded change within its refresh bound | Test |
| FR-2 | The widget shall render the queue quickly. | Must | *(planted defect: "the widget is fast" — banned vagueness on a Must row — contract-quality)* the widget is fast | Test |
| FR-3 | When the feed is unavailable, the widget shall show its last state with a staleness mark. | Must | seeded outage shows marked stale state | Test |

## 5.2 Unwanted behavior

| ID | If… | Then… |
|---|---|---|
| UB-1 | If the feed returns malformed data | render the last good state, mark stale |

*(planted defect: no row for the feed racing a queue mutation mid-read — the surface's obvious second failure path — completeness-critic)*

## 5.8 Definition of Done

| ID | Done criterion | Source | Blocking? |
|---|---|---|---|
| DOD-1 | FR-1 and FR-2 pass their fit criteria | FR-1, FR-2 | yes |

*(planted defect: Must FR-3 has no blocking DoD coverage — coherence)*

## 6.3 Risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | the feed lags under load | refresh bound documented |

*(planted defect: reads feeding the staleness decision come from a cached index; the registry's stale-read incident class (AP-15 lineage) is unaddressed — upstream-alignment)*
