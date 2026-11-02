# Local work tracker — Carina and the WorkTracker substrate — plan 001

| Field | Value |
|---|---|
| **Feature** | local-work-tracker |
| **PRD** | docs/prds/local-work-tracker |
| **Design** | docs/tech-design/local-work-tracker.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Local work tracker — Carina and the WorkTracker substrate |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Establish the work-item core: model, store, create and show

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=2 |
| **Pattern** | — |
| **Touches** | internal/worktracker/model, internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina, internal/docgate/config |

**Goal:** A work item is creatable and readable end to end — typed model, one-file-per-item store with atomic writes and exclusive-create identifiers, the Carina binary's first verbs, and the conformance-kit harness that every later behavior extends.

**Current behavior:** No work-item medium exists in this repo; the factory's only tracker is the hosted one, and the engine has no WorkTracker port to consume.

**Desired behavior:** Creating a work item persists one human-readable file under the configured work-items root and returns a stable sequential identifier that is never reused; showing it returns every field. The closed header grammar rejects an unknown key or out-of-set value with a defect naming file, operation, reason, and remedy, quarantining only that item. A simulated crash between the temporary write and the rename leaves the prior file intact. The work-items path key joins the closed config surface with its default at the operand root.

**Scope:**
- the typed work-item model and the closed on-disk grammar (header, body, empty append log)
- exclusive-create identifier allocation and the atomic write discipline
- the WorkTracker port's first methods with the Carina create and show verbs over one core
- the conformance-kit harness (a behavior table importing no adapter symbol) with its first rows

**Out of scope:**
- transitions, comments, signals, hierarchy, dependencies, queries (later tasks)
- any index, cache, daemon, or network surface

**Acceptance criteria:**
- [ ] create then show round-trips every header field and the body, in human and machine output, and the machine output validates against its golden schema
- [ ] two hundred sequential creates yield two hundred distinct, never-reused identifiers, each resolvable by show
- [ ] a malformed hand-edited item yields a defect naming file, operation, reason, and remedy, and every other item stays readable
- [ ] the kit's atomic-write row passes: a simulated crash before rename leaves the prior version whole

**Key interfaces:**
- `WorkItem` — the typed model (header fields, body, log) with its closed value sets
- `WorkTracker` — the consumer-side port; create and get are its first methods
- `Defect` — the shared failure quadruple, rendered by the Carina binary
- the conformance-kit behavior table — the executable port contract

**Traces:** prd:local-work-tracker/FR-1 · prd:local-work-tracker/FR-2 · prd:local-work-tracker/FR-15 · prd:local-work-tracker/UB-1 · design:local-work-tracker#3.1 · design:local-work-tracker#3.3 · design:local-work-tracker#3.4 · design:local-work-tracker#3.8 · design:local-work-tracker#DINV-2 · design:local-work-tracker#DINV-3

### T2 — Lifecycle: edit and idempotent transitions

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 100–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina |

**Goal:** A work item's title, body, and open-closed state change safely: edits touch only the named fields, and transitions converge under repeats and races.

**Current behavior:** Items can be created and shown but never changed; there is no lifecycle.

**Desired behavior:** Edit updates the named fields and the updated stamp, leaving everything else byte-identical. Close and reopen apply idempotently — repeating an already-applied transition succeeds as a recorded no-op, never an error — and the close reason accepts only its closed value set, rejecting anything else with a named defect.

**Scope:**
- the edit verb over title and body
- close and reopen transitions with the transition audit entry in the append log
- closed-set validation for state and close reason

**Out of scope:**
- the rich lifecycle (readiness, triage) — projected later via signals and predicates, never stored as state

**Acceptance criteria:**
- [ ] an edit changes exactly the named fields plus the updated stamp; a diff shows no other mutation
- [ ] closing a closed item succeeds as a recorded no-op, and the transition log carries one entry per applied change
- [ ] an out-of-set close reason is refused with a defect naming the value and the allowed set

**Key interfaces:**
- `WorkTracker` — gains edit and transition methods; the Carina edit, close, and reopen verbs
- the transition log entry — the audit record appended per applied change

**Traces:** prd:local-work-tracker/FR-3 · prd:local-work-tracker/FR-4 · design:local-work-tracker#3.10 · design:local-work-tracker#DINV-11

### T3 — The append log: comments and latest-wins decision records

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 120–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina |

**Goal:** Dialogue and frozen judgments accumulate on a work item without loss: comments append forever, and decision records resolve latest-wins per namespace.

**Current behavior:** The append log exists structurally but nothing writes to it.

**Desired behavior:** Comments append in time order and are never rewritten. Decision records are typed, schema-versioned entries under a closed namespace set; the latest record per namespace is authoritative; an identical repeat writes nothing. Concurrent appends to one item serialize so no entry is lost.

**Scope:**
- the comment verb and the append-only comment discipline
- the record verb: namespaced decision records with latest-wins reads and idempotent repeats
- per-item write serialization across the read-modify-write

**Out of scope:**
- the meaning of any disposition (triage semantics live in the predicates task)

**Acceptance criteria:**
- [ ] a burst of interleaved appends from two writers lands every entry in order with zero loss
- [ ] the latest record per namespace is returned as authoritative and an identical repeat is a recorded no-op
- [ ] an out-of-set namespace is refused with a defect naming the value

**Key interfaces:**
- the log entry record — comment, decision-record, and transition kinds
- `WorkTracker` — gains comment, record, and latest-record reads

**Traces:** prd:local-work-tracker/FR-5 · prd:local-work-tracker/FR-9 · design:local-work-tracker#3.10 · design:local-work-tracker#DINV-5 · design:local-work-tracker#DINV-11

### T4 — Signals: the governed vocabulary

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 100–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | parallel branch over shared packages; each verb self-registers from its own file into a derived command registry and each kit row into a self-registering conformance suite, and the WorkTracker port grows by per-concern sub-interfaces composed by embedding — extension is by new file, not shared-line append (AP-27, sharpening AP-23); the lone shared embed line falls to the recovery loop |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/localfs, internal/worktracker/model, internal/worktracker/conformance, cmd/carina |

**Goal:** Typed control state rides on work items through a governed signal vocabulary — names with roles and presentation — added and removed idempotently.

**Current behavior:** Items carry no signals; there is no vocabulary and no routing or authorization surface.

**Desired behavior:** The canonical seed vocabulary ships as a governed input under the store root: each signal has a name, a role from the closed role set, and presentation. Adding or removing a signal on an item is idempotent; adding an undefined signal is a defect naming it; operands may extend only the classification axis, never redefine engine-recognized names.

**Scope:**
- the store-level signal vocabulary with the canonical seed set
- signal add, remove, and define verbs with idempotent semantics
- closed-role validation and the classification-only extension rule

**Out of scope:**
- role-based selection (the query task) and readiness semantics (the predicates task)

**Acceptance criteria:**
- [ ] the seed vocabulary round-trips through the store and every signal reports its role
- [ ] adding a present signal and removing an absent one both succeed as recorded no-ops
- [ ] an undefined signal and an engine-recognized redefinition are each refused with a defect naming the offense

**Key interfaces:**
- the signal definition — name, role, presentation — and its vocabulary store
- `WorkTracker` — gains signal mutation and vocabulary reads

**Traces:** prd:local-work-tracker/FR-6 · design:local-work-tracker#3.14 · design:local-work-tracker#DINV-11

### T5 — Hierarchy: parents, children, and the epic rollup

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 100–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | parallel branch over shared packages; each verb self-registers from its own file into a derived command registry and each kit row into a self-registering conformance suite, and the WorkTracker port grows by per-concern sub-interfaces composed by embedding — extension is by new file, not shared-line append (AP-27, sharpening AP-23); the lone shared embed line falls to the recovery loop |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina |

**Goal:** Work items form the Feature to Milestone to Epic to Task shape: parent links resolve both directions and an epic reports its open children exactly.

**Current behavior:** Items are flat; no parent relation or rollup exists.

**Desired behavior:** Setting or clearing a parent maintains the relation both directions. An epic reports the count and identity of its open children. The hierarchy-role value set is closed, and a parent reference to a nonexistent item is refused naming the target.

**Scope:**
- parent set and clear with bidirectional resolution
- the epic rollup read (open-children count and identities)
- closed hierarchy-role validation

**Out of scope:**
- dependency ordering (its own task) and any auto-close sweep (engine-era behavior)

**Acceptance criteria:**
- [ ] a child appears under its parent and the parent from the child, after set and after clear
- [ ] an epic with a known fixture of children reports exactly its open ones
- [ ] a parent reference to a missing item is refused with a defect naming the target

**Key interfaces:**
- `WorkTracker` — gains parent mutation and children reads; the Carina parent verbs
- the hierarchy-role value set on the model

**Traces:** prd:local-work-tracker/FR-7 · design:local-work-tracker#3.14 · design:local-work-tracker#DINV-11

### T6 — Dependencies: blocked-by edges with cycle refusal

| Field | Value |
|---|---|
| **Blocked by** | T5 |
| **Size band** | 100–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | parallel branch over shared packages; each verb self-registers from its own file into a derived command registry and each kit row into a self-registering conformance suite, and the WorkTracker port grows by per-concern sub-interfaces composed by embedding — extension is by new file, not shared-line append (AP-27, sharpening AP-23); the lone shared embed line falls to the recovery loop |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina |

**Goal:** Ordering constraints are first-class edges: blocked-by relations resolve both directions, stay acyclic by refusal, and carry the cross-repo reference form opaquely.

**Current behavior:** No dependency relation exists; nothing can express ordering.

**Desired behavior:** Adding or removing a blocked-by edge maintains both directions. An edge that would close a cycle is refused with the cycle path named. A dangling target is refused naming it. The cross-repo reference form is stored and displayed without resolution.

**Scope:**
- dependency add and remove with bidirectional resolution
- the pre-write cycle check with the named-cycle refusal
- opaque storage of the cross-repo reference form

**Out of scope:**
- cross-repo resolution (deferred by the governing contract's waiting room)

**Acceptance criteria:**
- [ ] blockers and blocking both resolve after add and after remove
- [ ] an edge closing a cycle is refused with the full cycle path named and the edge set unchanged
- [ ] a dangling target and a malformed reference are each refused with a defect naming them

**Key interfaces:**
- `WorkTracker` — gains dependency mutation and edge reads; the Carina dep verbs

**Traces:** prd:local-work-tracker/FR-8 · prd:local-work-tracker/UB-2 · prd:local-work-tracker/UB-3 · design:local-work-tracker#DINV-10

### T7 — Queries: list, filter, and exact search

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 140–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | parallel branch over shared packages; each verb self-registers from its own file into a derived command registry and each kit row into a self-registering conformance suite, and the WorkTracker port grows by per-concern sub-interfaces composed by embedding — extension is by new file, not shared-line append (AP-27, sharpening AP-23); the lone shared embed line falls to the recovery loop |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina |

**Goal:** The backlog is discoverable: list with state and signal filters returns bounded, stably-ordered sets, and text search matches exactly — never fuzzily.

**Current behavior:** Items can only be read one by one; no listing, filtering, or search exists.

**Desired behavior:** List filters by state and by signal name or role, returning a deterministic creation-ordered set — including selecting the factory-implementable set by its authorization-role signal. Search over titles, bodies, and comments returns every exact match and only exact matches. Every query emits schema-versioned machine output beside the human form.

**Scope:**
- list with state and signal (name and role) filters, stable-sorted
- exact substring and token search over titles, bodies, and comments
- golden-schema machine output for both verbs

**Out of scope:**
- any search index (direct scan; an index is a measured-need decision recorded in the design's open questions)

**Acceptance criteria:**
- [ ] a fixture corpus returns the exact expected set and order for each filter, including the authorization-role selection
- [ ] a near-miss token does not match and the fixture yields zero false positives
- [ ] both verbs' machine output validates against golden schemas

**Key interfaces:**
- the query parameters type — state, signal, role, text
- `WorkTracker` — gains list and search; the Carina list and search verbs

**Traces:** prd:local-work-tracker/FR-10 · prd:local-work-tracker/FR-11 · prd:local-work-tracker/FR-15 · design:local-work-tracker#3.14

### T8 — The shared predicates: ready-set and triage-set

| Field | Value |
|---|---|
| **Blocked by** | T2, T3, T4, T5, T6 |
| **Size band** | 140–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/model, internal/worktracker/localfs, internal/worktracker/conformance, cmd/carina |

**Goal:** The engine's dispatch questions get one answer each: the ready-set and triage-set predicates exist exactly once, shared verbatim by the port and the Carina verbs.

**Current behavior:** Readiness and undecidedness are not computable; no predicate exists anywhere.

**Desired behavior:** The ready-set predicate — open, carrying the authorization signal, no open blocker, not a structural rollup, no closing disposition — and the triage-set predicate — open, no triage decision record, not a structural rollup, no open blocker, not in flight — each have exactly one implementation. The port and the CLI return byte-identical sets over any corpus, and destructive-decision reads reflect every prior write with no cache in the path.

**Scope:**
- the two predicate implementations over the model, consumed by port and CLI alike
- the ready and triage verbs
- direct-read discipline on the decision path

**Out of scope:**
- fairness, rotation, or scheduling order (engine-era; the predicate only answers membership)

**Acceptance criteria:**
- [ ] the ready set over a fixture corpus matches the hand-computed expectation exactly
- [ ] the triage set over the same corpus matches its hand-computed expectation exactly
- [ ] the CLI and the port return byte-identical sets for both predicates
- [ ] a membership read immediately after a qualifying write reflects that write

**Key interfaces:**
- the ready and triage predicate functions — the single shared implementations
- `WorkTracker` — gains the two set reads; the Carina ready and triage verbs

**Traces:** prd:local-work-tracker/FR-12 · prd:local-work-tracker/FR-13 · prd:local-work-tracker/FR-16 · design:local-work-tracker#DINV-6 · design:local-work-tracker#DINV-4

### T9 — The fault suites: strong reads, crash safety, and contention

| Field | Value |
|---|---|
| **Blocked by** | T2, T3 |
| **Size band** | 160–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | parallel branch over shared packages; each verb self-registers from its own file into a derived command registry and each kit row into a self-registering conformance suite, and the WorkTracker port grows by per-concern sub-interfaces composed by embedding — extension is by new file, not shared-line append (AP-27, sharpening AP-23); the lone shared embed line falls to the recovery loop |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/conformance, internal/worktracker/localfs |

**Goal:** The store's integrity claims become executable evidence: interleave, crash-injection, parallel-create, and concurrent-append suites join the conformance kit and run in the race-enabled build.

**Current behavior:** Atomicity and identifier discipline exist as implementation behavior with unit coverage, but no adversarial suite proves them under interleaving, crashes, or process concurrency.

**Desired behavior:** An interleave suite shows every post-write read on a decision path reflecting the write. A crash-injection suite across the write sequence's cut points yields zero torn or lost state. A multi-process parallel-create stress yields zero identifier collision or reuse. A concurrent-append stress loses zero log entries. All four ride the conformance kit.

**Scope:**
- the interleave (read-your-writes) suite
- crash injection across the atomic-write cut points
- parallel-create and concurrent-append stress harnesses

**Out of scope:**
- networked-filesystem semantics (out of the governing contract's scope)

**Acceptance criteria:**
- [ ] the interleave suite reports read-your-writes in every case
- [ ] crash injection at each cut point leaves items wholly old or wholly new with zero losses
- [ ] a multi-process create stress completes with zero collision and zero reuse
- [ ] a two-writer append stress completes with zero lost entries

**Key interfaces:**
- the conformance-kit fault rows — the executable form of the store's integrity invariants

**Traces:** prd:local-work-tracker/FR-14 · prd:local-work-tracker/UB-4 · prd:local-work-tracker/UB-5 · design:local-work-tracker#DINV-2 · design:local-work-tracker#DINV-3 · design:local-work-tracker#DINV-4 · design:local-work-tracker#DINV-5

### T10 — Prove the medium: hermetic lifecycle, capacity, and survival

| Field | Value |
|---|---|
| **Blocked by** | T7, T8, T9 |
| **Size band** | 120–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/worktracker/conformance, cmd/carina |

**Goal:** The finished medium is proven as a whole: a full lifecycle runs hermetically, queries hold their latency bounds at the capacity target, and the backlog survives factory removal.

**Current behavior:** Individual behaviors pass their suites, but no end-to-end, capacity, or survival evidence exists.

**Desired behavior:** A complete lifecycle — create, signal, link, comment, record, transition, query, close — completes with zero network syscalls. List, search, ready, and triage meet the governing contract's latency bounds on a ten-thousand-item fixture. Deleting the factory footprint leaves the store intact and readable. Every defect across a fixture sweep names its full quadruple.

**Scope:**
- the hermetic end-to-end lifecycle check with a syscall trace
- the capacity fixture and timed query suite
- the survival check and the defect-quadruple sweep

**Out of scope:**
- performance tuning beyond meeting the stated bounds (an index remains a measured-need decision)

**Acceptance criteria:**
- [ ] the full lifecycle completes with zero network syscalls in the trace
- [ ] list, search, ready, and triage meet the stated latency bounds on the ten-thousand-item fixture
- [ ] deleting the factory footprint leaves every work item readable and the kit green
- [ ] a defect sweep shows file, operation, reason, and remedy on every failure case

**Key interfaces:**
- the hermetic end-to-end harness and the capacity fixture — the medium's proof surface

**Traces:** prd:local-work-tracker/FR-17 · prd:local-work-tracker/FR-18 · prd:local-work-tracker/UB-6 · design:local-work-tracker#DINV-7 · design:local-work-tracker#DINV-8 · design:local-work-tracker#3.13

## Coverage

| Requirement | Tasks |
|---|---|
| prd:local-work-tracker/FR-1 | T1 |
| prd:local-work-tracker/FR-2 | T1 |
| prd:local-work-tracker/FR-3 | T2 |
| prd:local-work-tracker/FR-4 | T2 |
| prd:local-work-tracker/FR-5 | T3 |
| prd:local-work-tracker/FR-6 | T4 |
| prd:local-work-tracker/FR-7 | T5 |
| prd:local-work-tracker/FR-8 | T6 |
| prd:local-work-tracker/FR-9 | T3 |
| prd:local-work-tracker/FR-10 | T7 |
| prd:local-work-tracker/FR-11 | T7 |
| prd:local-work-tracker/FR-12 | T8 |
| prd:local-work-tracker/FR-13 | T8 |
| prd:local-work-tracker/FR-14 | T9 |
| prd:local-work-tracker/FR-15 | T1, T7 |
| prd:local-work-tracker/FR-16 | T8 |
| prd:local-work-tracker/FR-17 | T1, T10 |
| prd:local-work-tracker/FR-18 | T1, T9, T10 |
| prd:local-work-tracker/UB-1 | T1 |
| prd:local-work-tracker/UB-2 | T6 |
| prd:local-work-tracker/UB-3 | T6 |
| prd:local-work-tracker/UB-4 | T9 |
| prd:local-work-tracker/UB-5 | T9 |
| prd:local-work-tracker/UB-6 | T10 |
