# Context pack — the shared pipeline-context compiler — plan 001

| Field | Value |
|---|---|
| **Feature** | context-pack |
| **PRD** | docs/prds/context-pack |
| **Design** | docs/tech-design/context-pack-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Implement the context-pack compiler — the closed class declaration, the deterministic build-fresh assembler, and its docgate verb with the acceptance corpus |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The closed class declaration and the admissible-kind set

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 110–210 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/contextpack, internal/docgate/schema |

**Goal:** A per-stage declaration parses closed into an ordered list of class entries whose kinds come only from a closed admissible set that contains no authoring-trail kind, resolves each entry's selector deterministically to an ordered source list, and names a defect — never a partial result — on an unknown kind or a selector that does not resolve.

**Current behavior:** No context-pack declaration exists. Nothing names which stable classes a stage's context comprises, nothing bounds the kinds a declaration may reference, and nothing resolves a class to concrete sources; the independence boundary the PRD requires (a cold reviewer must never receive the authoring trail) has no mechanical form.

**Desired behavior:** A stage's declaration is closed-parse data: an ordered list of entries, each a `kind` drawn from a **closed admissible set** — the stable-context kinds (governing, subject, calibration, and their peers) — and a `selector` that resolves deterministically, for a given subject, to an ordered list of source paths. The admissible set is defined such that no authoring-trail kind (the delivery journal, review dispositions, orchestration notes) is a member, so a declaration cannot name one. Parsing is closed: an unknown key or a kind outside the admissible set is a named defect, not a lenient skip; a selector that resolves to no existing file is a named defect identifying the selector, never a partial declaration. Ordering is the declared order, never filesystem enumeration.

**Scope:**
- the closed-parse declaration: an ordered list of `{kind, selector}` entries per stage
- the closed admissible-kind set, defined to exclude every authoring-trail kind
- deterministic selector resolution to an ordered source list for a subject
- the named defects for an unknown/disallowed kind and for an unresolvable selector

**Out of scope:**
- assembling or emitting the pack, the manifest, or the digest — this task is the declaration and its validation only (T2)
- the docgate verb registration and the acceptance corpus (T3)
- broadening the admissible-kind set beyond the stable-context kinds — a later, separately-revisable data+enum change

**Acceptance criteria:**
- [ ] a well-formed stage declaration parses to its ordered `{kind, selector}` entries with each selector resolved to an ordered source list for a given subject
- [ ] a declaration naming a kind outside the admissible set is rejected with a named defect and yields no parsed declaration
- [ ] no authoring-trail kind is a member of the admissible set — a declaration cannot reference the delivery journal, dispositions, or orchestration
- [ ] a selector resolving to no existing file yields a named defect identifying the selector, with no partial declaration and no crash

**Key interfaces:**
- the closed-parse stage-declaration type and its parser, rejecting unknown keys and kinds
- the closed admissible-kind set as the type of a class entry's kind, authoring-trail-free by construction
- the deterministic selector resolver producing an ordered source list, and `schema.Defect` for the unknown-kind and unresolvable-selector findings

**Traces:** prd:context-pack/FR-3 · prd:context-pack/FR-4 · prd:context-pack/UB-1 · prd:context-pack/UB-3 · prd:context-pack/NFR-4 · prd:context-pack/NFR-5 · design:context-pack-v1#3.2 · design:context-pack-v1#3.4 · design:context-pack-v1#DINV-3 · design:context-pack-v1#DINV-6

### T2 — The deterministic build-fresh assembler and its manifest

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 130–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/contextpack |

**Goal:** Given a validated declaration and a subject, the assembler reads the resolved sources verbatim at one revision, concatenates them in declared order under a manifest header carrying schema, stage, subject, build revision, the ordered source list, and a body digest — producing byte-identical output for identical inputs with no wall-clock anywhere, reading only declared sources, writing none, and naming a defect on an unknown subject.

**Current behavior:** The declaration and admissible kinds exist (T1) but nothing turns them into a pack: there is no assembler, no manifest, no digest, and no build path — a session still has no single artifact to read in place of the corpus crawl.

**Desired behavior:** The assembler takes a T1-validated declaration and a subject and produces one pack: a manifest header (`schema`, `stage`, `subject`, `built_from` = the repo tree revision, `sources[]` = each included source's role and path in body order, `digest` = a content hash over the assembled body) followed by the body — each resolved source included byte-for-byte, in declared order, never reformatted or summarized. A declared class that resolves to no source for the subject is recorded in `sources[]` as absent rather than emitted as an empty section. The build always reads current sources at a single revision and returns bytes to a caller-chosen sink; it stores no authoritative pack, so a re-build after a source change reflects the change and no pack mixes two revisions. Output is a pure function of the declaration and the sources at that revision — declared order, verbatim bytes, fixed manifest serialization, no wall-clock value anywhere — so two builds of the same (stage, subject) at one revision are byte-identical. The assembler reads only files reached through a declared class and writes no source file. An unknown subject is a named defect, not a crash.

**Scope:**
- the assembler: resolved sources → manifest-headed, declared-order, verbatim pack bytes
- the manifest header fields (schema, stage, subject, built_from revision, ordered sources, body digest) and the absent-class record
- determinism — declared order, verbatim bytes, fixed serialization, no wall-clock (built_from is the only temporal anchor)
- build-fresh with no stored pack, and the named defect on an unknown subject

**Out of scope:**
- the declaration parse and kind validation (T1) — the assembler consumes a validated declaration
- the docgate verb wiring, the acceptance corpus, and the reference-stage measurement (T3)
- summarizing, compressing, or reordering any source content — the body is verbatim

**Acceptance criteria:**
- [ ] given a validated declaration and a subject, the assembler emits a manifest-headed pack whose `sources[]` exactly matches the resolved sources and whose body is those sources verbatim in declared order
- [ ] two builds of the same (stage, subject) at one revision are byte-identical, and no wall-clock value appears anywhere in the output
- [ ] a source changed between builds is reflected in the next build; no pack is served from storage and none mixes two revisions
- [ ] the assembler reads no file outside the declared sources and leaves every source byte-unchanged; an unknown subject yields a named defect and no artifact

**Key interfaces:**
- the assembler entry point: `(validated declaration, subject) → pack bytes` to a caller sink
- the manifest header type carrying schema, stage, subject, the build revision, the ordered source list, and the body digest, plus the absent-source record
- the deterministic serialization and the body-digest function; `schema.Defect` for the unknown-subject finding

**Traces:** prd:context-pack/FR-1 · prd:context-pack/FR-2 · prd:context-pack/FR-5 · prd:context-pack/FR-6 · prd:context-pack/FR-7 · prd:context-pack/UB-2 · prd:context-pack/UB-4 · prd:context-pack/NR-3 · prd:context-pack/NFR-2 · prd:context-pack/NFR-3 · design:context-pack-v1#3.1 · design:context-pack-v1#3.3 · design:context-pack-v1#3.5 · design:context-pack-v1#DINV-1 · design:context-pack-v1#DINV-2 · design:context-pack-v1#DINV-4 · design:context-pack-v1#DINV-5

### T3 — The docgate verb and the acceptance corpus

| Field | Value |
|---|---|
| **Blocked by** | T1, T2 |
| **Size band** | 100–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/contextpack, cmd/docgate |

**Goal:** A `docgate` context-pack verb exposes the assembler behind the tool's existing conventions, and a per-property acceptance corpus proves independence red-first (a trail-kind declaration rejected, a mid-review pack trail-free), determinism (byte-identical rebuild), freshness (a mid-stage source change reflected), fail-closed handling (missing source, unknown subject), and non-regression (sources byte-unchanged, docgate's other verbs' outputs unchanged) — with the pack's token size reported so the operator can measure the reference stage.

**Current behavior:** The declaration (T1) and the assembler (T2) exist and pass their own unit behavior, but nothing exposes them as a callable verb, nothing proves the cross-cutting safety properties as a corpus, and nothing surfaces the pack size the token-saving measurement reads.

**Desired behavior:** The assembler is registered as one more `docgate` verb following the tool's existing verb and error conventions, taking a stage and a subject and writing the pack to a caller-chosen sink (default a gitignored scratch path or standard output). An acceptance corpus assembles the per-property fixtures and asserts each: the independence leak-attempt — a declaration naming an authoring-trail kind is rejected, and a pack built over a subject that is itself under review contains zero authoring-trail bytes — red-first before the guard exists, green after; the determinism case — two builds byte-identical; the freshness case — a source changed between builds reflected, none served from storage; the fail-closed cases — a missing declared source and an unknown subject each yielding a named defect and no artifact; and the non-regression cases — every source byte-unchanged after a build, and `docgate`'s existing fold/gate verbs producing identical output with the new verb present. The verb reports the assembled pack's token size so the reference-stage before/after (the operator's post-merge measurement) has its per-session figure.

**Scope:**
- the `docgate` context-pack verb registered under the tool's existing conventions, writing to a caller-chosen sink
- the acceptance corpus with per-property fixtures: independence (red-first), determinism, freshness, fail-closed, non-regression
- the reported pack token size feeding the reference-stage measurement
- confirmation that `docgate`'s other verbs' outputs are byte-identical with the new verb present

**Out of scope:**
- the declaration parse (T1) and the assembler logic (T2)
- the operator's reference-stage before/after token comparison itself — a post-merge validation, not code (the verb only supplies the per-session size it reads)
- model routing, review tiering, and inline-vs-subagent orchestration — non-goals of the feature

**Acceptance criteria:**
- [ ] the context-pack verb builds a pack for a given stage and subject through docgate's conventions and writes it to the chosen sink
- [ ] the independence fixtures are red-first then green: a trail-kind declaration is rejected, and a pack over a under-review subject carries zero authoring-trail bytes
- [ ] the determinism, freshness, and fail-closed fixtures pass: byte-identical rebuild; a mid-stage change reflected with no stored pack; a missing source and an unknown subject each a named defect with no artifact
- [ ] every source is byte-unchanged after a build and docgate's existing verbs' outputs are byte-identical with the new verb present; the verb reports the pack's token size

**Key interfaces:**
- the context-pack verb, registered under the docgate command surface, taking a stage, a subject, and an output-sink argument
- the acceptance-corpus fixture layout with per-property expected outcomes, the leak-attempt corpus red-first
- the pack-size report the reference-stage measurement consumes

**Traces:** prd:context-pack/FR-4 · prd:context-pack/UB-1 · prd:context-pack/UB-2 · prd:context-pack/NR-1 · prd:context-pack/NR-3 · prd:context-pack/ABUSE-1 · prd:context-pack/ABUSE-2 · prd:context-pack/NFR-1 · design:context-pack-v1#3.7 · design:context-pack-v1#A-3 · design:context-pack-v1#A-7 · design:context-pack-v1#A-8

## Coverage

| Requirement | Tasks |
|---|---|
| prd:context-pack/FR-1 | T2 |
| prd:context-pack/FR-2 | T2 |
| prd:context-pack/FR-3 | T1 |
| prd:context-pack/FR-4 | T1, T3 |
| prd:context-pack/FR-5 | T2 |
| prd:context-pack/FR-6 | T2 |
| prd:context-pack/FR-7 | T2 |
| prd:context-pack/UB-1 | T1, T3 |
| prd:context-pack/UB-2 | T2, T3 |
| prd:context-pack/UB-3 | T1 |
| prd:context-pack/UB-4 | T2 |

## Deferred

| Requirement | Reason |
|---|---|
| prd:context-pack/NFR-1 | the reduction target is proven by the operator's reference-stage before/after measurement (PRD DOD-5), a post-merge validation; T3 supplies the per-session pack-size figure it consumes, but the comparison itself is not fleet-implementable code |
