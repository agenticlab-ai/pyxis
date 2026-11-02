# Session log — commit and push the e2e read-cap retune

Started: 2026-07-30 17:49 · CLI: Claude Code · Stage or skill: one-off chore

## P-1 — the live tech spec still states the old read cap

- **Found:** 2026-07-30 17:49
- **Problem:** The living tech spec describes the e2e design tool's read
  ceiling as forty files. The code now sets it to sixty-five. Anyone reading
  the spec to learn how the tool behaves will get the wrong number.
- **Blocked:** Nothing. The commit and push went ahead.
- **Settled:** 2026-07-30 17:56
- **Settled by:** human — I escalated it
- **How:** I asked whether to correct the spec, since it sat outside the two
  files I was asked to commit. The operator said to fix it in the same
  commit, so the spec now states sixty-five.
- **Refs:** `docs/techspec.md:780`; `internal/e2e/design/budget.go`
  (`DefaultReadCap`).

## P-2 — a guard refuses every commit made in the main checkout

- **Found:** 2026-07-30 17:52
- **Problem:** A commit guard rejects any commit created in the parent
  checkout and allows them only inside a separate worktree. Nothing in the
  operating manual mentions this, so the instruction to work on the current
  branch and the guard's demand for a worktree read as a contradiction until
  you hit the refusal.
- **Blocked:** The commit failed outright; the work had to be moved into a
  throwaway worktree and pushed from there.
- **Settled:** 2026-07-30 17:58
- **Settled by:** human — I escalated it
- **How:** I offered three routes and the operator chose the side worktree,
  which satisfies the guard on its own terms rather than stepping around it.
  Worth adding a line to the manual so the next session does not discover the
  rule by failing.
- **Refs:** `.git/pyxis-guard-hooks/pre-commit`; refusal text "refusing to
  mutate the parent checkout"; `core.hooksPath`.

## Note — this clone is shared

Another session was editing the operating manual, the pipeline map, and a
template throughout. Everything here was staged by explicit path and the
shared index was left clean while waiting on the operator.
