# Review-feedback capture — plan 001

| Field | Value |
|---|---|
| **Feature** | review-feedback-capture |
| **PRD** | docs/prds/review-feedback-capture |
| **Design** | docs/tech-design/review-feedback-capture.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Build the review-feedback ledger — the doc-review capture slice |
| **Status** | Draft — freezes at merge |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Build the review-feedback ledger storage class and its fold

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 150–300 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/reviewfeedback (ledger types + fold), cmd/docgate (fold verb wiring), docs/feedback/ |

**Goal:** a per-subject append-only ledger entry can be written to disk and a fold produces a byte-verified machine projection.

**Current behavior:** no ledger exists; human review comments are discarded.

**Desired behavior:** a per-subject directory holds one append-only markdown file per entry (id · source · author · timestamp · subject-anchor · verbatim comment); a fold reads all entries and regenerates a schema-versioned projection; the projection is byte-verified (a stale/hand-edited projection reds the gate); an entry file is append-only (capture writes the initial block, a later disposition transition appends, prior content never rewritten).

**Scope:**
- the entry schema (typed, append-only) and the per-subject directory layout
- the fold that produces the machine projection, wired as a docgate verb + byte-verify

**Out of scope:**
- the writer/adapters (T2/T3); disposition (T4); citations (T5)

**Acceptance criteria:**
- [ ] writing N entries under a subject yields N append-only files, each with all header fields populated
- [ ] the fold regenerates the projection deterministically; a byte-mismatch reds `verify-generated`
- [ ] appending a disposition transition to an entry leaves its captured text and attribution unchanged

**Key interfaces:**
- the ledger `Entry` schema-versioned type; the fold entry point; the projection contract

**Traces:** prd:review-feedback-capture/FR-7 · prd:review-feedback-capture/FR-1 · design:review-feedback-capture#3.1

### T2 — Introduce the CaptureSource port and the coded writer

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 100–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/reviewfeedback (CaptureSource interface + writer) |

**Goal:** the coded writer produces a ledger entry from a captured comment, taken through the consumer-side CaptureSource port.

**Current behavior:** T1 can persist an entry, but nothing produces one from a review comment.

**Desired behavior:** a consumer-side `CaptureSource` interface yields a schema-versioned captured comment (comment · author · timestamp · subject-anchor · source-id); the writer is the sole producer of entries (no hand-authoring path) and imports no adapter package (boundary lint-enforced). *(Plan decision: the port is built now — the frozen design's DINV-2 binds it, and the WR-4 code-host adapter forward-composes on it; only the doc-review adapter is live in this slice, added in T3.)*

**Scope:**
- the `CaptureSource` interface and the writer, with a test/stub adapter for the conformance suite
- the import-boundary lint that keeps the writer core adapter-free

**Out of scope:**
- the real doc-review adapter (T3); disposition (T4)

**Acceptance criteria:**
- [ ] the writer produces one entry per captured comment via the port, verified with a stub adapter
- [ ] the import-boundary lint fails if the writer core imports an adapter package
- [ ] no code path or guide permits hand-authoring an entry

**Key interfaces:**
- `CaptureSource` interface; `CapturedComment` type; the writer contract

**Traces:** prd:review-feedback-capture/FR-9 · design:review-feedback-capture#3.3

### T3 — Capture doc-review annotations through the port

| Field | Value |
|---|---|
| **Blocked by** | T2 |
| **Size band** | 80–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/reviewfeedback (doc-review adapter + confirm-hook shim) |

**Goal:** confirming a doc-review annotation persists it as a ledger entry, off the review's critical path.

**Current behavior:** doc-review applies the annotation's edit by propose-and-confirm and then discards the annotation.

**Desired behavior:** at annotation-confirm, a doc-review adapter (a thin shim delegating to the Go writer — the bash skill is not extended with logic) records the annotation as a ledger entry; capture runs off the critical path and fails open — a failed write neither blocks nor loses the comment (queued for retry); the existing propose-and-confirm edit behavior is unchanged.

**Scope:**
- the doc-review adapter implementing `CaptureSource`; the confirm-time shim; fail-open capture

**Out of scope:**
- disposition (T4); doc-review's edit behavior itself (NR-1, guarded in T8)

**Acceptance criteria:**
- [ ] a confirmed annotation produces one entry with all fields; the doc-review edit still applies unchanged
- [ ] a simulated write failure loses no comment and blocks no review (UB-1)
- [ ] the shim contains no logic beyond locating and invoking the writer

**Key interfaces:**
- the doc-review adapter (implements `CaptureSource`); the confirm hook contract

**Traces:** prd:review-feedback-capture/FR-1 · prd:review-feedback-capture/UB-1 · design:review-feedback-capture#3.3

### T4 — Tag dispositions at resolution (doc-review side)

| Field | Value |
|---|---|
| **Blocked by** | T2, T3 |
| **Size band** | 100–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/reviewfeedback (disposition + git observation) |

**Goal:** an entry closes to a typed disposition + outcome-ref when its resolving event is observed — deterministically, never by inference.

**Current behavior:** entries are captured open, with no disposition.

**Desired behavior:** the writer records the edit-commit an annotation produced at capture; the fold observes that commit's landing on the subject and tags the entry `applied` with a stable outcome-ref (merged sha / PR / amendment id — never a working-branch sha); an entry whose annotation produced no landed change closes `no-action` with a reason; one not yet resolvable stays open, flagged pending; one whose subject anchor no longer resolves is retained, flagged stale. Every disposition transition names the observed event it was set from.

**Scope:**
- the disposition state machine; the deterministic resolving-event (commit-landing) observation; and the pending, no-action, and stale-subject handling

**Out of scope:**
- fuzzy comment→diff inference (explicitly excluded); measures (T7)

**Acceptance criteria:**
- [ ] a landed annotation-edit closes its entry `applied` with a stable outcome-ref naming the observed commit
- [ ] a no-change subject closes `no-action`; an unresolved one stays pending; a superseded-or-deleted subject stays flagged-stale (UB-3, UB-5, UB-6)
- [ ] every disposition transition cites its resolving event; no code path sets a disposition without one

**Key interfaces:**
- the disposition enum + transition; the resolving-event observer; the outcome-ref shape

**Traces:** prd:review-feedback-capture/FR-3 · prd:review-feedback-capture/UB-3 · prd:review-feedback-capture/UB-5 · prd:review-feedback-capture/UB-6 · design:review-feedback-capture#3.4

### T5 — The review-feedback citation grammar and its enforced resolution check

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 100–250 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/docgate (registry-born check), internal/reviewfeedback (citation anchors) |

**Goal:** an entry resolves by its `review-feedback:` citation, and a dead citation or outcome-ref reds the gate.

**Current behavior:** `Learned-from: review-thread` refs resolve to nothing; nothing enforces resolution.

**Desired behavior:** entries carry a `review-feedback:<subject-stem>#<entry-anchor>` citation (a sibling of the existing grammar, grep-resolvable); `Learned-from: review-thread` resolves to a `review-feedback:` anchor through the amendment model's existing free ref slot (no amendment-model change); a registry-born docgate check (extending the check-registration pattern) fails any entry `outcome` ref or `Learned-from: review-thread` ref that does not resolve.

**Scope:**
- the `review-feedback:` citation family; the registry-born resolution check

**Out of scope:**
- any change to the amendment model (its free ref slot is used as-is); cross-links (T6)

**Acceptance criteria:**
- [ ] a `review-feedback:` citation resolves by grep to its entry; a dead ref reds the resolution check
- [ ] a `Learned-from: review-thread` ref pointing at a real entry passes; a dangling one reds the check
- [ ] the amendment-model files are unchanged by this task

**Key interfaces:**
- the `review-feedback:` anchor grammar; the resolution check registration

**Traces:** prd:review-feedback-capture/FR-4 · design:review-feedback-capture#3.5

### T6 — Cross-link entries to the existing captures, read-only

| Field | Value |
|---|---|
| **Blocked by** | T5 |
| **Size band** | 50–150 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/reviewfeedback (cross-link anchors) |

**Goal:** an entry cross-links to a reviewer M2 residual or an Argus `E-n` disposition, one-directionally.

**Current behavior:** no linkage between the ledger and the existing partial captures.

**Desired behavior:** an entry may cite the reviewer M2 report / Argus `E-n` where relevant; the links are one-directional (we cite them; `review-report/v1` and the Argus ledgers are never edited to cite back).

**Scope:**
- the one-directional cross-link citations from an entry

**Out of scope:**
- any write to `review-report/v1` or the Argus ledgers (NG-5)

**Acceptance criteria:**
- [ ] an entry's cross-link resolves to its target; the target files are unmodified

**Key interfaces:**
- the cross-link citation shape

**Traces:** prd:review-feedback-capture/FR-8 · design:review-feedback-capture#3.5

### T7 — Compute the ledger-native measures in the fold

| Field | Value |
|---|---|
| **Blocked by** | T1, T4 |
| **Size band** | 80–200 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/reviewfeedback (measures in the fold) |

**Goal:** the fold exposes disposition counts, the applied-vs-decline ratio, and `edit_distance` — all from the ledger alone.

**Current behavior:** no measures are computable.

**Desired behavior:** the fold's projection exposes disposition counts per type, the applied-vs-decline ratio over disposed entries, and an `edit_distance` frozen in each `applied` entry at write time (computed when the writer tags the outcome — no later diff read); all read by inspecting the projection (the event stream is deferred, WR-6).

**Scope:**
- the measure computations in the fold; `edit_distance` capture at disposition-write

**Out of scope:**
- the all-proposals rejection rate (optimizer's, NG-1); a consumer reading the measures (WR-5)

**Acceptance criteria:**
- [ ] disposition counts + applied-vs-decline ratio + `edit_distance` compute from the projection alone, for a subject class
- [ ] `edit_distance` is frozen in the entry at disposition-write, not recomputed on read

**Key interfaces:**
- the measures block of the projection; the `edit_distance` field

**Traces:** prd:review-feedback-capture/FR-5 · design:review-feedback-capture#3.6

### T8 — Guard the non-regressions

| Field | Value |
|---|---|
| **Blocked by** | T3 |
| **Size band** | 50–150 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/reviewfeedback (regression tests) |

**Goal:** the change is provably additive — doc-review, the reviewers' posture, the merge path, and the existing captures are untouched.

**Current behavior:** these behaviors exist and pass their suites today.

**Desired behavior:** doc-review's propose-and-confirm edit behavior is unchanged (NR-1); no pipeline path acquires a required human-review step (NR-2); merge authorization / required checks / risk lanes are unchanged (NR-3); `review-report/v1` and the Argus ledgers are unchanged (NR-4).

**Scope:**
- regression tests / inspections asserting each non-regression holds

**Out of scope:**
- none — this task only verifies, it changes no behavior

**Acceptance criteria:**
- [ ] the existing doc-review, reviewer, and merge-path suites pass unchanged
- [ ] no diff touches `review-report/v1`, the Argus ledgers, or the merge-path config

**Key interfaces:**
- the non-regression assertions

**Traces:** prd:review-feedback-capture/NR-1 · prd:review-feedback-capture/NR-2 · prd:review-feedback-capture/NR-3 · prd:review-feedback-capture/NR-4

## Coverage

| Requirement | Tasks |
|---|---|
| prd:review-feedback-capture/FR-1 | T1, T3 |
| prd:review-feedback-capture/FR-3 | T4 |
| prd:review-feedback-capture/FR-4 | T5 |
| prd:review-feedback-capture/FR-5 | T7 |
| prd:review-feedback-capture/FR-7 | T1 |
| prd:review-feedback-capture/FR-8 | T6 |
| prd:review-feedback-capture/FR-9 | T2 |
| prd:review-feedback-capture/UB-1 | T3 |
| prd:review-feedback-capture/UB-3 | T4 |
| prd:review-feedback-capture/UB-5 | T4 |
| prd:review-feedback-capture/UB-6 | T4 |
| prd:review-feedback-capture/NR-1 | T8 |
| prd:review-feedback-capture/NR-2 | T8 |
| prd:review-feedback-capture/NR-3 | T8 |
| prd:review-feedback-capture/NR-4 | T8 |
