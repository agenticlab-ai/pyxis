# Session log — raise the default per-Worker wall-clock budget from 45m to 1h

Started: 2026-08-05 06:23 · CLI: Claude Code · Stage or skill: chore

## P-1 — the commit guard refuses every commit in the operator's own checkout

- **Found:** 2026-08-05 06:25
- **Problem:** With the change made and staged, the commit was refused by the
  executor's parent-checkout guard, which rejects any commit created outside a
  per-task worktree. It is not leftover state from a crashed run: the guard
  re-arms itself before every run and has no uninstall path, so once the
  executor has run in a clone, that clone stays commit-locked. Pointing the
  hook path at itself also displaced the repo's own hooks, so the document
  gate no longer runs locally here.
- **Blocked:** The operator had asked for a pull request, and every ordinary
  way to make one from this checkout meant either overriding the guard or
  changing a shared setting another session could be relying on. Which cost
  was acceptable was not mine to choose.
- **Asked:** 2026-08-05 06:28 — put three routes to the operator: commit from
  a linked worktree, which the guard permits and which overrides nothing;
  restore the repo's own hook path, which brings the document gate back but
  disarms the guard for the whole clone; or bypass the guard once for this
  commit.
- **Decided:** 2026-08-05 06:29 — the operator chose the worktree route.
- **How:** Moved the branch into a temporary linked worktree, ran the document
  gate by hand there in place of the displaced hook, committed and pushed from
  the worktree, opened the pull request, then removed the worktree and
  returned the main checkout to a clean mainline. The guard was left armed and
  the hook path untouched. The pull request carries a note for the reviewer
  that the guard leaves a clone commit-locked and gate-less after any run,
  which may be worth its own look.
- **Refs:** internal/executor/workspace/guard.go · .githooks · core.hooksPath ·
  agenticlab-ai/pyxis#1002 · "pyxis: refusing to mutate the parent checkout; a
  worker commits only in its own worktree (AP-20/DINV-1)"
