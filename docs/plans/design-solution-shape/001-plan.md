# Design solution-shape — the non-frozen shape artifact and its co-reviewed split — plan 001

| Field | Value |
|---|---|
| **Feature** | design-solution-shape |
| **PRD** | docs/prds/design-solution-shape |
| **Design** | docs/tech-design/solution-shape-v1.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Implement solution-shape v1 — the non-frozen shape artifact, co-reviewed split, and docgate gate |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — The solution-shape document class, home, and parser

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 130–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/config, internal/docgate/techdesigns |

**Goal:** A solution-shape is a first-class document with its own configured home outside the design corpus, parsing to a typed value with a paired-design slug, a status, and a concrete body.

**Current behavior:** No solution-shape document type exists. The config surface carries `tech_designs` and `plans` roots but no shapes root, and `docs/tech-design/` is the only design-adjacent corpus a document may live in.

**Desired behavior:** A new closed config key roots the solution-shape corpus at a directory distinct from — and never nested under — the technical-design corpus. A shape parses to a typed value with a closed header (its paired design's slug and a status drawn from a closed set whose members are `authoritative` and `superseded-by-code`) and a concrete body carrying the schema and interfaces. Pairing resolves by slug. A malformed header, an unknown status, or a shape whose paired design is absent each yields a typed defect naming file, reason, and remedy.

**Scope:**
- the new closed config key, its default, and its unknown-key rejection
- the shape document parse: closed header, the status enum, and the concrete body
- pairing resolution by slug between a design and its shape
- typed defects for a malformed shape, an unknown status, and an unpaired shape

**Out of scope:**
- any gate check that runs over designs or shapes (T3, T4, T5, T6)
- the co-render (T2)

**Acceptance criteria:**
- [ ] a well-formed shape parses to its typed value with slug, status, and body populated
- [ ] a shape with an unknown status yields the named defect and no value
- [ ] a shape whose paired design is absent yields the named defect
- [ ] the shapes root resolves to a directory outside the technical-design corpus

**Key interfaces:**
- `config.Config` — a new solution-shapes path field, its `paths.*` parse case, and its default
- a shape parser producing the typed shape value (header, status enum, body)
- `schema.Defect` — the typed defect the parser emits

**Traces:** prd:design-solution-shape/FR-1 · prd:design-solution-shape/NR-2 · design:solution-shape-v1#3.1 · design:solution-shape-v1#DINV-3

### T2 — The combined review: present a design and its shape together

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 90–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | design-and-shape conversation presentation |

**Goal:** Reviewing a technical design that has a paired solution-shape presents both the design's decisions and the shape's concrete content in one operator conversation.

**Current behavior:** A design and its shape are separate source files, so a reviewer needs a defined combined-review order and exact source references.

**Desired behavior:** When a design has a paired shape (resolved by slug from T1), the operator conversation presents both source files together — the decisions and the concrete schema and interfaces visible in one review. The presentation identifies exact tracked paths, so the reviewed bytes are the bytes that will freeze. A design with no paired shape is reviewed alone.

**Scope:**
- resolve a design's paired shape before human review
- present the design and shape together with exact source paths
- preserve the ordinary review path for a design without a shape

**Out of scope:**
- the gate checks (T3, T4, T5) and the lifecycle (T6)
- the shape parser itself (T1)

**Acceptance criteria:**
- [ ] a design-and-shape pair is presented together with both decisions and concrete shape visible
- [ ] a design with no paired shape follows the ordinary single-document review path
- [ ] the presentation identifies the exact two tracked source files and their reviewed bytes

**Key interfaces:**
- the conversation presenter — where a design is joined with its paired shape for human review
- the T1 pairing resolution — the only linkage between the two files

**Traces:** prd:design-solution-shape/FR-2 · prd:design-solution-shape/NFR-1 · design:solution-shape-v1#3.7 · design:solution-shape-v1#DINV-2

### T3 — The gate: a frozen design carries no concrete shape

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 90–180 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/gate, internal/docgate/techdesigns |

**Goal:** The gate fails a technical design that carries concrete solution-shape content and passes one that only references its shape.

**Current behavior:** The gate lints design headers and structure but nothing forbids concrete schema or interface content from living inside a design body, so a design can freeze carrying volatile shape.

**Desired behavior:** A new check emits a `design-carries-shape` defect when a design body carries concrete schema or interface content that belongs to a shape; a design that only references its shape passes. Every technical design already merged passes unchanged. The defect names file, reason, and remedy.

**Scope:**
- detect concrete-shape content inside a design body
- emit the `design-carries-shape` typed defect and wire the check into the gate run
- leave every existing frozen design passing

**Out of scope:**
- witness, trace, and completeness checks (T4)
- index exclusion (T5) and lifecycle (T6)

**Acceptance criteria:**
- [ ] a design carrying concrete shape yields `design-carries-shape` with file, reason, and remedy
- [ ] a design that only references its shape passes clean
- [ ] every existing frozen design still passes the gate unchanged

**Key interfaces:**
- a new gate check returning `[]schema.Defect`, enumerated in the `Gate` run
- `techdesigns.Design` — the parsed design body the check inspects

**Traces:** prd:design-solution-shape/FR-3 · prd:design-solution-shape/NR-1 · prd:design-solution-shape/NFR-2 · design:solution-shape-v1#3.8 · design:solution-shape-v1#DINV-1

### T4 — The gate: witness, trace, and shape-completeness

| Field | Value |
|---|---|
| **Blocked by** | T1, T3 |
| **Size band** | 130–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | shares the gate enumeration with T3, T5, T6; each adds an independent check function, no shared body |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/gate, internal/docgate/techdesigns |

**Goal:** The gate flags a design invariant with no witnessing shape element, a shape element that traces to no decision, and a stateful design approved with no shape.

**Current behavior:** Nothing cross-checks a design's binding markup against its shape, so an invariant can lack any concrete witness and a stateful design can be approved with no shape at all.

**Desired behavior:** An `unwitnessed-invariant` defect fires when a design `DINV-n` has no witnessing element in its shape; an `untraced-shape-element` defect fires when a shape element cites no decision in its design; a `shape-incomplete` defect fires when a design that creates state or an interface has no paired shape. A pure-decision design that creates no state passes with no shape.

**Scope:**
- the witness check across a design's invariants and its shape
- the trace check from each shape element back to a decision
- the shape-incomplete check for a stateful design lacking a shape
- wire the three checks into the gate run

**Out of scope:**
- the design-carries-shape check (T3), index exclusion (T5), lifecycle (T6)

**Acceptance criteria:**
- [ ] a design invariant with no witnessing shape element yields `unwitnessed-invariant`
- [ ] a shape element citing no decision yields `untraced-shape-element`
- [ ] a stateful design with no paired shape yields `shape-incomplete`
- [ ] a pure-decision design with no state passes with no shape

**Key interfaces:**
- three new gate checks returning `[]schema.Defect`, enumerated in the `Gate` run
- `techdesigns.Design` — the invariant and decision anchors the checks read
- the T1 shape value — the witness/trace counterpart

**Traces:** prd:design-solution-shape/FR-7 · prd:design-solution-shape/UB-1 · design:solution-shape-v1#3.8 · design:solution-shape-v1#DINV-4

### T5 — The index excludes the solution-shape

| Field | Value |
|---|---|
| **Blocked by** | T1, T3 |
| **Size band** | 80–170 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | shares the gate enumeration with T3, T4, T6; independent check function, no shared body |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/techdesigns, internal/docgate/gate |

**Goal:** The technical-design index never contains a solution-shape, and a shape admitted to the design corpus is a defect.

**Current behavior:** The index is generated from designs under the technical-design corpus; nothing asserts that shapes stay out of it or that a shape file cannot sit in the design corpus.

**Desired behavior:** Index generation ignores the shapes root entirely. A solution-shape found in the generated design index — or a shape document placed under the technical-design corpus — yields a `shape-indexed-as-design` defect. The index still computes design liveness exactly as before (existing entries byte-unchanged).

**Scope:**
- keep index generation blind to the shapes root
- the `shape-indexed-as-design` defect for a shape reaching the design corpus or index
- preserve every existing design's index entry byte-for-byte

**Out of scope:**
- lifecycle transitions (T6) and the other checks (T3, T4)

**Acceptance criteria:**
- [ ] a solution-shape never appears in the generated technical-design index
- [ ] a shape document under the design corpus yields `shape-indexed-as-design`
- [ ] existing designs' index entries are byte-identical before and after

**Key interfaces:**
- `techdesigns.GenIndexBytes` and `IndexEntry` — the generation the shape stays out of
- the `Gate` run — where the shape-reaching-corpus defect is enumerated

**Traces:** prd:design-solution-shape/FR-5 · prd:design-solution-shape/NR-1 · design:solution-shape-v1#DINV-3

### T6 — The shape lifecycle: authoritative, superseded-by-code, and the change branch

| Field | Value |
|---|---|
| **Blocked by** | T1, T3 |
| **Size band** | 130–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | shares the gate enumeration with T3, T4, T5; independent check function, no shared body |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/gate, internal/docgate/techdesigns |

**Goal:** A shape's status flips from authoritative to superseded-by-code, a stale authoritative shape is flagged, a shape-confined change leaves the frozen design untouched, and a change to a design's binding markup is refused as a shape change.

**Current behavior:** T1 gives a status field but no transition rule, no stale detection, and no way to tell a shape-confined change from a change that alters the paired design's binding markup.

**Desired behavior:** The status flips authoritative → superseded-by-code as an explicit authoring act recorded in the shape header (the interim signal until an automatic code-exists signal is designed). A `stale-shape` defect fires when an `authoritative` shape's implementing code already exists. A change confined to shape content is legal on the non-frozen shape and edits no design; a change that would alter the paired design's binding markup is refused as a shape change and routed to the design's supersession path.

**Scope:**
- the status transition over the closed enum, recorded in the shape header
- the `stale-shape` defect for an authoritative shape whose code exists
- the discrimination between a shape-confined change and a binding-markup change
- wire the lifecycle check into the gate run

**Out of scope:**
- an automatic code-exists signal (deferred — design Q2)
- the code-diverges-from-shape detector (deferred — UB-2)

**Acceptance criteria:**
- [ ] a shape flips to superseded-by-code via the explicit authoring act
- [ ] an authoritative shape whose implementing code exists yields `stale-shape`
- [ ] a shape-confined change leaves the paired frozen design untouched
- [ ] a change altering a design's binding markup is refused as a shape change

**Key interfaces:**
- the T1 shape value — the status enum this task transitions
- a lifecycle gate check returning `[]schema.Defect`, enumerated in the `Gate` run
- `techdesigns.Design` — the binding markup a change is tested against

**Traces:** prd:design-solution-shape/FR-4 · prd:design-solution-shape/FR-6 · design:solution-shape-v1#3.6 · design:solution-shape-v1#DINV-5

## Coverage

| Requirement | Tasks |
|---|---|
| prd:design-solution-shape/FR-1 | T1 |
| prd:design-solution-shape/FR-2 | T2 |
| prd:design-solution-shape/FR-3 | T3 |
| prd:design-solution-shape/FR-4 | T6 |
| prd:design-solution-shape/FR-5 | T5, T6 |
| prd:design-solution-shape/FR-6 | T6 |
| prd:design-solution-shape/FR-7 | T4 |
| prd:design-solution-shape/UB-1 | T4 |
| prd:design-solution-shape/NR-1 | T3, T5 |
| prd:design-solution-shape/NR-2 | T1 |

## Deferred

| Requirement | Reason |
|---|---|
| prd:design-solution-shape/UB-2 | The code-diverges-from-approved-shape detector is a separate code↔shape comparison capability; the shape retires on code (FR-5, T6) regardless. Plan when a shape-vs-code diff surface is built. |
