# Review: codegate-v1 — design-review report

| Field | Value |
|---|---|
| **In one line** | Round 1: request-changes — 14 blocking + 7 advisory findings (dirty-tree unsoundness, incremental under-definition incl. a green-with-unrun-rungs leak, undesigned FR-7 checks, ephemeral-worktree environment false-negatives, AP-12 residue, the unowned acquisition pin among them); all 21 applied in revision; round 2: approve. |
| **Relevant-when** | auditing or consuming the review of design codegate-v1; asking why it was approved after a request-changes round; tuning tech-design-review calibration against real findings |
| **Subject** | design:codegate-v1 (post-lint draft; shape: none; record-fed from design-brainstorm:implementation-verification-tech-design; basis prd:implementation-verification) |
| **Moment** | new design |
| **Reviewer** | tech-design-review · `.pyxis/authoring/tech-design-review.md` (2026-07-16) |
| **Date(s)** | 2026-07-16 (rounds 1 and 2) |
| **Verdict** | approve |
| **Tracking** | agenticlab-ai/pyxis#373 |
| **Template version** | design-review-report/v1 |

## Round 1

*Process note: four lenses ran as independent parallel reviewers over the stage context pack, the served basis, the feeding record, and the sibling designs. Independent convergence: the `skip:not-reached` leak (coherence + feasibility), the DINV-2/DINV-7 acceptance gap (contract-quality + coherence), the DINV-5 scope-mechanism gap (three lenses), the dangling `#ALT` anchor (two lenses), and the lock/worktree residue (upstream + feasibility) — merged below.*

### FND-1 — every stateful mechanism presumes a clean committed tree; nothing asserts or refuses otherwise

- **Severity**: blocking (CAL-4: load-bearing for A-2/A-3/A-5 — rung-3 mechanics and carried-pass soundness collapse on a dirty tree)
- **Lens**: feasibility-completeness
- **Evidence**: "the re-run set is computed from `git diff <prior head sha>`" (§3.8) and "creates an ephemeral worktree at the workspace HEAD" (§3.9) — vs — the host's fix cycle edits between invocations with unstated commit cadence: on a dirty tree the head sha under-identifies the evaluated state (DINV-1's mismatch rule accepts a verdict for state X as current for Y), the diff measures change-since-HEAD not change-since-prior-run, and the worktree can see neither uncommitted tests nor the uncommitted non-test half (perpetual UB-4).
- **Acceptance condition**: a clean-tree invocation precondition (codegate scratch excepted; dirty ⇒ exit-2 refusal with a configuration-error verdict), DINV-1 stated against the asserted-clean tree, the host-commits-before-invoking expectation named at the seam, and a dirty-tree fixture.
- **Disposition**: applied — §3.6 precondition + refusal; DINV-1 clause; seam sentence; A-7 dirty-tree leg. Refuse-dirty is the derivation, not a new ruling: tree-hash binding would contradict the record's worktree-from-HEAD mechanics (#D-5) and re-import the stash gymnastics it rejected.

### FND-2 — §3.8 drops the record's "or that was not pass" clause: not-reached rungs escape both the re-run and carry sets

- **Severity**: blocking (CAL-1)
- **Lens**: coherence *(independently also raised by feasibility)*
- **Evidence**: "the previously-failed rung first (FR-3), then every rung whose inputs changed; rungs whose inputs are provably unchanged carry forward as carried passes" (§3.8) — vs — "re-running any rung whose inputs changed **or that was not pass**" (record #ALT-8, carried into #D-4). A rung-0 failure leaves rungs 1–4 `skip:not-reached`; a doc-only fix re-runs none of them, yielding a pass-class verdict with never-executed rungs — the §1 failure mode, emitted by the harness itself.
- **Acceptance condition**: restore the clause; add the schema rule that pass-class verdicts never contain `skip:not-reached`.
- **Disposition**: applied — §3.8 re-run set restored to the record's rule; DINV-3 extended; A-4/A-5 legs extended.

### FND-3 — DINV-1's universal consumer rule outlaws §3.8's own prior-verdict read, and the invalidation set covers two of five bindings

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: "a consumer treats a verdict whose recorded revision differs from the object at hand as absent" (DINV-1) — vs — "reads the workspace's prior verdict; if its manifest/baseline hashes match…" (§3.8): the incremental read is a consumer whose subject always differs; and diff-base/codegate-version changes leak through carried passes while §3.10 claims one version input.
- **Acceptance condition**: scope the absence rule to certification reads with the incremental state-read as the named exception; full-run on any binding change (diff base recorded resolved and equality-checked; version equality); DINV-8 restated as the full binding set.
- **Disposition**: applied — DINV-1 scoped; §3.8 invalidation extended; DINV-8 restated. (The record scoped invalidation narrower without deciding these two bindings — extension, not ruling dispute.)

### FND-4 — per-rung input sets undefined; the incremental economy claim rests on them

- **Severity**: blocking (CAL-4: load-bearing for A-5)
- **Lens**: feasibility-completeness
- **Evidence**: "every rung whose inputs changed" (§3.8) names no input function — repo verbs are whole-repo commands; any honest conservative default makes carried passes rare, which changes where M3's savings actually come from.
- **Acceptance condition**: define per-rung input sets as data with conservative defaults, and state honestly that first-failure ordering (cheap red cycles) is the primary fix-cycle economy, carried passes the secondary optimization.
- **Disposition**: applied — §3.8 defines input sets as registry-declared data (defaults: whole tree for repo-verb rungs, the proposal diff for rung 3) and reframes the economy claim.

### FND-5 — the ephemeral worktree lacks workspace dependency state: environment failure masquerades as "the tests bite"

- **Severity**: blocking (CAL-4: load-bearing for A-1/A-2 — a false negative on the exact adversarial family rung 3 exists to block)
- **Lens**: feasibility-completeness
- **Evidence**: "creates an ephemeral worktree at the workspace HEAD… runs the test verb" (§3.9) — vs — `git worktree add` materializes tracked files only: no node_modules/.venv, so the test verb dies for environmental reasons and the falsification check reads the failure as discrimination — tautological tests sail through on non-Go stacks.
- **Acceptance condition**: a two-leg protocol (the unreverted worktree must pass the targeted tests first, else rung error — environment distinguished from discrimination) plus data-shaped state provisioning (declared state dirs linked into the worktree).
- **Disposition**: applied — §3.9 two-leg protocol + manifest/ecosystem `state-dirs` provisioning; R4 and A-2/A-3 extended to all three stacks.

### FND-6 — FR-7's neutral checks are two names in a table: no mechanism, no data home, no acceptance row, NFR-1 unowned

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: "format, lint (repo verbs) · secret scan · forbidden-artifact hygiene" (§3.7 — the entire treatment) — vs — prd:implementation-verification/FR-7's fit (seeded secret + committed binary red with class-named findings; rung-0 wall-time meets NFR-1); §3.10's closed enumeration of the ecosystem table excludes these checks' data; no A-row traces FR-7; diff-vs-tree scope undecided.
- **Acceptance condition**: design both checks (mechanism + scope), name their data homes, add the A-row with FR-7's fixtures and the NFR-1 measurement point.
- **Disposition**: applied — §3.7 designs both as diff-scoped checks with rule data in the ecosystem table (+ manifest extension point); new A-12; NFR-1 owned there.

### FND-7 — FR-10's "recording observed baseline shrinkage" is dropped

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: §3.7 rung 2's complete FR-10 treatment has no shrink record; the verdict content list (§3.6) carries none; A-11 tests bind + never-writes only — vs — FR-10's fit: "a baseline-shrink run records the shrink in the verdict."
- **Acceptance condition**: a shrink record in the verdict's rung-2 entry + an A-11 leg.
- **Disposition**: applied — §3.6/§3.7 shrink record; A-11 extended.

### FND-8 — the configuration-error verdict is unemittable exactly when the workspace flag is the defect; UB-5 never reaches the body

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: "exit 2 configuration or usage error … refuses before any rung executes" (§3.6) — vs — UB-7/UB-3/UB-5's "emitting only a configuration-error verdict": the only designed emission target is a workspace-relative path, unresolvable when `--workspace` itself is invalid; UB-5's no-fallback semantics appear nowhere in the body.
- **Acceptance condition**: state the emission rule (artifact when the workspace resolves; stdout-carried verdict otherwise) and wire UB-5 in the body.
- **Disposition**: applied — §3.6 emission rule; §3.7 rung-2 sentence carries UB-5's no-baseline-less-fallback; A-7 asserts the stdout verdict.

### FND-9 — partial verb absence within a multi-check rung is unrepresentable: FR-6/FR-17 undecidable at the design's own granularity

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: rung 0 consumes two verbs plus verbless checks while DINV-4 admits one result per rung — with lint unbound, the single result can be neither pass (a skip occurred) nor skip (checks ran) — vs — FR-6's never-silent skip and FR-17's green-degraded trigger.
- **Acceptance condition**: per-check results within rung entries plus an aggregation rule (any skipped check ⇒ the rung records its skips and counts toward green-degraded).
- **Disposition**: applied — the verdict schema carries per-check results; DINV-4 restated over checks and rungs; FR-6/FR-17 semantics decidable; A-4 extended.

### FND-10 — AP-12 silently omitted while the design mints a lock, a worktree, and scratch with no liveness or janitor story

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment *(residue mechanics independently raised by feasibility)*
- **Evidence**: "guarded by a lock file in the same scratch directory" (§3.6), the worktree "removed within the run" (§3.9), scratch retention unstated (§3.4), Principles listing no AP-12 — vs — AP-12 (every throwaway resource ships with its janitor in the same change; absence-action needs an in-flight predicate) and A-8's own "subsequent run equals an uninterrupted run": a kill-9'd holder's existence-lock wedges the workspace busy forever; a dead rung-3 leaves a stale registration in the shared `.git`.
- **Acceptance condition**: flock-held descriptor (kernel-released), run-id-unique worktree paths with start-of-run self-pruning, a scratch retention rule, the shared-`.git` concurrency posture, an AP-12 Principles row, and a kill-during-rung-3 A-8 leg.
- **Disposition**: applied — §3.6/§3.9 mechanics + §3.4 retention; AP-12 row added; A-8 extended.

### FND-11 — "shipped exactly like docgate" requires an additive change to repo-layout's closed config schema that nothing owns; the pin itself is unprotected machinery

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: "Shipping follows repo-layout-and-system-footprint verbatim" (§3.1) — vs — that live design's closed-parse operand config enumerates a `docgate:` pin section only; the additive `codegate:` section is a schema change decided nowhere, and a proposal diff downgrading the pin is FR-8-class gaming outside the manifest-derived machinery set.
- **Acceptance condition**: own the additive section in one paragraph (this design declares it; AP-6 additive evolution), and add the pin path to the machinery-protection set or record the residual.
- **Disposition**: applied — §3.1 owns the additive `codegate:` pin section; §3.7's machinery set gains the acquisition-pin path as a codegate-known constant.

### FND-12 — §3.6 binds exit codes 0–3 and A-7 asserts exit 2 while Q3 declares the numerics open

- **Severity**: blocking (CAL-3: the protocol's test oracle formally un-bound by the doc itself)
- **Lens**: contract-quality
- **Evidence**: the §3.6 Decision's exit table and A-7's "exit 2" — vs — "Q3 | Exit-code numeric values … | open | fixed in code at first release" (§7).
- **Acceptance condition**: strike the numerics from Q3, leaving only the termination-grace constant; §3.6's 0–3 binding stands.
- **Disposition**: applied — Q3 narrowed.

### FND-13 — DINV-2 and DINV-7 are exercised by no A-row; A-8 and A-2 carry trace slips

- **Severity**: blocking (CAL-1)
- **Lens**: coherence *(independently also raised by contract-quality)*
- **Evidence**: A-8 traces #DINV-2 while testing nothing about commits; DINV-7 — a header-named security mitigation — appears in no A-row; §3.11's property tests name DINV-1/3/4 only; A-2's machinery seed lives in §3.7 but A-2 traces §3.9 only.
- **Acceptance condition**: a DINV-7 workspace-unchanged assertion; a DINV-2 structural verifier; A-8 retraced to #DINV-8; A-2's traces gain §3.7.
- **Disposition**: applied — A-3 gains the tree-hash-unchanged leg (#DINV-7); A-11 gains the no-git-mutation + ignored-path audit (#DINV-2); A-8 retraced; A-2 retraced.

### FND-14 — the workspace-scope half of DINV-5 has no designed mechanism, no platform matrix, and no violation routing

- **Severity**: blocking (CAL-1)
- **Lens**: coherence *(independently raised by contract-quality and feasibility)*
- **Evidence**: "workspace-scoped filesystem *expectation*" (§3.5) vs A-6's "an out-of-workspace write fails the run" and §3.5's "red/error fixture class" — vs — no detector named anywhere; portable child-process write enforcement is platform-bound (Landlock on Linux; nothing self-contained on macOS); which exit class a violation produces is unassigned.
- **Acceptance condition**: decide the mechanism and platform matrix; assign each DINV-5 violation class its exit class; align DINV-5/§3.5/A-6 wording with what is buildable, naming the macOS residual against ABUSE-4.
- **Disposition**: applied — §3.5/§3.6: Landlock-enforced write scope on Linux (violation ⇒ exit 3 with a violation-classed finding); macOS residual = strip + redirection, named, with A-6's write-scope leg pinned to the Linux CI matrix; env observation stays prevention-by-construction; hang stays error-timeout.

### FND-15 — the finding schema is fixed in an immutable Decision yet declared a provisional proposal with no evolution vehicle

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: the seven-field set inside §3.6's Decision — vs — Q1's "frozen when the VerificationGate adapter lands" with no vehicle on this ledger.
- **Acceptance condition**: Q1's resolution names the vehicle (a post-proposal field change supersedes §3.6's decision by successor design); the durable altitude (machine-parseable, id+location+message, logs by reference) stays binding regardless.
- **Disposition**: applied — Q1 resolution reworded.

### FND-16 — §3.10 cites a dangling `#ALT` anchor

- **Severity**: advisory (CAL-1: provenance preserved — §4 carries the correct `#D-6` citation for the same rejection)
- **Lens**: coherence *(independently also raised by contract-quality)*
- **Evidence**: "(`#ALT` runtime-defaults-file rejected …)" (§3.10) — vs — the record's ledger is ALT-1..8; the rejection lives in #D-6's reasoning.
- **Acceptance condition**: `#ALT` → `#D-6`.
- **Disposition**: applied.

### FND-17 — env-strip destroys toolchain caches; the allowlist-of-names cannot express the fix

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: DINV-5's strip — vs — cold GOCACHE/HOME per verb run (NFR-1/M3 pressure), and cache redirection needs env entries that *set values*, which "per-verb env allowlist" (names) doesn't express.
- **Acceptance condition**: manifest env entries may pin values; ecosystem defaults pin cache vars into scratch; the scope rule treats scratch and pinned cache paths as in-scope.
- **Disposition**: applied — §3.10 manifest schema + §3.5 scope note.

### FND-18 — the verdict path's gitignored-ness is presupposed but unowned

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: "a well-known gitignored workspace path" (§3.6) — vs — nothing establishes the ignore entry in operand repos; an unignored verdict swept into a proposal violates DINV-2 invisibly.
- **Acceptance condition**: codegate ensures the exclusion via local metadata (`.git/info/exclude`) at first run — not repo content, so DINV-6 holds — and refuses if the path is tracked; the machinery set gains the verdict path.
- **Disposition**: applied — §3.6.

### FND-19 — system git is an undeclared runtime dependency and a second determinism input

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: "exactly one version input" (§3.10) — vs — diff/-M/apply/worktree semantics ride the machine's git; NFR-3's "no runtime dependencies" is silently false for git.
- **Acceptance condition**: declare git with a minimum version (startup check; below-floor ⇒ configuration-error); record the observed git version in the verdict bindings.
- **Disposition**: applied — §3.1 dependency declaration; DINV-1's recorded set gains the git version; §3.10's claim corrected to "one *codegate* version input, git's recorded".

### FND-20 — "verb" does double duty for subcommands and manifest-bound commands

- **Severity**: advisory (CAL-1: every use disambiguates in context; terminological)
- **Lens**: coherence
- **Evidence**: "codegate carries two verbs" (§3.1) — vs — DINV-5's manifest verbs; the record says "subcommand" (#D-6).
- **Acceptance condition**: "subcommand" for `run`/`capture-baseline`; "verb" reserved for manifest bindings.
- **Disposition**: applied — §3.1, DINV-6, §3.10 reworded.

### FND-21 — P-1's third queued leg (the basis NR-2 corrective amendment) is never mentioned

- **Severity**: advisory (CAL-1: the queue is owned by the record's disposition, cited in full)
- **Lens**: coherence
- **Evidence**: "the queued additive amendment + small design supersession" (§3.6) — vs — the record's three-leg disposition including the corrective amendment on the basis's NR-2.
- **Acceptance condition**: name the third leg once at the seam.
- **Disposition**: applied — §3.6 sentence extended.

## Round 2

*Scope-locked re-review of the revision: each of FND-1..21's acceptance conditions verified met at its named anchor; the changed spans re-read for regressions — the new mechanics (clean-tree precondition, two-leg falsification, per-check verdict entries, flock/janitor rules, Landlock matrix, pin ownership) are internally consistent, carry their DINV/A-row updates, and contradict no recorded ruling (FND-1's and FND-14's resolutions are derivations forced by recorded rulings and the basis's NFR-3, noted in their dispositions). No new findings on changed spans.*

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-07-16 |
| 2 | approve | 2026-07-16 |

## Escalation brief

none

## Measurements

- **Rounds**: 2
- **Findings**: 14 blocking · 7 advisory
- **Dispositions**: 21 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
