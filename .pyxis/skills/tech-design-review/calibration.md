# tech-design-review calibration — tech-design-review-calibration/v1

The severity dial as data (prd:tech-design-review/FR-6). Findings cite
rows by id; contradiction is a structure-checklist red. Changes are
ordinary data PRs.

| Row | Lens | Default severity | Escalation condition |
|---|---|---|---|
| CAL-1 | coherence | blocking | declined without a recorded reason → invalid; declined with reason → escalate |
| CAL-2 | upstream-alignment | blocking | declined with reason → escalate |
| CAL-3 | contract-quality | advisory | becomes blocking when a binding element (Decision · DINV · A-row) goes untestable |
| CAL-4 | feasibility-completeness | advisory | becomes blocking when the gap is load-bearing for acceptance (the A-rows / the DoD it feeds) |
| CAL-5 | *(any)* | — | round cap reached without convergence → escalate |
