# impl-review reference — reading a persisted proposal review

| Field | Value |
|---|---|
| **In one line** | What a persisted impl-review carries — the report/verdict/finding schema and the triage states (review-absent · review-stale · dispute · escalation) — so a human reads a review's meaning without tooling. |
| **Relevant-when** | reading a review comment on a proposal; triaging a missing, stale, disputed, or escalated review; wiring a consumer that parses the payload; onboarding to what the reviewer emits |
| **Authority** | descriptive — the sources below govern; **the persisted payload governs on any conflict** |
| **Sources** | `docs/tech-design/impl-review-v1.md` (§3.1 schema · §4.3-equivalent states · §3.10 failure modes) · `docs/prds/impl-review/` (FR-5/FR-6/FR-7/FR-13, §4.3 flows/states, §6.6) · `internal/implreview/report/` (the `impl-review-report/v1` type + validation) · `internal/implreview/verdict/` (the downgrade-only verdict + staleness) |
| **Last reviewed** | 2026-07-17 |

impl-review persists **one self-contained artifact per proposal per review** on
the proposal's own medium — on GitHub, a bot-owned PR comment carrying a human
rendering plus a fenced ` ```impl-review ` machine block. The machine block is
the source of truth: the human rendering is a pure function of it, and every
measurement reads the payload fields, never the prose. This page distills that
payload's shape; when this page and a persisted payload disagree, the payload is
right.

## The report (`impl-review-report/v1`)

One closed, versioned payload holds a whole review — additive within v1, a
breaking change is v2 by supersession. It parses **closed**: an unknown key is
an error, never a lenient decode.

| Part | What it holds |
|---|---|
| **Envelope** | `schema_version` (must be `impl-review-report/v1`), `proposal_ref` (the proposal reviewed), `review_id` (`proposal_ref`+`review_id` is unique across reports), `band` (the review depth), and the live `verdict` (below). |
| **Rounds** (append-only) | the two-round record; each round is a distinct evaluation carrying its own `reviewed_revision`, its fresh-instance `context_id` + `provenance` (model · prompts version · calibration version), its finding set, and the `findings`/`candidates`/`killed` counts. A round is never mutated once recorded; a later round appends. |
| **Escalation brief** | present **iff** the verdict is `escalate` — the single conflict, the options with their risks, and the decision needed, addressed to the stage-9 human. |

**Invariants** (from `report.Validate`): one payload per proposal-review; the
human rendering never diverges from the payload; every finding cites a
calibration row its severity must match; every round binds a `reviewed_revision`;
a declined finding carries a reason.

## The finding (`FND-n`)

Findings are dense-numbered `FND-n` within a report (cited
`impl-review:<proposal-ref>#FND-3`). A finding is a **closable** unit — it names
its own acceptance condition at birth, and whether a revision closes it is
checked against that stated condition, never fresh taste.

The overlapping keys are **grammar-compatible with codegate's finding** — the
same JSON keys with the same meaning, so the worker's fixer parses codegate and
impl-review findings as one family — but impl-review declares its own type (a
compatibility test pins the keys; there is **no** Go import of codegate's struct,
and codegate is byte-unchanged):

| Field | Meaning |
|---|---|
| `id` | the `FND-n` anchor. |
| `path` · `line` · `message` · `log_ref` | the codegate-grammar-compatible fields: the file, the 1-based line (0 = file-level), the finding text, and a supporting-evidence reference. |
| `rule_id` | the lens+rule identity — impl-review's own field, never an overload of codegate's `check_id`. |
| `lens` | one of the four fresh-context lenses: `spec-fidelity` · `design-conformance` · `security-abuse` · `correctness-critic`. |
| `severity` | `blocking` or `advisory`; it **must** equal the severity its `cal_row` maps to. |
| `cal_row` | the calibration-row citation — the reviewed-data severity dial; required on every finding. |
| `acceptance` | the closable acceptance condition the finding is anchored to. |
| `disposition` | the recorded disposition (below), absent until one is made. |

A finding never restates a class the deterministic ladder (codegate) already
enforces — that mechanical floor is excluded by construction; style is not a
lens.

## The disposition (attribution-bound)

Each disposition records who judged a finding and how — the unit the adoption
metric is measured from, so its author matters:

| Field | Meaning |
|---|---|
| `actor` | `worker` or `operator`. |
| `verdict` | `applied` / `declined` (worker) or `accepted` / `rejected` (operator). |
| `reason` | **mandatory** on a `declined` disposition. |
| `attribution` | the medium author of a **separate** disposition artifact. |

The load-bearing rule: an **operator-class disposition counts only when its
`attribution` resolves to the operator's own medium identity** — the operator's
own PR review/comment on GitHub (or their own Carina decision-record), never a
field a worker could write into the bot's payload. The harvest drops an
unattributable operator disposition, so a forged "operator accepted" cannot
poison adoption. A worker's own disposition never resolves a blocker away: a
worker `applied` is an unverified claim that holds the verdict until a re-review
confirms it; a worker `declined` is an escalate condition, not a resolution.

## The verdict (downgrade-only)

The live verdict is exactly one of four values, and it is **downgrade-only** by
construction — it can force the HUMAN lane or bounce the proposal back to the
worker, and it can do nothing else. There is no value, field, or path that
grants the AUTO lane or writes a stage-9 sign-off; "the verdict grants AUTO" is
unrepresentable, not merely forbidden.

| Verdict | Meaning | Downgrade effect |
|---|---|---|
| `approve` | no findings that hold the verdict. | none — withholds a downgrade; the lane's own floor+classes still decide AUTO eligibility (an approve is not a grant). |
| `approve-with-advisories` | only advisory findings remain. | none (as `approve`). |
| `request-changes` | a blocking finding is unresolved. | returns the proposal to the worker. |
| `escalate` | a spec decision is needed (a declined blocking finding, a materially-defective charter, or a calibration escalation condition) — carries the escalation brief. | forces the HUMAN lane. |

Until the ten-review adoption checkpoint promotes the reviewer (a recorded,
per-repo, default-off operator flip), **the verdict gates nothing**: findings
deliver and persist, but PR-raise and lane routing proceed regardless. Consuming
the emitted downgrade signal into the merge lane is an owed lane amendment
(design §3.6 Q4); until it lands the verdict still persists on the proposal for
the stage-9 human.

## Triage states — what a non-terminal review looks like

Beyond the four verdicts, a review can record a triage outcome. On any of these
the reviewer **never silently passes** and never stalls indefinitely; in the
advisory period the host proceeds with the state flagged, and post-adoption the
outcome routes the proposal to a human.

| State | What it means | What to do |
|---|---|---|
| **skipped:\<class\>** | the proposal's risk class is the skip band; no review runs, and the skip is recorded (it counts against coverage as a recorded skip, not an absence). | nothing — an intended cheap-class skip. |
| **review-absent** | the reviewer failed or exceeded its bound (backend error, timeout, a persistence-write failure, or a schema-invalid round retried once) and recorded an absence rather than a result. A `review-absent` counts against coverage until backfilled. | in advisory the raise is flagged; post-adoption the proposal routes HUMAN. Re-invoke to backfill; a re-invocation resumes idempotently from the persisted round state. |
| **review-stale** | the proposal head moved past the tree the live verdict certified — the head ≠ the latest round's `reviewed_revision`. A stale verdict is treated as **absent** (it routes nothing, certifies nothing). | a post-review change forces a re-review (a scope-locked round 2); post-adoption a raise on a stale verdict is refused. |
| **dispute** | the worker disputes a blocking finding — recorded as a `declined` disposition with its **mandatory** reason. A reason-less dispute voids its round; round 2 is scope-locked to the named defects + changed spans, and round 3 is refused. | a **declined blocking** finding escalates (below); attrition never converts to an approval. |
| **escalation** | the reviewer emits an `escalate` verdict carrying the decision brief (the single conflict, options with risks, the decision needed) on the proposal — for a declined blocking finding, a defective charter, or a firing calibration escalation condition. | in advisory the brief is non-gating; post-adoption the proposal routes HUMAN with PR-raise held. |
