# prd-review calibration — prd-review-calibration/v1

The severity dial as data (prd:prd-review/FR-6). Findings cite rows by id;
a finding whose severity contradicts its cited row is a structure-checklist
red. Changes are ordinary data PRs.

| Row | Lens | Default severity | Escalation condition |
|---|---|---|---|
| CAL-1 | coherence | blocking | declined without a recorded reason → invalid; declined with reason → escalate |
| CAL-2 | upstream-alignment | blocking | declined with reason → escalate |
| CAL-3 | contract-quality | advisory | becomes blocking when the defect makes a Must row unverifiable |
| CAL-4 | completeness-critic | advisory | becomes blocking when the gap is load-bearing for §8 (contract readiness) |
| CAL-5 | *(any)* | — | round cap reached without convergence → escalate |
