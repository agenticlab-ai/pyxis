# Review: maintenance-jobs — design-review report

| Field | Value |
|---|---|
| **In one line** | Request changes: installation authority, effective cron environment, cutover, resource lifecycle, and extension/observability obligations need explicit mechanisms. |
| **Relevant-when** | Resolving the first maintenance-jobs design review before the exact-head human lock. |
| **Subject** | design:maintenance-jobs · shape: none · supersedes: — |
| **Moment** | draft |
| **Reviewer** | tech-design-review · repository guide at c13152fad903c05329889398771c7373aed50213 |
| **Date(s)** | 2026-10-05 |
| **Verdict** | request-changes |
| **Tracking** | https://github.com/agenticlab-ai/pyxis/pull/1443 |
| **Template version** | design-review-report/v1 |

**Review subject:** tech-spec
**Draft digest:** sha256:df52a1d0b77ea932396c58c56d3836ef5815cb4c47dcfe394735efeaa2160bd5

## Round 1

Reviewed committed subject c13152fad903c05329889398771c7373aed50213.
The upstream requirements' human-only routing section was excluded. Four parallel
lenses and independent adversarial scrutiny produced the following survivors.
Author dispositions below describe the subsequent working revision. This verdict
remains tied to the original digest; a fresh-digest report will record re-review.
None grants implementation authority.

### FND-1 — Cutover does not establish legacy process quiescence

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:maintenance-jobs#D-7 replaces the cron entry "in one backed-up crontab replacement" and promises "Cutover never enables both uncoordinated runners." Removing an entry does not stop an already running or dispatched legacy invocation. Upstream: PRD §5.4 forbids parallel old/new executors; DOD-5 requires verified cutover/rollback.
- **Acceptance condition**: Fence future legacy starts and drain or prove termination of already dispatched/running legacy processes before activating the new entry. Refuse cutover if quiescence cannot be proved. Exercise active and delayed-start legacy fixtures and the inverse rollback transition.
- **Disposition**: applied — D-7 defines the legacy lock, retired launch-path fence, exact process drain, refusal, and inverse rollback protocol; the PRD is unchanged.

### FND-2 — Failed installation can expose an unintended source binding

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:maintenance-jobs#D-2 says "publish the derived source binding before enabling new entries" and "Restore the prior binding on failed installation." D-1 makes that binding execution authority, but the lock only serializes installers. An existing entry can claim from the new binding before a failed cron write; restoring a file cannot undo that launch. Upstream: FR-8 published policy authority and UB-5 failed reconciliation.
- **Acceptance condition**: Synchronize execution authority with the entire install/verify/rollback transaction. An executor must never claim using an intermediate or failed generation; retain a verified old generation or explicitly unavailable state. Pause after the binding update, invoke the old cron entry, inject installation failure, and prove zero new-generation claims/launches.
- **Disposition**: applied — D-1 through D-3 specify generation state, a shared install/claim transaction lock, verified commit/rollback, and the paused-intermediate-state failure fixture.

### FND-3 — System timezone does not establish the effective cron environment

- **Severity**: blocking (CAL-4)
- **Lens**: feasibility-completeness
- **Evidence**: design:maintenance-jobs#D-2 preserves "all other bytes" but checks only "the system timezone." On a supported Linux cron provider, an earlier retained `CRON_TZ=UTC` changes the entry's scheduling timezone despite a Los Angeles system clock. Other inherited shell/delay settings can invalidate quoting or the grace window. Upstream: FR-3 native schedule and UB-4 timezone refusal.
- **Acceptance condition**: Validate the effective scheduler environment at each managed entry's insertion position. Refuse unsupported/mismatched scheduling, shell, or delay settings without altering unrelated globals. A Los Angeles host/job with retained `CRON_TZ=UTC` must refuse with zero writes. Provider semantics are documented in [Cronie's crontab manual](https://raw.githubusercontent.com/cronie-crond/cronie/master/man/crontab.5).
- **Disposition**: applied — D-2 validates provider-specific effective timezone, shell/startup, and delay directives at each managed entry position, with refusal fixtures.

### FND-4 — Core verb extension lacks an additive dispatch/usage seam

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:maintenance-jobs#Principles calls AP-27 "consumer-owned ports and additive adapters"; D-1 says `cmd/pyxis/maintenance.go` "supplies dispatch and composition." The current root has a manual dispatch switch and usage list. Upstream: AP-27 governs additive file units and derived registries, explicitly identifying those shared append surfaces.
- **Acceptance condition**: Correct the principle attribution and specify additive command registration/discovery with dispatch and usage derived from the same file unit, or record a reasoned deviation. Preserve existing verbs and verify registration conformance; no broad unrelated CLI rewrite is required.
- **Disposition**: applied — Principles uses AP-1 for ports and AP-27 for additive extension; D-1 defines self-registration and one derived dispatch/usage seam with conformance verification.

### FND-5 — Disposable clone/cache resources lack an owned terminal lifecycle

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:maintenance-jobs#D-5 creates a clone "keyed by repository/job/owner" and says "Caches and clones live in runtime state" but defines no terminal disposition or abandoned-resource handling. Upstream: AP-12 requires an accounted endpoint and a same-change janitor for new throwaway types. PRD §5.4 requires operator-managed retention of diagnostics and partial work, so automatic deletion of that evidence is not an acceptable remedy.
- **Acceptance condition**: Distinguish retained evidence from disposable runtime resources. Define ownership, success/failure/abandonment disposition and a safe same-change reaping path. Preserve active and unexported work, retained partial evidence, and all claims; cleanup must not introduce recovery or takeover.
- **Disposition**: applied — D-5 accounts for run-owned clones/caches, retained partial work, export/release journals, and a same-change conservative resource janitor with lifecycle fixtures.

### FND-6 — Persistent safety refusal is only observable as individual events

- **Severity**: blocking (CAL-2)
- **Lens**: upstream-alignment
- **Evidence**: design:maintenance-jobs#D-3 logs "skip/refusal reason" and D-1 reports the "latest shared attempted outcome"; neither recognizes repeated failures preventing eligible work. Upstream: AP-16 requires counting, alarming, or self-healing persistent safety skips.
- **Acceptance condition**: Define typed local counters for repeated dependency/safety refusals, expose them through status, and specify increment/reset rules with fixtures. Ordinary disabled/not-due/losing-claim skips need not imply failure. No notifications, retries, or takeover service are required.
- **Disposition**: applied — D-3 defines typed local lifetime/consecutive counters, increment/reset rules, status visibility, and failure/ordinary-skip fixtures.

### FND-7 — Execution-success evidence semantics need definition

- **Severity**: advisory (CAL-3)
- **Lens**: contract-quality
- **Evidence**: design:maintenance-jobs#D-4 names `succeeded`, `failed`, and `timed-out` without defining the artifact basis of `succeeded`. D-5 requires reports and D-6/D-7 distinguish scheduling success from the CI target, but the transition rule remains unspecified. Upstream: NFR-4 distinguishable outcomes and AP-13 evidence-based reporting.
- **Acceptance condition**: State how process completion, retained report/proposal evidence, claimed publication, and verification of the ten-minute target differ. An exit-zero fixture with a missing report or failed claimed publication must exercise the chosen outcome.
- **Disposition**: applied — D-4 defines verified report/publication success separately from the CI objective; D-5 adds missing-report/unpublished-proposal fixtures.

### FND-8 — A close schedule change still admits the old cron entry

- **Severity**: blocking (CAL-1)
- **Lens**: coherence
- **Evidence**: design:maintenance-jobs#D-3's five-minute grace claims to refuse "an old cron entry after a published schedule change." An installed 02:00 entry still admits a newly published 01:58 occurrence at 02:00; neither argv nor the source binding proves the installed schedule matches. Upstream: UB-4 requires stale-schedule fixtures to launch zero agents.
- **Acceptance condition**: Require published/installed schedule agreement or equivalent proof before claiming. The old 02:00 installation must skip the new 01:58 schedule, while a reconciled installation permits legitimate startup delay. Live prompt and disable updates must retain their behavior without reinstalling.
- **Disposition**: applied — the manifest carries installed cron/timezone fingerprints; D-3 refuses drift before claiming and explicitly verifies the 02:00/01:58 counterexample.

## Verdict history

| Round | Verdict | Date |
|---|---|---|
| 1 | request-changes | 2026-10-05 |

## Escalation brief

none

## Measurements

- **Rounds**: 1
- **Findings**: 7 blocking · 1 advisory
- **Dispositions**: 8 applied · 0 declined · 0 withheld-as-violation

## Overlay corrections

none
