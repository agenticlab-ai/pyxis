# PRD — Quality Gates & Merge Automation

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Northstar categories: C4 (integration), C5
(verification). Pipeline stages 8–9 (the merge half) and the safety net under
everything.

## Scope

How a change becomes merged code safely and unattended: local gates before a
proposal opens, risk-tiered approval, authorized auto-merge, and bounded
recovery from every red state. Users: the **factory** (workers/sweeps),
**engineers** (same gates on their own changes), the **operator** (HUMAN-lane
approvals).

## Features

### GATE-1 — Change finalization
**What:** The required end-of-change protocol (`/finalize-change`).
**Requirements (as built):**
- Pre-flight gate → docs-first step (design doc + operational docs; living-docs reconcile handoff when the coded drift signal fires) → conventional-commit on the **current** branch (never auto-create/switch branches) → hand off to proposal raising.
**Origin:** finalize-change skill; living-docs-layer.md §7.

### GATE-2 — Pre-proposal quality gate (local CI mirror)
**What:** A proposal can only open on a green local gate (`/raise-pr` + gate script).
**Requirements (as built):**
- Mirrors the repo's required CI exactly: secret scan (gitleaks), parse/lint/test mirrors auto-detected per subproject, branch-name + referenced-issue check, proposal metadata (conventional-commit title, closes-reference), design-doc-first for features, foreign-lease guard.
- Checks are diff-scoped where possible; blocking vs non-blocking classes; SKIP with reason when a precondition is absent.
- A green run mints a short-lived revision-pinned pass token (fast path ~30 min); a guard hook re-runs the gate on any direct proposal-create attempt, so the gate cannot be bypassed.
**Origin:** pre-pr-gate.sh; gh-pr-guard.sh; raise-pr command.

### GATE-3 — Design-doc-first gate
**What:** Every feature-typed change is design-backed.
**Requirements (as built):**
- A feat-typed proposal must touch a design doc or declare `Design: <path>` (which must exist at HEAD); enforced both locally and in CI. Documented opt-out (flag + label) for rare exceptions.
**Origin:** design-doc-gate.md.

### GATE-4 — Risk-tiered merge approval (two lanes)
**What:** Replace all-or-nothing human review with a calibrated AUTO/HUMAN split.
**Requirements (as built):**
- **Stage A — deterministic floor** (data, not judgment): hard-gate path globs (control plane), self-unlock guard (changes to the risk policy/classifier are always HUMAN), foundation floor (DAG common-root, core paths, new contracts, high in-degree), provenance gates. Any hit → HUMAN, non-negotiable.
- **Stage B — narrow auto-candidates:** only safe change classes (docs-only, comment-only, test-addition, pure addition, rename-only, whitespace) on non-control-plane paths.
- **Stage C — intelligence, downgrade-only:** an LLM reviewer may demote a candidate to HUMAN, never promote; ambiguity/timeout/error → HUMAN (fail closed).
- AUTO approvals are submitted by a dedicated **reviewer bot identity** at the exact head revision — history distinguishes human-vetted from bot-cleared merges.
- Risk policy is versioned **data** with a closed schema, a validator, and a kill switch; a **calibration loop** measures predicted-vs-realized risk after a maturation window and *proposes* floor changes (graduation is propose-and-confirm, never auto-applied).
- Rollout is shadow-mode first (classify + log, don't act) before enforcement, with agreement-rate measurement and earned-data AUTO-tier promotion.
- **Baseline status:** classifier + policy data, supervisor lanes (approve-on-green, risk events, foundation stamp), maturation janitor, `/risk-calibration`, and shadow-mode measurement shipped; **platform enforcement** (reaper reads the lane; CODEOWNERS path-guard on control-plane paths) **and end-to-end validation still open** at capture time.
**Origin:** risk-tiered-pr-approval.md (#806); plan #1031 (epics #1032/#1033 closed, #1034/#1035 open); risk-policy.yaml; classify-pr.sh.

### GATE-5 — Merge automation
**What:** Green, authorized proposals merge themselves.
**Requirements (as built):**
- Qualification: the closing task carries the explicit merge-authorization signal ∧ checks green ∧ no hold ∧ risk lane satisfied (AUTO: bot-approved; HUMAN: human approval present).
- Squash-merge + branch delete; **idempotent and raceable** — two sweeps racing produce one merge + one recorded no-op, never an error.
- Guardrails: empty-diff proposals are refused and labeled; design-doc proposals are human-merge-only; epics auto-close when all members close.
- Two redundant paths (in-node sweep + hosted cron backstop) so a down node never blocks merges.
- Review-gated repos are honored: where protection requires a human approval (admin-enforced, no bypass), the sweep merges as the bot only once approval + green checks are present — and logs "awaiting review" distinctly instead of erroring.
**Origin:** script-gated-supervisor.md §2; merge-ready-reaper; risk-tiered-pr-approval.md; human-review gating (#720/#722).

### GATE-6 — Bounded recovery loops
**What:** Every red state has an automated, capped recovery path that ends in
green or a human.
**Requirements (as built):**
- **CI-fix:** a settled-red proposal is re-engaged with the failing logs, up to N attempts, then flagged worker-failed.
- **Merge-conflict:** reconcile rounds up to a cap, then flagged.
- **Stuck-proposal lifecycle:** settled-red proposals are flagged; recovered CI unflags automatically.
- **Review-feedback loop:** pending human review comments dispatch a worker that addresses each thread with a reply (applied/rejected/out-of-scope), bounded rounds, then escalates as feedback-stalled.
- Every cap escalates to a human with the history attached; nothing loops forever, nothing dies silently.
- Known open limitation at capture time: shared-file conflicts that require the recovery worker to merge main can still fail to converge (open incident harbormaster#1001).
**Origin:** ci-failure-self-heal.md; stuck-pr-recovery.md; pr-review-feedback-loop.md.

### GATE-7 — Required-checks calibration
**What:** Merge gating matches what each repo actually runs.
**Requirements (as built):**
- The required-context set is derived per repo (flag or repo config) at bootstrap; branch protection, the merge sweeps, and the local gate all consume the same derived set (one predicate, one place).
**Origin:** required-checks calibration (#344).
