# Requirement problems

### P-1 — FR-3 — local-origin enforcement boundary

**Raised by:** Local code-host design review, 2026-09-28.

**Problem:** The AEC bare origin and agent share an OS account. The agent can edit a server hook and Git refs there. A signed approval can be verified by Pyxis consumers, but it cannot by itself provide the existing hosted route's branch-protection guarantee against direct writes to that origin.

**Ruling:** operator · The AEC test will keep the bare origin under the shared OS account. · Limit FR-3 to exact-head receipt consistency and disclose the lack of host-level protection and approval identity proof.

**Source:** Operator answer in the AEC session on 2026-09-28: “Keep the shared-account origin and accept signed-record verification without host-level mainline protection.” The later signing-key answer narrows “signed-record” to an unsigned consistency record.

### P-2 — NFR-1 — independent operator key custody

**Raised by:** Local code-host design review, 2026-09-28.

**Problem:** A key stored in the same agent-accessible filesystem or environment would let the agent produce its own approval signature. The route needs a signer that the agent cannot invoke without the operator's act.

**Ruling:** operator · An independent signing key is outside the requested test setup. · Replace NFR-1 with explicit disclosure of unauthenticated local approval identity, and do not claim signature verification.

**Source:** Operator answer in the AEC session on 2026-09-28: “ignore this requirement.”

### P-3 — UB-4 — proposal publication race

**Raised by:** GitHub code review on PR #1312, discussion 4125900289, 2026-09-28.

**Problem:** A branch-first, proposal-second write can leave a proposal naming a stale branch head when another writer moves the branch between pushes. A compare-and-swap on the proposal ref alone does not prevent the mismatch.

**Ruling:** operator · Split ref writes allow a stale proposal record after a branch race. · Publish and revise the branch ref and its proposal ref in one atomic push with an expected old OID for each. Refuse origins without atomic push support. Replace the partial branch-first recovery case with pairwise atomic retry. The operator still reviews the resulting exact draft head.

**Source:** Operator reply “approved” on 2026-09-28 to the request to approve the common-case proposal as written at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`).

### P-4 — DOD-4 — release evidence cycle

**Raised by:** GitHub code review on PR #1312, discussion 4125900303, 2026-09-28.

**Problem:** DOD-4 needs an AEC installed proof before final release publication, while the design said AEC must pin that final release before it exists.

**Ruling:** operator · The final release cannot be a prerequisite of its own installed acceptance test. · Pack and verify a candidate artifact set from the intended merged release commit, pin it in AEC through the local artifact-source override, and run DOD-4 before publication. The final tag must point to the same commit and publish that same verified set; AEC then repins the published source and runs installation smoke checks. Any changed commit or artifact hash requires repeating the installed proof.

**Source:** Operator reply “approved” on 2026-09-28 to the request to approve the common-case proposal as written at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`).

### P-5 — UB-1 — clone-stable local origin identity

**Raised by:** GitHub code review on PR #1312, discussion 4125900312, 2026-09-28.

**Problem:** The draft promised distinct-repository rejection without defining repository identity. A clone can check retained record and object consistency, but that check alone cannot establish which origin supplied them.

**Ruling:** operator · The single-operator local route needs to validate the configured origin's proposal, head, target, and landing, but cannot meaningfully authenticate a distinct origin under the accepted shared-account policy. · Remove the repository-ID ref and cross-origin receipt guarantee. A fresh clone of its configured origin can still check stored record and Git-object consistency; a copy preserving refs and objects may reuse those records, and verification must state that it does not prove repository identity.

**Source:** Operator asked for common cases in a personal Pyxis setup, then replied “approved” on 2026-09-28 to the revised proposal at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`).

### P-6 — FR-4 — task landing target

**Raised by:** GitHub code review on PR #1312, discussion 4125958223, 2026-09-28.

**Problem:** `implement-now` task proposals target a feature-integration branch, while the receipt and stale-ref checks named only mainline. A receipt for mainline would not bind the actual task landing.

**Ruling:** operator · A mainline-only receipt does not guard task landings on an integration branch. · Every proposal and receipt names its fully qualified target ref and exact old full SHA. Pyxis derives that ref from the configured mainline for a draft or the approved feature-integration destination for a task; an agent-provided alternate target is refused. Landing atomically updates that target and rejects a moved branch or target. The operator still reviews each exact code head.

**Source:** Operator reply “approved” on 2026-09-28 to the request to approve the common-case proposal as written at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`).

### P-7 — FR-7 — task landing result

**Raised by:** GitHub code review on PR #1312, discussion 4125958223, 2026-09-28.

**Problem:** `implement-now` task proposals can target a feature-integration branch, so verifying only a mainline result misses the actual code landing.

**Ruling:** operator · Verifying mainline can miss a task's actual landing result. · Bind the implementation receipt to the workflow-derived feature-integration target and verify that ref's landed bytes. Refuse an alternate target chosen by the proposal. The operator still reviews each exact code head.

**Source:** Operator reply “approved” on 2026-09-28 to the request to approve the common-case proposal as written at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`).

### P-8 — UB-2 — moved task target during landing

**Raised by:** GitHub code review on PR #1312, discussion 4125958223, 2026-09-28.

**Problem:** A task target branch can move between approval and landing just as mainline can; the old failure row named only mainline.

**Ruling:** operator · An implementation target can move after approval and must fail the same stale-ref check as mainline. · Atomically compare and swap the named target ref and leased proposal branch with the final proposal state. A moved target or branch refuses the whole landing and keeps the receipt evidence for new review.

**Source:** Operator reply “approved” on 2026-09-28 to the request to approve the common-case proposal as written at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`).

### P-9 — FR-2 — proposal discovery scope

**Raised by:** Exact-head lock packet review for PR #1312 at `9236f9c395b1446bdbce868694a1d7f3ecc9a612`, 2026-09-28.

**Problem:** The revised FR-2 fit criterion names a clone of the configured origin, but the packet did not attribute that scope change to a product ruling.

**Ruling:** operator · Proposal discovery needs to work from the configured origin; independent origin identity is outside the approved common-case scope. · Require a clone of the configured origin to list the feature and exact proposed head, without claiming cross-origin identity verification.

**Source:** Operator reply “approved” on 2026-09-28 to the common-case proposal at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`), whose FR-2 row already had this scope.

### P-10 — FR-6 — retained lock verification scope

**Raised by:** Exact-head lock packet review for PR #1312 at `9236f9c395b1446bdbce868694a1d7f3ecc9a612`, 2026-09-28.

**Problem:** The revised FR-6 requirement and fit criterion limit clone verification to the configured origin and disclaim independent repository identity, but the packet called that change unruled.

**Ruling:** operator · A clone can validate retained receipt, reviewed-head, and landing bytes without proving an independent origin identity. · Require consistency verification from a clone of the configured origin and an explicit disclaimer of approver and repository identity authentication.

**Source:** Operator reply “approved” on 2026-09-28 to the common-case proposal at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`), whose FR-6 row already had this scope.

### P-11 — ABUSE-2 — replay fixture scope

**Raised by:** Exact-head lock packet review for PR #1312 at `9236f9c395b1446bdbce868694a1d7f3ecc9a612`, 2026-09-28.

**Problem:** The revised ABUSE-2 row narrows receipt-replay testing to a push on the configured origin, but the packet called the row unruled.

**Ruling:** operator · FR-4 rejects reuse after a head or target changes on the configured origin; copied-origin replay protection is outside the approved scope. · Test same-origin replay after a push and disclose that a copied origin carrying the same records is not independently identified.

**Source:** Operator reply “approved” on 2026-09-28 to the common-case proposal at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`), whose ABUSE-2 row already had this scope.

### P-12 — NFR-1 — independent repository identity disclosure

**Raised by:** Exact-head lock packet review for PR #1312 at `9236f9c395b1446bdbce868694a1d7f3ecc9a612`, 2026-09-28.

**Problem:** The revised NFR-1 row adds the lack of independent repository identity to the local security disclosure; P-2 alone covered only the waived approval-signing key.

**Ruling:** operator · Without a separate origin-ID protocol, the configured-origin check does not authenticate an independent repository identity. · Doctor, receipts, and clone verification must disclose that limit alongside the already accepted unauthenticated approver and absent host-level protection.

**Source:** Operator reply “approved” on 2026-09-28 to the common-case proposal at PR #1312 head `78b13bbeb974cede10d4ae5119bf1a354dd9dacb` (digest `sha256:1d85f040d154c58cdda2878b558084ac420d13da8482db6f73caf584f0274622`), whose NFR-1 row already had this scope.

### P-13 — FR-9 — empty-origin draft preparation

**Raised by:** GitHub code review on PR #1312 at prepared head `072c96c6eeab000421420bcd94e68bfb62148101`, 2026-09-28.

**Problem:** FR-9 promises a base-free proof for the first mainline push, while D-2 and D-6 apply that case to a local draft lock without defining how D-4 prepares it. The existing preparation path fetches and resolves the configured mainline, rebases onto it, and loads readiness sources from it before a zero-OID landing lease can run. The local first-push hook in issue #1305/#1307 separately handles publication from the installed Pyxis init epoch. The outstanding product choice is whether drafts must wait for that bootstrap push or must gain a separate base-free draft compiler and readiness path; FR-9's first-mainline-push promise remains in either case.

**Ruling:** operator · Drafts should use the normal prepared-mainline path after the first mainline push. · Require the configured mainline ref on the selected bare origin before opening or preparing a local draft; if absent, refuse with an actionable first-push remedy. Keep FR-9's base-free proof for that separate first mainline push, then use normal history gates. The already initialized AEC origin needs no additional mainline commit.

**Source:** Operator answer “ok, push something to the mainline” in the AEC session on 2026-09-28, responding to the choice between pushing mainline before drafts and supporting drafts before the first mainline push. The AEC origin already had `refs/heads/main` at `29d26c2949c5d644f4db62f7dfd8e92804276be1` when checked; no placeholder push is needed.
