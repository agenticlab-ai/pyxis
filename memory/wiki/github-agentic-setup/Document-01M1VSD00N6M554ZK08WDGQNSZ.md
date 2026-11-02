# GitHub setup for agentic workflows — settings and discipline that keep the merge pipeline honest

| Field | Value |
|---|---|
| **In one line** | The repo settings, merge-train discipline, generated-artifact hygiene, and `gh` mechanics that keep an agent-driven PR pipeline from stranding, conflicting, or landing off `main`. |
| **Relevant-when** | configuring the repo for agent-driven PRs; a stacked train stranded or a PR shows "merged" but its content is not on `main`; a doc PR conflicts on `index.*`/`effective.*` on every merge; choosing branch/merge/protection settings; an agent driving `gh` |
| **Authority** | descriptive — the sources below govern; on conflict they win |
| **Sources** | `AGENTS.md` (default implementation rules — merge trains, one concern per PR) · `docs/observability/incidents.md` (I2) · `.github/workflows/docs-gate.yml` · GitHub repo-settings & branch-protection docs (external) |
| **Last reviewed** | 2026-07-10 |

## Why this page

Agentic workflows push many small PRs fast, often stacked. That exposes GitHub defaults a human-paced team rarely hits: PRs that read "merged" but never reached `main`, generated files that conflict on every concurrent merge, and `gh` calls a guard blocks. The fixes are mostly repo settings plus a short discipline — captured here so they are set once, not relearned per incident.

## 1. Repo settings to turn on

- **Automatically delete head branches** (`delete_branch_on_merge`) — the load-bearing one. On merge, the head branch is deleted and GitHub retargets any open PR based on it to that branch's base (i.e. `main`), so a stacked child follows its parent onto `main` instead of merging into a now-dead intermediate. With it **off** (the default), stacked children strand off `main` while every badge still reads "merged" (incident I2, hit twice here). Enable it in **Settings → General → Pull Requests → "Automatically delete head branches"**, or `gh api --method PATCH /repos/<org>/<repo> -f delete_branch_on_merge=true`.
- **Required status checks** (branch protection) — make the `docs-gate` context required before merge; that is what makes the ledger contract non-bypassable. The pre-push hook runs the identical predicate for desk parity (`.github/workflows/docs-gate.yml`).
- **Squash-merge default** — fold/regenerate steps produce noisy commits; squash keeps `main` one-commit-per-PR and reverts clean.
- **(Optional) allow auto-merge** — lets a green, approved PR merge itself, so agents need not poll a check they cannot hurry.

## 2. Stacked-PR trains — merge bottom-up, let deletion retarget

A PR badge certifies the PR's **base branch**, never `main`: "merged" ≠ "on `main`." So:

- Merge **bottom-up**. Deleting each base branch on merge (§1) retargets the next child to `main`. Merging a child while its base branch still exists merges it into the dead intermediate → **stranded**.
- **Verify the train landed:** `git merge-base --is-ancestor <head-tip> origin/main` for every PR before reporting it done.
- **Un-strandable shape:** a PR that targets `main` directly cannot strand. Re-land stranded content as direct-to-`main` PRs.

The rule and its recovery live in `AGENTS.md` (default implementation rules) and incident I2; this is the orientation, not the authority.

## 3. One concern per PR — docs standalone, code stacked

Every PRD amendment, technical design, and incident change ships as its **own** PR; implementation stacks on the approved contract. For the PRD ledger this is machine-enforced by `check-single-amendment` (docgate FR-27). Stacking is what lets "standalone" and "ordered" coexist — which is why §2's discipline matters.

## 4. Generated artifacts are conflict magnets under churn

When PRs merge concurrently, any committed generated file derived from shared state — `docs/tech-design/index.*`, a ledger's `effective.*` — conflicts on nearly every merge.

- **Rebase-and-regenerate** (`docgate gen-index` / `fold`); never hand-merge a generated file — the textual merge is meaningless, the regeneration is the truth.
- **Structural fix:** do not commit a cross-plane join at all — compute it on read (the coverage-on-read change, issue #70). Fewer committed generated surfaces that many PRs touch means fewer conflict magnets (AP-23).

## 5. Driving `gh` as an agent

- **Fetch before basing a branch.** `main` moves fast; a stale base means a needless rebase or a conflicting PR.
- **Create PRs via the API when `gh pr create` is blocked.** A PreToolUse guard or a wrong-project-dir hook can intercept `gh pr create`; `jq -n '{title,head,base,body}' | gh api --method POST /repos/<org>/<repo>/pulls --input -` does the same and names the exact repo.
- **Poll external state sparingly.** A merge is a human action GitHub will not push to you; watch it with a bounded background poll, not a tight loop.

## Failure modes, at a glance

| Symptom | Cause | Fix |
|---|---|---|
| Every PR says "merged" but `main` lacks the content | stacked children merged into intermediates (base branches not deleted) | enable auto-delete (§1); merge bottom-up; verify with `is-ancestor` |
| Every PRD/design PR conflicts on `index.*` / `effective.*` | a committed generated artifact derives from shared state | rebase-and-regenerate; stop committing cross-plane joins (#70) |
| `gh pr create` blocked or opens a PR in the wrong repo | a PreToolUse hook or a wrong project dir | `gh api … /pulls` with an explicit `/repos/<org>/<repo>` |
