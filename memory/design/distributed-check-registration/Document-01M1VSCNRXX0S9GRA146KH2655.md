# Distributed check registration — conflict-free parallel authorship of gate checks

| Field | Value |
|---|---|
| **Status** | Draft → Approved *(merge requires Approved; merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) — surface is governance/infrastructure, so a named reviewer is mandatory |
| **Date** | 2026-07-10 |
| **Relevant-when** | adding or changing a docgate/plangate check; deciding where a new check's verb, help, and applicability live; triaging a keep-both merge conflict on a gate enumeration, dispatch switch, or help block; superseding the registration mechanism |
| **Scope** | How independent checks compose into the docgate and plangate umbrellas, their CLI dispatch, and their help — so a new check is contributed from its own file and no shared line is edited. **Not**: what any individual check verifies (each check's own design owns that); the history-diff base of merge-base checks (the sibling merge-base-resolution decision); the plan-layer overlap gate itself (plan-new-v1 owns it — this design only states how it composes). |
| **Tracking** | issue TBD — parallel-implementation robustness (content-conflict half) |
| **Builds-on** | docs/tech-design/docgate-v1.md · docs/tech-design/docgate-v2.md · docs/tech-design/plan-new-v1.md |
| **Supersedes** | — |
| **Serves** | design:docgate-v1 · design:plan-new-v1 |
| **Security/privacy impact** | none — a compile-time/startup composition change over existing report-only checks; no runtime surface, no new data, no network |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The umbrella that runs every check is a hand-maintained enumeration, and the
CLI that dispatches a single check is a hand-maintained switch with a paired
help block. Every "add a capability" task appends to all three of these central
lists, and a check function added beside its neighbors also interleaves its body
into a shared file. Under parallel authorship these are conflict magnets: a set
of sibling tasks decomposed from one plan each add a check, and as they land
one after another every not-yet-merged branch re-conflicts on the same
enumeration line — approaching a quadratic number of keep-both resolutions
against a main that moves the whole time. The resolution is always "keep both"
(the checks are independent — there is never a real either/or), yet it is
manual, repeated, and races the next sibling to land.

This is the failure mode **AP-23** exists to prevent, on the exact surfaces it
names (dispatch tables, usage strings), and its own remedy is the one taken
here: *remove conflict magnets structurally* rather than keep re-merging the hot
line. The planning layer already gates overlap — two parallel tasks whose
declared file surfaces intersect must be sequenced or carry an explicit
acceptance — but today "add a check" is forced to touch the shared surfaces, so
the planner faces only bad options: sequence every check-adding task (destroying
the parallelism the plan created) or record an overlap acceptance (which lets
the siblings race and conflict). The cheapest moment to fix this is before the
check set grows further: every check added under the central-list shape is
another future migration, and the set is actively growing.

This decision addresses the **content-conflict** half of parallel-branch
robustness; the **history** half — a behind-but-additive branch mis-read as
deleting merged documents — is a separate, independently-revisable decision and
is out of scope here.

## 2. Decision

**Decision:** Each check contributes itself — its verb, help, applicability, and
run behavior — by registering a descriptor from its own file; the umbrella, the
CLI dispatch, and the help are **assembled from the registry**, ordered
deterministically by verb, never hand-enumerated. Adding a check is creating one
file and editing no shared line, so two tasks adding two checks touch two
disjoint files and cannot conflict — and, living in separate files, their bodies
never interleave. The plan-layer overlap gate is retained unchanged as the
safety net for surfaces that genuinely cannot be distributed; the registry
removes the gate enumeration from that category, so independent checks stop
forcing a sequence-or-accept choice.

A reader who stops here should see the shape: contribution is distributed to the
unit, assembly is centralized and deterministic, and the shared edit point is
gone rather than merely formatted for cleaner merges.

## 3. The design

### 3.1 The contribution model

**Decision:** A check is a descriptor — a stable verb, a one-line help string,
an applicability flag (whether it requires a history base), and a run behavior
that takes a single context value carrying everything a check may read (the repo
root, the resolved history base, and the loaded configuration). Bundling the
inputs behind one context is what lets every check share one run signature
despite their heterogeneous needs; a check reads only the fields it uses.

Each check calls the registrar from its own file, at package initialization.
The registrar records the descriptor under its verb. The umbrella iterates the
registered checks, skipping the base-requiring ones when no base is present,
and runs each; the CLI resolves an unknown verb by looking it up in the
registry and running the same behavior; the help block is printed by walking the
registry. Because the umbrella and the individual dispatch invoke one and the
same run behavior per check, they cannot drift — a class of bug the twin
hand-written call sites previously allowed.

### 3.2 Boundaries and non-goals

- **Not solved here** — what a check verifies (each check's own design/PRD
  anchor owns that); the memoization of per-ledger-directory work shared across
  the directory-scoped checks (an assembly detail of the implementation, not a
  contract); the history-diff base (sibling decision).
- **Owned by a future design** — any richer applicability predicate than
  "needs a base" should it arise; a shape document pinning the concrete
  descriptor and registrar structure, if the implementation warrants one.
- **Implementation must not infer** — that registration order carries meaning
  (assembly sorts; order is an accident of initialization); that the registry
  may be written after startup (it is contributed only at initialization and
  read-only thereafter); that distribution weakens any check — a check is byte
  for byte the same logic, only relocated and self-registered.
- **Must remain unchanged** — checks stay report-only and never mutate
  (docgate-v1#DINV-3); defects remain data; every check stays runnable on its
  own by verb; the gate's verdict on a given tree is unchanged.

### 3.3 Invariants

- **DINV-1** — Assembly is deterministic: the registered checks are ordered by
  verb, never by registration order, so the umbrella's output and the help are
  stable regardless of initialization order.
- **DINV-2** — A check is contributed from exactly one file. There is no shared
  enumeration, dispatch, or help line for a second check to edit; adding a check
  is a single-file addition.
- **DINV-3** — The umbrella and the individual-verb dispatch run one and the
  same behavior per check; the two entry points cannot disagree.
- **DINV-4** — The registry is write-only during initialization and read-only
  afterward; a verb is unique across the registry, and a collision is a build- or
  test-time failure, never a silent runtime overwrite.

### 3.4 Observability

none — the composition is resolved at compile and startup time and has no
runtime surface of its own. Its correctness is guarded by tests, not telemetry:
the deterministic ordering (DINV-1) and verb uniqueness (DINV-4) are asserted in
the suite, and a missing or duplicate contribution fails the build or the tests
rather than emitting a defect. The checks themselves keep whatever observability
their own designs specify, unchanged.

### 3.5 Security & privacy

none — matches the header row. This relocates and assembles existing
report-only checks; it introduces no new surface touching identity,
permissions, secrets, user data, network boundaries, provider egress, billing,
or production controls, and adds no data handling or network access.

### 3.6 Compatibility and migration

**Decision:** The migration is behavior-preserving and incremental. Each check
moves to its own file and self-registers; the moment a check is registered its
line leaves the central enumeration, its case leaves the dispatch switch, and
its entry leaves the hand-written help. Old and new coexist mid-migration — the
umbrella runs the registered checks and the shrinking hand-written remainder
together — so the change lands check by check, not as one big cutover, and the
central lists are deleted only once empty. The gate's verdict on any given tree
is identical before and after; the only intended difference is that the defect
stream and the help are ordered by verb (deterministic) rather than by the
former hand-written order. This is a `refactor` in effect: no check gains or
loses power.

### 3.7 Composition with the plan-layer overlap gate

The planning layer gates parallel tasks whose declared file surfaces intersect,
requiring them to be sequenced or to carry an explicit acceptance. That gate is
correct and is kept as-is. What changes is the input it sees: once a check is
added by creating its own file, two check-adding tasks declare disjoint
surfaces, so the gate passes them as genuinely parallel with no acceptance
needed. The explicit acceptance is thereby returned to its honest use — a real
either/or between two tasks — and must not be used to wave through additions to
a shared list. Any surface that genuinely cannot be distributed falls back to
AP-23's line-per-item single-sourcing, and the overlap gate remains its
backstop; the registry's job is to shrink that residual set, starting with the
gate enumerations, which need not be in it at all.

### 3.8 Generality — other shared-append surfaces

The pattern is: replace a central list that every unit appends to with
distributed contribution plus deterministic assembly. It applies wherever a
"list everyone edits" grows under parallel authorship. The high-contention
surfaces are the two umbrella enumerations — docgate's and plangate's — with
their paired dispatch switches and help; both are covered by this decision. The
low-contention surfaces are the closed parse registries pinned to template
versions, which change rarely and deliberately; where any of them begins to
grow with features it takes AP-23's line-per-item form rather than a registry.
Distinguishing the two — distribute what grows per feature, single-source
per-line what is pinned — is the audit this decision generalizes to.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Keep the central list, format it one-item-per-line (AP-23's fallback) | Helps a plain list, but the umbrella is executable code and the dispatch/help are structured — line-per-item does not make a `switch` or an enumeration merge cleanly, and the interleaved-body conflict remains. It is the mitigation AP-23 itself ranks below structural removal. |
| Code generation from a table | The table is a central file every add still edits, and the generated artifact re-conflicts on every addition; "regenerable" trades a hand-merge for a mandatory regenerate step in the loop. Self-registration has no central file at all. |
| `.gitattributes merge=union` on the enumeration | A band-aid: it does nothing for the dispatch switch, the help text, or interleaved function bodies, and it silently duplicates and reorders — violating deterministic output. |
| Planning-layer serialization alone | Sequencing every check-adding task destroys the parallelism the plan deliberately created; it treats independent work as if it collided. The overlap gate is right only for *genuine* shared surfaces, which this decision removes the checks from. |
| A single explicit registry slice literal in one file | That literal is precisely the conflict magnet — one line every contributor edits. Distribution to per-check files is what removes the shared line. |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | Initialization-time self-registration is less explicit than a visible central list — a check could be hard to discover | An author cannot find where checks live | One check per file under a discoverable naming convention; the help and umbrella both enumerate the live set at runtime; a test lists the registered verbs, so the full set is always inspectable | registry enumeration test; the derived help output |
| R2 | Assembly that trusted registration order would be non-deterministic across builds | Flaky gate output; unstable help | Order by verb in assembly (DINV-1); never rely on initialization order | ordering test asserting strict verb sort |
| R3 | Two files register the same verb | One check silently shadows another | Verb uniqueness is a checked invariant (DINV-4) — the tests fail on a duplicate; optionally the registrar refuses a duplicate outright | uniqueness test |
| R4 | The registry is mutated after startup by later code | Order- and content-instability re-enter | Contribution happens only at initialization; the registry is read-only afterward (DINV-4) | review + the read-only accessor surface |
| R5 | The behavior-preserving claim is untested and the migration changes a verdict | A refactor silently weakens the gate | Acceptance requires a byte-identical gate verdict on the real corpus before and after, plus the full race suite green | real-corpus diff; `go test -race` |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Adding a check is a single new file — the change touches no umbrella enumeration, no dispatch switch, and no help block | Inspection | the diff of a representative added check (or the demonstration check) shows one new file only | §2 · #DINV-2 |
| A-2 | The umbrella's defect order and the help order are strictly by verb and stable across runs and builds | Test | ordering test | #DINV-1 |
| A-3 | Every check remains individually invocable by its verb and produces the same defects as within the umbrella | Test | per-verb dispatch test | #DINV-3 |
| A-4 | The migrated gate's verdict on the real corpus is identical to the pre-migration verdict; the full race suite is green | Test | real-corpus verdict diff; suite run | §3.6 · design:docgate-v1 |
| A-5 | A duplicate verb fails the build or the tests, never a silent runtime shadow | Test | uniqueness test | #DINV-4 |
| A-6 | The docgate and plangate umbrellas are both assembled from the registry — neither retains a hand-written check enumeration | Inspection | both umbrellas' post-migration source | §3.8 · design:plan-new-v1 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | Does the registrar refuse a duplicate verb outright (fail-fast) or is a test-time uniqueness assertion sufficient? | implementation | nothing | open | decided at implementation; DINV-4 holds either way |
| Q2 | Do the directory-scoped checks (per-ledger-directory) join the same registry with a directory-aware run, or keep an explicit directory loop while the repo-wide checks distribute? | implementation | nothing | open | decided at implementation against the shared read/fold memoization; the repo-wide checks are the contended surface |
| Q3 | Is the shared context value identical across docgate and plangate, or does each gate carry its own registry with a gate-specific context? | implementation | A-6 | open | decided at implementation; the pattern is identical even if the context types differ |

## Principles

- **AP-23 — honored.** This is the principle's charter case: the dispatch tables
  and usage strings it names are removed as conflict magnets *structurally*
  (self-registration + deterministic assembly), which the principle ranks above
  its own line-per-item fallback; work decomposition's file-surface declarations
  then see disjoint surfaces for independent checks.
- **AP-19 — honored.** Composition resolves at compile and startup time; the
  steady-state runtime cost is a sort over a small set, and nothing recurring is
  added.
- **AP-22 — honored.** Parallel authors adding checks converge without
  coordination — two additions are two files, and assembly is order-independent,
  so there is no "exactly one author" assumption.
- **AP-10 — honored.** "Please append to the enumeration and the switch and the
  help" — a discipline — becomes structurally unnecessary: there is no shared
  line to remember to edit, so the rule is enforced by the shape, not by care.
- **AP-14 — honored.** Acceptance is the real-corpus byte-identical verdict and
  the race suite, not the fixture set alone.
- **RW-5 — honored.** One decision — how checks compose — in one document,
  design-first, before the migration it governs.
- **Deviation:** none.
