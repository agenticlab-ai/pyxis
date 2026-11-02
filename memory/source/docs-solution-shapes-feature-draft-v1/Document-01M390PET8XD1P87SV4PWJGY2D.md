# Feature draft — solution shape

| Field | Value |
|---|---|
| **Design** | feature-draft-v1 |
| **Status** | authoritative |

Implementation reference for the paired design, approved with it in its
introducing change. No implementation is asserted; retire this shape when its
code lands.

## S-1. The draft branch and its working files

One branch per draft; a reopen uses a fresh branch from the mainline. Working
files are ordinary markdown on that branch and never reach the mainline: lock
preparation compiles and removes them in one commit.

Witnesses **DINV-1**, **DINV-2**, **DINV-10**.

```text
config   paths.drafts: drafts                the working-file root (a closed .pyxis/config.yaml key)
config   draft.branch_prefix: draft/         the draft branch prefix

branch   draft/<feature>                     first version
branch   draft/<feature>-v<N+1>              reopen while v<N> binds

drafts/<feature>/prd.md                      PRD, in the PRD template in force (§0 included)
drafts/<feature>/tech-spec.md                tech spec: decisions, each naming the requirements it serves
drafts/<feature>/brainstorm/<NNN>.md         brainstorm records, one file per session
drafts/<feature>/reviews/<NNN>.md            review reports, each recording the draft digest it read
drafts/<feature>/problems.md                 requirement problems and their rulings (author, reason, problem)
drafts/<feature>/events.md                   the draft's event log (EVT-1 to EVT-4, EVT-6); the proposal for design Q4
```

The draft digest is the SHA-256 over the paths and bytes of the contract
files, `prd.md` and `tech-spec.md`; review reports, brainstorm records,
problems, and events are excluded, so a review can record the digest of what it
read. After lock preparation the digest is recomputed from the stored exact
texts and their recorded paths, giving the same value for the same content. A
round after preparation reverts the preparation commit, restoring the working
files byte for byte. A failed step never rewrites a working file.

## S-2. Commands

Readiness has one implementation in the memory tool; every other command
calls it rather than re-deriving a verdict.

Witnesses **DINV-3**, **DINV-4**, **DINV-10**; traces A-2, A-3, A-5.

```text
pyxis draft open <feature>            create the branch, scaffold working files, open the draft PR (EVT-1)
pyxis draft reopen <feature>          refuse while the current plan has unfinished tasks; else branch from the mainline (EVT-6)
memory ready <feature> [--json]       compile in process and print the readiness verdict; exit 1 when not ready (EVT-4)
pyxis draft prepare-lock <feature>    rebase, compile once, write records, remove working files, render the approval request as the PR description, request approval
pyxis draft abandon <feature> --reason "..."   reduce the draft PR to one Learning; delete the branch after merge
pyxis draft status [<feature>]        report contract state: drafting v1 · drafting vN+1 while vN binds · locked at vN · document pipeline
```

The draft PR's required check runs `memory ready <feature>` on the PR head.

## S-3. Compile mapping

The compiler reuses the reconciler's PRD-row mapping; each file's exact text is
also kept as a literal Document.

Witnesses **DINV-2**, **DINV-11**; traces A-4, A-10, A-17.

```text
working file            unit                        record (subject · family)
prd.md                  requirement row (FR, NR…)   the reconciler's claim kind · prd/<feature> · <row id>
prd.md                  whole file                  Document (literal body) · prd/<feature> · prd
prd.md                  Q row                       Question · prd/<feature> · Q<n>   (+ its Answer)
prd.md                  R row                       Decision (rationale, fit) · prd/<feature> · R<n>
tech-spec.md            decision D-<n>              Decision serves <named rows> · design/<feature> · D-<n>
tech-spec.md            whole file                  Document (literal body) · design/<feature> · design
brainstorm/<NNN>.md     whole file                  Document (literal body) · brainstorm/<feature> · <NNN>
reviews/<NNN>.md        whole file                  Document (literal body) · review/<feature> · <NNN>
problems.md             problem P-<n>               Question · prd/<feature> · P-<n>
problems.md             ruling on P-<n>             Answer (author, reason, triggering problem) + the changed claim
events.md               event                       Learning tagged event:<name> · feature/<feature> · —
```

Working-file grammar the compiler reads; witnesses **DINV-2**, **DINV-11**; traces A-3, A-10.

```text
tech-spec.md   ### D-<n> — <title>
               **Decision:** <the decision>
               **Serves:** FR-3 · UB-2              requirement IDs of this feature's PRD
prd.md         Q table columns: Q · Question · Owner · Needed by · Status · Resolution
               an empty Resolution is an open question (FR-12); its text becomes the Answer
problems.md    ### P-<n> — <requirement ID> — <gist>
               **Raised by:** <session>   **Problem:** <what is wrong>
               **Ruling:** <author> · <reason> · <the change>   (absent while open)
changes        from the draft branch's history of prd.md: every requirement row changed
               after it first appeared, paired with the ruling naming it, else unruled (UB-1)
```

Identity and issuance time are assigned once, at lock preparation; every other
field is a pure function of the working files and the mainline. Compiled
records are native: no `reconcile:source`, `reconcile:prd`, or
`source:prd-ledger` tag and no source-file provenance, so the reconciler never
pairs a stored exact text with a removed working file.

## S-4. The readiness verdict

Witnesses **DINV-3**; traces A-3, A-21, A-23.

```json
{
  "schema_version": "draft-readiness/v1",
  "feature": "<feature>",
  "draft_digest": "sha256:<hex>",
  "mainline": "<commit>",
  "ready": false,
  "unmet": [
    {"condition": "FR-13", "item": "prd/<feature>#FR-7", "remedy": "add a tech-spec decision serving FR-7"},
    {"condition": "FR-16", "item": "reviews/002.md", "remedy": "re-run the tech-spec review on the current draft"}
  ]
}
```

Conditions are the readiness rows in order: FR-12, FR-13, FR-14, FR-16, FR-38,
FR-39, FR-40, FR-41, FR-42, plus UB-7 for an unreadable part.

## S-5. The approval port

Witnesses **DINV-4**; traces A-5.

```go
// Proposed port on the consumer side (internal/draft/approval); adapters under
// internal/adapters/approval/<medium>/.
type Approval struct {
	Approver    string // the approving identity, verified by the adapter
	Head        string // the exact commit the approval covers
	CaptureLine string // "" when lineless
	PullRequest int
}

type Approvals interface {
	// Approved returns the operator's current approval of head, or ErrNoApproval
	// when none covers exactly this head.
	Approved(ctx context.Context, feature, head string) (Approval, error)
}
```

The code-host adapter reads the pull request's reviews; the local test adapter
reads fixtures.

## S-6. Branch protection on the mainline

Witnesses **DINV-4**; traces A-5.

```text
required pull request reviews: 1
require review from code owners: true        CODEOWNERS: memory/prd/** @mdarabi
                                                         memory/design/** @mdarabi
dismiss stale pull request approvals: true
require approval of the most recent push: true
enforce for administrators: true
required status checks: docs-gate, memory ready (draft PRs)
```

Agent sessions authenticate as their own account and never hold the
operator's credentials. A proposal that adds no contract records needs no code
owner, so the merge lane's machine identity can still approve and merge it
(merge-lane-v1).

## S-7. The lock record

Witnesses **DINV-9**; traces A-9, A-14.

```text
tag       lock/<feature>/v<N>        annotated, on the squash-merge commit
message   version: <N>
          approver: <login>          roles: product, engineering
          capture: <capture line>    or: lineless
          head: <approved commit>    pull-request: <number>
          readiness: sha256:<hex>
```

A mainline CI job on the push event of a lock merge (recognized by its
lock-preparation commit) reads the approval through the port and writes the
tag. The locked version renders with `memory render prd/<feature> --at lock/<feature>/v<N>`.

## S-8. The native-link rule in the reconciler

Witnesses **DINV-5**, **DINV-6**; traces A-6.

```go
// Proposed addition to internal/memory/reconcile. A requirement claim is native
// when its subject maps to no PRD ledger at the trusted base or the candidate.
func nativeClaim(prds []PRD, claim *record.Record) bool

// authorizedPRDServes gains one branch: a Decision's serves edge to a native
// claim is authorized when both records are active; every ledger-mapped claim
// keeps today's document-declared authorization.
```

The evaluator version increments; trusted baselines replay under their own
evaluator, so the rule applies only once it is at the trusted base.

## S-9. The document gate after the release

Witnesses **DINV-7**; traces A-7.

```text
check-contract-corpus-closed   (merge-base check, self-registered)
  refuse  added files under paths.prds (a new NNN entry), paths.tech_designs (*.md, not index.*),
          paths.brainstorms, paths.reviews
  accept  added files under paths.plans
  remedy  "new contract work uses a feature draft: pyxis draft open <feature>"
```

## S-10. The plan handoff

Witnesses **DINV-8**; traces A-8, A-18.

```text
plan-new <feature>       draft-born (no PRD ledger): precondition = memory ready on the locked version
                         document-pipeline: precondition = docgate contract-gate <feature>  (contract-gate-v2)
plangate fold            traces prd:<feature>/<row> resolve against the locked records for a draft-born feature
plangate project         unchanged; reads only a merged plan document
plan header              Basis: lock/<feature>/v<N>     the locked version a draft-born plan was generated from
reconcile sources        skip paths.plans/<feature>/ when <feature> has no PRD ledger (no plan capture)
```
