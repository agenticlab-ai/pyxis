# Requirement problems

The following corrective edits implement the operator-directed working-draft repair recorded verbatim, with its bounded interpretation, in problem:forgejo-feature-draft#A-5. Final exact-head contract approval remains separate. Argus still has no general rule-acceptance grant.

### P-1 — FR-5 — Enforce human approval before merge

**Raised by:** Independent PRD review `reviews/001.md`, FND-1.
**Affected requirements:** FR-5, FR-6, DOD-3.
**Problem:** FR-6 checks the human approval only after a prepared contract has merged. An unapproved merge can already put compiled records on mainline before tag refusal. The frozen feature-draft design requires enforceable protection before merge.
**Requested ruling:** Approve this FR-5 addition: before permitting a prepared contract merge, enforce the separately configured human's authenticated approval of the exact current head, dismiss approval after a new push, and bind privileged actors to the same rule. If native protection cannot enforce it, require an equivalent enforceable mechanism or refuse with a named remedy. Extend the FR-5 fit criterion and DOD-3 with unapproved, stale, agent, and privileged-bypass negative merge evidence. Preserve FR-6's independent post-merge review and mainline verification.
**Ruling:** operator · Working-draft repair instruction and bounded interpretation recorded in problem:forgejo-feature-draft#A-5 · Enforce the separately configured human’s authenticated approval of the exact current prepared head before merge, dismiss stale approval after a push, cover privileged actors, and fail closed with an enforceable protection remedy; preserve independent post-merge verification.
**Status:** answered — corrective working-draft change applied under A-5; final human contract approval remains pending.

### P-2 — FR-1 — Validate the destination before sending credentials

**Raised by:** Independent PRD review `reviews/001.md`, FND-2.
**Affected requirements:** FR-1, UB-1, DOD-5.
**Problem:** The current Must rows prove zero writes on an origin mismatch, but an authenticated read or Git request can disclose a credential before a write. ABUSE-3's intended pre-credential check lacks blocking acceptance evidence.
**Requested ruling:** Require exact selected-host/repository validation before any credential-bearing API or Git request. Refuse credential-bearing redirects to an unbound destination. Extend FR-1, UB-1, and DOD-5 with wrong server, port, base path, owner, repository, scheme, and hostile-redirect fixtures proving zero credential-bearing requests and zero writes. Retain output-canary checks.
**Ruling:** operator · Working-draft repair instruction and bounded interpretation recorded in problem:forgejo-feature-draft#A-5 · Validate the exact selected host/repository before credential-bearing API or Git requests; refuse credentials on unbound redirects and require zero credential-bearing requests and writes in mismatch fixtures.
**Status:** answered — corrective working-draft change applied under A-5; final human contract approval remains pending.

### P-3 — FR-7 — Keep full memory integrity at the final head

**Raised by:** Independent PRD review `reviews/001.md`, FND-3.
**Affected requirements:** FR-7, DOD-6 (reserved).
**Problem:** FR-7's "applicable" and "remaining" checks do not explicitly require the separate full memory-integrity gate at the final head, contrary to the narrow operator A-4 exception and the existing integrity obligation.
**Requested ruling:** The implemented opening/round policy may defer only final-draft-readiness; all other applicable checks, including memory integrity, remain required. Preparation, prepared-head publication, and locking require both the full memory-integrity gate (including lossless problem/delivery bindings) and current-digest readiness. Reserve DOD-6 for positive complete-gate evidence and missing-binding/invalid-evidence final-head refusals. A-4 remains a temporary exception for intermediate documentation of this source problem; it is not the implemented policy.
**Ruling:** operator · Working-draft repair instruction and bounded interpretation recorded in problem:forgejo-feature-draft#A-5 · Defer only final readiness on intermediate production pushes; retain every other applicable check, and require complete memory integrity plus current-digest readiness for preparation, prepared-head publication, and lock; add blocking DOD-6 evidence.
**Status:** answered — corrective working-draft change applied under A-5; final human contract approval remains pending.

### P-4 — FR-2 — Cover reopened and document-pipeline status

**Raised by:** Independent PRD review `reviews/001.md`, advisory FND-4.
**Problem:** The stated fit criterion names drafting/locked states but lacks explicit reopened-draft and unmigrated document-pipeline cases promised by the body and frozen status contract.
**Requested ruling:** Extend the FR-2 fit criterion to cover no contract, a new draft, a locked version, a reopened draft while the previous version still binds, and an unmigrated document-pipeline feature. Keep zero other-host CLI calls and explicit absent/ambiguous state handling.
**Ruling:** operator · Working-draft repair instruction and bounded interpretation recorded in problem:forgejo-feature-draft#A-5 · Explicitly test no contract, a new draft, a locked version, a reopened version while the previous lock binds, and an unmigrated document-pipeline feature, without another host’s CLI.
**Status:** answered — corrective working-draft change applied under A-5; final human contract approval remains pending.

### P-5 — DOD-3 — Acceptance evidence for P-1

**Raised by:** Already-proposed P-1 corrective patch, recorded under problem:forgejo-feature-draft#A-5; separate single-row attribution for the compiler.
**Problem:** The blocking delivery evidence must exercise the pre-merge approval obligation, not only refuse a later tag.
**Ruling:** operator · Same bounded repair instruction and interpretation as P-1, problem:forgejo-feature-draft#A-5; no additional product choice · Require pre-merge protection or an equivalent enforceable mechanism, positive exact-head human evidence, and unapproved, stale, agent, rejected, changed-head, and privileged-bypass merge negatives; retain independent post-merge review.
**Status:** answered — companion correction applied under A-5; final human contract approval remains pending.

### P-6 — UB-1 — Acceptance evidence for P-2

**Raised by:** Already-proposed P-2 corrective patch, recorded under problem:forgejo-feature-draft#A-5; separate single-row attribution for the compiler.
**Problem:** Zero writes does not prove that an unbound destination received no credential.
**Ruling:** operator · Same bounded repair instruction and interpretation as P-2, problem:forgejo-feature-draft#A-5; no additional product choice · Refuse before any credential-bearing API/Git request or host write; test mismatched destination components and hostile redirects for zero credential-bearing requests and zero writes.
**Status:** answered — companion correction applied under A-5; final human contract approval remains pending.

### P-7 — DOD-5 — Acceptance evidence for P-2

**Raised by:** Already-proposed P-2 corrective patch, recorded under problem:forgejo-feature-draft#A-5; separate single-row attribution for the compiler.
**Problem:** The blocking credential evidence must include delivery to an unbound destination as well as output leakage.
**Ruling:** operator · Same bounded repair instruction and interpretation as P-2, problem:forgejo-feature-draft#A-5; no additional product choice · Require hostile-redirect and origin fixtures with zero credential-bearing requests to unbound destinations, zero output-canary leaks, and zero false PR/tag.
**Status:** answered — companion correction applied under A-5; final human contract approval remains pending.
