# Init hardening — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | Solution-space grill for `prd:init-hardening` — where the front-loaded check engine lives (one predicate home shared with doctor), the init rollback mechanism, admission/routing mechanics including the non-git remedy, coordinate/origin ordering, `ci_mode` materialization and migration, gh probe depth, and the first-use code-gate seeding owner. |
| **Relevant-when** | designing or changing `pyxis init`'s check/admission/rollback mechanics, the preview report surface, `ci_mode`'s workflow materialization, gh preflight probes, or first-use `codegate.json` seeding; checking prior design rulings or refused shapes for the init-hardening build |
| **Basis** | `prd:init-hardening` (merged ledger; installer aligned via its amendment 001) |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-07-28 |
| **Status** | gated: ready-for-design *(→ freezes at the introducing PR's merge; until then the doc-review loop may add/supersede review-anchored rulings)* |
| **Tracking** | — |
| **Feeds** | `docs/tech-design/init-hardening-v1.md` |
| **Prior records** | `brainstorm:init-hardening` · `design-brainstorm:installer-tech-design` · `design-brainstorm:existing-repo-onboarding-tech-design` |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/prds/init-hardening/effective.md` | the basis — FR-1..16, NR-1..3, UB-1..3, Q1–Q4, the P-1..P-7 routing note |
| `docs/tech-design/index.md` (origin/main) | live designs whose Serves touch this ground: `installer-v1` (now stale-serves on the superseded anchors), `existing-repo-onboarding-v2`, `codegate-v1`, `config-driven-portability-v1` |
| `docs/tech-design/installer-v1.md` §3.5/§3.6/§7 | the pipeline being restructured (converge→seed→deps→hooks→wiring→commits→green-proof→welcome); its open Q-1 (prompt surface) is what this basis closes; the pyxis-config concept binding (its D-7) homes `codegate.json` |
| `docs/tech-design/existing-repo-onboarding-v2.md` §2/§3.1–3.2 | adopt's settled dispatch (non-git → UB-5 refusal; empty → routes to init), the one-classifier rule (DINV-2), converge's scan⟂write exposure |
| `internal/converge/doctor.go` + `internal/wire/{execute,runner}.go` | the predicates that already exist: doctor's prerequisite probes (git/node/gh-when-required via `wire.Table`), wire's gh preflight + failure-classification data |
| `docs/architecture-principles.md` | the AP registry for ALT scoring (AP-11, AP-13, AP-15..18, AP-20, AP-22, AP-30 most implicated) |
| `docs/observability/incidents.md` | I3 (consequential CLIs fail closed pre-action) and I5 (two evaluators of one predicate drift) govern this ground |
| `docs/wiki/glossary.md` | term check — installer/onboarding vocabularies settled; no `preflight` term exists |
| `docs/tech-design/design-brainstorm-record-model.md` | this record's contract |

**Overlaps surfaced:**

- `existing-repo-onboarding-v2` §3.2 · **adopt's dispatch is settled design**: a non-git target refuses by name (its UB-5), an empty tree routes to init (its UB-1). Basis Q2's "amend onboarding to admit non-git" option would reopen a live design's settled mechanics, not just a PRD row.
- `internal/converge/doctor.go` · **the prerequisite predicates already have a home**: doctor evaluates git/node presence and gh-when-required (derived from `wire.Table` — AP-11's one-source pattern). The front-loaded checks either consume this home or replace it; a second evaluator is the I5 failure shape.
- `installer-v1` · live, `stale-serves` on FR-3/FR-19/UB-1 (superseded by installer amendment 001); this effort's design builds on it (pipeline, payload, wiring) rather than superseding it — the new design's Serves covers the fresh FR-23/FR-24/UB-11 anchors.
- `config-driven-portability-v1` · the remote *name* resolves from config (the coordinate mechanics must honor `repo.Remote`, never hardcode `origin`).
- `codegate-v1` + `installer-v1` D-7 · `.pyxis/codegate.json` is the pyxis-config concept surface; its consumer is the verify ladder — Q1's seeding owner must not create a second manifest authority.

## Design questions

- **DQ-1**: Where does the front-loaded check engine live — which package owns the environment/admission predicates (FR-3..FR-9) so that init, preview (installer FR-24), and doctor consume one implementation (AP-11, I5)?
- **DQ-2**: Admission and routing mechanics — the exact emptiness predicate (what besides the repository metadata directory is tolerated: nothing?), and the non-git-with-content remedy (basis Q2: init's two-step message vs an onboarding change).
- **DQ-3**: The rollback mechanism (FR-10, UB-1..3) — how the write set is captured and reversed (journal · snapshot-diff · reuse of adopt's undo machinery), including the created-repo case and foreign-file preservation.
- **DQ-4**: Coordinate elicitation ordering — where the `owner/name` exchange, its validation, the existence probe (`use`), and the remote configuration land relative to the checks and the first write (FR-6..FR-8 × FR-9's zero-write invariant).
- **DQ-5**: `ci_mode` materialization — the mechanism that makes the workflow knob-conditional (class-policy vs converge-side condition), and the Q3 migration disposition for already-installed repos.
- **DQ-6**: gh probe depth (basis Q4) — which probes run at check time (auth identity, repo existence, scope prediction) and where advisory degradation is acceptable given FR-10's rollback backstop.
- **DQ-7**: First-use code-gate seeding (basis Q1) — which surface owns detecting code-without-manifest and seeding/instructing: implement-now's preflight, doctor's remedy, or plan-new's projection.

## Panel log

- **DQ-1**: replaces (the scattered predicate homes become one check seam) — fired — a new package boundary every verb consumes; I5 is the failure shape if wrong; orthogonal placements exist
- **DQ-2**: extends (the existing admission seam in the installer; routing is message shape) — skipped — the emptiness predicate is a tightening, and Q2 is a routing ruling, not a seam
- **DQ-3**: opens (an undo engine for init that nothing owns today) — fired — reversal mechanics have structurally distinct shapes with different failure modes
- **DQ-4**: extends (the knob-resolution and wire seams gain one conditional exchange and an ordering rule) — skipped — sequencing inside settled seams; FR-9 forces the order
- **DQ-5**: extends (the class-policy/converge materialization seam gains a knob condition) — skipped — mechanism follows the existing ownership-class machinery; Q3 is a disposition ruling
- **DQ-6**: extends (wire's existing preflight + classification data) — skipped — probe depth is a dial inside a settled seam; the rollback backstop bounds the risk
- **DQ-7**: opens (a seeding surface none of the three candidates owns today) — fired — genuine cross-feature placement with orthogonal candidates

## Decisions

### D-1 — DQ-1: one dedicated check package; doctor delegates

- **Question**: Which package owns the front-loaded checks that init, preview, and doctor all consume?
- **Answer**: **ALT-2** — a dedicated check package (working name `internal/preflight`): init and preview consume it pre-write; doctor's prerequisite rows delegate to the same suite; wire keeps only its per-operation guards.
- **Reasoning**: one predicate home with three named consumers makes I5's drift impossible by construction (AP-11); converge stays tree-only (AP-20); the seam is testable in isolation. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 1
- **Status**: live

### D-2 — DQ-2: strict emptiness; the two-step remedy (answers basis Q2)

- **Question**: How strict is the emptiness predicate, and what remedy does a non-git folder with content receive?
- **Answer**: **Strict + two-step.** Empty means nothing but the repository metadata directory itself — OS droppings are content and appear in the conflict list. A non-git target with content refuses with the runnable two-step (`git init -b <branch>`, then `pyxis adopt`). Adopt's contract and design stay untouched.
- **Reasoning**: onboarding-v2 §3.2 settled adopt's dispatch (non-git refuses before inspection) — reopening a live design plus a PRD amendment to save one printed command is a bad trade; a tolerated-droppings list is a judgment call that grows forever. This answers basis Q2. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 2
- **Status**: live

### D-3 — DQ-3: the write-journal rollback

- **Question**: How does a failed init restore the target?
- **Answer**: **ALT-4, the write journal** — installer-owned writes route through one journaling write primitive; converge's contribution ingests its returned Report; git-init and remote operations are accounted as journal kinds; failure unwinds in reverse. Foreign entries are untouched by construction; a remotely created repository is reported with its cleanup command, never deleted.
- **Reasoning**: exact reversal with foreign-preservation for free (AP-12, AP-17); snapshot-diff violates FR-10's preserve-and-name clause; adopt's durable receipt is the wrong tool for a transient seconds-scale need. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 3
- **Status**: live

### D-4 — DQ-5: `ci_mode` materializes knob-conditionally; migration is advise-only (answers basis Q3)

- **Question**: The workflow becomes knob-conditional — what happens to an already-installed repo carrying it under `ci_mode=local`?
- **Answer**: **Advise-only.** The workflow's class-policy entry is knob-conditional; when excluded it is simply unlisted, and converge never acts on unlisted paths (the never-touch property). Doctor reports the mismatch with both remedies (flip the knob · delete the file). Converge gains no deletion behavior; fresh installs are born right.
- **Reasoning**: removal needs new delete-an-operand-file machinery and would enact a knob choice made while the knob was decorative (`brainstorm:init-hardening#F-9`); advise-only falls out of the existing shapes for free (AP-17, AP-18, AP-20). Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 4
- **Status**: live

### D-5 — DQ-6: auth + existence + best-effort scope prediction (answers basis Q4)

- **Question**: How deep do the gh probes go at check time — auth-present only, auth + per-wiring existence, or best-effort scope prediction on top?
- **Answer**: **Best-effort scope prediction on top of auth + existence.** Check time verifies the authenticated identity and the coordinate's repository per wiring mode (`use`: exists + readable; `create`: absence expected), and additionally probes token scopes against the declared operation set where the token class exposes them — a detectable gap warns pre-write; an unknowable scope (fine-grained/App tokens) degrades to advisory and never blocks; a late permission failure still classifies at the operation and lands as rollback + hand-off.
- **Reasoning**: authority chose earlier warning over probe purity (the recommendation stopped at auth + existence): a knowable gap surfaced before five stages run is worth the imprecise cases, which are exactly the advisory-degradation boundary basis Q4 asked about — acceptable everywhere beyond auth-present, with FR-10 as the backstop.
- **Anchor**: 2026-07-28 · turn 5
- **Status**: live

### D-6 — DQ-7: codegate seeds its own manifest (answers basis Q1)

- **Question**: Which surface owns detecting code-without-manifest and seeding `.pyxis/codegate.json`?
- **Answer**: **ALT-10** — codegate gains the one-shot `seed` verb (`pyxis exec codegate -- seed`): stack detection from the tree's marker files, the template manifest written by its owner. Welcome, doctor's FR-15 advisory, and implement-now's manifest-absent refusal all route to that one command. The verb lands via a `prd:implementation-verification` amendment — the follow-up stage 1 already assigned that feature.
- **Reasoning**: owner-writes-its-own-config keeps one authority and one detection source (AP-11, AP-8, AP-18); the executor stays proposals-only, doctor stays zero-mutations, the planner keeps observing rather than declaring. Answers basis Q1. Recommendation adopted.
- **Anchor**: 2026-07-28 · turn 6
- **Status**: live

## Alternatives

### ALT-1 — Doctor's checks grow the preflight face (converge-homed)

- **Shape**: the predicates stay in `internal/converge` (doctor's checks section); init/preview call a read-only check suite exported from there, pre-elicitation; doctor keeps reporting the same rows post-install.
- **Trade-offs**: wins — zero migration of existing probes, one home by construction; costs — converge's scope creeps beyond tree reconciliation into environment probing (it already does, via doctor), and init's earliest stage depends on the converge package for non-tree concerns.
- **Principles**: AP-11 honored (one predicate home); AP-18 honored (observe); AP-20 strained (converge's blast surface widens conceptually).
- **Incidents**: I5 honored directly (no second evaluator); none further checked (registry 2026-07-28).
- **Verdict**: rejected — a workable single home, but it permanently widens converge's scope into environment probing; displaced by ALT-2 (D-1).
- **From**: DQ-1 · panel

### ALT-2 — A dedicated check package; doctor delegates

- **Shape**: a new `internal/preflight` (name illustrative) owns every environment/admission predicate — floors, gh auth, coordinate resolvability, artifact reachability, cross-knob coherence; init and preview consume it pre-write; doctor's prerequisite section *delegates* to the same suite and keeps its report shape.
- **Trade-offs**: wins — the check seam is named, testable in isolation, and converge stays tree-only (clean hexagon); costs — a one-time migration of doctor's three probes and one more package boundary.
- **Principles**: AP-11 honored (one home, doctor delegating); AP-20 honored (narrowest scope per package); AP-16 honored (each check reports, none skips silently); AP-30 honored (floors/hosts from release/config data, not literals).
- **Incidents**: I5 honored (the delegation removes even today's near-duplication between doctor's probes and wire's preflight); I3 honored (checks precede action by construction).
- **Verdict**: **chosen** — D-1.
- **From**: DQ-1 · panel

### ALT-3 — Init-local stage 0, doctor unchanged

- **Shape**: the checks are a new first stage inside the installer pipeline only; doctor keeps its own copies of the overlapping probes.
- **Trade-offs**: wins — smallest diff, no cross-package design; costs — two evaluators of the same predicates from day one, guaranteed drift.
- **Principles**: AP-11 violated (two homes); AP-20 nominally honored.
- **Incidents**: **I5 matched as the failure shape** — the exact two-evaluators drift the registry records.
- **Verdict**: rejected — the I5/AP-11 violation is disqualifying; kept as the recorded strawman boundary.
- **From**: DQ-1 · panel

### ALT-4 — Write-journal undo (transactional log)

- **Shape**: every mutating step lands in an in-process journal (path + kind: file-write · dir-create · git-init · remote-op); installer-owned writes route through one journaling write primitive; converge's contribution ingests its returned Report; on failure the journal unwinds in reverse — files, emptied dirs, then the metadata dir when init created it. Foreign entries are untouched by construction (never journaled). A remotely created repository is never deleted — reported with its cleanup command (fail-closed on destructive remote acts).
- **Trade-offs**: wins — exact reversal, foreign-preservation for free, the accounting doubles as the report; costs — every write path must journal (a later unjournaled path = debris), and an uncatchable kill (SIGKILL/power) loses the in-process journal — residue then surfaces as admission conflicts on retry.
- **Principles**: AP-12 honored (every acquisition accounted); AP-17 honored (reversal deletes only recorded writes; remote deletion refused); AP-13 honored (restore verified against the journal).
- **Incidents**: none matched (registry 2026-07-28); I5 not implicated (no predicate duplication).
- **Verdict**: **chosen** — D-3.
- **From**: DQ-3 · panel

### ALT-5 — Snapshot-diff restore

- **Shape**: snapshot the target's entry set pre-run (degenerate under strict admission: ∅ or `{.git}`); on failure delete everything absent from the snapshot.
- **Trade-offs**: wins — no journaling discipline; costs — **it deletes foreign entries planted mid-run**, the exact FR-10 violation; distinguishing foreign from init's writes requires the write enumeration anyway, collapsing this into ALT-4 with worse properties.
- **Principles**: AP-17 violated on the foreign-file path (ambiguity resolves destructively).
- **Incidents**: none checked further (registry 2026-07-28) — the basis violation is disqualifying first.
- **Verdict**: rejected — fails FR-10's preserve-and-name clause unless it degrades into ALT-4.
- **From**: DQ-3 · panel

### ALT-6 — Reuse adopt's undo machinery

- **Shape**: init writes an adoption-plan-class durable receipt (`adoption-plan/v1` lineage) with undo-data; failure consumes it the way `pyxis undo` reverses an adoption.
- **Trade-offs**: wins — one undo engine in the product; costs — a durable artifact + lifecycle for a seconds-scale transient need; couples init's rollback to adopt's receipt schema (churn hits both); adopt's undo answers a different question (reversing an *applied* adoption while preserving post-adoption work).
- **Principles**: AP-20 strained (shared schema widens both blast radii); AP-11 not implicated (different function, not a shared predicate).
- **Incidents**: none matched (registry 2026-07-28).
- **Verdict**: rejected — a durable receipt and cross-feature schema coupling for a transient need (D-3); shared low-level file-removal primitives stay welcome.
- **From**: DQ-3 · panel

### ALT-7 — implement-now's preflight seeds the manifest

- **Shape**: the executor, before its first sweep, detects code-without-manifest, runs stack detection, and writes `.pyxis/codegate.json` itself.
- **Trade-offs**: wins — the manifest exists exactly when first needed; costs — an unattended engine writing operand config outside its proposal perimeter (its contract is proposals-only writes), and detection buried inside a sweep nobody watches.
- **Principles**: AP-20 strained (the executor's write perimeter widens); AP-21 strained (a config-shaping judgment made at the least-attended site).
- **Incidents**: none matched (registry 2026-07-28).
- **Verdict**: rejected — an unattended operand-config write outside the executor's proposals-only perimeter (D-6).
- **From**: DQ-7 · panel

### ALT-8 — doctor's remedy seeds it

- **Shape**: doctor detects the gap (the basis's FR-15 advisory) and gains a mutation mode that writes the manifest.
- **Trade-offs**: wins — detection and remedy in one place; costs — **doctor is contractually read-only, zero mutations** — this breaks its cleanest promise for one convenience.
- **Principles**: AP-29 strained (a measurement surface becomes an actor); the doctor contract violated outright.
- **Incidents**: none matched (registry 2026-07-28).
- **Verdict**: rejected — breaks doctor's zero-mutations contract (D-6).
- **From**: DQ-7 · panel

### ALT-9 — plan-new's projection seeds it

- **Shape**: at plan freeze, the planner infers the stack from the plan's tasks and seeds the manifest alongside the projection.
- **Trade-offs**: wins — rides an attended moment; costs — the planner writes plans and tracker items, not operand config; stack knowledge at plan time is inference over prose, weaker than marker files on disk.
- **Principles**: AP-18 strained (declares from plan text rather than observing the tree); AP-20 strained.
- **Incidents**: none matched (registry 2026-07-28).
- **Verdict**: rejected — declares the stack from plan prose rather than observing the tree (D-6).
- **From**: DQ-7 · panel

### ALT-10 — codegate seeds its own manifest (a `seed` verb)

- **Shape**: the manifest's owner gains the one-shot verb — `pyxis exec codegate -- seed` — which detects the stack from the tree (the ecosystem `MarkerFiles`, the one detection source) and writes the template manifest; every other surface only routes to it: the welcome text names it, doctor's FR-15 advisory names it, implement-now's manifest-absent refusal names it. The verb lands via a `prd:implementation-verification` amendment — the follow-up the stage-1 record already assigned that feature (`brainstorm:init-hardening#O-2`).
- **Trade-offs**: wins — owner-writes-its-own-config (no second authority), detection observed from the tree, every surface's remedy is one command; costs — a cross-feature amendment before the ladder is self-healing, and the human runs one command once.
- **Principles**: AP-11 honored (one detection source, one seeding home); AP-8 honored (sanctioned, schema'd extension surface); AP-18 honored (observes marker files); AP-29 honored (doctor stays a measurement).
- **Incidents**: I5 honored (no duplicate detector); none further matched (registry 2026-07-28).
- **Verdict**: **chosen** — D-6.
- **From**: DQ-7 · panel

## Seam map

- **S-1**: the check suite (`internal/preflight`, name illustrative) — opens — behind it: every environment/admission predicate (toolchain floors, gh auth, coordinate resolvability, artifact reachability, cross-knob coherence) with per-check named refusals and report rows; consumers: init (refuse pre-write), preview (report), doctor (prerequisite delegation) — feeds: the design's Scope and its Serves for `prd:init-hardening` FR-3..FR-9 + `prd:installer` FR-24.
- **S-2**: the rollback journal — opens — behind it: the journaling write primitive, converge-Report ingestion, git-init/remote-op accounting, and the reverse unwind — feeds: the design's Scope and its Serves for `prd:init-hardening` FR-10 + UB-1..3.
- **S-3**: the extended seams (composite) — extends — the installer's admission predicate, knob resolution (+ the conditional coordinate exchange), the class-policy entry for the workflow (knob-conditional), wire's preflight (auth/existence/scope probes), and the welcome/doctor truth rows — feeds: Scope + Serves for `prd:init-hardening` FR-1/FR-2, FR-6..FR-8, FR-11..FR-16 + `prd:installer` FR-23/UB-11.
- **S-4**: the codegate `seed` verb — opens (in the sibling feature, by its amendment) — behind it: marker-file stack detection + the template-manifest write, owned where the manifest lives; this design only routes to it — feeds: the design's Dependencies note (D-6).

## Facts

- **F-1**: doctor already evaluates the prerequisite predicates — git/node presence, gh-required-when the captured knobs declare gh ops (derived from `wire.Table`, AP-11-style) — in `internal/converge/doctor.go`; the front-loaded checks overlap it exactly — source: `doctor.go` `prerequisites()`/`ghRequired()`.
- **F-2**: `existing-repo-onboarding-v2` §3.2 settles adopt's dispatch: non-git → named refusal before inspection (UB-5); empty tree → routes to init, no plan written (UB-1) — source: the live design.
- **F-3**: adopt reuses converge's classifier under the one-classifier invariant (its DINV-2), the I5 lesson made structural — source: `existing-repo-onboarding-v2` §2/§3.1.
- **F-4**: init's write set is already enumerated per stage and staged by explicit path (config + gitignore, converge's Repaired/Regenerated, `run.seeded`, hooks) — the raw material any rollback consumes — source: `internal/installer/installer.go` `collectCommitPaths`.
- **F-5**: wire's gh preflight (`gh --version`), the pre-action slug guard, and the transient/permission/not-found classification patterns exist as data — source: `internal/wire/execute.go`, `runner.go`.
- **F-6**: the remote name resolves from config (`repo.Remote`; config-driven portability) — coordinate/remote mechanics must honor it, never hardcode `origin` — source: upstream `tail.go` `originSlug(ctx, target, run.repo.Remote)`.
- **F-7**: installer-v1 §3.5 fixes the current stage order (converge → seed → dependencies → hooks → wiring → commit-tree → epoch → green-proof → welcome); its §7 Q-1 (the prompt surface) is the question this basis closes — source: the live design.
- **F-8**: `.pyxis/codegate.json` is the pyxis-config concept surface bound in installer-v1 (its D-7); its consumer is the verify ladder (`invoke.go` `ManifestRelPath`); python/node ecosystem defaults now carry `MarkerFiles` (detection groundwork) with empty verb sets — source: installer-v1 §3.7, `internal/verify`.
- **F-9**: registry lessons governing this ground: I3 (consequential CLIs refuse before action on any misparse/defect) and I5 (two evaluators of one predicate drift) — source: `docs/observability/incidents.md`.
- **F-10**: DQ-4's ordering is principle-forced, not a decision: FR-9 (zero writes before checks pass) + read-only probes fix it — the coordinate is elicited and shape-validated at check time, the `use` existence probe runs against the coordinate via the API (no remote needed), and the remote is configured only at apply, under the config-resolved remote name (F-6); under `create`, an already-existing repository is the ensured end state (AP-22) — source: `prd:init-hardening` FR-6..FR-9 + AP-22.

## Fog

- **G-1** (non-blocking — rides the authored design as its own Q-n): an uncatchable kill (SIGKILL, power loss) loses the in-process journal — residue then surfaces as admission conflicts on retry; whether a scratch-persisted journal (outside the target) is worth its complexity is the design's call.

## Out of scope

- **O-1**: Any change to adopt's contract or the onboarding-v2 design — the non-git remedy is init's two-step message; adopt's settled dispatch stands. (from D-2)
- **O-2**: Converge deletion machinery — no path exists by which converge removes an operand file; the knob-excluded workflow is unlisted, advised on, never deleted. (from D-4)
- **O-3**: A durable rollback receipt or artifact class for init — the journal is transient and in-process; adopt's receipt schema stays adopt's alone. (from D-3)
- **O-4**: Seeding the verification manifest from the executor, doctor, or planner — each breaks a standing contract (proposals-only · zero-mutations · observe-don't-declare); only codegate's own verb writes it. (from D-6)

## Candidate glossary terms

- **T-1**: preflight (the check pass) — the read-only, pre-write evaluation of every environment/admission predicate init's resolved knobs imply, one suite consumed by init, preview, and doctor (D-1) · avoid: "readiness verb" (a declined product surface, `brainstorm:init-hardening#O-3`), "precheck" (unanchored).

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-07-28 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | pass — core four covered (§2 Decision ← D-1/D-3, panel-logged · §4 ← ALT-1..10 all verdicted · Serves/Scope ← S-1..S-4 + the basis anchors · §3.2 ← O-1..O-4), zero blocking fog (G-1 non-blocking, rides the design as its own Q-n) |
| **Amend route** | — |
