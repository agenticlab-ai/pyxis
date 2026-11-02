# Feature draft — draft, review, and lock a feature's PRD and tech spec together — plan 002

| Field | Value |
|---|---|
| **Feature** | feature-draft |
| **PRD** | docs/prds/feature-draft |
| **Design** | docs/tech-design/feature-draft-v1.md |
| **Plan version** | 002 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Feature draft — draft, review, and lock a feature's PRD and tech spec together |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Authorize native requirement links in the memory reconciler

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 180–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/memory/reconcile/prd.go · internal/memory/reconcile/analyze.go · internal/memory/reconcile/testdata/native |

**Goal:** A decision written natively in memory may serve a requirement claim that was itself written natively, while every claim captured from a PRD ledger keeps document-declared coverage.

**Current behavior:** The reconciler authorizes a serves edge only through PRD ledgers and the design index, so a native decision serving a native claim is reported as unauthorized coverage.

**Desired behavior:** A claim whose subject maps to no PRD ledger, at the trusted base or the candidate, is native; a native decision's serves edge to an active native claim is authorized. A serves edge from a native decision to a ledger-mapped claim stays refused. The evaluator version increments, trusted baselines replay under their own evaluator, and the rule applies only once it is at the trusted base.

**Scope:**
- the native-claim predicate over PRD ledger mapping
- the authorization branch for native serves edges
- the evaluator version increment and historical replay

**Out of scope:**
- every other reconciliation finding, whose classification stays byte-identical

**Acceptance criteria:**
- [ ] a native decision serving a native claim passes the memory gate with no finding
- [ ] a native decision serving a ledger-mapped claim is refused as unauthorized coverage
- [ ] a proposal that adds the rule and relies on it in the same change is refused by the trusted-base evaluator
- [ ] the live corpus reproduces its debt ratchet exactly under the new evaluator

**Key interfaces:**
- the native-claim predicate
- the serves authorization used by the reconciler's coverage check

**Traces:** prd:feature-draft/FR-29 · prd:feature-draft/NR-4 · prd:feature-draft/UB-6 · prd:feature-draft/ABUSE-4 · design:feature-draft-v1#3.10 · design:feature-draft-v1#DINV-5 · design:feature-draft-v1#DINV-6

### T2 — Compile a draft's PRD and its documents into records

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/compile/prd.go · internal/draft/compile/document.go · internal/draft/compile/testdata/prd |

**Goal:** A draft's PRD working file and its other documents compile in process into native memory records, deterministically.

**Current behavior:** Only the reconciler turns a folded PRD ledger into claims, and it tags them as document captures.

**Desired behavior:** The compiler reads the PRD working file in the PRD template's grammar and maps each requirement row to the claim kind the reconciler's mapping assigns, each Q row to a Question whose Resolution cell becomes its Answer, and each working file to a native literal Document holding its exact text. It computes the draft digest over the contract files. Compiled records carry no source-capture or PRD-ledger provenance. Everything except identity and issuance time is a pure function of the working files and the mainline.

**Scope:**
- the PRD working-file reader and the row-to-claim mapping reuse
- Q rows with their Resolution cells
- native literal Documents for every working file, and the draft digest

**Out of scope:**
- the tech spec, problems, and reviews (T3); writing records to disk (T7)

**Acceptance criteria:**
- [ ] compiling the same working files against the same mainline twice gives identical record content
- [ ] each PRD row kind in the fixture maps to the claim kind the reconciler assigns to the same row
- [ ] no compiled record carries a source-capture or PRD-ledger tag or a source-file provenance
- [ ] an empty Resolution cell yields an open Question and a filled one yields its Answer

**Key interfaces:**
- the in-process compile entry point shared by every reader
- the draft digest

**Traces:** prd:feature-draft/FR-3 · prd:feature-draft/FR-4 · prd:feature-draft/FR-29 · design:feature-draft-v1#3.6 · design:feature-draft-v1#DINV-2

### T3 — Compile the tech spec, requirement problems, reviews, and requirement changes

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/compile/techspec.go · internal/draft/compile/problems.go · internal/draft/compile/changes.go · internal/draft/compile/testdata/techspec |

**Goal:** The rest of a draft compiles in process: tech-spec decisions serving named requirements, requirement problems with their rulings, review reports with the digest they read, and the list of requirement changes made during the draft.

**Current behavior:** Design decisions reach memory only through a merged design document and the design index.

**Desired behavior:** Each tech-spec decision compiles to a Decision serving the requirements its Serves line names. Each requirement problem compiles to a Question, and its ruling to an Answer carrying the author, reason, and triggering problem. Each review report compiles to a Document recording the draft digest it read. From the draft branch's history, every requirement row changed after it first appeared is listed with the ruling that names it, or marked unruled.

**Scope:**
- tech-spec decisions and their Serves lines
- requirement problems and rulings
- review reports with their recorded digest
- the requirement-change list from the draft branch's history

**Out of scope:**
- refusing any edit: unruled changes are listed, never refused

**Acceptance criteria:**
- [ ] a decision whose Serves line names FR-3 and UB-2 compiles to a Decision serving exactly those two claims
- [ ] a ruling compiles to an Answer carrying its author, reason, and triggering problem
- [ ] a requirement row edited after its first commit with no ruling appears in the change list marked unruled
- [ ] a review report compiles recording the draft digest it names

**Key interfaces:**
- the change list consumed by the approval request (T7)
- the Serves-line grammar of the tech-spec working file

**Traces:** prd:feature-draft/FR-4 · prd:feature-draft/FR-9 · prd:feature-draft/FR-18 · prd:feature-draft/UB-1 · prd:feature-draft/ABUSE-1 · design:feature-draft-v1#3.6 · design:feature-draft-v1#DINV-11

### T4 — Judge readiness over the draft's own content

| Field | Value |
|---|---|
| **Blocked by** | T2, T3 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/memory/cli/ready.go · internal/draft/readiness/content.go · internal/draft/readiness/testdata/content |

**Goal:** `memory ready <feature>` refuses a draft whose own content is incomplete and names every unmet condition.

**Current behavior:** No readiness verdict exists for a draft.

**Desired behavior:** The verb compiles the draft in process and refuses while any open question or requirement problem lacks an answer, any Must requirement lacks a serving tech-spec decision, any requirement lacks a fit criterion or verification method, any part is unreadable, or the last PRD and tech-spec reviews did not read the version's digest or leave a blocking finding without a disposition. It prints one closed verdict naming each condition, item, and remedy, and exits non-zero when not ready. Staleness follows the design's draft digest, which covers the PRD and the tech spec: a brainstorm record edited after the last reviews does not make them stale, a scope the operator kept for UB-2 on 2026-09-24.

**Scope:**
- the answered-questions, Must-coverage, fit-criterion, and unreadable-part conditions
- the review-binding condition over recorded digests and finding dispositions
- the closed verdict and its exit codes

**Out of scope:**
- cross-feature and structural conditions (T5)

**Acceptance criteria:**
- [ ] one fixture per condition is refused naming exactly that condition, item, and remedy
- [ ] a fixture failing every condition names all of them in one verdict
- [ ] a draft changed after its last reviews is refused until both reviews record the new digest
- [ ] a ready fixture exits zero with an empty unmet list

**Key interfaces:**
- the readiness verdict schema
- the `memory ready` verb and its exit codes

**Traces:** prd:feature-draft/FR-11 · prd:feature-draft/FR-12 · prd:feature-draft/FR-13 · prd:feature-draft/FR-14 · prd:feature-draft/FR-16 · prd:feature-draft/FR-40 · prd:feature-draft/UB-2 · prd:feature-draft/UB-7 · prd:feature-draft/ABUSE-3 · design:feature-draft-v1#3.8 · design:feature-draft-v1#DINV-3

### T5 — Judge readiness over links, currency, and structure

| Field | Value |
|---|---|
| **Blocked by** | T4 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/readiness/links.go · internal/draft/readiness/structure.go · internal/draft/readiness/testdata/links · .github/workflows/draft-ready.yml |

**Goal:** Readiness also refuses a draft that depends on superseded content, cites what does not resolve, misses a required PRD section, or holds an inconsistent link graph, with one result wherever it runs.

**Current behavior:** These checks exist only in the contract gate, over document machine faces.

**Desired behavior:** The verdict adds four conditions: no part depends on another feature's superseded or retired records; every citation resolves against memory or the document corpus; the PRD holds every section its template requires, §0 and contract readiness included; and requirement-to-decision links have no dangling target, broken supersession, or cycle. The same draft version and mainline give a byte-identical verdict locally, in hosted checks, and at the lock. The hosted readiness check runs on every pull request: it judges a draft pull request's head and passes, naming no draft, on a pull request that holds none, so the required check never blocks other proposals.

**Scope:**
- currency against superseded and retired records
- citation resolution across memory and documents
- PRD template sections and link-graph consistency
- the hosted readiness check on every pull request, and local, hosted, and lock parity

**Out of scope:**
- the document-pipeline contract gate, which stays for ledger features

**Acceptance criteria:**
- [ ] one fixture per condition is refused naming exactly that condition
- [ ] a draft citing a retired requirement of another feature is refused naming the citation
- [ ] the parity matrix gives identical verdicts in all three places for every fixture
- [ ] the hosted check judges a draft pull request's head and passes, naming no draft, on a pull request that holds none

**Key interfaces:**
- the readiness verdict schema, extended with the four conditions

**Traces:** prd:feature-draft/FR-38 · prd:feature-draft/FR-39 · prd:feature-draft/FR-41 · prd:feature-draft/FR-42 · prd:feature-draft/NFR-3 · design:feature-draft-v1#3.8 · design:feature-draft-v1#3.9 · design:feature-draft-v1#DINV-3

### T6 — Open a draft and report each feature's contract state

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 180–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | cmd/pyxis/draft.go · internal/draft/lifecycle/open.go · internal/draft/lifecycle/status.go · internal/config · the draft working-file templates |

**Goal:** `pyxis draft open <feature>` starts a new feature as a draft, and `pyxis draft status` reports every feature's contract state.

**Current behavior:** New features start as separate PRD and design documents in separate proposals.

**Desired behavior:** Opening creates the draft branch under the configured prefix, scaffolds the working files under the configured drafts root from the templates, and opens a code-host draft pull request; a feature that already has a PRD ledger is refused, since existing features are not migrated, and a draft-born feature that already has a locked version is refused, naming `draft reopen`. Whoever runs it, the operator included, the draft pull request is opened under the configured agent identity, because the code host never lets a pull request's author approve it; with no agent identity configured, opening refuses, naming the setup step. Status reports each feature as drafting its first version, locked at version N, drafting version N+1 while N binds, or still in the document pipeline. Nothing reaches the mainline before a lock.

**Scope:**
- the open verb, its branch and draft pull request, and the scaffold
- the refusal of a feature with a PRD ledger, and the agent identity that authors every draft pull request
- the closed configuration keys for the drafts root and branch prefix
- the status verb and its four states

**Out of scope:**
- lock preparation and the lock (T7, T8)

**Acceptance criteria:**
- [ ] opening a new feature yields one branch, one draft pull request, and the scaffold, with zero mainline commits
- [ ] opening a feature that has a PRD ledger is refused naming the day-one rule, and one that already has a locked version is refused naming the reopen command; a draft the operator opens has its pull request authored by the agent identity; with no agent identity configured, opening refuses naming the setup step and creates nothing
- [ ] status reports each of the four states on fixtures
- [ ] an unknown configuration key under the draft block is refused

**Key interfaces:**
- the draft verbs' argument grammar and exit codes
- the drafts-root and branch-prefix configuration keys

**Traces:** prd:feature-draft/FR-1 · prd:feature-draft/FR-2 · prd:feature-draft/FR-5 · prd:feature-draft/FR-7 · prd:feature-draft/FR-17 · design:feature-draft-v1#3.1 · design:feature-draft-v1#3.9 · design:feature-draft-v1#DINV-1

### T7 — Prepare the lock

| Field | Value |
|---|---|
| **Blocked by** | T1, T3, T4, T5, T6, T25 |
| **Size band** | 240–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/lock/prepare.go · internal/draft/lock/request.go · internal/draft/lock/testdata/prepare |

**Goal:** `pyxis draft prepare-lock <feature>` turns a ready draft into the exact version the operator approves.

**Current behavior:** No step turns working files into landed records.

**Desired behavior:** The step rebases the draft onto the current mainline, re-runs readiness there, compiles once, writes the records with evidence taken at that tree, stores each file's exact text, removes the working files in the same commit, renders the approval request as the pull request's description, and requests the operator's review. The approval request, rendered as the pull request's description, shows the PRD's §0, the readiness result, every requirement change with its ruling and the ruling's declared author, or marked unruled, and every declined blocking review finding with its ruling and declared author. It stays within the code host's description limit, linking whatever does not fit, and links the complete PRD and tech spec, whose exact texts the same pull request adds as literal Documents, so the approval view, the description with those Documents, misses no part of the contract. A later round reverts the preparation commit. A failed step leaves the working files unchanged and names the step.

**Scope:**
- rebase, readiness re-run, one compile, and the record write
- working-file removal and the revert path
- the approval request as the pull request description

**Out of scope:**
- reading the approval and the lock record (T8, T9)
- a reopened draft's differences and lock guards (T21, T22)

**Acceptance criteria:**
- [ ] the preparation commit adds only records and removes every working file, and the memory gate passes at the landing tree
- [ ] reverting the preparation commit restores the working files byte for byte with the same draft digest
- [ ] the approval request lists every requirement change with its ruling and the ruling's declared author, the unruled one marked, and every declined blocking finding with its ruling and declared author, and on a contract larger than the code host's description limit it stays within the limit and links the complete PRD and tech spec the pull request adds
- [ ] a draft whose mainline moved since its last readiness pass cannot finish preparation without the re-run

**Key interfaces:**
- the prepare-lock verb and its exit codes
- the approval request rendering

**Traces:** prd:feature-draft/FR-2 · prd:feature-draft/FR-3 · prd:feature-draft/FR-5 · prd:feature-draft/FR-18 · prd:feature-draft/FR-24 · prd:feature-draft/FR-29 · prd:feature-draft/UB-1 · prd:feature-draft/UB-8 · prd:feature-draft/UB-9 · design:feature-draft-v1#3.7 · design:feature-draft-v1#DINV-1 · design:feature-draft-v1#DINV-10

### T8 — Read the lock approval through a port and scope code ownership

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 160–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/approval/port.go · internal/adapters/approval/github · internal/adapters/approval/local · .github/CODEOWNERS |

**Goal:** A lock merges only on the operator's approval of the exact head, read through an approval port, with the operator owning the contract records.

**Current behavior:** Main requires no pull-request review, and admins are not bound.

**Desired behavior:** The approval port returns the operator's approval of a given head, with its capture line, or reports none. The code-host adapter reads the pull request's reviews; the local adapter reads fixtures for tests. The operator owns the contract record paths and the abandonment record's path, so a lock and an abandonment each need the operator's review, while a proposal adding neither keeps its merge policy, the merge lane included. The operator applies the branch protection named in the solution shape.

**Scope:**
- the approval port, the code-host adapter, and the local adapter
- code ownership of the contract record and abandonment record paths
- the branch-protection settings, applied by the operator

**Out of scope:**
- writing the lock record (T9)

**Acceptance criteria:**
- [ ] an approval of head H returns the operator and the capture line, or lineless when none is given
- [ ] a review of an older head, or by an agent account, returns no approval
- [ ] an abandonment proposal needs the operator's review, and a proposal adding no contract or abandonment record needs none
- [ ] the repository settings match the solution shape's branch protection (inspection)

**Key interfaces:**
- the approval port and its no-approval error

**Traces:** prd:feature-draft/FR-17 · prd:feature-draft/FR-45 · prd:feature-draft/FR-46 · prd:feature-draft/FR-47 · prd:feature-draft/UB-3 · prd:feature-draft/UB-11 · prd:feature-draft/ABUSE-2 · prd:feature-draft/ABUSE-8 · design:feature-draft-v1#3.9 · design:feature-draft-v1#3.14 · design:feature-draft-v1#DINV-4

### T9 — Record each lock and verify it from a fresh clone

| Field | Value |
|---|---|
| **Blocked by** | T8 |
| **Size band** | 140–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/lock/record.go · .github/workflows/lock-record.yml · internal/draft/lock/testdata/record |

**Goal:** Every lock leaves an annotated lock record on its merge commit that a fresh clone can read and verify.

**Current behavior:** A merge leaves no record of who approved it or of its capture line.

**Desired behavior:** On the mainline push of a lock merge, a job reads the approval through the port and writes the annotated lock record carrying the version, the approver in both roles, the capture line, the approved head, the readiness digest, the pull request, and the number of proposals since the draft opened. A non-lock merge writes none, and re-running the job converges. A fresh clone renders the locked version as of its lock record.

**Scope:**
- the mainline job and the lock-record writer
- the lock record's fields and its idempotency
- fresh-clone verification

**Out of scope:**
- offline authenticity of the approval (design Q3)

**Acceptance criteria:**
- [ ] a lock merge produces exactly one lock record carrying every field, the number of proposals since the draft opened included
- [ ] a non-lock merge produces none, and a second run of the job writes nothing
- [ ] a fresh clone renders the locked version after the draft's working state is deleted

**Key interfaces:**
- the lock record's name and fields

**Traces:** prd:feature-draft/FR-19 · prd:feature-draft/FR-45 · prd:feature-draft/NFR-4 · prd:feature-draft/EVT-5 · design:feature-draft-v1#3.9 · design:feature-draft-v1#DINV-9

### T10 — Reopen a locked feature from its locked text and abandon a draft

| Field | Value |
|---|---|
| **Blocked by** | T6, T7, T8, T9 |
| **Size band** | 180–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/lifecycle/reopen.go · internal/draft/lifecycle/abandon.go · internal/draft/lifecycle/testdata/reopen |

**Goal:** A locked feature changes only through a draft reopened from its locked text, and an abandoned draft leaves one reason record.

**Current behavior:** A locked contract changes through amendments and superseding designs.

**Desired behavior:** Reopening starts the next version's draft branch, its pull request authored by the agent identity as an opened draft's is, whose first commit carries the binding version's stored PRD and tech-spec text byte for byte, with empty problems and events files and no brainstorm or review files, so its draft digest equals the binding version's. It is refused while any task of the current plan is unfinished, naming each task and the remedy; a task of a merged plan that is not yet projected counts as unfinished. While the reopened draft is open, the binding version stays the one planning and execution read. Abandoning reduces the draft's pull request to one memory Learning stating why, which lands only with the operator's approval, and deletes the branch after that merge.

**Scope:**
- the reopen verb and its seed from the binding version's stored text
- the unfinished-task refusal, unprojected tasks included
- the abandon verb and its reason record

**Out of scope:**
- what a reopened draft's lock writes and refuses (T21, T22)
- migrating existing document-pipeline features

**Acceptance criteria:**
- [ ] with no agent identity configured, reopening refuses naming the setup step; otherwise reopening a locked fixture from the operator's command opens a pull request authored by the agent identity whose first commit carries the binding version's PRD and tech-spec text byte for byte, with empty problems and events files and no brainstorm or review files, and the draft digest equals the binding version's
- [ ] a reopen is refused naming each unfinished task, a merged plan's unprojected task included, and proceeds once each is finished or cancelled
- [ ] while a reopened draft is open, status reports drafting the next version while the current one binds, and planning reads only the current one
- [ ] abandoning lands exactly one Learning only with the operator's approval, never an agent account's, and deletes the branch after that merge

**Key interfaces:**
- the reopen and abandon verbs

**Traces:** prd:feature-draft/FR-20 · prd:feature-draft/FR-21 · prd:feature-draft/FR-43 · design:feature-draft-v1#3.1 · design:feature-draft-v1#3.14 · design:feature-draft-reopen#3.1 · design:feature-draft-reopen#DINV-1

### T11 — Plan a draft-born feature from its locked version

| Field | Value |
|---|---|
| **Blocked by** | T4, T5, T9, T24, T25 |
| **Size band** | 160–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/plangate/plan/precheck.go · plan-new-guide · internal/plangate/plan/testdata/draftborn |

**Goal:** plan-new plans a draft-born feature from its binding locked version's readiness, never from the contract gate, and refuses a feature that has no locked version.

**Current behavior:** plan-new's precondition calls the contract gate, which refuses a feature that has no PRD ledger.

**Desired behavior:** For a feature with no PRD ledger, the planning precondition is `memory ready` on the latest locked version. A feature with no locked version is refused, while a feature whose reopened draft is open plans from its binding version. A plan generated from it names its lock record in its Basis header row, with its PRD and Design rows naming the feature's memory subjects, so every task it delivers traces that locked version. A document-pipeline feature keeps the contract gate. The overrides of sibling rows apply only to features with no PRD ledger. Projection and the executor are unchanged.

**Scope:**
- the draft-born branch of the planning precondition
- the Basis, PRD, and Design rows the guide writes for a draft-born plan
- the plan-new guide's draft-born steps

**Out of scope:**
- the plan header grammar (T24)
- plangate's governing inputs and trace resolution for draft-born plans (T12)

**Acceptance criteria:**
- [ ] a locked draft-born fixture plans without calling the contract gate
- [ ] a draft-born fixture with no lock, and a locked fixture whose cited requirement another feature has since retired, are each refused, while a feature with an open reopened draft plans from its binding version
- [ ] a document-pipeline fixture still calls the contract gate
- [ ] a plan generated from a locked fixture names its lock record in its Basis row, and its PRD and Design rows name the feature's memory subjects

**Key interfaces:**
- the planning precondition's feature-kind branch

**Traces:** prd:feature-draft/FR-6 · prd:feature-draft/FR-25 · prd:feature-draft/FR-26 · prd:feature-draft/FR-31 · prd:feature-draft/FR-44 · prd:feature-draft/NR-1 · prd:feature-draft/ABUSE-6 · prd:feature-draft/ABUSE-7 · design:feature-draft-v1#3.11 · design:contract-gate-v2#3.10

### T12 — Fold a draft-born plan against its locked version in memory

| Field | Value |
|---|---|
| **Blocked by** | T2, T11, T24 |
| **Size band** | 200–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/plangate/plan/gates.go · internal/plangate/plan/traces.go · internal/plangate/plan/testdata/draftborn |

**Goal:** Plangate folds a draft-born plan against the locked version's records in memory, with no PRD ledger or design document.

**Current behavior:** Plangate loads the PRD projection and the design document from files and resolves traces and coverage against them.

**Desired behavior:** For a draft-born feature, plangate's governing inputs are the locked requirements and tech-spec decisions read from memory as of the lock the plan's Basis names, in place of the PRD projection and the design document. Traces and coverage resolve against those records, so a requirement retired by a later version stays resolvable for a plan generated from an earlier one, while a plan generated from the later version that traces it is refused. A plan that leaves a requirement neither covered nor deferred with a reason, or that breaks a projection rule, is refused. A projection refusal leaves the lock intact and is reported naming the disagreeing rule.

**Scope:**
- the draft-born governing inputs read from memory as of the Basis lock, replacing both document loaders
- trace resolution and coverage over the locked records
- projection rules on draft-born plans

**Out of scope:**
- projection and the executor, which stay unchanged
- keeping draft-born plans out of memory (T23)

**Acceptance criteria:**
- [ ] a draft-born plan fixture with no PRD ledger or design document folds green end to end, its requirements and decisions read from memory as of its Basis lock
- [ ] a draft-born plan tracing a nonexistent requirement, or leaving a requirement uncovered and undeferred, is refused naming the trace or the requirement
- [ ] after version 2 retires a requirement that a plan based on version 1 traces, that plan still folds clean, and a plan based on version 2 tracing it is refused
- [ ] a projection refusal of a draft-born plan leaves the locked version unchanged

**Key interfaces:**
- plangate's governing-input loader for draft-born features

**Traces:** prd:feature-draft/FR-15 · prd:feature-draft/FR-37 · prd:feature-draft/FR-44 · prd:feature-draft/UB-10 · design:feature-draft-v1#3.11 · design:feature-draft-reopen#3.8 · design:feature-draft-reopen#DINV-5

### T13 — Author drafts with the stage skills

| Field | Value |
|---|---|
| **Blocked by** | T3, T6 |
| **Size band** | 60–140 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | feature-brainstorm-guide · prd-new-guide · tech-design-brainstorm-guide · tech-design-new-guide · the draft working-file templates |

**Goal:** The authoring skills write a new feature's draft working files, and tech-spec sessions record requirement problems instead of editing requirements.

**Current behavior:** Each authoring skill writes its own document class and lands it in its own proposal.

**Desired behavior:** For a new feature, the brainstorm, PRD, tech-spec brainstorm, and tech-spec skills read and write only that draft's working files. A tech-spec session that finds a requirement wrong, missing, or unbuildable records a requirement problem and leaves the row unchanged; a product ruling from the operator, or Argus within its grant, applies the change with its author, reason, and problem. A draft-born tech spec carries its concrete layouts itself: the tech-spec skill writes no separate solution shape, which the document gate would refuse without a paired design.

**Scope:**
- the four authoring guides' draft branches
- the requirement-problem and ruling steps
- the working-file templates

**Out of scope:**
- the reviewers (T14) and Argus (T15)

**Acceptance criteria:**
- [ ] each authoring skill run on a new-feature fixture writes only that draft's working files
- [ ] a tech-spec fixture that finds a requirement defect produces one requirement problem and leaves the row byte-identical
- [ ] a ruling fixture applies its change with the author, reason, and triggering problem
- [ ] the tech-spec skill run on a new-feature fixture writes its concrete layouts into the tech spec and no solution-shape document

**Key interfaces:**
- the requirement-problem and ruling grammar in the working files

**Traces:** prd:feature-draft/FR-8 · prd:feature-draft/FR-9 · prd:feature-draft/FR-10 · prd:feature-draft/FR-33 · prd:feature-draft/FR-34 · design:feature-draft-v1#3.13 · design:feature-draft-reopen#3.2

### T14 — Review drafts with both reviewers

| Field | Value |
|---|---|
| **Blocked by** | T4, T6 |
| **Size band** | 60–140 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | prd-review-guide · tech-design-review-guide · the draft review template |

**Goal:** The PRD reviewer and the tech-spec reviewer review a draft in any round, and each report records the version it read.

**Current behavior:** Each reviewer reviews one merged-or-proposed document per moment.

**Desired behavior:** For a draft-born feature, both reviewers take the draft's PRD and tech spec as subjects in any round, write their reports as the draft's working files, and record the draft digest they read, so readiness can refuse a lock whose last reviews are stale.

**Scope:**
- the two reviewer guides' draft subjects
- the report's recorded draft digest

**Out of scope:**
- readiness's review-binding condition (T4)

**Acceptance criteria:**
- [ ] each reviewer completes a verdict on a draft fixture and writes its report as a working file
- [ ] each report records the draft digest it read, and a later change makes readiness report it stale

**Key interfaces:**
- the review report's draft-digest field

**Traces:** prd:feature-draft/FR-16 · prd:feature-draft/FR-32 · prd:feature-draft/UB-2 · design:feature-draft-v1#3.13

### T15 — Drive drafts with Argus

| Field | Value |
|---|---|
| **Blocked by** | T7, T8, T13, T14 |
| **Size band** | 80–160 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | argus-drive-guide · the argus skill |

**Goal:** Argus runs a draft's rounds and requests its lock, while the approval stays the operator's.

**Current behavior:** Argus drives features stage by stage through separate proposals.

**Desired behavior:** For a new feature, Argus opens the draft, runs rounds through the draft-aware skills, rules on requirement problems only where its authority contract grants that act, prepares the lock, and requests the operator's approval; an approval attempted by Argus is refused.

**Scope:**
- Argus's draft drive and its lock request
- the refusal of an Argus approval

**Out of scope:**
- Argus's problem and delivery ledgers, which stay as today

**Acceptance criteria:**
- [ ] an Argus drive fixture reaches a prepared lock with an approval request to the operator
- [ ] an approval attempted by Argus is refused and the request stays with the operator

**Key interfaces:**
- the drive guide's draft stage entries

**Traces:** prd:feature-draft/FR-10 · prd:feature-draft/FR-35 · prd:feature-draft/UB-3 · design:feature-draft-v1#3.13

### T16 — Show other open drafts touching the same requirements or decisions

| Field | Value |
|---|---|
| **Blocked by** | T2, T3, T6 |
| **Size band** | 160–240 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/inflight/notice.go · internal/draft/inflight/testdata |

**Goal:** While a draft is open, its readers see a non-blocking notice of other open drafts touching the same requirements or decisions.

**Current behavior:** Memory's in-flight report reads records on other branches, and drafts hold none until lock preparation.

**Desired behavior:** The notice compiles other open drafts' working files in process, bounded to the open draft pull requests, and lists those touching the same requirements or decisions. It never blocks readiness or the lock.

**Scope:**
- the in-process compile of other open drafts
- the notice and its bound to open draft pull requests

**Out of scope:**
- overlap tracking or forced re-checks, which the PRD excludes

**Acceptance criteria:**
- [ ] a two-draft fixture sharing a requirement, and one sharing a tech-spec decision, each show each draft the other
- [ ] the notice never changes a readiness verdict

**Key interfaces:**
- the notice's listing of open drafts and shared items

**Traces:** prd:feature-draft/FR-23 · design:feature-draft-v1#3.13 · design:feature-draft-v1#A-19

### T17 — Emit and retain draft events

| Field | Value |
|---|---|
| **Blocked by** | T3, T6, T7, T10 |
| **Size band** | 180–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/events/log.go · internal/draft/events/pending.go · internal/draft/events/testdata · internal/draft/lifecycle/open.go · internal/draft/lifecycle/reopen.go · internal/draft/lifecycle/abandon.go · internal/draft/lock/prepare.go · internal/draft/compile/changes.go |

**Goal:** Every draft step emits its event, and the events are retained with the lock or the abandonment record.

**Current behavior:** No draft events exist.

**Desired behavior:** Opening, a failed step, a requirement change, a refused lock, and a reopen each append their event with its fields to the draft's event log, which compiles into memory at lock preparation or rides the abandonment record; the lock record carries the lock event. Each event is emitted at its own step: the open, reopen, and abandon verbs, lock preparation, and the change listing, which emits one requirement-change event per ruling that changes a requirement, keyed by the ruling's commit so a repeated preparation emits nothing twice; abandonment runs the listing too. The checkpoint inventory (T26) reads them. This resolves the design's open question on where a draft's events live before its lock, which the design assigns to implementation, by adopting its proposed working-file event log. A failed step changes no working file: its event is written to the draft's pending-event ref, which travels with the draft branch, and is appended to the log by the next step that succeeds, from any clone, or carried by the abandonment record.

**Scope:**
- the event log and its fields
- the emission hooks in the open, reopen, and abandon verbs, lock preparation, and the change listing
- retention at lock preparation and abandonment

**Out of scope:**
- the lock event's writer (T9)

**Acceptance criteria:**
- [ ] each event is emitted with its fields on its fixture
- [ ] a locked fixture's events are in memory, and an abandoned fixture's ride its reason record
- [ ] an injected step failure leaves every working file byte-identical, and its event reaches the log after the next step that succeeds, including a step run from a fresh clone
- [ ] a ruled requirement change followed by abandonment emits its requirement-change event once, and preparing the lock twice emits it once

**Key interfaces:**
- the event names and fields

**Traces:** prd:feature-draft/EVT-1 · prd:feature-draft/EVT-2 · prd:feature-draft/EVT-3 · prd:feature-draft/EVT-4 · prd:feature-draft/EVT-6 · prd:feature-draft/UB-9 · design:feature-draft-v1#3.4 · design:feature-draft-v1#DINV-10

### T18 — Describe the feature draft in the agent instructions, pipeline map, and domain model

| Field | Value |
|---|---|
| **Blocked by** | T16, T17, T19 |
| **Size band** | 20–80 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | AGENTS.md · pipeline-map · docs/domain-model.md · docs/techspec.md |

**Goal:** Agents learn from the repository's own surfaces that new features are contracted through drafts.

**Current behavior:** The agent instructions, pipeline map, and domain model describe per-stage documents frozen at their own merges.

**Desired behavior:** The three surfaces describe the feature draft for new features and keep the per-stage document rules for existing documents. The architecture rollup's header states that it is not a feature's tech spec. The operator confirms the agent-instruction changes in review.

**Scope:**
- the agent instructions, pipeline map, and domain model
- the architecture rollup's header note

**Out of scope:**
- the product vision's §4 note, which the operator writes

**Acceptance criteria:**
- [ ] each surface names the feature draft for new features and the document rules for existing documents (inspection)
- [ ] the architecture rollup's header disambiguates it from a feature's tech spec (inspection)

**Key interfaces:**
- none — documentation only

**Traces:** prd:feature-draft/FR-36 · design:feature-draft-v1#3.12

### T19 — Prove the draft end to end from a fresh clone

| Field | Value |
|---|---|
| **Blocked by** | T7, T8, T9, T10, T11, T12, T13, T14, T21, T22, T23, T25 |
| **Size band** | 200–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/e2e · internal/draft/e2e/testdata |

**Goal:** One fixture feature runs the whole draft process from a fresh clone, and the abuse and failure matrices hold.

**Current behavior:** No end-to-end draft fixture exists.

**Desired behavior:** A fixture feature goes from an opened draft through rounds that revise every part — PRD, tech spec, brainstorm records, and review reports — at least twice, a PRD revision after a tech-spec revision included (FR-2's fit also names a plan revision, which a draft cannot hold because the plan follows the lock; the operator kept that row as it is on 2026-09-24), then a refused lock, a ruled requirement change, lock preparation, the operator's approval, the lock record, a plan generated from the locked version, projection, and executor pickup, then a reopen that retires one requirement and locks the next version by its differences, reproduced from a fresh clone. Every abuse fixture is refused, except the silent rewrite, which appears marked unruled in the approval request, and every injected step failure leaves the draft intact and names the step.

**Scope:**
- the end-to-end fixture from a fresh clone
- the abuse matrix and the failure-injection matrix

**Out of scope:**
- the document pipeline's closure (T20)

**Acceptance criteria:**
- [ ] the end-to-end fixture — every part revised in at least two rounds with no revision refused, and its reopen — passes from a fresh clone with zero executor changes
- [ ] the abuse matrix is refused in every case, the silent rewrite surfaced as unruled
- [ ] every injected step failure leaves the working files byte-identical and names the step
- [ ] every refusal in both matrices names its condition, item, and next action

**Key interfaces:**
- none — the fixture drives the verbs of T4 to T12, T21, and T22

**Traces:** prd:feature-draft/FR-2 · prd:feature-draft/UB-9 · prd:feature-draft/NR-1 · prd:feature-draft/NFR-1 · prd:feature-draft/NFR-2 · design:feature-draft-v1#3.15 · design:feature-draft-v1#A-17 · design:feature-draft-v1#DINV-10

### T20 — Close the document pipeline to new contract documents

| Field | Value |
|---|---|
| **Blocked by** | T13, T14, T15, T16, T17, T18, T19, T26 |
| **Size band** | 120–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate/gate/reg_check_contract_corpus_closed.go · internal/docgate/gate/testdata/closed |

**Goal:** Once the draft process is proven, the document gate refuses every new contract document and names the feature draft as the path.

**Current behavior:** The document gate accepts new PRD ledger entries, designs, brainstorm records, and review reports.

**Desired behavior:** A self-registered merge-base check refuses an added PRD ledger entry, technical design, brainstorm record, or review report, with the remedy naming the feature draft, and accepts added plan documents. The document pipeline's other checks keep applying to existing documents and never to a draft-born feature, and existing features keep their merged documents as their binding contract.

**Scope:**
- the closure check and its remedy
- the acceptance of plan documents

**Out of scope:**
- any change to existing documents, which keep their bytes and checks

**Acceptance criteria:**
- [ ] each added contract-document fixture is refused with the feature-draft remedy
- [ ] an added plan document is accepted
- [ ] an edit to an existing document still triggers the existing immutability refusal

**Key interfaces:**
- the closure check's verb and remedy

**Traces:** prd:feature-draft/FR-27 · prd:feature-draft/FR-30 · prd:feature-draft/UB-5 · prd:feature-draft/NR-3 · prd:feature-draft/NR-5 · design:feature-draft-v1#3.12 · design:feature-draft-v1#DINV-7

### T21 — Lock a reopened draft by its differences

| Field | Value |
|---|---|
| **Blocked by** | T7, T10 |
| **Size band** | 220–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/lock/diff.go · internal/draft/lock/testdata/diff |

**Goal:** A reopened draft's lock writes exactly what differs from the binding version, and each version renders in full as of its lock.

**Current behavior:** Lock preparation writes every compiled unit as new; nothing compares a draft with a binding version.

**Desired behavior:** For a reopened draft, lock preparation compares each compiled unit with the binding version's active record at its address. A new unit is written. A changed unit supersedes the active record, where a unit counts as changed when its own content or any record it relates to changed, so a decision serving a reworded requirement is re-issued. An unchanged unit writes nothing. Each record compiled from a PRD or tech-spec row the draft no longer holds is revoked, a question's answer with it, and the literal PRD and tech-spec Documents are superseded when their text changed. Problems, brainstorm records, review reports, and events of earlier versions are never superseded or revoked. Readiness judges the binding records with the draft's differences applied.

**Scope:**
- the unit comparison against the binding version
- supersession, re-issuing a unit whose related record changed
- revocation of a removed row's records
- the untouched history of earlier versions

**Out of scope:**
- the reopened lock's refusals (T22)

**Acceptance criteria:**
- [ ] with a binding version that carries a problem, a brainstorm record, a review report, and an event, locking a reopened draft that rewords one requirement, removes one, and adds one writes exactly a superseding record, a Revocation for each record of the removed row, a new record, and a superseding PRD Document, writes nothing for unchanged rows, supersedes or revokes none of the history, and passes the memory gate at the landing tree
- [ ] a tech-spec decision whose text is unchanged but which serves a reworded requirement is re-issued to serve the new record, and readiness finds the requirement served
- [ ] rendering the feature as of each lock record shows that version's full contract, unchanged rows included

**Key interfaces:**
- the comparison's per-unit verdict: new, superseding, unchanged, or revoked

**Traces:** prd:feature-draft/FR-22 · design:feature-draft-reopen#3.6 · design:feature-draft-reopen#DINV-2 · design:feature-draft-reopen#DINV-3

### T22 — Guard a reopened draft's lock

| Field | Value |
|---|---|
| **Blocked by** | T21 |
| **Size band** | 140–220 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/lock/guard.go · internal/draft/readiness/numbering.go · internal/draft/lock/testdata/guard |

**Goal:** A reopened draft locks only from the binding version's text, only with a change, and never by reusing a number or ID.

**Current behavior:** Lock preparation checks readiness only; nothing ties a reopened draft to the text it started from.

**Desired behavior:** Lock preparation refuses a reopened draft whose first commit does not carry exactly the binding version's stored PRD and tech-spec text, naming the version that text matches, if any. It refuses one whose PRD and tech-spec text equal the binding version's, naming the abandon remedy, and writes no lock record. Readiness refuses a working file that reuses a brainstorm, review, or problem number the feature already used, naming the next free number, and a row that reuses an ID an earlier version retired. A proposal that changes a locked version's records outside a reopen is refused.

**Scope:**
- the first-commit text check
- the no-change refusal
- the number and ID reuse refusals
- the refusal of changes to locked records outside a reopen

**Out of scope:**
- the unit comparison (T21)

**Acceptance criteria:**
- [ ] a reopened draft that started from an older version, and one seeded by hand, are each refused naming the version the text matches, if any
- [ ] a reopened draft with no change to its PRD or tech-spec text is refused with the abandon remedy, and no lock record is written
- [ ] a reused brainstorm, review, or problem number, or a reused retired row ID, is refused naming the item and, for a number, the next free one
- [ ] a proposal changing a locked version's record outside a reopen is refused

**Key interfaces:**
- the reopened lock's refusal conditions in the readiness verdict

**Traces:** prd:feature-draft/FR-20 · prd:feature-draft/UB-4 · prd:feature-draft/ABUSE-5 · design:feature-draft-v1#DINV-9 · design:feature-draft-reopen#3.6 · design:feature-draft-reopen#3.7 · design:feature-draft-reopen#DINV-4 · design:feature-draft-reopen#DINV-6 · design:feature-draft-reopen#DINV-7

### T23 — Keep draft-born plans out of memory

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/memory/reconcile/source.go · internal/memory/reconcile/testdata/plans |

**Goal:** The reconciler never captures a draft-born feature's plan into memory, while document-pipeline plans stay captured.

**Current behavior:** The reconciler's source discovery captures every plan document.

**Desired behavior:** Source discovery skips the plan documents of a feature with no PRD ledger, so no draft-born plan is captured or reported missing. A document-pipeline feature's plans are still captured as today.

**Scope:**
- the plan-document exclusion in the reconciler's source discovery

**Out of scope:**
- plangate's handling of draft-born plans (T12)

**Acceptance criteria:**
- [ ] after reconciliation, memory holds no record or capture of a draft-born fixture's plan, and no finding asks for one
- [ ] a document-pipeline fixture's plan is still captured

**Key interfaces:**
- the reconciler's source-discovery rule for plan documents

**Traces:** prd:feature-draft/FR-3 · design:feature-draft-v1#3.11 · design:feature-draft-v1#DINV-8

### T24 — Carry the lock basis in a feature-neutral plan header

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 180–260 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/plangate/plan/parse.go · internal/plangate/plan/plan.go · internal/plangate/plan/gates.go · plan-template · internal/plangate/plan/testdata/header |

**Goal:** The plan header records a draft-born plan's lock basis, and a plan that no feature supplies folds and projects without feature-only fields.

**Current behavior:** The plan header's row set is closed and always requires PRD and Design rows naming documents, so a plan without a feature's documents cannot fold.

**Desired behavior:** The header gains a Basis row naming the lock record a draft-born plan was generated from, and an Intake row that names a plan no feature supplies. The feature-only rows — Feature, PRD, Design, and Basis — are required only of a feature's plan, and a non-feature plan names itself in its Intake row instead; plan-task identities and epic markers derive from whichever row names the plan, unchanged for feature plans: a draft-born plan's PRD and Design rows name the feature's memory subjects, a document-pipeline plan keeps its document paths, and a plan from a non-feature intake omits all three, citing its intent and decision in its task traces, with coverage checked only where a governing PRD exists. The plan model and the template carry the same rows. The same projection accepts every kind, and the handoff it renders carries no feature-only field.

**Scope:**
- the Basis and Intake rows, the feature-only rows' conditions, and identities derived from the naming row, in the parser, the plan model, and the template
- the fold branch for a plan with no governing PRD
- the handoff's freedom from feature-only fields

**Out of scope:**
- reading a draft-born plan's governing inputs from memory (T12)
- a non-feature intake itself, which the PRD defers

**Acceptance criteria:**
- [ ] a draft-born header with a Basis row and memory-subject PRD and Design rows parses, and a document-pipeline header parses unchanged
- [ ] a hand-built non-feature fixture — intent cited from an existing requirement, one decision, one plan — folds and projects through the same projection with no Feature, PRD, Design, or Basis row
- [ ] the non-feature fixture's projected issues carry the same fields as a feature plan's for the same tasks, with no feature-only field empty or faked
- [ ] an existing document-pipeline feature's merged plans fold to byte-identical machine faces and project identically before and after this change

**Key interfaces:**
- the plan header's row set and its feature-only conditions

**Traces:** prd:feature-draft/FR-26 · prd:feature-draft/FR-44 · prd:feature-draft/NR-5 · design:feature-draft-v1#3.11

### T25 — Accept rulings and declines only from their authority

| Field | Value |
|---|---|
| **Blocked by** | T3, T4 |
| **Size band** | 120–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/readiness/authority.go · internal/draft/readiness/testdata/authority |

**Goal:** A requirement ruling counts only from the operator or from Argus under a granting authority contract, and a declined blocking finding counts only with the operator's ruling.

**Current behavior:** The compiler records a ruling's declared author, and readiness counts any declined finding as dispositioned.

**Desired behavior:** Readiness accepts a product ruling whose declared author is the operator, or Argus where its authority contract grants that act. Any other ruling is refused: readiness names it, its requirement problem stays open, and its change is listed unruled. A blocking review finding counts as dispositioned when applied, or when declined with the operator's ruling; a decline without that ruling is refused. The check reads the declared author, which is not proof of who wrote the ruling: per the operator's 2026-09-24 ruling, the operator's approval of the exact version, whose request lists every ruling with its declared author, is what vouches for each one.

**Scope:**
- the ruling-authority check against the operator and Argus's authority contract
- the operator-ruling check on declined blocking findings

**Out of scope:**
- authenticating identities, which the lock approval does (T8)

**Acceptance criteria:**
- [ ] an operator ruling, and an Argus ruling under a granting authority contract, are accepted
- [ ] an Argus ruling without a grant, and any other agent's ruling, are refused naming the ruling, with its problem left open
- [ ] a blocking finding declined without the operator's ruling is refused, and one declined with it passes

**Key interfaces:**
- the authority conditions in the readiness verdict

**Traces:** prd:feature-draft/FR-10 · prd:feature-draft/FR-16 · design:feature-draft-v1#3.6 · design:feature-draft-v1#3.8

### T26 — Record the checkpoint inventory

| Field | Value |
|---|---|
| **Blocked by** | T17 |
| **Size band** | 100–180 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/draft/checkpoint/inventory.go · internal/draft/checkpoint/testdata |

**Goal:** When the first three draft-born features have locked and finished implementation, the checkpoint inventory is computed and kept in memory.

**Current behavior:** Nothing detects the post-launch checkpoint or records its inventory.

**Desired behavior:** The inventory verb reports the checkpoint as not yet due, naming what is missing, until the first three draft-born features have locked and their plans' tasks have all finished. Once due, it writes one checkpoint-inventory record to memory carrying the three features, the count of new files under the document corpus's PRD, tech-design, brainstorm, and review roots for them, and their reopens by confirmed cause, read from the retained events. Running it again writes nothing new.

**Scope:**
- the checkpoint trigger over locked, fully implemented draft-born features
- the inventory's fields and its record in memory

**Out of scope:**
- the adopt, iterate, or kill verdict, which the operator records

**Acceptance criteria:**
- [ ] before the trigger holds, the verb reports not yet due naming the missing condition, and writes nothing
- [ ] on a fixture with three locked, fully implemented draft-born features, it writes one record carrying the features, the new corpus file count, and the reopens by cause
- [ ] a second run writes nothing new

**Key interfaces:**
- the checkpoint inventory verb and its record

**Traces:** prd:feature-draft/EVT-7 · design:feature-draft-v1#3.4

## Coverage

| Requirement | Tasks |
|---|---|
| prd:feature-draft/FR-1 | T6 |
| prd:feature-draft/FR-2 | T6, T7, T19 |
| prd:feature-draft/FR-3 | T2, T7, T23 |
| prd:feature-draft/FR-4 | T2, T3 |
| prd:feature-draft/FR-5 | T6, T7 |
| prd:feature-draft/FR-6 | T11 |
| prd:feature-draft/FR-7 | T6 |
| prd:feature-draft/FR-8 | T13 |
| prd:feature-draft/FR-9 | T3, T13 |
| prd:feature-draft/FR-10 | T13, T15, T25 |
| prd:feature-draft/FR-11 | T4 |
| prd:feature-draft/FR-12 | T4 |
| prd:feature-draft/FR-13 | T4 |
| prd:feature-draft/FR-14 | T4 |
| prd:feature-draft/FR-15 | T12 |
| prd:feature-draft/FR-16 | T4, T14, T25 |
| prd:feature-draft/FR-17 | T6, T8 |
| prd:feature-draft/FR-18 | T3, T7 |
| prd:feature-draft/FR-19 | T9 |
| prd:feature-draft/FR-20 | T10, T22 |
| prd:feature-draft/FR-21 | T10 |
| prd:feature-draft/FR-22 | T21 |
| prd:feature-draft/FR-23 | T16 |
| prd:feature-draft/FR-24 | T7 |
| prd:feature-draft/FR-25 | T11 |
| prd:feature-draft/FR-26 | T24 |
| prd:feature-draft/FR-27 | T20 |
| prd:feature-draft/FR-29 | T1, T2, T7 |
| prd:feature-draft/FR-30 | T20 |
| prd:feature-draft/FR-31 | T11 |
| prd:feature-draft/FR-32 | T14 |
| prd:feature-draft/FR-33 | T13 |
| prd:feature-draft/FR-34 | T13 |
| prd:feature-draft/FR-35 | T15 |
| prd:feature-draft/FR-36 | T18 |
| prd:feature-draft/FR-37 | T12 |
| prd:feature-draft/FR-38 | T5 |
| prd:feature-draft/FR-39 | T5 |
| prd:feature-draft/FR-40 | T4 |
| prd:feature-draft/FR-41 | T5 |
| prd:feature-draft/FR-42 | T5 |
| prd:feature-draft/FR-43 | T10 |
| prd:feature-draft/FR-44 | T11, T12 |
| prd:feature-draft/FR-45 | T8, T9 |
| prd:feature-draft/FR-46 | T8 |
| prd:feature-draft/FR-47 | T8 |
| prd:feature-draft/UB-1 | T3, T7 |
| prd:feature-draft/UB-2 | T4, T14 |
| prd:feature-draft/UB-3 | T8, T15 |
| prd:feature-draft/UB-4 | T22 |
| prd:feature-draft/UB-5 | T20 |
| prd:feature-draft/UB-6 | T1 |
| prd:feature-draft/UB-7 | T4 |
| prd:feature-draft/UB-8 | T7 |
| prd:feature-draft/UB-9 | T7, T19 |
| prd:feature-draft/UB-10 | T12 |
| prd:feature-draft/UB-11 | T8 |
| prd:feature-draft/NR-1 | T11, T19 |
| prd:feature-draft/NR-3 | T20 |
| prd:feature-draft/NR-4 | T1 |
| prd:feature-draft/NR-5 | T20, T24 |
| prd:feature-draft/NFR-1 | T19 |
| prd:feature-draft/NFR-2 | T4, T19 |
| prd:feature-draft/NFR-3 | T5 |
| prd:feature-draft/NFR-4 | T9 |
| prd:feature-draft/ABUSE-1 | T3 |
| prd:feature-draft/ABUSE-2 | T8 |
| prd:feature-draft/ABUSE-3 | T4 |
| prd:feature-draft/ABUSE-4 | T1 |
| prd:feature-draft/ABUSE-5 | T22 |
| prd:feature-draft/ABUSE-6 | T11 |
| prd:feature-draft/ABUSE-7 | T11 |
| prd:feature-draft/ABUSE-8 | T8 |
| prd:feature-draft/EVT-1 | T17 |
| prd:feature-draft/EVT-2 | T17 |
| prd:feature-draft/EVT-3 | T17 |
| prd:feature-draft/EVT-4 | T17 |
| prd:feature-draft/EVT-5 | T9 |
| prd:feature-draft/EVT-6 | T17 |
| prd:feature-draft/EVT-7 | T26 |
