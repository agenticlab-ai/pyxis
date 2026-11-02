# Idea — Repo-neutral governance: the `.pyxis/` contract, ephemeral per-CLI projection, and capability resolution

**Status: idea / draft for the rewrite · 2026-07-10 · Owner @mdarabi.**
Provenance: distilled from a design conversation that grew out of a harbormaster
experiment — serving the pyxis repo's own issues autonomously (issue → PR in
pyxis) — whose adoption step scattered ~40 rig files across pyxis's `.claude/`
and violated pyxis's "one folder to rule it all" (`.pyxis/`) intent. This idea
captures *how a repo should be governed by the rig without being polluted by it,
independent of which agentic CLI does the implementation.* It is pre-PRD: a
settled set of principles and open design knots, not yet a contract.

## 1. The problem this solves

The rig (harbormaster today, **pyxis** as the rewrite) implements issues in
arbitrary org repos by launching an agentic CLI worker inside a worktree. For
that worker to carry the rig's guardrails and skills, the first instinct was to
**inject the rig's config into the repo** — an "adoption" step that symlinked the
rig's skills, scripts, hooks, and a `settings.json` into the target repo's
`.claude/` directory.

On pyxis this produced ~40 entries under `.claude/` (all 20 rig skills including
supervisor-only ones a worker never uses, 15 scripts, `settings.json`,
`commands/`), converting pyxis's clean two-symlink `.claude/` alias into a sprawl.
Three things were wrong with it, and they generalize:

1. **It is CLI-specific.** `.claude/` binds to Claude Code. The rig must run
   across Claude Code, Codex, Cursor, opencode, Aider, and whatever comes next.
   Any solution expressed in one CLI's config format is the wrong layer.
2. **It commits rig metadata into the repo**, outside the repo's own convention
   surface (`.pyxis/`), fighting the repo's consolidation and creating a drift
   surface in every served repo.
3. **It can override what the user built.** If the repo owner already has, say, a
   Claude Code code-review plugin, injecting the rig's own review *takes over*
   something they own.

The rest of this document is the model we converged on to avoid all three.

## 2. Two interfaces, kept strictly separate

All the confusion above came from crossing two boundaries that must stay apart:

- **Repo ↔ rig — this is `.pyxis/`, and it is tool-neutral.** The repo puts there
  whatever it wants the rig to know for implementation: config, repo-specific
  procedures, templates, its own hooks, a manifest, capability choices. One
  folder, the repo's single source of truth, readable by *any* implementer
  regardless of CLI. A repo with nothing rig-specific carries nothing extra.
- **Rig ↔ agentic CLI — this is a runtime adapter, and its output is ephemeral.**
  Whatever `.claude/`, `.cursor/`, `AGENTS.md`, or opencode config a given CLI
  needs is **generated at worker launch into the ephemeral worktree**
  (git-ignored, discarded with the worktree after the PR) — never committed. The
  adapter is the *only* place CLI-specificity is allowed to live, because it is
  per-CLI by nature and its artifacts never touch the repo's committed tree.

Committing `.claude/` into the repo violated both at once: CLI-specific **and**
committed metadata outside `.pyxis/`.

## 3. Core principles

- **The rig is a guest, not an owner.** It behaves like a guest in the user's
  repo and in the user's CLI setup. It offers; the repo decides; the rig honors.
- **The user owns their agent config.** `.claude/`, `.codex/`, `.cursor/`, their
  plugins, their hooks, their code-review — all authoritative, none of the rig's
  business. The rig never touches, shadows, or replaces it.
- **`.pyxis/` is the repo's opt-in channel**, tool-neutral, versioned. It is the
  stable contract between repo and rig; every per-CLI adapter reads from it.
- **CLI-specific config is ephemeral and invisible.** Generated into the worktree
  at launch, added to `.git/info/exclude` (local, uncommitted) so it never
  surfaces in the diff, and discarded with the worktree. The empty-diff / charter
  gates must ignore it.
- **Enforcement lives in the harness, around the session — not injected into the
  CLI session.** The rig's own opinions (tiered review, risk approval, CI-pass,
  claim discipline, artifact-verify) act on the *proposed PR*, from outside the
  worker's session. This is inherently CLI-agnostic and needs zero repo files. It
  also resolves the code-review conflict cleanly: the rig's review is a PR-stage
  concern, so the user's in-session review plugin runs untouched and the rig never
  competes with it in-session.

## 4. Capability negotiation — how collision is dissolved

The rig does not *decide* what runs; it **offers a menu, and the repo picks,
declaratively, ahead of time**. For each capability the rig can provide
(code-review, PR-gate, commit-conventions, doc-reconcile, test-gate, …), the
resolution is settled at setup and recorded in `.pyxis/`. At runtime the harness
reads that resolution and knows exactly what to run and what to stay out of — no
runtime discovery, no clobber, no surprise.

### 4.1 Resolution precedence (which single implementation runs)

Highest precedence to lowest, **first present wins**:

1. **User's own native config** — the thing they built for themselves (their
   `.claude/` plugin, their hook). If present, it wins.
2. **`.pyxis/` declaration** — the repo speaking explicitly to the rig in the
   neutral surface. If present, next.
3. **Rig default** — the always-there fallback.

> Open point to confirm: tiers 1 and 2 are treated as *distinct* layers here —
> the user's own native tooling ranks above the repo's explicit-to-rig `.pyxis/`
> declaration. The alternative reading is a single "the repo provided something"
> tier above the rig default. The ordering between "what they built natively" and
> "what they wrote in `.pyxis/`" is the one bit still to lock.

### 4.2 Rig-as-floor: batteries included, opt-out not opt-in

The rig is the **floor, not the opt-in**. A repo that says nothing is still fully,
competently implemented on rig defaults — the right stance for a system whose job
is to implement (a "do nothing unless invited" default would leave a zero-config
repo un-served). Consequences:

- **Silence = the rig default runs.** To get *nothing* for a capability, the repo
  must say `none` explicitly. The rig is present-by-default and steps aside only
  when told — a *helpful* guest: always overridable, never requiring an invite.

### 4.3 Composition: `both`, with the rig always last

Resolution picks one implementation. **Composition** is a separate axis: for the
*observing* capabilities, the repo can additionally run the rig's on top of its
own. The invariant is **the rig always goes last**:

- Going last means the user's stack runs against the state *it* assumes — the rig
  never mutates the ground under their feet — and the whole sequence stays legible
  in the PR. The rig is purely additive and terminal: "you did what you wanted;
  then we did ours on top," visibly.
- Ordering is therefore a **system rule, not user config**: the repo picks `both`
  where offered; "theirs-first, ours-last" is implied.

### 4.4 The gate-vs-mutate partition (what can even be `both`)

The last-place invariant tells you which capabilities can compose:

- **Observing / gating capabilities** (tiered review, risk approval, CI-pass
  check) read the final result and vote pass/block. Running last is not just safe
  — it is their natural home. These are **composable**; `both` is allowed.
- **Mutating capabilities that overlap the user's** (auto-format to a rig
  standard, auto-reconcile living docs) *change files*. Running last means the
  rig's edits land after the user's linter/formatter already ran — so they dodge
  the user's checks. That has no safe ordering, so these are **exclusive**:
  `none | theirs | ours`, never `both`.

Design rule that falls out: **a capability is offerable as `both` only if it is
observe-only at the point it composes.**

### 4.5 Failure semantics

Short-circuit gate chain: the user's runs; only if it passes does the rig's get to
run; if the rig's blocks, the PR is blocked **by the rig**, visibly. No ambiguity
about who stopped what.

### 4.6 The resulting grammar (per capability)

A small, complete grammar:

- **Resolve** down `user-native → .pyxis → rig-default` (first present wins).
- Optionally **compose** the rig's gate on top, terminal (`both`, rig-last) — for
  observing capabilities only.

Worked example (code review): the user has their own review → it *resolves* at
tier 1 and runs first; they picked `both`, so the rig's tiered review runs last;
both are visible in the PR.

## 5. Trust model

Because the rig runs the repo's (and the user's) committed config on its node,
executing repo-authored scripts, the trust question is real. The reasoning:

- **Self-hosted, single-trust-domain.** The node is the user's own container on
  their own infrastructure (harbormaster/pyxis runs self-contained in Docker). A
  bad repo script's blast radius is the user's own environment.
- **No new trust burden.** If the user would run these scripts on their laptop
  anyway, running them in their own container is the same trust surface — the
  malware/bad-script check is one they already own. The rig adds no work.
- **The precise boundary: blast radius is the *node*, not the *repo*.** A node
  that serves several repos holds their clones, their claims, and — the real
  escalation — **credentials with write access to those repos**. So repo A's bad
  `.pyxis/` script, on a node that also serves repo B, can reach B's checkout and
  the node's token. This stays "the user's own stuff" only while every served repo
  is *one trust domain* (your org, your box) — the intended case.
- **Compensating controls already exist** (from the harbormaster work): a
  **serve-allowlist** (only repos deliberately enrolled run on the node,
  deny-by-default) and an **enrollment audit** (the repo's `.pyxis`/config is
  reviewed before it is served). The thing to *not* do is auto-serve across trust
  boundaries; the allowlist is what forbids it.

## 6. What this means near-term (harbormaster) vs long-term (pyxis)

pyxis is the rewrite that replaces harbormaster, so most of this is **architecture
for pyxis**, not something to build into harbormaster. The split:

- **Actionable now in harbormaster:**
  - Stop the injection: close the pyxis adoption PR; a served repo should carry no
    rig files.
  - Serve foreign repos **harness-side**: a worker runs with the brief plus
    rig-side gates (claim, verify-shipped, CI-pass) — the tool-agnostic path even
    within harbormaster — and drops the `.claude/` injection requirement.
  - Retire (not merely demote) the repo-injection "adoption" machinery — it
    institutionalizes exactly the thing this idea rejects.
- **Architecture for the pyxis rewrite:** the whole model above — `.pyxis/` as the
  neutral source of truth, the ephemeral per-CLI projection adapter, harness-side
  enforcement, and the capability-resolution grammar.

## 7. Open design knots (the next real work)

1. **The capability schema.** Enumerate the menu the rig offers, choose the right
   *grain* (one "code-review" toggle vs. finer sub-capabilities), and tag each as
   **composable-gate** or **exclusive-mutator** (§4.4). This is the main remaining
   detail; the principles above are settled, the menu is not.
2. **Tier-1 vs tier-2 ordering** (§4.1): confirm user-native ranks above the
   `.pyxis/` declaration, or collapse them.
3. **Per-CLI projection adapters.** For each supported CLI, define what `.pyxis/`
   projects into and *where it is written*: a runtime location the CLI is pointed
   at (external settings/plugin dir — safe, no collision) vs. git-ignored files
   inside the worktree (needed when a CLI only reads the project dir — requires the
   additive-merge care so the user's committed config always wins).
4. **The `.pyxis/` contract + versioning.** The schema (capability resolutions,
   projections, manifest) is the stable repo↔rig interface; version it with a
   closed-key parse (as `.pyxis/config.yaml` already does) so unknown keys are
   defects, not silent drops.
5. **Ephemeral-invisibility mechanics.** Standardize the `.git/info/exclude`
   projection so generated CLI config never enters a diff and never trips the
   empty-diff / charter-consistency gates.

## 8. Principles honored

This idea is an early-stage capture and cites the repo's principles by intent
rather than a design's formal `## Principles` block:

- **Rewrite principles** — it is explicitly forward architecture for pyxis, framed
  against the harbormaster experiment it learned from.
- **The guest stance and the neutral `.pyxis/` surface** are the load-bearing new
  commitments; everything else (resolution, composition, trust) derives from them.

## References

- Harbormaster experiment that motivated this: serving `agenticlab-ai/pyxis#58`
  (issue → PR in pyxis) and the adoption PR (`agenticlab-ai/pyxis` #71) whose
  footprint exposed the problem.
- Compensating trust controls from harbormaster: the serve-allowlist and
  enrollment-audit work (harbormaster #1140), and the multi-repo execution design
  it built on (harbormaster #1125).
