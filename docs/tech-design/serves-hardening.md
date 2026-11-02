# Serves hardening — corpus refs resolve, liveness at introduction, Approved before merge

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-08 — operator, via the introducing PR review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator) |
| **Date** | 2026-07-08 |
| **Relevant-when** | changing what `lint-tech-designs` proves about a design header; touching `Serves` resolution, the corpus-ID scan, or the status check; triaging a design PR red on liveness or corpus resolution |
| **Scope** | The FR-20 lint deltas: `corpus:` reference resolution, superseded/retired-unit rejection for designs new since the merge base, and the Approved-status check. **Not**: the ref grammar and existing resolution semantics (docgate-v2 §3.1, unchanged), the precedence clause (FR-16→FR-20 carries it verbatim), index flags (`stale-serves` stays a flag, per the conventions). |
| **Tracking** | issue #54 · PR #56 (requirements) |
| **Builds-on** | docs/tech-design/docgate-v2.md · docs/tech-design/tech-design-conventions.md |
| **Supersedes** | — |
| **Serves** | prd:docgate/FR-20 |
| **Security/privacy impact** | none — same hermetic lint; no new data, identity, or egress surface |
| **Template version** | tech-design-template/v1 |

## 1. Problem

Incident I1's fix (FR-16) closed the WR-stub hole narrowly, and the review
(#54 A4/E5) found what stayed open. `corpus:` references — the majority of
real `Serves` anchors while requirements live in the placeholder corpus —
resolve "by review" only, so `corpus:IMPL-99` lints green today. Resolution
checks existence, not liveness: a brand-new design can anchor itself to a
unit the ledger already superseded, attaching new work to dead intent with
only a computed index flag to show for it. And the conventions promise "a
merged design carrying `Draft` is a gate error" (§3.1) — never specified,
never built. FR-20 (amendment 007) puts all three in the contract; this
design decides the mechanics.

## 2. Decision

**Decision:** `lint-tech-designs` gains three checks, all decidable from
bytes plus git history (docgate-v2#DINV-2 preserved). **Corpus resolution:**
a `corpus:<ID>` reference must match a requirement ID actually present in
the flat corpus files — the scan extracts the baseline families' IDs from
the placeholder documents and treats the set as the resolution universe;
no format is imposed on the corpus (NR-3), the IDs are simply read.
**Liveness at introduction:** with a merge base, a design file added since
the merge base whose `Serves` cites a `prd:` unit with source-map status
`superseded-by` or `retired` is a defect; designs that predate the diff are
untouched — their staleness remains the index's `stale-serves` flag, the
conventions' decided behavior. **Status check:** a non-grandfathered design
whose `Status` row does not record `Approved` is a defect — the amendment
model's merge-target rule, applied to the design class.

## 3. The design

### 3.1 Mechanics

- **Corpus ID scan** — over every flat `.md` directly under the configured
  PRDs path (README included; ledger directories excluded): collect all
  matches of `(SCAF|PLAN|IMPL|GATE|FLEET|IDEN|GOV|KNOW|OBS|BENCH|VNEXT)-[0-9]+`.
  The union is the `corpus:` resolution set. The scan is read-only pattern
  extraction — deliberately not a parse, because the corpus has no schema
  to parse (NR-3).
- **New-design detection** — `git diff --name-status <base>` filtered to
  `A` rows under the configured tech-designs path (the same subprocess
  shape as `check-immutable`). For each added design, each `prd:` ref is
  looked up in that feature's `effective.source.json`; status other than
  `live` is a defect naming the unit, its status, and its successor.
- **Status check** — the already-parsed header's `Status` value must
  contain `Approved`; the grandfather set (config) is exempt — the two
  pre-convention documents legitimately carry `Draft` forever.

### 3.2 Boundaries and non-goals

- **Not solved here** — *temporal* precedence for corpus refs (the corpus
  has no introduction ledger; existence is the strongest provable claim);
  semantic fit of a design to the requirement it serves (review's job,
  docgate-v2#DINV-2).
- **Owned by a future design** — corpus retirement semantics if the
  placeholder corpus ever gains a supersession vocabulary; requirement-level
  liveness once real per-feature PRDs absorb the corpus (the `corpus:` type
  itself then retires per the conventions).
- **Implementation must not infer** — that a pre-existing design serving a
  now-superseded unit is a defect (it is a flag — the conventions decided
  re-anchoring is review's call, not the gate's); or that the corpus scan
  may validate corpus content (it extracts IDs, nothing more).
- **Must remain unchanged** — every FR-16-era verdict on the current
  corpus: the fixture set keeps its colors (FR-20's fit criterion).

### 3.3 Invariants

- **DINV-1** — every new defect is decidable from bytes + git history; no
  judgment, no network (docgate-v2#DINV-2 carried forward).
- **DINV-2** — the liveness check fires only for designs **added** since
  the merge base: the gate never retro-reds frozen documents for the
  ledger moving under them.
- **DINV-3** — the corpus scan imposes no structure on corpus files: a
  corpus file with zero extractable IDs is legal and simply contributes
  nothing to the resolution set.

### 3.4 Observability

Unchanged from docgate-v1 §3.4. New defect reasons join the catalog:
unresolvable corpus ref (names the ID and the scanned file count),
dead-unit anchor (names unit, status, successor), non-Approved status.
The index continues to surface `stale-serves` for the post-merge case —
flag there, defect here, by design.

### 3.5 Security & privacy

none — matches the header row: the lint reads tracked files and git
history it already read; no new surface of any kind.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| resolve `corpus:` by review forever (status quo) | the majority anchor type stays unverifiable while the minority (`prd:`) is machine-checked — the inversion I1 taught |
| give the corpus a real ID registry file | a hand-maintained registry wearing authority is the drift pattern; the IDs are already in the files — read them |
| defect (not flag) for *all* designs serving superseded units | retro-reds immutable documents that cannot be fixed, forcing supersession churn; the conventions already decided flag-and-review |
| checking `Approved` only at merge time via a merge-target mode | the amendment model's precedent is validate-always (a draft PR flips the row when review passes); two modes is a twin predicate |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | corpus ID regex misses an unconventional ID form and a valid ref reds | author friction | the families are closed and uniformly formatted in the corpus today (fixture proves it); an unmatched form is a corpus authoring anomaly worth surfacing | corpus-scan fixture: every currently-cited corpus ref resolves |
| R2 | a renamed design file (R-status) evades new-design detection | liveness check skipped | rename of a merged design is already red under FR-17's fence; rename of an unmerged file within a PR still shows the new path as added against the merge base | fixture: renamed-in-PR design still checked |
| R3 | `Approved`-always rule makes red the natural state of design-PR CI until the flip | authors read red as broken | the defect remedy says exactly "flip Status to Approved when review passes — the amendment model's own rule"; identical friction already exists for amendments | defect-message review |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | `corpus:` ref to a nonexistent ID red naming the ID; every corpus ref currently in the live corpus resolves green | Test | fixtures + real-corpus run | prd:docgate/FR-20 |
| A-2 | with a merge base: an added design serving a superseded unit red naming unit/status/successor; a pre-existing design serving the same unit green | Test | temp-repo fixture | prd:docgate/FR-20 · DINV-2 |
| A-3 | a non-grandfathered design with `Status: Draft` red; the grandfathered two green; all four live designs green | Test | fixtures + corpus run | prd:docgate/FR-20 |
| A-4 | every FR-16 fixture keeps its verdict under FR-20 | Test | fixture suite diff | §3.2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | When real PRDs absorb a corpus area, does the lint offer a migration hint (corpus ref → the absorbing ledger unit), or is that the amendment `diff`'s job? | tech design (that migration's) | nothing now | open | — |

## Principles

- **AP-10 — honored.** Three review-discipline rules become checks; "resolve by review" survives only where nothing stronger is provable (semantic fit).
- **AP-13 — honored.** "This design serves a live requirement" becomes a verified claim at exactly the moment it is cheap to fix — introduction.
- **AP-14 / AP-17 — honored.** Fixtures execute every clause; ambiguity (unknown status value, missing source map) is a defect, never a pass.
- **AP-11 / INV-30 — honored.** New-design detection reuses the one git-diff subprocess shape; no second diff predicate grows.
- **RW-5 — honored.** The mechanics land here before the code; FR-20 carries the contract.
- **Deviation:** none.
