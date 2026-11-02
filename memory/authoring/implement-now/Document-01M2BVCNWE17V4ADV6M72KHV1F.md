# implement-now — drive one frozen plan through scoped executor sweeps

Drive one **frozen, projected plan** on the same hardware under ambient
harness policy. The user-facing default is the shipped self-drive watcher: it
derives the plan's durable GitHub state, runs the first ready frontier
immediately, polls at the adapter's cadence, and invokes another exact
plan-scoped executor sweep after human merges unlock work. The lower-level
`cmd/implement-now` binary remains one bounded frontier sweep; use it directly
only when the operator explicitly asks for **once** mode.

The contracts are `docs/prds/implement-now/` plus
`docs/tech-design/implement-now-routed-execution.md` for each sweep, and
`docs/prds/self-drive-wake/` plus `docs/tech-design/wake-watch-v2.md` for the
driver. Input is the code-host repo `owner/name` and the feature slug or plan
directory. Resolve them from the active plan when unambiguous; otherwise ask.

## Steps

1. **Gate the plan.** Require a merged, frozen plan already projected onto the
   tracker. Run `plangate report docs/plans/<feature>` and require every task to
   carry a projected issue. Read its machine report to obtain the exact
   `<feature>/<version>` plan reference; never guess a version or read plan JSON
   as an executor substitute.
2. **Select the mode from user intent.** Default `/implement-now`, "implement
   this plan", and "run the executor" to **drive mode**. Select **once mode**
   only for an explicit request such as "one sweep", "run once", or
   `/implement-now --once`. Do not ask the user to choose when their request
   already decides it.
3. **Carry the invoking harness.** Pass `--ambient-backend
   <claude|codex|cursor|opencode>` from the current generated harness surface.
   An explicit routing default retains precedence; `.pyxis/config.local.yaml`
   recursively overrides `.pyxis/config.yaml` for this machine. Set
   `executor.routing.default: opencode` and `executor.review.backend: cursor`
   there to split implementation and review. Never infer a backend by scanning
   installed CLIs. Review retains its separately configured backend or inherits
   the implementation backend under executor policy.

   OpenCode can use the operator's available provider models through
   `harness-config.opencode.models`, keyed by `implementation-economy`,
   `implementation-strong`, and `implementation-frontier`. Each value is an
   exact `provider/model` from `opencode models`. This optional policy binds
   after semantic profile selection, applies to implementation and its
   continuations, and fails if the selected profile is missing or a configured
   model is unavailable. It is ordinary local policy; evidence-activated
   routing requires its qualified native catalog and rejects these overrides.
4. **Drive by default — one command, either medium:**

   ```text
   go run ./cmd/pyxis watch --repo <checkout> --repo-slug owner/name \
     --plan docs/plans/<feature> --ambient-backend <this-harness>
   ```

   The same command drives a GitHub, Forgejo, or Carina plan. The watcher reads
   `integrations.bindings.tasks` and `integrations.bindings.code_host` to select
   task tracking and proposals independently; legacy configurations retain
   `knobs.tracker`. It polls the selected adapters at their advised cadence.
   With an explicit code-host binding, `--repo-slug` is optional. Keep the process
   attached and surface progress. The watcher scopes its awaited set from the
   frozen plan and invokes `implement-now` with the exact `--plan-ref`, the
   same tracker selection, routing label, `initiator=drive-wake`, and ambient
   backend, so the watcher and its sweeps never disagree on the medium.
   Identical state never hot-loops; completion exits after every plan task
   closes.
5. **Run once only when explicitly requested:**

   ```text
   go run ./cmd/implement-now --repo <checkout> --repo-slug owner/name \
     --tracker <gh|carina> --label <projected-routing-label> \
     --plan-ref <feature>/<version> --ambient-backend <this-harness> \
     [--carina-root <store>] [--unattended] [--base <branch>] \
     [--journal <path>] [--worker-budget 1h]
   ```

   `--plan-ref` is the exact plan scope on every tracker (the grammar is
   shared by GitHub, Forgejo, and Carina): it admits
   only items carrying the projection's exact plan-task marker before blocker
   reads, and an invalid scope refuses before workspace or external tracker
   work. The sweep claims ready tasks serially, runs them to the configured
   parallel width, and exits after opening genuinely-green proposals or
   reporting terminal classes.

   Explicit integration bindings take precedence over the legacy `--tracker`
   selection. Forgejo connections name their base URL and token environment
   variable in `.pyxis/config.yaml`; secrets stay outside the file.

   Execution diagnostics are local and ignored: `.pyxis/logs/executor/`
   holds the event journal and operator output, `.pyxis/logs/watch/` holds
   watcher events, and `.pyxis/logs/agents/` retains each worker's stdout and
   stderr in the owning checkout. A `--journal` override must stay inside
   `.pyxis/logs/`. Carina workitems, delivery ledgers, and review evidence
   remain shared through Git.
6. **Read the classed output.** `succeeded` means a verified green proposal and
   current required review evidence; `failed` means work ran but did not reach a
   verifiable terminal; `blocked` means a dependency remains open; `skipped`
   means another worker owns the claim. Empty frontier means zero model calls.
   `pre-publish-reviewed` counts succeeded tasks whose review ran at the local
   revision before the push and covers the published head, so no post-green
   review was owed for them.
7. **Know the per-task cycle.** Each Worker converges locally before it pushes
   anything: implement → commit → local gate (the `code-gate` CI mirror, with a
   bounded fixer loop, `executor.caps.ci_self_heal_attempts`) → with a
   configured `executor.review`, review the exact local revision as a fresh
   sibling → on `request-changes`, remediate in the same worktree, re-gate,
   re-review (bounded by `executor.review.remediation_rounds`, default 2) → on
   acceptance, push once, open the proposal, and wait for required remote CI
   (the once-per-candidate authoritative backstop; a red remote run still gets
   the bounded self-heal loop). A local gate that never goes green, a review
   that escalates, or a remediation bound that runs out fails the task closed
   with its work committed on its branch and nothing pushed. `pre_publish:
   false` under `executor.review` restores the post-green-only review.
   **Task session continuity** (on by default; `executor.session.continuity:
   false` disables it): every implementation-role launch of a task — the
   implementation, its local-gate fixers, its review remediations, a respawn,
   a remote-CI heal — continues the task's own harness conversation (claude
   `--session-id`/`--resume`, codex `exec resume`, opencode `--session`;
   cursor has no resume and runs fresh), recorded in the machine-local
   ledger `<git common dir>/pyxis/task-sessions.jsonl`. A session is never
   resumed across tasks, after a harness change, into a recreated worktree,
   or for the reviewer. While a task's proposal is open, **new human review
   activity** (any comment on any surface, or a changes-requested review, by
   an account other than the engine's own) wakes one **feedback round**: the
   sweep admits the task, the resumed session receives the new comments as
   data, and the same converge path (local gate → review → push) answers on
   the same branch. Human rounds are unbounded — each is one human action,
   serviced once (the cursor advances at admission) — while automated review
   remediation is bounded by `remediation_rounds` (default 3). `feedback-rounds`
   in the summary counts them; `task_session` / `feedback_round` events carry
   the detail.
8. **Keep merge authority human.** Neither mode merges or stacks. In drive mode,
   the watcher observes the human merge and starts the next newly-ready frontier;
   no manual re-invocation is needed. In once mode, stop after the report.
9. **Handle stops and escalations.** `Ctrl-C` cancels in-flight workers and
   releases their claims; rerunning drive mode re-derives durable state. A task
   exhausting its bounded self-heal or respawn cap fails closed with recoverable
   WIP and an escalation record.

## Never

Run a skill-issued GitHub sweep without the exact plan scope; use a shared
routing label as if it were plan identity; silently downgrade drive mode to one
sweep; invoke the current `feature-drive dogfood` command as a production
lifecycle; merge, approve, or hold merge credentials; stack a dependent proposal
or base it on an unmerged blocker; reuse an agent session across tasks; treat a
clean process exit as success; bypass tracker truth with plan JSON; let a Worker
loop unbounded; infer a harness from installed software; hand-author backend CLI
arguments instead of using its registered adapter.
