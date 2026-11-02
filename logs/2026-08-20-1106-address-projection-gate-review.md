# Session log — address projection gate review

Started: 2026-08-20 11:06 · CLI: Codex · Stage or skill: GitHub comment addressing

Nothing escalated this session.

## P-1 — The gate needs a separate contract change before its code can continue

- **Found:** 2026-08-20 11:10
- **Problem:** The review exposed both a real false positive and a change to how section replacement and source-map lineage must work. The current code change has no approved requirement or design for those semantics.
- **Blocked:** I cannot add the required contract documents to the code proposal because the repository forbids mixing those governed classes with implementation, and I cannot rewrite frozen inputs in place.
- **Asked:** 2026-08-20 11:10 — whether to park the code proposal and start the required contract-first sequence as separate proposals.
- **Decided:** Still waiting
- **How:** No implementation change was made after confirming the conflict.
- **Refs:** PR #1091; `docs/prds/docgate/effective.md`; `docs/tech-design/prd-amendment-model.md`; FR-30; `internal/docgate/fold/congruence.go`; `internal/docgate/fold/fold.go`
- **Decision update:** 2026-08-20 11:19 — the operator approved parking the code proposal and starting the separate contract-first sequence.
- **How update:** Start a contract-only branch from the current merge target and run the PRD amendment review gates before returning to implementation.
