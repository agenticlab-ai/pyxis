# PRD lifecycle home — post-merge gate events live in the tracker; the ledger stays contract-only

| Field | Value |
|---|---|
| **Status** | Approved (2026-07-08 — operator, via the introducing PR review) *(merge is the freezing event; "superseded" is computed by the index, never written here)* |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi (operator — document-system surface) |
| **Date** | 2026-07-08 |
| **Relevant-when** | recording a launch review or post-launch verdict for a merged PRD; asking where a feature's lifecycle state lives; authoring an amendment that needs a tracking pointer; revisiting amendment-model Q1 |
| **Scope** | The home of post-merge PRD lifecycle facts (gate passages, post-launch verdicts) and the envelope's new optional `Tracking` row (FR-24). **Not**: the draft-phase gate ladder (the PRD template owns it), the fold or op vocabulary (unchanged — that is the point), tracker tooling (none exists yet; the decision is medium-shaped, not tool-shaped). |
| **Tracking** | issue #54 · PR #56 (requirements) |
| **Builds-on** | docs/tech-design/prd-amendment-model.md · docs/tech-design/tech-design-conventions.md |
| **Supersedes** | — |
| **Serves** | prd:docgate/FR-24 |
| **Security/privacy impact** | none — a documentation-convention decision plus one optional envelope field; no runtime surface |
| **Template version** | tech-design-template/v1 |

## 1. Problem

The amendment model froze with Q1 open: do post-base lifecycle facts
(launch review passed, post-launch verdict) enter the ledger as an op
kind, or stay in the work tracker? The question is now live, not
theoretical (#54 C1): the docgate PRD merged with "launch review pending,
post-implementation" in its frozen Status row — the review will happen,
and today its verdict has no sanctioned home. Relatedly (#54 A1's process
half), amendment 005 shipped a requirement deliberately ahead of its
implementation and the envelope had nowhere to name what tracks that
work; the one tracker (an issue) was closed by convention drift, and
confusion followed.

## 2. Decision

**Decision:** Post-merge lifecycle facts live in the **work tracker**,
anchored by the base PRD's `Tracking` row — the ledger records **contract
changes only**. A gate passage or post-launch verdict is recorded on the
feature's tracking reference (today: the GitHub issue or PR the base
names; any future tracker medium equally, since the pointer is a typed
reference, not a URL convention). When a verdict has contract
consequences — a *kill* retires rows, an *iterate* re-scopes — those
consequences travel as **normal amendment operations** whose
`Learned-from` cites the verdict (`operator-decision:` or
`telemetry-query:` evidence), which is exactly the model's existing
shape. The envelope gains an optional `Tracking` row under
`amendment-template/v2`, validated by declared version (FR-24), so an
amendment whose implementation lands later carries a durable pointer to
the work that owes it.

Why the tracker and not a sixth op: the ledger's integrity properties
(typed ops, deterministic fold, byte-verified projection) exist to keep
the *contract* trustworthy. Lifecycle is status, not contract — folding
status into the projection would make the "current contract" document
churn on non-contract events and would demand fold releases for every
new lifecycle kind. The model already leaned this way; the kill-verdict
concern it raised dissolves because the *contract effect* of a kill was
always going to be ordinary retire ops — the verdict is evidence, the
amendment is the change.

## 3. The design

### 3.1 The convention (binding, three rules)

- **Rule 1** — the base PRD's `Tracking` row is the lifecycle anchor:
  every post-merge gate event (launch review, post-launch verdict per
  template §6.7) is recorded there, dated, by the accountable owner.
- **Rule 2** — a lifecycle fact never edits the ledger, the projection,
  or any frozen row; a fact with contract consequences is *evidence* for
  a normal amendment, never a bypass of one.
- **Rule 3** — an amendment that specifies work landing outside its own
  PR (implementation to follow, migration pending) declares that work's
  tracker in its envelope `Tracking` row (v2); closing that tracker
  before the work exists is the anti-pattern this row exists to make
  visible.

### 3.2 The envelope change

`amendment-template/v2` = v1's seven fields plus optional `Tracking`.
Validation is by **declared version** (FR-24): v1 envelopes never carry
`Tracking` (unknown field → defect), v2 envelopes may; merged v1
amendments are grandfathered forever by their own declaration — nothing
is retrofitted. The template bump ships with updated goldens in the same
change, per the template's own appendix rule. The template pin in the
config surface moves to v2 at the same moment (an instrument pin, not a
gate-policy key — see docgate-fence-completion §3.2).

### 3.3 Boundaries and non-goals

- **Not solved here** — tracker tooling or automation (no tracker port
  exists; the convention is medium-neutral by citing the `Tracking`
  reference, whatever medium it names); the draft-phase gate ladder
  (template-owned, unchanged).
- **Owned by a future design** — engine-era: the WorkTracker port makes
  lifecycle facts queryable data; the Conductor consumes them per
  pipeline stage. This convention is deliberately the thinnest thing
  that survives that arrival.
- **Implementation must not infer** — that the projection should render
  lifecycle state (it must not — the projection is the contract view);
  or that `Tracking` values are validated as URLs (they are references,
  checked for presence-when-declared only).

### 3.4 Observability

none — no runtime surface; the convention's health is visible in review
(a lifecycle event with no tracker record is a review finding, exactly
like a missing evidence citation). The envelope half is observable
through the existing defect stream (FR-24 fixtures).

### 3.5 Security & privacy

none — matches the header row: a documentation convention and one
optional header field; no identity, data, or egress surface.

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| a sixth op kind (`record`) folding lifecycle into the projection | status churns the contract document; every lifecycle kind becomes a fold release; the model's closed-vocabulary Decision would need wholesale supersession — cost without a consumer |
| lifecycle in the projection header, written by the fold from tracker state | the fold would read a medium (impure — docgate-v1#DINV-1) and the projection would no longer be a pure function of the ledger |
| a per-feature `LIFECYCLE.md` beside the ledger | a third document class with its own mutability rules, for data the tracker already holds; more machinery than the fact deserves |
| mandatory `Tracking` on every amendment | most amendments are self-contained (their PR is the work); mandatory-when-empty breeds placeholder values, the vagueness Appendix A bans |

## 5. Risks

| ID | Risk | Impact | Mitigation | Verification / evidence |
|---|---|---|---|---|
| R1 | tracker records rot (issues closed, mislabeled) with no gate | lifecycle facts lost | the anchor is the base's `Tracking` row (frozen, always findable); rule 3 makes premature closure visible; engine-era tracker port adds the mechanical half | review discipline; #54's own history is the cautionary fixture |
| R2 | v2 envelope adoption lags and Tracking stays unused | the A1 process gap recurs | the authoring skill defaults new amendments to v2; the row is optional, so cost is zero until needed | skill template default |
| R3 | two lifecycle sources (tracker + frozen Status rows) confuse readers | contradictory-looking state | the frozen Status row is a snapshot at merge (already true, stated in the amendment model); the wiki's document-classes entry says "post-merge lifecycle: see the Tracking ref" | wiki update in the editable-docs PR |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | envelope v2 with `Tracking` green; v2 with an unknown field red; v1 with `Tracking` red; v1 without green; declared v3 red | Test | FR-24 fixtures | prd:docgate/FR-24 |
| A-2 | the template bump ships `amendment-template/v2` + config pin + goldens in one change | Inspection | the implementing PR | §3.2 |
| A-3 | the docgate PRD's pending launch review is recorded per rule 1 when it happens — the convention's first live use | Inspection | the tracking ref's record | §3.1 |
| A-4 | the fold, op vocabulary, and projection bytes are unchanged by this design (contract-only ledger preserved) | Test | existing goldens untouched except envelope fixtures | §2 |

## 7. Open questions

| ID | Question | Owner | Blocks | Status | Resolution |
|---|---|---|---|---|---|
| Q1 | When the WorkTracker port exists, do lifecycle facts become typed events the Conductor reads, and does this convention then get superseded by that design? | engine design | nothing now | open | expected yes — this convention is written to be cheap to supersede |

## Principles

- **AP-7 / INV-33 — honored.** The ledger and projection stay contract-pure; nothing frozen is edited; lifecycle rides the medium built for status.
- **AP-10 — honored (by restraint).** No machinery is built where a convention plus one optional field suffices; the mechanical halves (FR-24 validation) are code.
- **AP-21 — honored.** The verdict is judged once, recorded on the tracking ref, and consumed as frozen evidence by any consequent amendment.
- **AP-6 — honored.** The envelope change is versioned, closed, additive; validation is by declared version, so history never lies.
- **AP-1 — honored.** The convention names a typed reference, not a GitHub URL shape — tracker media stay swappable.
- **RW-5 — honored.** Amendment-model Q1 is resolved by an explicit design, not by drift.
- **Deviation:** none.
