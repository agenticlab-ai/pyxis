# Review: forgejo-feature-draft — design-review report

| Field | Value |
|---|---|
| **In one line** | request-changes — the installed lock-recording entrypoint and recovery after a prepared-head push need defined paths. |
| **Relevant-when** | Reviewing this Forgejo draft’s lock delivery and interrupted preparation behavior. |
| **Subject** | design:forgejo-feature-draft · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · draft-born guide read 2026-10-04 |
| **Date(s)** | 2026-10-04 |
| **Verdict** | request-changes |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1365 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec  
**Draft digest:** sha256:7ca390f9bbbd47613d3b5f33081acdc904d59677316b2eec94dfe228f956ac10

## Round 1

I verified clean tracked HEAD `1be3af0998cb6669f4a46e8180c02bf9b7de1271` against base `eaac1a9d2d8358128fe83f2f42d7ed82bd1bbf2a`, and verified the supplied tech-spec and §0-stripped PRD-body SHA256 values. I read those subject files, brainstorm/003, problems.md P-9, PRD review009, applicable frozen-design slices, governing references, and relevant current source seams. I applied all four draft-adapted lenses and screened candidate findings against the authorized P-9 ruling. The shared digest, publication authentication, eleven intermediate checks, and stale007 readiness result are supplied ground; I did not rerun them or execute tests, gates, or network probes.

### FND-1 — Installed Forgejo merges have no defined lock-recorder entrypoint

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: D-6 changes `cmd/draft-lock-record` to select the bound host but says to keep existing push-range discovery without specifying how an installed Forgejo mainline merge invokes it or supplies the before/after range. The existing trigger is GitHub-specific (`.github/workflows/lock-record.yml`); the installed Forgejo workflow `.pyxis/dist/forgejo-workflows/docs-gate.yml` has a mainline push trigger but no lock-recording step. PRD FR-6 and blocking DOD-4 require the merged contract to yield a verified annotated tag. A direct recorder fixture could pass while an installed merge leaves no lock or planning basis.
- **Acceptance condition**: Define the installed Forgejo lock-recording entrypoint, its validated mainline range, repository and credential inputs, and repeat-invocation behavior. Make the installed merge-to-tag proof exercise that entrypoint after the human merge, with exact tag readback, while retaining the GitHub route.
- **Disposition**: applied — author defines the installed frontdoor's explicit validated --before/--after invocation in D-6, retains GitHub's workflow route and requires the actual installed post-human-merge command plus repeated/fresh-clone tag proof. Existing FR-6/DOD-4 are unchanged. Await scope-locked round2 verification; original reviewer finding/verdict/digest remain preserved.

### FND-2 — Recovery after publishing prepared head H has no re-entry protocol

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: D-4 calls a failed body update or reviewer request a pending approval-request event for H, but gives no durable event shape or recovery reader. The current event schema accepts a closed set of kinds without that event or H (`internal/draft/events/log.go:18`, `:68`). In the current hosted preparation seam, the leased push precedes body/ready/reviewer calls, while the author branch is aligned to H only after all succeed (`internal/draft/lock/prepare.go:342`, `:355`). A retry after an intervening failure meets the remote/local-head refusal at `prepare.go:138`. PRD UB-6 and blocking DOD-5 require kill/retry reconciliation without losing working bytes or duplicating work.
- **Acceptance condition**: Specify crash-safe evidence and a re-entry path for a remote prepared H with an unchanged local author branch, including how a restart verifies H, digest, lease, and the same PR before completing the body and review request. Cover interruption after the push and after each subsequent host update from a new process or fresh clone.
- **Disposition**: applied — author defines D-4's draft-preparation-intent/v1 evidence ref, retained R/H objects, leased phases and verified re-entry before local/remote equality; recovery reuses H/PR and reconciles body/ready/reviewer effects without duplicate uncertain writes. The closed draft-event/v1 schema remains unchanged. New-process/fresh-clone kill/retry cases cover every publication boundary. Existing UB-6/DOD-5 are unchanged. Await scope-locked round2 verification; original reviewer finding/verdict/digest remain preserved.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-04 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 2 blocking · 0 advisory
- **Dispositions**: 0 applied · 0 declined · 0 withheld-as-violation · 2 pending

## Overlay corrections

none
