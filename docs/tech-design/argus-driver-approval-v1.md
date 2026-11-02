# Argus driver approval on GitHub

| Field | Value |
|---|---|
| **Status** | Approved |
| **Owner** | @mdarabi |
| **Approver(s)** | @mdarabi or the authenticated Argus driver under the operator's mainline approve-merge grant; independent security review required |
| **Date** | 2026-10-05 |
| **Relevant-when** | changing hosted approval identities, driver decision publication, or feature lock recording |
| **Scope** | GitHub delegated approval and its lock evidence; existing draft lifecycle, readiness, human approval, local Git, and execution mechanics retained |
| **Serves** | prd:feature-draft/FR-1 · prd:feature-draft/FR-2 · prd:feature-draft/FR-3 · prd:feature-draft/FR-4 · prd:feature-draft/FR-5 · prd:feature-draft/FR-6 · prd:feature-draft/FR-7 · prd:feature-draft/FR-8 · prd:feature-draft/FR-9 · prd:feature-draft/FR-10 · prd:feature-draft/FR-11 · prd:feature-draft/FR-12 · prd:feature-draft/FR-13 · prd:feature-draft/FR-14 · prd:feature-draft/FR-15 · prd:feature-draft/FR-16 · prd:feature-draft/FR-56 · prd:feature-draft/FR-18 · prd:feature-draft/FR-19 · prd:feature-draft/FR-20 · prd:feature-draft/FR-21 · prd:feature-draft/FR-22 · prd:feature-draft/FR-23 · prd:feature-draft/FR-24 · prd:feature-draft/FR-25 · prd:feature-draft/FR-26 · prd:feature-draft/FR-55 · prd:feature-draft/FR-29 · prd:feature-draft/FR-54 · prd:feature-draft/FR-31 · prd:feature-draft/FR-32 · prd:feature-draft/FR-33 · prd:feature-draft/FR-34 · prd:feature-draft/FR-57 · prd:feature-draft/FR-36 · prd:feature-draft/FR-37 · prd:feature-draft/FR-38 · prd:feature-draft/FR-39 · prd:feature-draft/FR-40 · prd:feature-draft/FR-41 · prd:feature-draft/FR-42 · prd:feature-draft/FR-43 · prd:feature-draft/FR-44 · prd:feature-draft/FR-45 · prd:feature-draft/FR-58 · prd:feature-draft/FR-51 · prd:feature-draft/NR-1 · prd:feature-draft/NR-7 · prd:feature-draft/NR-4 · prd:feature-draft/NR-5 · prd:feature-draft/UB-1 · prd:feature-draft/UB-2 · prd:feature-draft/UB-15 · prd:feature-draft/UB-4 · prd:feature-draft/UB-14 · prd:feature-draft/UB-6 · prd:feature-draft/UB-7 · prd:feature-draft/UB-8 · prd:feature-draft/UB-9 · prd:feature-draft/UB-10 · prd:feature-draft/UB-16 · prd:feature-draft/NFR-5 · prd:feature-draft/NFR-2 · prd:feature-draft/NFR-3 · prd:feature-draft/NFR-4 · prd:feature-draft/ABUSE-1 · prd:feature-draft/ABUSE-2 · prd:feature-draft/ABUSE-3 · prd:feature-draft/ABUSE-4 · prd:feature-draft/ABUSE-5 · prd:feature-draft/ABUSE-6 · prd:feature-draft/ABUSE-7 · prd:feature-draft/ABUSE-8 · prd:feature-draft/EVT-1 · prd:feature-draft/EVT-2 · prd:feature-draft/EVT-3 · prd:feature-draft/EVT-4 · prd:feature-draft/EVT-5 · prd:feature-draft/EVT-6 · prd:feature-draft/EVT-7 · prd:feature-draft/FR-52 · prd:feature-draft/FR-53 · prd:feature-draft/NR-8 · prd:feature-draft/FR-59 · prd:feature-draft/FR-60 · prd:feature-draft/DOD-18 |
| **Supersedes** | feature-draft-local-git |
| **Builds-on** | docs/tech-design/memory-durable-evidence-v1.md · docs/tech-design/memory-causal-admission-v1.md · docs/tech-design/memory-staged-contracts.md · docs/tech-design/plan-new-routed-demand.md · docs/tech-design/standalone-docs.md · docs/tech-design/solution-shape-v1.md · design:argus-agent-v1 · design:argus-authority-contract-structure-arm |
| **Tracking** | issue:1438 |
| **Security/privacy impact** | yes — authenticated identity, delegated approval, and mainline authorization; reviewer: @mdarabi or granted Argus with independent security-abuse review |
| **Template version** | tech-design-template/v1 |

This is proposed merge-target content. The Approved header takes effect only
when the reviewed proposal lands; it asserts no prior exact-source approval.
This successor incorporates feature-draft-local-git's binding lifecycle,
compile, readiness, round, reopen, memory, planning, and explicit local Git
mechanisms unchanged. It replaces that predecessor's operator-only GitHub
approval restriction with the explicit delegated route below. All existing
local Git bindings and its declared operator procedure remain unchanged.

## 0. Read to rule — the human reviewer's section (~3 min)

*The body is the contract; this section only routes attention — on any conflict
the body governs; machines ignore it.*

**Judge these**

| # | The call | Could have gone | If you disagree |
|---|---|---|---|
| 1 | GitHub authenticates the driver account while its mainline grant authorizes the decision. | Require a human at every lock. | D-1, D-2 and D-7 make granted approval the driver's act. |
| 2 | A same-account author uses a typed commented review. | Require a second GitHub account. | D-2 and D-3 distinguish this from a human or native approving review. |
| 3 | The merge's first parent supplies policy; candidate and local overlays cannot grant authority. | Read current candidate config. | D-1 and D-6 preserve the authorization boundary. |

**Sanity-check the numbers** — one publication per command invocation, no automatic write retry, full head and policy commit binding (D-3, D-4).

**The author's open questions** — none for this scope (§7).

**Skip the rest** — structure and schema → docgate; design substance → independent technical review; implementation defects → codegate and acceptance tests.

## Principles

AP-6: closed versioned decisions and configuration. AP-11: one authority
parser and one approval adapter. AP-25: the driver protocol needs only the
native CLI, files, and shell. AP-26: an approval or platform merge badge does
not replace verified mainline content. The core consumes an approval port;
GitHub API and credential handling stay in the adapter.

## 1. Context and scope

Pyxis v0.1.5 refuses Argus lock approval even when `approve-merge` is granted.
The operator directed the driver to own PR review and approval in issue 1438.
GitHub also prohibits authors from submitting their own approving review. A
second GitHub account is unnecessary for a Pyxis driver decision when the
host can authenticate a commit-bound commented review.

This route supports GitHub contract and implementation PRs. It adds no
credential store, service, reviewer, automatic merge, authority promotion,
branch-protection mutation, or fallback to local Git. Existing independent
review and readiness checks still precede the driver's decision.

## 2. Solution shape

**Decision:** `pyxis approve` publishes an exact-head decision through the GitHub approval
adapter. The account and grant are read from immutable mainline objects. The
review carries the driver's evidence and canonical decision. A lock recorder
uses the merge's first parent as its policy version and verifies the host
review before emitting an immutable lock tag. A fresh clone reads that tag
and the existing exact compiled contract.

## 3. Decisions

### D-1 — Mainline owns the approval policy

**Decision:** `approval.operator_login` defaults to the historical `mdarabi`
identity; `approval.argus_login` is absent by default. They are closed config
keys and must be distinct accounts. A configured Argus account also requires
`approve-merge` in the valid mainline authority contract. Reuse its existing
parser, structural evaluator, inherited action bundles, and revoked kill
switch. Read raw `.pyxis/config.yaml` and the authority contract from the
host default branch's current commit, observed independently of the PR's
landing base. Implementation task PRs may target their feature integration
branch; that branch supplies no approval policy. Prepared contract PRs still
target mainline. Never read candidate or local overlay bytes to grant
authority. A local overlay may name the credential environment variable.
The operator alone changes policy. This implementation does not promote any
repository.

**Serves:** FR-56 · FR-58 · FR-59 · UB-15 · NR-8

### D-2 — A host review authenticates the driver

**Decision:** Authenticate the configured driver via GitHub's account endpoint.
Publish `APPROVE` when it differs from the PR author, and `COMMENT` when it
matches. Both require the canonical `argus-approval/v1` decision. Host-returned
review author and `commit_id`, not a principal string in the body, establish
the authenticated account and reviewed revision. An ordinary commented review
never authorizes a lock. A separate native approving review required by branch
protection remains required; a Pyxis decision does not satisfy it by fiction.

**Serves:** FR-58 · FR-59 · FR-60 · NR-8

### D-3 — Bind the whole approval subject

**Decision:** The closed decision carries schema, Argus principal, repository,
PR number, full head, full pre-merge policy commit, drive slug, review evidence,
optional capture line, and the prepared digest for contract PRs. Serialize
canonical JSON in one marked review block. Reject unknown fields, duplicate
or noncanonical keys, trailing content, multiple markers, empty evidence, and
invalid full SHAs. A draft PR must name its explicit reviewed digest matching
the single `Draft-Digest` trailer on its native preparation commit. Non-draft
PRs omit the digest. Both cold contract reviews remain the readiness check's
responsibility; implementation review remains its own protocol. The evidence
is the driver's accountable summary and references, not an assertion that the
approval command reran or independently authenticated a reviewer.

**Serves:** FR-57 · FR-58 · FR-59 · UB-15

### D-4 — Observe, publish once, exact-read, observe again

**Decision:** Require an open nondraft PR at the explicitly reviewed head.
Observe its base, author, branch, state, and draft flag, together with the host
default branch and its policy commit, before and immediately before publication.
Preserve an outstanding operator changes-requested review.
Write once with the exact `commit_id`; never automatically retry a failed
write. Read the returned review ID from its exact endpoint, require the same
identity and canonical body, then reobserve the target. Movement observed by the
final preflight produces zero writes. GitHub provides no atomic expected-head
or expected-base condition on review creation: movement after that observation,
including before submission, can leave one exact old-head decision. The final
observation returns a stale result, and the decision cannot authorize the changed
head or policy. The command
does not merge or mutate protection.

**Serves:** FR-59 · FR-60 · UB-15

### D-5 — Select the latest authorized decision

**Decision:** Human approvals keep their existing latest-submitted-review
selection. Driver selection accepts only the configured authenticated account,
current head, policy commit, and requested contract digest. A later dismissed
or changes-requested driver review revokes approval; ordinary discussion
comments and pending reviews do not replace it. Malformed typed decisions fail
closed. An outstanding operator changes request also prevents driver selection.

**Serves:** FR-56 · FR-58 · FR-59 · UB-15 · NR-8

### D-6 — Preserve historical lock bytes and identify delegated locks

**Decision:** Keep human `draft-lock/v1` records byte-compatible. New driver
locks use `draft-lock/v2`, adding `approval_kind: argus`, `policy_commit`, and
`approval_evidence`. Approver fields contain the actual host account; ReviewID
is the real GitHub review ID, including a commented review. The lock core
derives policy from the merge's first parent and supplies it through a bound
approval port along with the prepared head and digest. Refuse a delegated
result without that exact pre-merge policy binding. Existing immutable-tag,
idempotent-retry, compiled-text, and fresh-clone validation stay in place.

**Serves:** FR-58 · FR-60 · NR-8

### D-7 — Argus performs review and its granted act

**Decision:** Replace the guide's unconditional lock-approval prohibition with
an act-time grant check. For contracts, inspect the exact prepared contract
after both current-digest cold reviews and required native checks. For
implementation PRs, verify required checks and the independent exact-head
implementation review. Approve through the native command and journal the
review ID and policy commit. Without grant request the operator; keep explicit
local Git's operator procedure. Continue to verify landed content before
planning or reporting delivery.

**Serves:** FR-57 · FR-59 · FR-60 · DOD-18

### 3.8 Invariants

- **DINV-1** — Only trusted mainline policy and a host-authenticated configured account authorize a driver decision.
- **DINV-2** — The repository, PR, head, pre-merge policy, and prepared digest must match the reviewed decision.
- **DINV-3** — Same-account commented reviews are attributed to Argus and never presented as human or native GitHub approvals.
- **DINV-4** — A publication invocation performs at most one write and never merges or changes host protection.
- **DINV-5** — Historical human lock tags remain unchanged; a driver lock retains its host review and policy identity in a fresh clone.

### 3.9 Observability

- **Events/logs** — the command returns the exact host review ID, account, head, policy commit, kind, and review evidence; publication and observation failures name their boundary.
- **Metrics** — existing Argus delivery records retain the approval act and the real review identity; no new dashboard or service is introduced.
- **Alerts** — named command failures stop the drive and remain an open item; no background alerting daemon exists for this command.
- **Debugging surface** — inspect the PR's exact review endpoint and the annotated lock record; a potentially completed failed write is inspected before a deliberate retry.
- **Trace/correlation** — repository, PR number, drive slug, full head, policy commit, digest, and review ID join the decision, drive ledger, and lock.

### 3.10 Security & privacy

This changes hosted identity and approval authority. The configured credential
travels only in the scoped GitHub child process environment. Token values never
enter configuration, decision bodies, lock tags, or diagnostics. Candidate and
local grants are ignored; the existing authority parser validates the mainline
contract and its revoked kill switch (DINV-1). Typed canonical bodies bind the
whole subject and host review author (DINV-2, DINV-3). Outstanding operator
changes requests stop driver approval. Publication is not retried and host
protection is not changed (DINV-4). A single account can run author and driver
sessions: the host proves the account, while the independent review workflow
and driver evidence establish the accountable judgment boundary. Historical
and driver tag validation preserve durable attribution (DINV-5).

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| Keep human-only lock approval at every tier | Contradicts the operator's driver ownership request and makes approve-merge ineffective for feature locks. |
| Require a second GitHub account | Adds credential and account setup when the host already authenticates commit-bound commented reviews. Separate native reviews remain a repository option. |
| Treat any comment saying approve as approval | Does not bind the account, PR, version, policy, or digest and allows forged labels. |
| Change or bypass GitHub protection | Would remove a distinct enforcement boundary and is outside this approval command's scope. |

The interfaces and compatibility follow from these choices:

Core decision validation and authority evaluation live in
`internal/draft/approval`. The GitHub adapter owns host observation,
publication, and credential scoping. `cmd/pyxis` is parsing and composition;
`cmd/draft-lock-record` wires the policy reader. The bound port carries the
pre-merge policy and digest. Unbound historical ports cannot authorize an
Argus result. No executor port or local Git receipt is reused to pretend a
host-authenticated review exists.

## 5. Risks and failure handling

One account can run both authoring and driver sessions. GitHub authenticates
that account, not the model or independence of those sessions. Independent
review is enforced by the existing review workflow, with the driver accountable
for its cited evidence. Required separate native reviews can still prevent
merge. Missing host data, malformed policy, revoked grants, changed versions,
and exact-read mismatches refuse with no new lock. A failed publication may
have succeeded remotely; inspect its host record before a deliberate retry.

## 6. Verification

Unit and adapter conformance tests cover authenticated author and nonauthor
publication, exact-read identity, required digest, malformed schemas, stale
head and mainline, grant absence, revoked or malformed authority, wrong account,
operator changes request, latest-review revocation, and no write on preflight
refusal. Real Git fixtures prepare and merge a contract, record a driver lock
with a pre-merge policy port, then clone and render it; a candidate-policy
result creates no tag. Existing operator and local Git suites remain required.
The full race suite, vet, and pinned lint run on the final source candidate.

| ID | Acceptance criterion | Verify | Required evidence | Traces to |
|---|---|---|---|---|
| A-1 | Author and nonauthor approvals publish the appropriate host event, exact-read the same review ID/body, and report the driver identity | Test | adapter conformance tests | prd:feature-draft/FR-58 · prd:feature-draft/FR-59 · DINV-3 |
| A-2 | Wrong account, absent or revoked grant, candidate policy, stale head/policy/digest, malformed decision, and operator changes request cannot authorize a lock | Test + Failure injection | positive/refusal tests | prd:feature-draft/UB-15 · DINV-1 · DINV-2 |
| A-3 | Moves observed by final preflight write nothing; a head or mainline move between that observation and submission leaves at most one exact old-head decision, reports staleness, and refuses authorization for the changed head or policy; no automatic write retry or protection mutation exists | Test + Inspection | race interleavings and publication tests, reviewed adapter | prd:feature-draft/FR-59 · prd:feature-draft/FR-60 · DINV-4 |
| A-4 | A fresh clone reads the driver v2 lock and renders its exact compiled contract while existing human v1 and explicit local Git fixtures pass | Test | real Git fresh-clone fixture and existing suites | prd:feature-draft/FR-60 · prd:feature-draft/NR-8 · DINV-5 |

## 7. Open questions

None for this scope. Native GitHub review-count policy and a separate reviewer
account remain repository governance choices; this route does not change them.
