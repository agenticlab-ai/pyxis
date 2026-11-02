# Implementation verification — brainstorm record

| Field | Value |
|---|---|
| **In one line** | What pyxis itself verifies on agent-written code before it merges — the layered check taxonomy (floor → tests → execution reality → anti-gaming → scope), bound technology-independently per repo (verbs/manifest, not commands), degrading honestly on brownfield repos that lack assumed blocks (baselines, capability gaps priced into autonomy). |
| **Relevant-when** | brainstorming or contracting stage-8/10 verification content or the definition of genuinely-green; the verification-verb/manifest binding or ecosystem facts table; pyxis-owned checks (falsification/tautology, test-delta audit, exec-probe, AC↔test traceability); brownfield onboarding assessment, grandfather/ratchet baselines, or autonomy priced by provable surface; checking prior rulings on verification scope vs implement-now / merge-lane / contract-compiler boundaries |
| **Idea** | "in pyxis we are adding the 'auto implementation phase' in which an issue (task) would be turned into a code. but a code without proper verification and testing is meaningless, think through all the things that should be tested and verified for a give piece of code in a project/repo" · "one more consideration before moving forward, whatever we are building should work indpendent of the underlying technology, meaning it is independent of programming language that used in the repo, repo setup, built system of the project, etc" · "we need to ensure our solution would work in the existing repositories which might not have all the building blocks that we might assume for. for example, if they do not have e2e test suites, how our system would respond" (operator, 2026-07-16, verbatim, three turns) |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-07-16 |
| **Status** | gated: ready-for-prd |
| **Tracking** | agenticlab-ai/pyxis#356 *(the introducing PR)* |
| **Feeds** | docs/prds/implementation-verification/ |
| **Prior records** | brainstorm:contract-compiler |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| docs/product-vision.md (§4, §6, §7) | the governing pipeline map — stage 8 produces proposals with evidence packages, stage 9 human review, stage 10 e2e verification; §6 trust model (evidence packages, traceability, autonomy × risk); §7 structured DoD with verification methods |
| docs/prds/README.md | baseline index — GATE-1..7 (quality/merge parity), IMPL-1..10 (implementation parity), SCAF-2/3 (adoption + language profiles as data); known-gaps list names stage-10 e2e and evals-as-primitive as unclaimed |
| docs/prds/vnext-requirements.md | the dense overlap neighborhood — VNEXT-1 (verification graph), VNEXT-2 (DoD schema w/ verification method + command), VNEXT-3 (which-test-proves-this traceability), VNEXT-4 (drift: claims↔tests, skipped mandated checks), VNEXT-5 (evidence packages), VNEXT-6 (repo intelligence: how to build/test/run; flaky lists), VNEXT-8/9/10 (autonomy, risk drives which-tests-run, policy: mandatory tests) |
| docs/prds/quality-and-merge.md + autonomous-implementation.md (headers) | GATE-2 local CI mirror ("mirrors the repo's required CI exactly", auto-detected per subproject), GATE-4 risk-tiered lanes, GATE-7 required-checks calibration; IMPL-2 artifact-verified success + hollow detection |
| docs/prds/implement-now/effective.md | the stage-8 native executor (PRD 2026-07-15, problem review pending): drives tasks to "genuinely-green" = PR exists + required checks green (M2); consumes a local gate + CI, does not define what checks should exist; NG-1 never merges; risk "green that isn't green" |
| docs/brainstorms/contract-compiler.md | stage-6 compiler (verification-graph producer) gated needs-investigation 2026-07-10 — this layer cannot assume it exists |
| docs/wiki/glossary.md | existing vocabulary to reuse: Verification methods (Test · Demo · Inspection · Analysis · Eval · Human judgment), DoD §5.8, floor-clearing, AUTO/HUMAN lane, protected path |
| .pyxis/pipeline.md | stage 8 already runs via the external fleet (harbormaster) — the native path is implement-now, in flight |

**Overlaps surfaced:**

- `implement-now` (PRD in problem review) — the stage-8 *driver*: claims a task, implements, drives to genuinely-green, never merges. It consumes "the local gate + required checks" as given; nothing in it defines what those checks are or should be. Complementary, with a boundary to draw: implement-now drives to green; this feature defines what green means and supplies the checks. Its top risk 3 ("green that isn't green") is exactly this feature's subject.
- `VNEXT-2` (structured DoD schema) — the *demand* side: every AC carries a verification method + executable command. Assumes the repo has a command — the brownfield gap. This feature is the *supply/execution* side: how any method is bound, run, and honestly degraded on an arbitrary repo. Adjacent, not covering.
- `VNEXT-1` / `brainstorm:contract-compiler` (gated needs-investigation) — stage 6 compiles the verification graph this layer would execute. Deferred; this feature must work graph-less (plan-time charters) and slot the graph in later.
- `VNEXT-5` (evidence packages) — the package is the *output contract*; this feature's checks are the producer of most package rows (tests run, results, known limitations, skipped-for-capability records).
- `VNEXT-6` (repo intelligence) — "knows how to build, test, lint, run; which tests are flaky" as *descriptive memory, never governing*. Tension to resolve: the proposed verification manifest is *governing policy data* (pyxis-config, human-confirmed). Same facts, different authority class.
- `VNEXT-8/9/10` — autonomy ladder, risk-drives-which-tests-run, policy-says-which-tests-mandatory: the *pricing* surface exists as requirements; new ground is capability-priced autonomy (what a repo can *prove* caps its lane) and degradation policy as data.
- `GATE-2` / `GATE-7` / `SCAF-3` (baseline parity) — local CI mirror, required-checks calibration, language profiles as data: the harbormaster ancestors of mirroring, calibration, and the ecosystem table. Parity claims "mirror what the repo has"; unclaimed: checks *pyxis brings* (falsification, test-delta audit, exec-probe, AC↔test), tech-neutral verb binding, brownfield assessment/ratchets.
- `merge-lane` / floor-clearing (glossary; argus-unattended-drive FR-2/3) — "every required mechanical check green" *consumes* required checks; hard-gates on control-plane paths are a partial machinery-separation precedent. The anti-gaming layer (tautological tests, test weakening, baseline gaming) is claimed nowhere.
- Known-gaps list (prds/README) — "E2E verification as a stage (stage 10)" and "Evals as a verification primitive" are named, unclaimed new work; this feature's outer layers land there. Scope question live: include stage 10 or stop at proposal time.

No source covers the idea; the center of gravity — pyxis-owned verification content, tech-neutral binding, brownfield degradation, anti-gaming — is open ground.

## Framings

- **The verification engine (a substrate port)** — the feature is pyxis's technology-neutral verification *capability*: verification verbs (build/test/lint/exec-probe/coverage) bound per repo as human-confirmed manifest data, an ecosystem facts table, the pyxis-owned neutral checks (falsification, test-delta audit, scope fidelity, AC↔test traceability), brownfield baselines/ratchets, and the capability-gap report. Gates are *consumers*: implement-now's drive-to-green loop, merge-lane's required checks, VNEXT-5's evidence rows, stage 10 later. Thesis: *the unclaimed hole is the port — "how does pyxis verify anything, anywhere"; every gate moment consumes it.* Opens: onboarding assessment as the manifest's authoring moment; VNEXT-6 reconciliation; stage-10 lands as a second consumer, not a rebuild.
- **What green means (the proposal-gate contract)** — the feature is the stage-8 *exit criterion*: pyxis's layered definition of a genuinely-verified proposal (mechanical floor → functional tests → execution reality → anti-gaming → scope fidelity), packaged as the gate implement-now drives to and the merge-lane floor requires. Verbs/manifest/brownfield are implementation properties of the gate, not the identity. Thesis: *the product is the bar, not the plumbing — ship the definition of done-done.* Opens: immediate consumer (implement-now is in review now); GATE-2 parity absorbed into a principled successor.
- **Provable surface (verification as trust pricing)** — the feature is the brownfield adoption economics: onboarding measures what a repo can prove, prices gaps into autonomy/lanes/review depth (never hides them), ratchets baselines so repos monotonically improve, and emits capability-building backlog work. Checks and verbs are instruments; the *pricing loop* is the product. Thesis: *the differentiator is working honestly on any repo as found — verification depth is what autonomy costs.* Opens: the §6 trust model gets its measurement arm; capability-building tasks as the factory's own onboarding flywheel.
- **The local CI loop** *(operator, 2026-07-16 · t1)* — verification and implementation go hand in hand: not a phase after a phase but a latency-ordered ladder of feedback layers (lint → build → unit tests → …) the implementing engine iterates against — fail → compact feedback → fix → re-verify → advance — with the ladder's top being the proposal gate. Engineered for minimal back-and-forth: feedback is small and actionable (never log dumps shipped between phases), re-runs are incremental, retries bounded, and the harness is co-located with the implementer ("merged together if needed"); remote CI stays the authority backstop. Thesis: *the feedback loop is the product — a verification layer not wired into the implementer's loop is just late bad news.* Subsumes the engine framing (verbs/manifest become the ladder's repo bindings) and the green-means framing (the bar becomes the loop's exit condition); the pricing framing stays adjacent.
  - **Refined at t2 — one capability, two surfaces** *(operator, 2026-07-16 · t2)*: part of what is built must port to the remote CI pipeline — e.g. the same e2e implementation runs as a GitHub CI job. The two surfaces have different jobs: the **local loop** helps the implementation system implement better (feedback, fast iteration); the **remote gate** exists because an engineer can bypass the factory pipeline entirely and implement by hand — it enforces "certain qualification plus no regression at all" on everything that lands, author-independent, with no help intent. One implementation, two harnesses; parity by construction, not by mirroring.

**Destination:** **one capability, two surfaces** — the local CI loop (the implementer's feedback harness: latency-ordered ladder, compact actionable feedback, incremental re-verification, bounded retries, co-located with the worker) **and** the remote CI gate (the same checks run as the code host's CI: author-independent, enforcing qualification + no-regression on everything that lands, degrading knowingly when factory metadata is absent) — one check implementation, parity by construction. Verbs/manifest/ecosystem-table/brownfield binding are internals; the green bar is the loop's exit condition; autonomy/trust pricing economics deferred out. Operator's rationale: verification and implementation go hand in hand locally (minimize back-and-forth), and the remote gate is a real deliverable because an engineer can bypass the factory pipeline entirely — whatever lands must meet "certain qualification plus no regression at all."

## Decisions

### D-1 — the identity: one verification capability, two surfaces

- **Question**: which framing is the feature — the verification engine (substrate port), what-green-means (the proposal-gate bar), provable surface (trust pricing), or the operator's local-CI-loop lens, and does the remote gate co-headline?
- **Answer**: two surfaces, one capability — the local CI loop and the remote CI gate are both deliverables sharing one check implementation; verbs/manifest/brownfield binding demoted to internals; the bar is the loop's exit condition; pricing economics deferred out.
- **Reasoning**: operator's t1–t2 lens — verification interleaves with implementation (fail → compact feedback → fix → advance; efficiency is load-bearing), and the remote gate has its own enforcement semantics ("not to help the implementation") because humans can bypass the factory; recommendation adopted as offered.
- **Anchor**: 2026-07-16 · t3
- **Status**: live

### D-2 — the loop seam: verify-side here, drive-side in implement-now

- **Question**: where is the boundary between this feature and implement-now — who owns which half of the loop?
- **Answer**: verify-side here, drive-side there. This feature ships the **agent-free verification harness**: the ladder (checks, latency ordering, per-repo binding), check implementations, the compact-feedback contract (findings, never logs), incremental re-verification, and structured per-invocation verdicts on one schema local+remote. implement-now's worker **hosts** it: launches/prompts the implementing agent, feeds findings back, owns fix-attempt counts, self-heal caps, escalation, PR raising.
- **Reasoning**: matches implement-now's PRD as written (caps are its design-owned policy — no mid-review amendment); an agent-free harness makes the remote gate the *identical* artifact with the drive side simply absent; "merged together" is satisfied by co-location (in-process/subprocess call, zero service chatter). Accepted cost: the feedback contract spans two features — designed once, jointly. Recommendation adopted.
- **Anchor**: 2026-07-16 · t4
- **Status**: live

### D-3 — the v1 ladder: deterministic rungs + anti-gaming, judgment deferred

- **Question**: which rungs ship in v1 — parity minimum (verbs only), the deterministic ladder + anti-gaming checks, or the fuller contract (coverage, AC↔test, charter scope)?
- **Answer**: the deterministic ladder + anti-gaming. Rung 0 instant: format/lint verbs + secret scan + forbidden-artifact hygiene (neutral, no repo capability needed). Rung 1: build/typecheck verb. Rung 2: full test suite + **no-new-failures baseline** (ratchet shrink-only — brownfield-ready day one). Rung 3 (pyxis-owned, novel): **falsification** (new tests must fail with the implementation reverted) + **test-delta audit** (deleted/skipped/weakened tests flagged). Rung 4: exec-probe (manifest-supplied smoke). Remote gate: same rungs, enforcement semantics.
- **Reasoning**: every v1 verdict is deterministic — trustworthy and cheap, no judgment calls; rung 3 is the smallest set addressing "green that isn't green" (implement-now's named top risk); baselines make it work on repos as found. Deferred: changed-line coverage, AC↔test traceability (needs DoD-schema plumbing), LLM judgment lenses, mutation/fuzz, stage-10 feature e2e, evals. Recommendation adopted.
- **Anchor**: 2026-07-16 · t5
- **Status**: live

### D-4 — remote CI: not day-one; design-guiding only for v1

- **Question**: how does the remote gate enforce — what is required to land, and from when (full-ladder shadow→required vs qualification-subset vs required-day-one)?
- **Answer**: deferred — "we do not need to support remote CI on day one; it just guides the design and planning for now." v1 delivers the local loop only. The remote surface stays in the feature's contract as a **binding design constraint**: the harness must be portable to a code-host CI job unchanged (agent-free, headless-runnable, same rungs, same verdict schema); its wiring and enforcement semantics are decided when that phase starts.
- **Reasoning**: operator's — delivery sequencing, not an identity change; D-1's two-surface identity stands, D-2's agent-free harness/host split is what makes the deferral safe (portability by construction, not by later rework).
- **Anchor**: 2026-07-16 · t6
- **Status**: live

### D-5 — verb binding: hand-authored manifest, ecosystem defaults, skip-with-record

- **Question**: where do verb→command bindings come from in v1 — a hand-authored manifest, runtime detection, or detection-assisted onboarding?
- **Answer**: a hand-authored, closed-schema, versioned manifest in `pyxis-config/`: verb→command bindings (lint, format-check, build, test, exec-probe), test-file globs, baseline location, per-verb env passthrough. Ecosystem-table defaults keep a common-stack manifest ~5 lines. An absent verb ⇒ that rung **skips-with-record** — degradation visible in every verdict, never silent.
- **Reasoning**: runtime never guesses — it executes frozen, human-confirmed data (judge-once); the dogfood repo's manifest is trivial so detection buys nothing yet; exec-probe is undetectable in principle, so declared config must exist regardless. Detection-assisted onboarding + manifest drift checks deferred. Recommendation adopted.
- **Anchor**: 2026-07-16 · t7
- **Status**: live

### D-6 — the bet and its observables: dogfood + catch + cheap cycles + portability dry-run

- **Question**: what is the bet, and what observably proves the feature works?
- **Answer**: bet — agent-written code is trustworthy at proposal time *only if* verification interleaves with implementation (cheap deterministic feedback per fix cycle, anti-gaming at the top), achievable on any repo via declared verbs. Observables (candidate M-rows): (1) implement-now's dogfood reaches genuinely-green with this harness as its local gate; (2) **the catch test** — three seeded adversarial diffs (tautological test · deleted/weakened test · baseline addition) all caught, zero land green; (3) a post-fix re-verification costs measurably less than a full ladder run; (4) one advisory headless run in a GitHub Actions job proves portability without shipping enforcement. Guardrail: rung-3 wrong-block occurrences counted and bounded (a falsely-red gate trains ignoring red). Note: observable 2's third seed implies rung 0 gains a **machinery-protection path check** (manifest/baseline files flagged in implementation diffs) — extends D-3 consistent with P-4.
- **Reasoning**: each observable maps to a stated motivation — the loop (1, 3), green-that-isn't-green (2), tech-independence + remote guidance (4); dogfood-only would ship the novel rung unproven. Recommendation adopted.
- **Anchor**: 2026-07-16 · t8
- **Status**: live

### D-7 — segments: the implementing agent P1, the operator P2

- **Question**: who is this for — primary segment, secondary, anti-segment?
- **Answer**: P1 = the implementing agent, via its host (implement-now's worker now, the fleet worker later) — the loop's direct consumer whose iteration the core design optimizes. P2 = the operator — manifest author, verdict/evidence reader, buyer of "green means verified." Anti-segments: human engineers as gate subjects in v1 (remote deferred, D-4 — they benefit indirectly later); repos without a pyxis operator (no one to author the manifest, D-5).
- **Reasoning**: honest about who touches it daily — the machine runs the loop hundreds of times, the human reads one verdict per proposal; matches the harness/host split (D-2). Divergence from implement-now's human-first P1 is deliberate: this feature's efficiency requirement must win design trade-offs, and machine-P1 keeps that unambiguous. Recommendation adopted.
- **Anchor**: 2026-07-16 · t9
- **Status**: live

### D-8 — the four jobs: fast feedback · definitive verdict · trustable green · brownfield-honest

- **Question**: are these the jobs — two agent jobs, two operator jobs?
- **Answer**: confirmed. J1 (P1): check fails mid-implementation → compact actionable feedback in seconds → fix and advance (today: log archaeology or PR-time discovery). J2 (P1's host): task looks done → one definitive genuinely-green verdict → raise the proposal (today: green = "whatever required checks happen to exist"). J3 (P2): reviewing an agent proposal → trust green already means verified, gaming caught pre-review → review substance (today: no defense; implement-now's named top risk). J4 (P2): red-main / capability-poor repo → verification binds honestly via baselines + skip-with-record → adopt without a cleanup campaign (today: all-or-nothing).
- **Reasoning**: each job traces to a live ruling (D-1/D-2/D-3/D-5/D-6) and a named pain; no human-engineer job — that stays anti-segment until the remote phase. Trimming to agent-jobs-only would orphan rung 3 and the baseline mechanism. Recommendation adopted.
- **Anchor**: 2026-07-16 · t10
- **Status**: live

### D-9 — appetite: core + fixtures, bounded by implement-now's dogfood

- **Question**: what bounds the build — core proven on pyxis only, or core + multi-stack fixtures + CI dry-run?
- **Answer**: core + fixtures, dogfood-bound. In: the harness core (ladder execution; workspace-scoped, time-bounded, env-stripped verb runner; verdict schema), the five rungs incl. machinery-protection, the manifest + ecosystem defaults, the joint feedback contract (P-5), incremental re-verification, the catch-test suite as permanent tests, **multi-stack fixture repos** in the harness's own tests (tech-independence proven by test, not claimed), one advisory headless CI dry-run. Out: everything O-1..O-4 plus real second-repo onboarding. Bound: ready to serve as implement-now's local gate for its dogfood; budget follows the reusability bet (this core is the future remote gate), not a task count.
- **Reasoning**: every item serves a live ruling; dropping fixtures would ship the tech-independence requirement as an untested claim and silently drop D-6's observable 4. Recommendation adopted.
- **Anchor**: 2026-07-16 · t11
- **Status**: live

## Facts

- **F-1**: implement-now — the stage-8 native executor — has a draft PRD (2026-07-15, problem review pending); "genuinely-green" is defined as PR exists + all required checks green (M2), hollow ≠ succeeded; it consumes the local gate + CI as given — source: docs/prds/implement-now/effective.md §1, §3.2
- **F-2**: VNEXT-2 already specifies the structured DoD schema: every AC carries requirement ID, verification method (test/eval/manual), executable command where applicable, owner of judgment, required evidence — source: docs/prds/vnext-requirements.md VNEXT-2
- **F-3**: the stage-6 contract compiler (producer of the verification graph) is gated needs-investigation (2026-07-10) — this layer cannot assume compiled verification graphs exist — source: docs/brainstorms/contract-compiler.md Gate
- **F-4**: floor-clearing is versioned policy data: zero hard-gate hits + operator-ratified AUTO class covering the whole diff + every required mechanical check green; hard-gates already protect control-plane paths (partial machinery-separation precedent) — source: docs/wiki/glossary.md; argus-unattended-drive FR-2/FR-3
- **F-5**: stage 8 already runs today via the external fleet (harbormaster claims and implements pyxis tasks); the native path is implement-now — source: .pyxis/pipeline.md
- **F-6**: harbormaster ancestors exist for three pieces: GATE-2 (local CI mirror, auto-detected per subproject), GATE-7 (required-checks calibration, per-repo derived contexts), SCAF-3 (language profiles as data) — source: docs/prds/README.md baseline index
- **F-7**: "E2E verification as a stage (stage 10)" and "Evals as a verification primitive" are named unclaimed gaps (not parity, not VNEXT-specified) — source: docs/prds/README.md §Not in the baseline
- **F-8**: the glossary already fixes the verification-method vocabulary: Test · Demo · Inspection · Analysis · Eval · Human judgment (judgment only with named owner + rubric) — source: docs/wiki/glossary.md (prd-template Appendix A)

## Fog

- **G-1** (non-blocking — blocks: the remote-gate delivery phase): the remote gate's enforcement semantics — required-vs-advisory rollout (shadow→required?), what "no regression at all" binds to remotely on brownfield repos, how bypass-PR verdicts/evidence surface to humans — all unresolved while remote is design-guiding only (D-4)

## Out of scope

- **O-1**: autonomy/trust pricing economics (capability-priced lanes, the gap report as a trust instrument, capability-building backlog generation) — a later, separate feature; this feature's degradation records are its future input (from D-1)
- **O-2**: out of v1, future rungs (WR seeds, not refusals): changed-line coverage · AC↔test traceability (blocked on structured-DoD plumbing, VNEXT-2) · LLM judgment lenses (scope/spec fidelity) · mutation/fuzz · stage-10 feature-level e2e · evals — v1 stays fully deterministic (from D-3)
- **O-3**: remote CI delivery — workflow wiring, required-check registration, enforcement rollout — out of v1; WR: when the remote-gate phase starts; until then remote portability binds the design (from D-4)
- **O-4**: detection-assisted onboarding (probe the repo, propose bindings, capture baselines from observed runs) and manifest drift detection — deferred; WR: when a second repo onboards / the assessment phase starts (from D-5)

## Candidate glossary terms

- **T-1**: **verification manifest** — the closed-schema, versioned file in `pyxis-config/` binding verification verbs to a repo's commands (+ test globs, baseline location, env passthrough); governing policy data, human-authored · avoid: bare *manifest* (GOV-1's primitive manifest), *repo memory* (VNEXT-6's descriptive model — this is governing, that is descriptive)
- **T-2**: **ladder / rung** — the latency-ordered sequence of verification layers the implementer climbs (rung 0 instant → rung 4 reality); a rung is one layer with one verdict · avoid: *phases* (pipeline stages own it), *gates* (merge-lane and PRD sign-offs own it)
- **T-3**: **falsification check** — the rung-3 check that reverts the non-test half of a diff and demands the new tests fail; passing-without-implementation ⇒ tautological · avoid: *mutation testing* (a different, deferred technique — systematic seeded mutants, not one revert)

## Parked technical constraints

- **P-1**: the falsification check (new tests must fail with the implementation reverted) is pure orchestration — git operations + the test verb + a test-file glob; needs no per-language tooling; an optional `test-focus` verb makes it cheaper where ecosystems support it — context: session pre-thinking, 2026-07-16
- **P-2**: verification verbs all share one execution shape (subprocess → exit code + logs + optional report), so one generic exec adapter + per-repo command bindings covers the mechanical layer; format-specific code confines to report parsers (coverage/test-result normalizers) driven by an ecosystem facts table — context: session pre-thinking, 2026-07-16
- **P-3**: repo-supplied verb commands are untrusted input — run them workspace-isolated, time-bounded, fail-closed, environment-stripped (manifest declares required env); clean-env execution is simultaneously the security posture and the fix for the ambient-credential bug class — context: session pre-thinking, 2026-07-16
- **P-4**: baseline/grandfather files are the most gameable artifact in a brownfield setup — an implementer adding its own failure to the grandfathered set goes green; baselines are verification machinery: never writable in the same PR as an implementation, additions human-lane only, ratchet direction (shrink) automatic — context: session pre-thinking, 2026-07-16
- **P-5**: the compact-feedback contract (the finding schema the harness returns: check ID, location, message, machine-parseable) is designed **jointly** with implement-now's worker prompting — the efficiency requirement lives or dies at this seam (D-2's accepted cost) — context: t4
- **P-6**: the incremental re-verification strategy (what re-runs after a fix: affected-only vs cheap-first-full; rung-restart rules when a fix regresses an earlier rung) is design-owned; the product bar is only "a fix cycle costs visibly less than a full ladder run" — context: t3–t4
- **P-7**: the structured verdict (rung results + skips-with-record) is the natural producer of VNEXT-5 evidence-package rows (tests run/results, known limitations); formal evidence-package integration is design-owned, the verdict schema should anticipate it — context: Orientation overlap, t3

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-07-16 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered, zero blocking fog |
