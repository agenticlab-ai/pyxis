# PRD — Scaffolding & Onboarding

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Feature IDs are stable and citable.
Northstar category: C11 (portfolio & substrate). Pipeline: upstream of stage 1
(a project must be factory-managed before the pipeline can run in it).

## Scope

Bringing repositories under factory management: creating new projects from
nothing, adopting existing ones, and applying the managed baseline (CI, labels,
protection, templates) idempotently. Users: the **operator** (org-level
actions) and **engineers** (new projects).

## Features

### SCAF-1 — Project scaffolding wizard
**What:** Stand up a complete, factory-managed repository from nothing in one
run (`/scaffold-project`).
**Requirements (as built):**
- Four-question wizard: language, project name, visibility, ships-an-LLM-runtime?; every answer also available as a non-interactive flag.
- One run (< 60s): create repo → clone → render the language profile with substitutions (the LLM-runtime answer conditionally renders the observability/session-id contract) → initial commit/push → bootstrap (CI + labels + protection) → open a "write your first design doc" welcome issue.
- Idempotent: re-running a half-finished scaffold applies only missing/drifted pieces. `--dry-run` previews everything.
- Copy-paste-fixable errors for common failures (no create permission, unknown language, non-admin protection phase).
- Wires the factory bot's credentials into the new repo (App installation + repo/org variables) so automation works from the first issue.
**Origin:** development-factory.md §5–6; scaffold-project.sh; scaffold-credential-wiring.md.

### SCAF-2 — Repo bootstrap (adopt an existing repo)
**What:** Make any existing org repo factory-managed by applying the repo-level
baseline from the language profile.
**Requirements (as built):**
- Two phases split along the permission boundary: **template-push** (write access: lands `.github/` tree + labels, via PR by default or `--direct` push) and **apply-protection** (admin: full-replacement branch-protection update). Single-shot wrapper runs both; a non-admin gets phase 1 + a clear hand-off.
- `--update` re-applies idempotently (only drifted files land); labels upserted, never duplicated.
- **Per-repo required-checks calibration**: the required CI contexts are derived from what the repo actually runs (flag or repo-config override), never a forced template superset; merge gating consumes the same derived set.
**Origin:** development-factory.md §4; bootstrap-repo.sh; required-checks calibration (#344).

### SCAF-3 — Language profiles
**What:** Opinionated, versioned stack descriptors that scaffold/bootstrap render.
**Requirements (as built):**
- Python profile: pinned toolchain (uv, ruff, pytest, pyright), src-layout, full CI workflow suite, release automation (release-please), CODEOWNERS, issue/PR templates, canonical label set.
- Substitution variables ({{package_name}}, {{repo_name}}, {{org}}, {{owner_handle}}, {{bot_app_slug}}, {{include_observability}}).
- Profiles are data: adding a language = adding a profile directory, no engine change.
**Origin:** templates/<language>/ (T12); development-factory.md §6.

### SCAF-4 — Org adoption sweep
**What:** One-shot, operator-approved classification of every repo in the org
(`/org-adopt`).
**Requirements (as built):**
- Enumerate all org repos; classify each: managed / adopt / archive / delete / excluded; present the full plan; act only on explicit approval (bootstrap adopted, archive stale, file operator issues for deletes).
- Never writes to excluded repos. `--dry-run` is the safe default.
**Origin:** org-adopt skill; org-policy.toml.

### SCAF-5 — Welcome path
**What:** A new or adopted repo starts the pipeline by itself.
**Requirements (as built):**
- Scaffold ends with a welcome issue prompting the first design doc; the engineer's path to a productive project is *install + write a design doc* — no wiki crawl.
**Origin:** factory-northstar.md §1; scaffold-project.sh.
