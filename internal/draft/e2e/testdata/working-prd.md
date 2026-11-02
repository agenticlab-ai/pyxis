# Widget — PRD

| Field | Value |
|---|---|
| **Template version** | prd-template/v2 |

## 0. Read to rule

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-99 | This is a human-only pointer. | none | Must | low | none | Inspection | — |

## 5. System requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | The widget shall open. | Usability | Must | high | It opens. | Test | G-1 |

| ID | Existing behavior protected | Why protected | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|
| NR-1 | Old widgets open. | Compatibility | low | Old fixture passes. | Test | FR-1 |

| ID | Area | Requirement | Fit criterion | Verify |
|---|---|---|---|---|
| NFR-1 | Performance | Open quickly. | Under 1 ms. | Test |

| ID | If… (trigger) | Then the system shall… | User sees | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|---|
| UB-1 | offline | queue the open action | pending state | Must | medium | No loss. | Test | FR-1 |

| ID | Actor and intent | Vector | Impact | Severity | Mitigated by | Verify |
|---|---|---|---|---|---|---|
| ABUSE-1 | Attacker forces open | malformed packet | disclosure | critical | reject packet | Test |

| ID | Event name | Fires when | Properties (typed) | Source | Feeds |
|---|---|---|---|---|---|
| EVT-1 | widget.opened | open succeeds | id:string | server | M-1 |

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-1 | Open test passes. | FR-1 | Test | CI run | yes |

| ID | Behavior | Eval set / harness | Threshold | Negative examples |
|---|---|---|---|---|
| EVAL-1 | Correct answer | fixture | 99% | wrong answer |

| # | We believe that… | Will result in… | Measured by… |
|---|---|---|---|
| G-1 | quick opens help | retention | weekly use |

| # | When… (situation) | I want to… (motivation) | So I can… (outcome) | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J-1 | opening widgets | preview | avoid errors | retry | interview |

| Metric | Goal | Signal | Baseline | Target (at checkpoint) | Feeds from |
|---|---|---|---|---|---|
| M-1 | improve success | completion | 50% | 95% | EVT-1 |

## 7. Learning

| # | Question | Owner | Needed by | Status | Resolution |
|---|---|---|---|---|---|
| Q-1 | Who owns retries? | PM | lock | open | |
| Q-2 | What stores the answer? | PM | lock | resolved | Memory stores it. |

| # | Risk / assumption / rabbit hole | Likelihood | Impact | Mitigation or test | Owner |
|---|---|---|---|---|---|
| R-1 | Retries may duplicate work. | M | H | Test replay. | Engineer |
