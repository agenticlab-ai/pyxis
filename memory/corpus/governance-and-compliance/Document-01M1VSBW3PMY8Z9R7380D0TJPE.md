# PRD — Governance & Compliance

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Northstar category: C10 (governance). A
plane across every stage: the factory keeps *itself* true.

## Scope

The factory's primitives must stay present, unmodified, and customized only
through sanctioned paths — across every managed repo, org tier, and machine.
Users: the **operator** (policy), the **factory** (enforcement).

## Features

### GOV-1 — Primitive manifest
**What:** The machine-decidable registry of everything the factory manages.
**Requirements (as built):**
- Every managed primitive is declared with an enforcement class: **L** linked (reserved-name shared assets, must resolve to the canonical source), **M** merged (required keys inside shared config files — asserted key-level, repo-local keys preserved), **D** data invariants (ignore rules, never-tracked paths, required living docs), **V** vendored (pinned, reviewed artifacts run locally, never fetch-and-exec).
- Primitives are declared per distribution tier (repo / org / user); the manifest is the single source of truth enforcement reconciles toward; "modified vs older vs local" is decidable only because the manifest exists.
**Origin:** factory-manifest.yaml; factory-governance.md.

### GOV-2 — Drift detection & repair (rig-doctor)
**What:** Verify — and where safe, repair — the primitive set everywhere.
**Requirements (as built):**
- Modes: check (report-only), fix (idempotent repair), dry-run (zero writes).
- **Mechanical** findings are auto-fixed: re-link, key-level JSON merge, append missing line, drop missing skeleton. **Non-mechanical** findings escalate to the operator: a real file shadowing a reserved name, a never-tracked path that got tracked, a stub living doc. Destructive repairs never auto-run.
- Findings file as one deduped compliance issue per repo (marker-gated, auto-closed on the next compliant pass); compliance issues are routed to humans, never back into the autonomous pool (self-deadlock guard).
**Origin:** rig-doctor.sh; factory-governance.md.

### GOV-3 — Org policy
**What:** Org-level rules every node obeys.
**Requirements (as built):**
- An exclusion list names repos the factory must never touch — zero writes of any kind (no labels, comments, PRs, CI, archives); synced to every node automatically; honored by every write path including adoption and doctor.
**Origin:** org-policy.toml.

### GOV-4 — Sanctioned per-repo customization
**What:** Repos can adapt the factory without forking it.
**Requirements (as built):**
- A per-repo config file with a **closed schema** (unknown key = hard error, named in the error), knobs tiered **self** (repo may set freely) vs **operator** (needs approval); validated on every apply.
- Repo-local extensions are real files under non-reserved names, living beside the managed (linked) primitives; the doctor verifies reserved names and ignores sanctioned extensions.
- *(Pyxis extends this into the `pyxis-config/` directory: config + custom prompts per action/skill — see product-vision.md.)*
**Origin:** .rig-config.yaml + schema + validator; factory-governance.md §8.

### GOV-5 — Control-signal taxonomy
**What:** Workflow state markers are a governed vocabulary, not decoration.
**Requirements (as built):**
- A canonical signal set with defined semantics: routing (pool vs operator vs needs-plan), **merge authorization** (the signal that alone permits auto-merge), status/recovery (worker-failed, merge-conflict, verify-pending, hold, feedback-stalled), classification (kind/area/incident/risk-lane), structural (epic/plan/pinned).
- Provisioned identically to every repo at bootstrap; upserts never duplicate.
**Origin:** templates labels.yaml; CLAUDE.md "labels are control signals".

### GOV-6 — Governance meta-rules
**What:** Rules about the rules — the invariants that keep governance trustworthy.
**Requirements (as built):**
- **Governing artifacts are human-authored inputs, never derived from the governed system** (a manifest is written by people; enforcement reconciles reality toward it, never regenerates it from reality).
- Shared-content files are merged **key-level**, never byte-replaced (factory hooks and repo-local settings coexist).
- Changes to governance data (risk policy, manifests) are always human-lane (self-unlock guard, GATE-4).
**Origin:** factory-governance.md; governance-not-rebuilt-from-governed principle.
