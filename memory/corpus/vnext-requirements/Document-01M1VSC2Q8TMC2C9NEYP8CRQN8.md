# PRD — vNext Requirements (beyond the harbormaster baseline)

**Status: draft v0.1 — operator-fed requirements (2026-07-01), reviewed and
accepted with noted reshapings · NOT part of the as-built baseline.**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). These are the rewrite's *new-ground*
requirements at product altitude; each block graduates into one or more
per-feature PRDs via pipeline stage 1 before implementation. IDs are stable
and citable. Vision context: [product-vision.md](../product-vision.md) §6/§11.

---

## VNEXT-1 — Contract compilation & quality gate (pipeline stage 6)

**What:** Pyxis must not accept every PRD/design/DoD as executable input.
Before planning, every feature contract is **analyzed, classified, and
compiled** — bad contracts produce bad autonomous work, so validating the
contract itself is a load-bearing capability, arguably the most important in
the product.

**Requirements:**
- Classify every contract: **executable / ambiguous / incomplete /
  contradictory / unverifiable**. Anything but executable bounces back with
  named defects; it never enters planning.
- Detect at minimum: missing acceptance criteria; vague requirements ("make it
  better", "fast", "intuitive"); contradictions between PRD and design;
  requirements with no verification path; requirements implying unapproved
  product decisions; hidden dependencies on external systems; missing rollback
  or migration plan; missing observability requirements; missing edge cases.
- Compile an executable contract producing: a **requirement graph**, a
  **verification graph** (requirement → how it will be proven), a **risk
  assessment**, a **missing-information report**, an **ambiguity report**,
  **suggested tests/evals**, **required human decisions**, an **estimated
  cost**, and an **implementation-readiness score**.
- The compiler's outputs are structured data (AP-6) consumed by planning
  (stage 7), verification (stage 10), and the drift sentinel (VNEXT-4).

**Reshaping note:** the operator's "contract quality gate" and "contract
compile" items are one stage — the gate is the compiler's verdict.

## VNEXT-2 — Structured definition-of-done schema

**What:** "Done is machine-checkable" stays philosophy until the DoD has a
required, structured format.

**Requirements:** every acceptance criterion carries: requirement ID; expected
behavior; verification method (test / eval / manual-judgment) with an
executable location/command where applicable; owner of judgment; risk level;
required evidence; and whether it is machine-verifiable or human-verifiable.

```yaml
acceptance_criteria:
  - id: AC-3
    requirement: Existing users can still log in with password after magic-link login is added.
    verification:
      type: e2e_test
      command: npm run test:e2e -- auth/password-login
    required_for_merge: true
```

- The schema is closed (unknown keys are errors, INV-32) and versioned.
- Human-verifiable criteria route to the right human (owner of judgment) as
  explicit review items; machine-verifiable ones become gates the factory runs.
- The evals half (for LLM-runtime products) uses the same schema with
  `type: eval`.

## VNEXT-3 — End-to-end traceability

**What:** Every generated change is traceable across the full chain — first-class
and queryable, not implied. Agent-written code demands explainability. This is
the **substrate** for the drift sentinel (VNEXT-4), evidence packages
(VNEXT-5), and routing scorecards (VNEXT-7) — build it first.

**Requirements:**
- The chain: raw idea → PRD requirement → design decision → task → code diff →
  test/eval → proposal → merge/release → incident/learning (if any).
- Pyxis can answer: *why was this file changed? which requirement caused this
  change? which test proves this requirement works? which design decision
  authorized this implementation? which policy allowed this PR to merge? which
  previous incident influenced this guardrail?*
- Every component action carries trace context (AP-5); links are IDs into the
  immutable document layer (AP-7 makes them permanently resolvable).

## VNEXT-4 — Conformance & drift sentinel

**What:** Detect when **reality diverges from documents or policy**. Drift
detection is central to a trustworthy factory.

**Requirements — detect at minimum:**
- Approved design says one architecture; code implements another.
- Living rollup says a feature exists; tests show it does not.
- PRD says a requirement is supported; no test covers it.
- Repo policy says agents cannot modify a path; a proposal does.
- An incident lesson mandates extra checks for a change class (e.g. auth); a
  proposal in that class skipped them.
- Findings are filed as work items with the evidence chain attached; policy
  violations block at the gate (fail closed), documentation drift routes to
  reconciliation, conformance drift escalates per risk tier.

**Relationship to baseline:** extends (never replaces) the shipped mechanics —
the coded drift signal, living-docs reconcile, doctor/manifest drift, and
incident registry. The new ground is *semantic conformance* (design↔code,
claims↔tests, lessons↔behavior), which requires VNEXT-3's traceability and
LLM judgment at explicit judgment points; runs continuously via pipeline
stage 11 and at gate time for policy classes.

## VNEXT-5 — Evidence packages on every proposal

**What:** Every factory-produced proposal includes a structured **evidence
package** — its own review kit. This makes review easier and builds the trust
that lets teams raise autonomy levels (§6 of the vision).

**Requirements — package contents:** requirements addressed (by ID); design
decisions followed (by anchor); files changed; tests added; tests run (with
results); evals run (with results); screenshots/recordings where relevant;
known limitations; risk tier; rollback plan; cost used; agent/backend used;
human decisions required.

**Reshaping note:** contents **scale with risk tier** — a docs-only change
carries a minimal package; an auth change carries the full kit including
rollback. A fixed maximal package on every change would violate the
token/size-consciousness constraint (AP-6) and drown reviewers in ceremony.

**Relationship to baseline:** generalizes the shipped `pass_report`
(artifact-verified success) and spend attribution from orchestrator-internal
records into a per-proposal, reviewer-facing contract.

## VNEXT-6 — Repo intelligence (repo memory)

**What:** Before Pyxis safely modifies a repo, it needs a **living,
execution-verified model** of that repo — architecture, commands, conventions,
ownership, risks, dependencies, and historical failures. Without repo memory,
every agent starts partially blind.

**Requirements — the model knows:** how to build, test, lint, and run the app;
which tests are flaky; which modules are high-risk; which files are generated;
which areas require human review; which deployment paths exist; which
architectural boundaries must not be crossed; which past incidents relate to
current code; which local commands are safe or unsafe.

**Reshaping notes:**
- The model is **descriptive memory, never governing policy** — where it
  disagrees with human-authored policy (`pyxis-config/`, org policy), policy
  wins (the governance-is-never-derived-from-the-governed invariant).
- Claims must be **verified by execution** (the build command actually builds;
  the flaky-test list comes from observed runs), and the model is maintained
  by stage-11 maintenance + the drift sentinel — otherwise it rots like any
  wiki (the predecessor's "README that lied for a week").

## VNEXT-7 — Backend scorecards & routing intelligence

**What:** Since Pyxis supports multiple CLIs/models, it should **learn which
backends are good at which tasks** — CLI plurality becomes agent-routing
intelligence, not just adapter flexibility.

**Requirements — track per backend × task type:** success rate; cost; time to
completion; regression rate; review-rejection rate; test-failure rate;
hallucinated-file rate; prompt-following score; security/policy violation
rate.

**Requirements — route on it:** cheap model for simple doc updates; stronger
model for architecture changes; multi-agent review for risky code
(agent A implements, agent B critiques); fall back to another CLI when one
fails.

**Reshaping notes:**
- Routing decisions remain **judged once and frozen as data** per task (the
  inherited judge-once invariant): learning updates the routing *policy*;
  dispatch reads the frozen decision — no per-call improvisation.
- Scorecard-driven changes need statistical guardrails: minimum sample sizes
  before a routing change (reuse the risk-calibration graduation pattern) and
  deliberate exploration, or a backend that never receives work can never
  redeem its score.

**Relationship to baseline:** extends shipped rungs + routing-as-data
(IMPL-6/7) and absorbs the predecessor's model-routing backlog (#1003).

## VNEXT-8 — Autonomy levels

**What:** Explicit autonomy modes, so adoption can start conservative and earn
its way up — not every team trusts the factory equally on day one.

**Requirements:**
- The ladder: **L0** draft documents only → **L1** implementation plans →
  **L2** branches + proposals, never merge → **L3** auto-merge low-risk after
  gates → **L4** auto-release low-risk → **L5** auto-remediate known
  production issues.
- The level is **per repo and composable with risk tiers** (L3 for docs while
  auth stays L2) — a matrix, not a scalar.
- Defaults are conservative; raising a level is an explicit human policy
  change (human-lane, VNEXT-10); every level's behavior is enforced by the
  policy engine, not by convention.

**Relationship to baseline:** generalizes the shipped shadow-mode / risk-lane
rollout thinking into a product-level dial.

## VNEXT-9 — Expanded risk model

**What:** Every feature, task, proposal, and file change carries a risk tier —
and risk drives far more than the merge lane.

**Requirements:**
- Risk affects: which agent/backend may work on it (VNEXT-7); whether human
  review is required; which tests must run; whether auto-merge is allowed
  (VNEXT-8); whether a rollback plan is required (VNEXT-5); whether additional
  security checks run; the worker's isolation profile (VNEXT-11).
- Risk factors include at minimum: auth changes; payments; data migrations;
  permissions; security-sensitive code; infrastructure; production config;
  user-data handling; large refactors; public API changes; low-test-coverage
  areas.
- Factors and thresholds are versioned policy data (closed schema), with the
  deterministic-floor + downgrade-only + self-unlock invariants inherited from
  the baseline's risk-tiered approval (GATE-4).

## VNEXT-10 — Unified policy engine

**What:** Governance exists in the baseline but scattered (org exclusions,
repo knobs, risk policy, guard denylists, credential scopes). Pyxis needs
**one declarative policy model** — expressed as repo/org policy files.

**Requirements — policies define:** who can approve contracts; who can approve
high-risk changes; which files agents cannot modify; which commands agents
cannot run; which changes can auto-merge; which secrets are available to which
workers; which tests are mandatory; which external services may be called;
which repos can use which CLIs/models; which telemetry is allowed; which
updates are allowed.

- Closed schemas, versioned, validated on load (INV-32); org policy > repo
  policy > defaults, with the tiering explicit.
- Policy changes are always human-lane (the self-unlock guard, generalized).
- Every enforcement point (gates, guards, routing, merge, telemetry, updater)
  reads the same policy model — one predicate, one place.

## VNEXT-11 — Worker isolation & permissioning contract

**What:** "Agent workers are isolated" is not enough — **the exact boundary is
the product.**

**Requirements — Pyxis defines, per risk tier, and enforces:** filesystem
isolation; network isolation; secret access; git permissions; runtime limits;
token/cost limits; command allowlist/denylist; container lifecycle; log
redaction; artifact capture; cleanup guarantees; recovery after crash.

**Reshaping note:** the *contract* is engine-level and declarative (policy
data); *enforcement* is per runtime adapter (container, sandbox, CLI hooks) —
an adapter declares which isolation guarantees it can honor, and the policy
engine refuses to route work whose required profile the adapter cannot meet.

## VNEXT-12 — Pipeline-wide failure recovery

**What:** Autonomous work fails often; the factory must stop and resume
without losing its mind — across the *whole pipeline*, not only
implementation.

**Requirements:** worker crash recovery; stuck-task detection; retry policy;
partial-work preservation; branch cleanup; rebase-conflict handling;
failed-test diagnosis; automatic rollback of bad attempts; escalation with
useful context (VNEXT-13); no duplicated work after restart; **idempotent
stage execution** for every pipeline stage.

**Reshaping note — mostly inherited:** the baseline already delivers the core
of this list and it is the predecessor's strongest muscle (WIP checkpointing +
resume, dual-TTL lease reaping, fencing, bounded ci-fix/conflict/feedback
loops, idempotent-and-raceable everything — IMPL-3/5/9, GATE-6). The vNext
requirement is to **preserve those invariants and generalize them to all
twelve stages** (a crashed contract-compilation or e2e-verification resumes
the same way implementation does), plus the genuinely new pieces:
failed-test *diagnosis* and automatic rollback of bad attempts.

## VNEXT-13 — Human escalation protocol

**What:** The factory escalates only when judgment is required — and every
escalation must be worth a human's attention. The goal is not just fewer
escalations; it is **high-quality escalations**.

**Requirements — every escalation contains:** what happened; why the factory
is blocked; what decision is needed; recommended options; risks of each
option; relevant logs/diffs/tests (via VNEXT-3 traceability); how to unblock;
and whether this is a **factory gap** or **required governance**.

- The gap-vs-governance self-classification feeds the interventions-per-feature
  cause taxonomy (vision §10) — escalations are the raw data of the inversion
  test.
- Escalations are structured data (AP-6) delivered through the WorkTracker
  port (operator items), never free-form log dumps.

---

*Sequencing note (advisory, for the design phase): VNEXT-3 (traceability) and
VNEXT-2 (DoD schema) are substrates — VNEXT-1, 4, 5, and 7 consume them.
VNEXT-10 (policy engine) underlies 8, 9, and 11.*
