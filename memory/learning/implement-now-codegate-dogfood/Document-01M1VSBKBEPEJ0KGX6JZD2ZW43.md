# implement-now dogfood — learnings from the codegate epic

**Date:** 2026-07-19
**Author surface:** operator-driven session (attended dogfood)
**Subject:** the pyxis-native `implement-now` execution engine, exercised end-to-end
by running the **implementation-verification (codegate)** plan through it.
**Status:** descriptive learnings record — not authoritative. Where a finding
below is operational-failure-shaped it names the incident/principle it should
graduate into; where it is a live defect it links the issue.

---

## 1. Context

The goal was to finish the codegate epic (`docs/plans/implementation-verification/001`,
tasks T1–T11 plus the rung-2 wiring T5 left undone) by **dogfooding
`implement-now`** — running the engine as the implementer and watching it
closely, rather than hand-writing every task. The engine did produce real,
mergeable work; it also hit a series of limitations that are worth recording,
because most of them are structural (they will recur) rather than one-off bugs.

Everything shipped in the end. The point of this doc is the *friction*, not the
result.

### What shipped

| Task | PR | How it landed |
|---|---|---|
| T8 — incremental re-verification | #492 | salvaged from an engine attempt (agent test-shape bug fixed) |
| T9 — runtime hardening (Landlock/janitors) | #493 | salvaged; Landlock enforcement gated off (F2) |
| T11 — release lane + advisory parity | #495 | salvaged; engine built it but could not push the workflows (F1) |
| T10 — multi-stack matrix + catch suite | #498 | implemented directly (would have re-hit F1) |
| rung 2 — no-new-failures + capture-baseline | #516 | the gap T5 #480 left unwired (F4) |

---

## 2. Engine findings (the ones that will recur)

Each finding: **what happened · impact · how it was handled · recommendation.**

### F1 — The `workflow` OAuth-scope wall

**What.** The engine authenticates with a GitHub token that lacks the
`workflow` scope. Any task whose implementation adds or modifies
`.github/workflows/` cannot be pushed — GitHub rejects it: *"refusing to allow
an OAuth App to create or update workflow … without `workflow` scope."*

**Impact.** T11 (`codegate-release.yml`, `codegate-dry-run.yml`) and T10
(`codegate-catch-suite.yml`) were fully implemented by the engine but could not
be published; the commits sat on the engine's local worktree, invisible to
anyone reading GitHub. From the outside it looked like the tasks had not been
started.

**Handling.** Salvaged: pushed with a second, `workflow`-scoped GitHub account
(the engine/assistant account has only `repo` scope; a separate account carries
`workflow`). The git credential helper also had to be forced to the `gh` helper
because osxkeychain cached the wrong token.

**Recommendation.** Either (a) grant the engine's token `workflow` scope, or
(b) have the engine **detect this specific rejection and raise a typed
"needs-elevated-scope" escalation** — handing the workflow files to a human or
elevated lane — instead of surfacing a generic push failure (or, worse,
silently leaving the work unpushed). A task that touches CI config is a
predictable, detectable class.

### F2 — The local gate is platform-blind (gate-fidelity gap)

**What.** The dogfood ran on macOS. The engine's local gate (build/vet/test)
is a macOS gate. Platform-specific code — T9's **Linux Landlock** write-scope
enforcement — is only exercised on the Linux CI matrix. A green macOS gate does
not certify the Linux path.

**Impact.** T9's Landlock enforcement broke the verbs *on Linux CI only*: a
confined `go`/`vet`/`test` verb could not write its build cache, so rung 0 went
red and the falsification `go test` blew its 5-minute bound recompiling the
world. None of this was visible to the macOS gate the engine drove to green.

**Handling.** Gated Landlock enforcement **off by default** (opt-in via
`CODEGATE_LANDLOCK=1`), matching the macOS residual, so the rest of T9
(janitors, kill-recovery, the confinement seam, the platform-neutral
classification) could land green. The enforcement machinery ships intact for a
later Linux-matrix verification (design acceptance A-6).

**Recommendation.** The engine's convergence signal should mirror **CI's
platform matrix**, not just the host it runs on — or, at minimum, detect that a
diff introduces platform-guarded code (`//go:build linux`, a `_linux.go` file)
and refuse to claim convergence until a matching CI leg is green. A single-host
dogfood is structurally blind to cross-platform code; this is not a bug to fix
once but a boundary to respect.

### F3 — Drive-to-green flails on failures it cannot observe

**What.** The engine's bounded self-heal ("drive-to-green": local gate mirrors
CI, then a bounded CI self-heal loop) can only fix what it can **reproduce**.
When CI fails for a reason the engine cannot see locally — the F2 Landlock case,
or an environment/`HOME` interaction inside a stripped-env confined verb — the
fixer spends its attempt budget re-pushing speculative fixes that cannot work,
because it is guessing at a red it cannot even render.

**Impact.** The T9 Landlock allow-set widening (a plausible fix — add the
toolchain cache dirs to the write allow-set) failed on Linux CI, and the engine
had no way to tell *why* from macOS. Left to itself it would have burned its
retries; the same trap the harbormaster predecessor hit.

**Handling.** Recognized the class explicitly ("this cannot be fixed blind")
and made a deliberate scope reduction (gate the leg off, F2) rather than
flailing.

**Recommendation.** The self-heal loop should **classify** a CI red as
*reproducible-locally* vs *unreproducible-locally* (platform/env-specific) and,
for the latter, **fail closed and escalate immediately** rather than consume
attempts on fixes it cannot validate. "A red I can reproduce and iterate on" and
"a red I cannot even see" are different regimes and should route differently.

### F4 — Green + merged ≠ charter delivered (verified-done ≠ works)

**What.** T5's PR **#480** ("Rung 2: the test verb, the no-new-failures
baseline, and capture-baseline") merged green — but delivered only the
*foundations*: the `baseline` package, the failure-identity parser, and the
`go test -json` verb. The headline feature was left **unwired**: no check
registered on rung 2, the `baseline` package imported nowhere (dead code), and
`capture-baseline` a literal `"T5 scope: stub"`. CI was green precisely because
nothing exercised the missing wiring.

**Impact.** Rung 2 — the no-new-failures regression gate (FR-10), arguably the
single most valuable rung — was **silently absent** from the ladder for days,
behind a green-CI, merged-PR façade. It surfaced only through a manual
"did we actually finish the epic?" audit, then took a full follow-up (#516) to
close.

**Handling.** Audited the epic against origin/main directly, found the stub,
and wired rung 2 (the registered check + a real `capture-baseline`) in #516.

**Recommendation.** A merged PR with green CI is **not** evidence the *charter*
was delivered — this is exactly harbormaster principle **P6 (verified-done ≠
works)** recurring. The engine, or the review lane, needs a
**charter-completeness check**: does the diff deliver the acceptance criteria,
or only pass the tests it happened to ship? **Code that no test exercises is a
red flag** a completeness critic should catch. (That codegate — a
verification harness — was itself shipped with an unverified rung is the
instructive irony here.)

### F5 — The Model-A uncommitted-work backstop works, but the salvage is manual

**What.** When the agent finished implementing but left its changes
uncommitted, the engine's Model-A backstop committed them with a generic marker
(`executor: capture the agent's uncommitted implementation`) so the work was
not lost.

**Impact.** Net positive — T11's entire implementation survived this way and was
recoverable. But the mechanical commit message is unsuitable for a PR, so every
salvage had to reshape history into a clean, well-described commit before
raising the PR.

**Recommendation.** Keep the backstop (it saved real work). Improve it to
capture the agent's **intended** commit message — from a checkpoint, or derived
from the charter — instead of a generic marker, so a salvage does not have to
re-author the commit narrative.

### F6 — The local tracker (Carina) reported phantom completions — [#490](https://github.com/agenticlab-ai/pyxis/issues/490)

**What.** The engine's local work-item tracker reported tasks `completed` with
no merged implementation: the local close-state diverged from the committed
store.

**Impact.** The tracker's "what's left" signal was untrustworthy — it claimed
completions that had never merged, so the plan's own progress view could not be
believed.

**Handling.** Filed #490, corrected the wrongly-closed state, and drove the rest
of the epic by auditing GitHub / origin-main directly rather than trusting the
local tracker.

**Recommendation.** (Tracked in #490.) A task's completion predicate must derive
from the **committed/merged artifact** (verify-by-artifact), never a local
self-report — harbormaster **P1 (one predicate, one place)** and **P3
(verify-by-artifact)** again. An unverified local close-state is a phantom-loop
source.

### F7 — Agent-authored tests can encode the agent's own misunderstanding

**What.** T8's incremental-re-verification tests (agent-authored) built prior
verdict rungs with empty or mismatched `Checks`, which correctly tripped a
real safety guard (`checksMatchRegistry` forces a rerun when a prior rung's
shape does not match the registry). The tests *expected* "carry" where the
guard correctly forced "rerun" — the agent had misread the contract of the very
guard it was testing against.

**Impact.** Three failing tests that presented as a code defect but were a
test-expectation defect.

**Handling.** Fixed the **tests** (populate matching `Checks`), not the code —
the guard was correct.

**Recommendation.** A failing agent-written test is **not** proof of a code bug;
the test may bake in the agent's misunderstanding. Salvaging engine output needs
an adversarial pass: *does the test's expectation match the actual contract, or
just the author's belief about it?*

---

## 3. Engineering learnings (codegate itself)

These are design/mechanics lessons from the work, independent of the engine.

- **L1 — "Zero per-stack code" means data, not branches.** Generalizing the
  falsification check for three stacks did not mean `if stack == "python"`; it
  meant moving the go-specific focusing/parsing into an **ecosystem-table
  strategy entry** (a data-shaped dispatch, even though the entry points at
  code) and leaving the check stack-blind. An inspection test mechanizes the
  claim by asserting no harness file names a per-stack literal.

- **L2 — A new capability that *degrades gracefully* is safe to add
  incrementally.** Rung 2 **skips** (`skip:parser-absent`) for a stack with no
  failure-identity parser. That single decision let rung 2 land without
  touching the just-merged python/node matrix — they skip it, go evaluates it.
  Prefer "skip when you can't" over "run and fail" for a capability not every
  subject supports.

- **L3 — A self-comparison fixture carries a maintenance tax.** The dry-run
  parity test compares a live run against a committed **reference verdict**.
  Adding rung 2 changed the ladder's shape, so the reference (captured at
  rungs 0/1/3/4) no longer matched and the advisory job went red. The reference
  must be **re-captured whenever a rung or check is added** — a cost
  proportional to how often the ladder changes.

- **L4 — Audit against `origin/main`, never a shared working clone.** A
  read-only exploration mapped the shared main clone, which was ~30 commits
  behind `origin/main`; its "this file does not exist" conclusions were wrong
  for the actual tip (the file had merged since). Fresh checkout, or query
  `origin/main:<path>` directly.

---

## 4. Recommendations for implement-now (summary)

1. **Token scope** — give the engine `workflow` scope, or detect the rejection
   and raise a typed elevated-scope escalation (F1).
2. **Gate fidelity** — mirror CI's platform matrix; treat platform-guarded code
   as a single-host blind spot and gate convergence on a matching CI leg (F2).
3. **Self-heal routing** — classify CI reds as reproducible vs
   unreproducible-locally; fail closed + escalate fast on the latter instead of
   spending retries on unvalidatable fixes (F3).
4. **Charter-completeness** — a completeness critic that checks acceptance
   criteria are delivered (not just that shipped tests pass), and flags code no
   test exercises (F4).
5. **Tracker truth** — completion derives from the merged artifact, never a
   local self-report (F6 / #490).
6. **Backstop message** — capture the agent's intended commit message, not a
   generic marker, so salvages need no history-reshaping (F5).

## 5. Cross-references

- Live defect: [#490](https://github.com/agenticlab-ai/pyxis/issues/490) (F6).
- Recurring principles worth graduating into the registry /
  `docs/observability/incidents.md`: **P6** verified-done ≠ works (F4), **P1**
  one predicate one place + **P3** verify-by-artifact (F6), the
  bounded-then-fail-closed self-heal discipline (F3).
- Ships-green ≠ works is the codegate epic's own thesis — F4 is the harness
  failing to verify itself before it existed to do so.
