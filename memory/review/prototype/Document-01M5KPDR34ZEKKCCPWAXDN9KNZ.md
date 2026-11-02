# Review: prototype tech spec version 4 — design-review report

| Field | Value |
|---|---|
| **In one line** | Approve — D-1 faithfully carries A-9's Git refusal and manual-removal exception without adding scope. |
| **Relevant-when** | Auditing the first tech-spec review episode for prototype version 4 or implementing its removal behavior. |
| **Subject** | design:prototype · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at 450618c9550debe0c53053f7870809b20e35e97a |
| **Date(s)** | 2026-10-22 |
| **Verdict** | approve |
| **Tracking** | local branch draft/prototype-v4 (no pull request yet) |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:e2e70a6296bacda8fffd24d2b028cff38dd7a1fadc6691e8174bf248299870a0

## Round 1

Reviewed the entire tech spec and its embedded layouts from committed head
`450618c9550debe0c53053f7870809b20e35e97a`, after completing the PRD review.
The PRD requirement basis came from that same head, with section 0 excluded.
`pyxis draft digest prototype` returned the specified head and shared digest.
The local branch is `draft/prototype-v4`; remote draft status could not be
verified because the status command's github.com lookup failed.

The diff from `1b9cb770` and the diff from the locked version-3 design document
`memory/design/prototype/Document-01M5GNA983KBPD4N28RCZKPEW6.md` both show only
D-1's added removal exception. Locked brainstorm record-001 and record-002,
the locked PRD and design memory, and problem authority entries A-1 through
A-9 supplied the decision basis. Recorded rulings were accepted as given.
Draft-inapplicable document lint and gate-enforced defects were excluded.

The four lenses ran sequentially in this session:

1. **contract-quality** — D-1 through D-5 have concrete operations and named
   verification. D-1 now states the initialized-submodule outcome explicitly:
   report Git's refusal, remove nothing, never force removal, and leave
   removal to the operator by hand. Its verification already covers each
   refusal; the ordinary committed-prototype removal case remains testable.
2. **coherence** — The new exception qualifies D-1's ordinary removal path.
   D-2's release call and ordinary removal demonstrations remain compatible
   with that explicit exception. The create/refuse/remove ordering, routing,
   findings transfer, guide responsibilities, and stopped-process checkpoint
   rules do not introduce a surviving contradiction across D-1 through D-5.
3. **upstream-alignment** — The changed paragraph carries every part of
   problem:prototype#A-9 faithfully and cites its existing anchor. It preserves
   A-6 and A-8's layout and concurrency rulings and record-002's live D-2,
   D-3, D-4 and chosen alternatives. The served PRD rows retain their meaning;
   no PRD row requires overriding Git's refusal. The five-task estimate stays
   within the appetite. AP-17, AP-31 and incident I13 support keeping the
   ruled refusal without adding a removal mechanism.
4. **feasibility-completeness** — The Git claim and the compiler, citation,
   configuration, runtime, and adapter claims were checked against the
   available implementation. The declared failure handling, confinement,
   authentication state, time bound, and retained diagnostics provide a
   buildable path within the recorded scope.

Factual checks used read-only inspection:

- Installed Git reports version `2.55.0`. Its `git-worktree` manual's `remove`
  description distinguishes clean worktrees from worktrees containing
  submodules and requires force for the latter. The installed Git executable
  contains the diagnostic `working trees containing submodules cannot be
  moved or removed`. Committing changes does not remove that separate
  restriction. No worktree removal or Git-state mutation was run.
- `internal/draft/compile/document.go` admits numbered brainstorm files,
  captures their literal bodies, and binds the digest only to the two
  contract files. `internal/draft/readiness/links.go` resolves brainstorm
  anchors across local and locked documents. These support D-3's reuse claims.
- `internal/harnessconfig/config.go` loads the effective catalog through the
  configuration layer. `internal/adapters/harness/factory.go` and
  `cmd/implement-now/resolver.go` provide the shared configured runtime
  construction seam D-5 describes.
- The four harness adapters use the shared brief assembler.
  `internal/executor/runtime/brief.go`, `access.go`, `memory.go`, and
  `taskstate.go` support the prototype purpose, implementation confinement,
  omitted memory pack, and sibling runtime-state preparation. Already-landed
  support is evidence of feasibility, not a requirement to rebuild it.
- `internal/argus/boundary/boundary.go` supplies the attribution scope boundary
  D-1 relies on to exclude prototype application code from the draft proposal.

Every candidate was challenged against the explicit exception, its ruling,
the complete requirement basis, existing verification, and calibration. No
finding survived adversarial screening. No findings.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve | 2026-10-22 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
