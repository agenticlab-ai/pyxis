# Feature-draft skill evaluation — 2026-09-27

Retained acceptance evidence for feature-draft/002 T13, T14, T15 and T19.
The widget function is synthetic evaluation input, not a shipped Pyxis
capability. These files are inert test data.

## Inputs and authority

- Idea: `Add a widget summary`.
- Guides: `.pyxis/authoring/{feature-brainstorm,prd-new,tech-design-brainstorm,tech-design-new,prd-review,tech-design-review,argus}.md`, as present at main `83b6071b1833669c8a78d5641901e0500690e8e3`.
- The authoring worker executed the four authoring stages in order in an
  isolated directory. Its host supplied live scripted product/design
  answers; those answers authorize evaluation only. `questions.md` retains
  the stage-1 questions and answers; `host-transcript.md` identifies the
  supplied status and suppressed publication calls.
- Simulated operator rulings use the canonical `operator` token. The
  stage-1 and architecture principals are identified as simulator `alice`.
  No real operator approval or production lock is asserted.
- The evaluator initialized an isolated Git repository for exact-head
  review. Authoring files were committed there; Git/code-host publication
  from the authoring worker was suppressed. The Go integration test uses
  fresh local repositories and a bare remote.

## Observed authoring results — T13

| Stage | Output | Result |
|---|---|---|
| feature-brainstorm | `draft/brainstorm/001.md` | Six live decisions, explicit ready-for-prd gate; only draft working files written. |
| prd-new | `draft/prd.md` | Synthesized the gated record without re-asking settled product decisions; only working files written. |
| tech-design-brainstorm | `draft/brainstorm/002.md`, `draft/problems.md` P-1 | Seeded FR-1 archived input despite a preservation contract. Recorded one requirement problem and left the whole defective PRD byte-identical. |
| product ruling | P-1 followed by FR-1 edit | Recorded author, reason, triggering problem and exact change before applying it. |
| resumed technical brainstorm | `draft/brainstorm/003.md` | Live selection of single-scan layout, rejection of two passes, explicit panel waiver for pure in-process scope, ready-for-design gate. |
| tech-design-new | `draft/tech-spec.md` | Concrete Go types, control flow, invariants and acceptance inside D-1; no separate shape or contract document created. |

At P-1 discovery the defective PRD SHA-256 was
`c3df9aab0112137514ed6c7192925c9b6abc24a5e4b38d6f89d1589dd36e5fc5`
before and after discovery. `snapshots/prd-defect.md` retains those bytes.
`snapshots/first-review/` retains the first mechanically valid reviewed
contract. The final `draft/` retains both review rounds and product rulings.

## Observed reviews — T14

Both reviewers ran four lenses in parallel with independent adversarial
screening. Human-only §0 was removed from reviewer input.

| Pass | Committed subject head | Digest | Verdicts |
|---|---|---|---|
| 1 | `1bb0b471d27d3bf7cce8166c79d2dedfd8d04265` | `sha256:c0c1df4adc7ca513d54d95275420ec7acbc6b83643e7ecb451c891f208c70e04` | Both request-changes: overlapping invalid-input/cancellation error identities. Reports 001/002. |
| 2 | `395f519ae7300c41c454d708a9c1f9fe5ec6d64e` | `sha256:943d43dbd5da94884f517ad453dfbdcb74895218e56b708e0c10d1ebdb2b3c64` | Both approve after recorded P-2/D-6 precedence and combined-case evidence. Reports 003/004. |

The compiler/readiness judge recognized the first blocking findings.
After the product edit it returned FR-16 for both old reports with the
remedy to rerun each review. Their original digests were preserved.
The second pass examined only the changed spans and prior finding;
all four lenses returned no candidates, and independent screening
confirmed closure. Current mechanical readiness is true.

## Argus behavior and executable boundary — T15

A separate fresh worker ran the scripted Argus draft scenario at tier-2.
It left FR-1 unchanged and requested the operator's ungranted product
ruling; it did not substitute a reasonable assumption or claim rule
acceptance. After host-supplied ruling, current reviews and prepared-head
observations, it reconciled the completed effects, retained the operator
request, and rejected an agent-account APPROVED review. It did not repeat
prepare, plan, materialize work, or claim a landed lock.

The scripted host supplied the prepare observation; that worker's response
alone is not evidence of a real code-host or prepare invocation. The
companion Go test independently executes lifecycle open, Git history,
readiness refusal, compile and lock preparation on this actual authored
corpus. It verifies the exact-head operator request, literal contract
documents, attributed P-1 ruling, removal of working files, absence of
planning, and ErrNoApproval for Argus's exact-head APPROVED review. The
request stays pending; main and lock tags remain unchanged.

Command: `go test -race ./internal/draft/e2e -run '^TestSkillAuthoredDraftRequiresCurrentReviewsAndOperatorApproval$' -count=1`

Result: pass on 2026-09-27. The local code-host adapter and child
memory-gate process are doubles; production source verification is
performed separately by the repository's full code gate and hosted CI.
The test recreates the ruling history with new fixture commit IDs rather
than depending on the isolated evaluation repository's object database.

## Deviations and repairs

- The author used a custom UB claim-column heading. Real compilation
  refused UB-1; the author restored the canonical template heading before
  review. No product content was changed for that repair.
- An initial coherence worker read §0. That worker's result was discarded
  and replaced by a fresh worker receiving the filtered committed input.
- The evaluator's P-2 heading initially named four requirement IDs, which
  the closed grammar refused. It was repaired to a single UB-1 heading
  while keeping the related UB-2..UB-4 product change explicit.
- The evaluator initially appended explanation to `applied`, which the
  closed disposition grammar rejected. Explanation moved to separate
  prose; the disposition is exactly `applied`.
- The fixture's ranked `Must (rank n)` priorities exposed a real readiness
  defect. Readiness now recognizes the leading Must token. The companion
  readiness regression was red before the repair and green after it.

These observations cover authoring and review behavior plus the local
approval boundary. They do not certify a production launch, human merge,
remote GitHub permissions, or the future widget implementation.
