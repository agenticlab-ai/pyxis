# PRD — Identity & Security

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Northstar category: C9 (identity & trust).
A plane across every pipeline stage.

## Scope

Who may do what, and how we know who did: the two-identity model, credential
lifecycle, action guards, and secret hygiene. Users: the **factory** (acts as
bot), **engineers** (act as themselves), the **operator** (trust decisions).

## Features

### IDEN-1 — Two-identity model
**What:** Fleet actions and human actions are never confusable.
**Requirements (as built):**
- The fleet acts as a dedicated **bot identity** (a GitHub App): claims, commits, proposals, and merges by the factory are attributed to the bot, so history distinguishes machine work from human work.
- Engineers act as **themselves** on their own machines; fleet code never uses a personal token.
- AUTO-lane review approvals come from a **separate reviewer bot identity** — approval provenance is never mixed with either the worker bot or the operator (see GATE-4).
**Origin:** fleet-identity-separation.md; risk-tiered-pr-approval.md §4.

### IDEN-2 — Scoped, short-lived credentials
**What:** Least privilege, minted on demand, impossible to stale-out the fleet.
**Requirements (as built):**
- Tokens are minted inline per operation class — read / triage / work / merge-sweep scopes — with ~1-hour lifetimes; nothing is persisted into any shared auth store (a persisted expiring token once took down the whole pool — never again).
- Git pushes authenticate through a per-clone credential helper that mints fresh tokens transparently (short cache) and self-heals a dangling helper path.
- The signing key is placed once per node, mode-restricted, held on the state volume; rotation is an operator action.
**Origin:** mint-app-token.sh; git-credential-factory-app; the #201 regression lesson.

### IDEN-3 — Action guards (the factory polices itself)
**What:** Unsafe actions are intercepted before they execute, uniformly across CLIs.
**Requirements (as built):**
- Pre-execution guards block: known-broken invocations, auth-store mutations (`gh auth login`/`switch`), pushes by a fenced (lease-lost) worker, any mutation outside the worker's own workspace, and direct proposal-creation that bypasses the quality gate.
- One canonical denylist; per-CLI shims translate it to each agentic CLI's hook system; hooks **fail closed** (a CLI whose hooks fail open gets an explicit failClosed guard).
- A turn-threshold guard enforces the worker checkpoint-and-exit protocol mechanically.
**Origin:** hook-registry.md; bash-guard.sh; gh-pr-guard.sh; worker-turn-guard; cursor failClosed guard.

### IDEN-4 — Secret hygiene
**What:** Secrets stay out of code, images, and logs.
**Requirements (as built):**
- Secret scanning (gitleaks) runs in the local gate and CI on every change.
- Credentials are collected once by a setup wizard into a **cross-platform secure store** (macOS Keychain / Linux secret-service / Windows Credential Manager / encrypted-file fallback), with validity probes at entry and a rotation verb; runtime copies are mode-600 on the node's state volume; container images never bake secrets (runtime injection only).
- Prompt/history logs are redacted at capture time and never git-tracked (enforced as a data invariant).
**Origin:** node-setup-wizard.md; factory-governance.md §7.
