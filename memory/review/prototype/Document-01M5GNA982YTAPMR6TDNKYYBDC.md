# Review: prototype tech spec v3 — design-review report

| Field | Value |
|---|---|
| **In one line** | approve-with-advisories — D-1 faithfully carries A-8; one existing security statement overstates the locality of a Git fetch. |
| **Relevant-when** | Auditing the first tech-spec review episode for prototype version 3, its layout exception, or the fetch advisory. |
| **Subject** | design:prototype · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at 73c752fef308bf5eb019dd3b8895650d57531883 |
| **Date(s)** | 2026-10-21 |
| **Verdict** | approve-with-advisories |
| **Tracking** | local branch draft/prototype-v3 (no pull request yet) |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:af81d7a0c4377db50895d1e58be50da3d427f4a5ec8b2a6e9e647aef10aad1c8

## Round 1

Reviewed the entire tech spec from `git show 73c752fef308bf5eb019dd3b8895650d57531883:drafts/prototype/tech-spec.md`, after completing the PRD review in `013.md`. The requirement basis was the PRD body at that same commit, excluding its human-only routing section. Compared the subject with `6ce17ba4` and the locked version-2 design memory. The change adds D-1's explicit A-8 exception and limits the separate-Git-directory refusal fixture to a Git directory not named `.git`.

The four lenses ran sequentially in this session, in this order:

1. **contract-quality** — checked D-1 through D-5, their concrete layouts, refusals and verification procedures. The changed layout definition has an observable Git basis, and the narrowed refusal fixture no longer promises detection of the accepted exception.
2. **coherence** — checked creation and removal, route resolution, findings capture and answers, launch purpose, runtime-state ownership and checkpoint ordering across decisions. Checked the chosen shapes against the locked design brainstorm's live D-2 through D-4 and alternatives ledger.
3. **upstream-alignment** — checked each Serves claim against FR-1 through FR-6, NR-1, NR-2 and UB-1, the locked brainstorm records, the operator's A-1 through A-8 rulings, principles and incident lessons. D-1 expressly accepts the `.git`-named separate directory as ordinary using Git's reported main checkout, with the prototype path derived from that directory. This faithfully qualifies A-6 under A-8; neither extra detection nor stronger concurrency guarantees are required.
4. **feasibility-completeness** — checked the existing compiler and citation seams, effective harness catalog, adapter construction and purpose dispatch, isolated authentication state, confinement, process-quiescence contract and worker time bound. Checked verification against the real dependency interfaces, the five-task appetite, and AP-31/I13's limit on unasked guarantees.

After the four passes, every candidate was challenged against the recorded decisions, existing mechanisms, gate responsibilities and a concrete smallest closure. Only FND-1 survived. It concerns unchanged text, which is within this first round's whole-subject scope, and does not challenge a recorded ruling. No finding restates the lint or gate floor.

Verification: the draft digest command returned the exact head and digest above. Git 2.55.0's read-only `rev-parse` and `worktree list --porcelain` reports confirmed this linked worktree's common directory and reported main checkout; `git worktree add -h` confirmed the `--no-track` option. The named separate-directory exception was assessed against A-8 and the common-directory derivation; no repository fixtures were created because Git-state mutations were prohibited. Source checks covered `internal/draft/compile/document.go`, `internal/draft/readiness/links.go`, `internal/harnessconfig/config.go`, `cmd/implement-now/resolver.go`, `cmd/implement-now/run.go`, the harness adapters, and the shared runtime's brief, memory, access, environment, task-state and port contracts. The read sources match the reviewed commit. Remote draft-status confirmation was unavailable because its `github.com` DNS lookup failed; local branch, head, digest and version-2 lock-tag checks succeeded.

### FND-1 — The fetch is covered by an inaccurate claim that nothing leaves the machine

- **Severity**: advisory (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design: D-1, Security and privacy, says the operation performs “one fetch” and then “It reads no credential and sends nothing off the machine.” D-1 create step 4 explicitly fetches the configured mainline remote as `draft open` does; `internal/draft/lifecycle/open.go`, `Open`, invokes `git fetch` with `cfg.Remote` and `cfg.Branch`. For a network remote this contacts another machine and may use Git's configured authentication. PRD NR-1 requires that prototype commits and proposals remain unpublished; it does not require an offline or credential-free fetch. AP-31 and incident I13 favor withdrawing this stronger claim rather than adding machinery to enforce it.
- **Acceptance condition**: Narrow D-1's security paragraph to acknowledge the configured mainline fetch's normal Git network and authentication behavior, while stating that the command does not publish the prototype branch or its changes. Remove or qualify the absolute credential and off-machine claims; no new mechanism is needed.
- **Disposition**: declined — the sentence is unchanged since version 1 and was approved in both earlier lock episodes. Version 3 is scoped to the layout change under problem:prototype#A-8 (delivery:prototype#E-53), and the claim governs no requirement: NR-1 asks only that the prototype is not published, which the fetch does not do. Applying it would move the digest and send both subjects to a second round for wording on no task's path. Carried as an open item for the next contract version. (Advisory decline; no escalation.)

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | approve-with-advisories | 2026-10-21 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 0 blocking · 1 advisory
- **Dispositions**: 0 pending · 0 applied · 1 declined · 0 withheld-as-violation

## Overlay corrections

none
