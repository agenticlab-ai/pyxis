# Amendment 002 — single-user Argus attribution

| Field | Value |
|---|---|
| **Amends** | argus-boundary-enforcement *(chain through 001)* |
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Date** | 2026-09-29 |
| **Learned-from** | review-thread:https://github.com/agenticlab-ai/pyxis/pull/1311#discussion_r4136826481 · review-thread:https://github.com/agenticlab-ai/pyxis/pull/1311#discussion_r4136826488 |
| **Blast radius** | tier: contract-changing; touches: FR-12, FR-14, DOD-10, DOD-11, DOD-14, DOD-15; reason: replaces the drive-inaccessible, forgery-resistant requirement and its blocking proof rows with a single-user audit contract that the shared-account workflow can actually meet |
| **Template version** | amendment-template/v2 |

## Operations

### op: supersede FR-12

Reason: the Argus drive can invoke Git and set its own environment in the
current shared-account workflow. A repo-local commit hook cannot guarantee
that a misbehaving drive neither omits nor forges its own stamp. The operator
chose this simpler single-user workflow and retained exact-head human review.
The requirement now states the checkable audit property, while the existing
protected-path fail-closed and positive-human rules remain separate.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-14 | Argus shall supply its problem slug on every drive-authored Git commit in the configured Pyxis workflow. When that slug reaches the `prepare-commit-msg` hook, the hook shall append `Argus-Drive: <slug>` and a content binding for the commit's tree and parent; when feature-draft preparation rebases an already stamped commit, it shall verify the original binding and refresh only a valid one without stamping an unstamped source commit. At CI, the attribution gate shall re-derive the binding from Git without a mutable platform badge or ambient credential. This is an audit signal in the single-user shared-account workflow; it does not assert that an agent with raw Git access cannot omit or forge the signal. | Preserves verifiable attribution for ordinary drive commits and draft rounds while stating the shared-account limit and relying on exact-head operator review for merge | Must | high | a real hook-produced drive commit and its rebased draft version derive as Argus; a stale or replayed binding is refused before rebase and underivable at the gate; a source unstamped commit remains unstamped; CI reads only in-tree Git metadata without a credential | Test | G1, J3, P-2, AP-14, AP-26, design:argus-draft-attribution#DINV-2 |

### op: supersede DOD-10

Reason: the live-drive proof still matters, but it must cite the replacement
attribution requirement rather than the superseded stronger guarantee.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-14 | A drive commit produced through the configured workflow carries a verifiable stamp, and a protected-path change it introduces is refused by the attribution gate | FR-14; FR-9 | Test | real-hook commit and gate verdict | yes |

### op: supersede DOD-11

Reason: the old anti-forgery proof claimed a guarantee against a raw-Git actor
that the shared account does not enforce. Keep the replay, malformed, and
absence checks that are observable at the gate.

| ID | Done criterion | Source | Verify | Required evidence | Blocking? |
|---|---|---|---|---|---|
| DOD-15 | A stale or replayed `Argus-Bind` is refused before draft rebase and underivable at the gate, a protected-path diff with neither a valid Argus signal nor positive human attestation fails closed, and a positively Argus change cannot be laundered by a human-attestation trailer | FR-14; FR-13; FR-10 | Test | binding, absence, malformed, and both-signal fixtures | yes |
