# Session log — add an enforced Go coverage gate

Started: 2026-08-31 14:47 · CLI: Codex · Stage or skill: /prd-amend

## P-1 — Whether test coverage may become a blocking check

- **Found:** 2026-08-31 14:47
- **Problem:** The approved gate says measurements do not block, while the requested percentage would make one blocking. The current suite is also below the requested floor.
- **Blocked:** I could not safely turn the measurement into a required check or leave the existing branch permanently red without the operator making that policy call.
- **Asked:** 2026-08-31 14:47 — I explained that a blocking percentage needs an approved contract change and that the current baseline would have to be raised.
- **Decided:** 2026-08-31 14:47 — the operator instructed me to add the coverage threshold to CI at eighty-five percent.
- **How:** Amend the product contract and supersede the conflicting design before implementation; keep the local and remote gates identical and raise the suite to the new floor.
- **Refs:** `docs/tech-design/code-gate-mirror.md`, `.pyxis/config.yaml`, `.github/workflows/docs-gate.yml`, `go test -coverpkg=./... ./...`.

Follow-up: contract inspection established that deterministic statement coverage is a code property, not the environment-dependent cost, speed, or size measurement the design excludes. The required check set and its numeric thresholds are config-owned, so no PRD amendment or design supersession is needed; implementation proceeds through the existing single-source gate.

Implemented through the existing single-source code gate. The exact repository-wide check passed at 85.11 percent, and the full race-enabled Go suite passed.

Refs: `go run ./cmd/coverage-gate -min 85`, `go test -race ./...`, `.pyxis/config.yaml`, `.github/workflows/docs-gate.yml`.

## P-2 — The percentage applies to every module or file

- **Found:** 2026-08-31 17:05
- **Problem:** The first implementation enforced a weighted repository-wide average. That allowed individual packages to remain well below the requested floor.
- **Blocked:** The delivered predicate did not express the operator's intended coverage rule.
- **Asked:** 2026-08-31 17:05 — the operator reviewed the result and clarified that the floor applies to each module or file, not the aggregate.
- **Decided:** 2026-08-31 17:05 — no module or file may remain below eighty-five percent.
- **How:** Replace aggregate-only admission with per-file admission, derive package results from those files, and raise every failing unit before reporting success.
- **Refs:** `internal/coveragegate`, `cmd/coverage-gate`, `.pyxis/config.yaml`.
