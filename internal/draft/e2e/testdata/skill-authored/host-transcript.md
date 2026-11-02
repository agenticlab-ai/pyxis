# Scripted host transcript — widget-summary authoring fixture

This is isolated evaluation data. The scripted host is not a production approval authority. No real code host is contacted.

## H-1 — draft status

- Requested call: `go run ./cmd/pyxis draft status widget-summary`
- Supplied host result: slug `widget-summary`; branch `draft/widget-summary`; draft pull request `PR1`; no merged PRD ledger or locked version; open working files use the standard empty templates.
- Result use: author only under `drafts/widget-summary/` in this fixture. No draft-open call is needed.

## H-2 — stage-1 gate

- Requested host action: choose the explicit stage-1 verdict for the prepared record.
- Supplied host result: turn 6 A, ready-for-prd; simulator principal alice; date 2026-09-27; no actual Pyxis contract gate; PRD synthesis authorized as evaluation.

## H-3 — intended stage-1 publication

- Intended calls on the supplied existing branch: `git add drafts/widget-summary/brainstorm/001.md`; `git commit -m 'docs(widget-summary): record fixture brainstorm'`; `git push origin draft/widget-summary`.
- Execution: suppressed by fixture protocol. No actual Git operation or code-host request occurred.
- Supplied publication result: none; local fixture files are the evaluation output.

## H-4 — PRD draft status

- Requested call: `go run ./cmd/pyxis draft status widget-summary`.
- Supplied host result: same open draft as H-1; parent instructs synthesis into drafts/widget-summary/prd.md from the exact gated record; no new product question; stop before design.

## H-5 — intended PRD publication

- Intended calls on the existing draft branch: `git add drafts/widget-summary/prd.md drafts/widget-summary/brainstorm/001.md`; `git commit -m 'docs(widget-summary): author fixture requirements'`; `git push origin draft/widget-summary`.
- Execution: suppressed by fixture protocol; no real Git or code-host request.
- Supplied publication result: none; authored local artifacts are reported for the next fixture stage.
- Review and lock: pending; no reviewer, design stage, or production approval was run.
