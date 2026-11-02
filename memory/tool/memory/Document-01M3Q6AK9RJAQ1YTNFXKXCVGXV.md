# memory — the repository's software memory

Typed, immutable records under `memory/<subject>/`, one JSON file each, shared
through git like any other file. Every document people read — the PRD of a
subject, a design, the why behind a claim, what is designed and what is not,
the review packet of a branch, the context pack of a task — is rendered from
the records on each run. During onboarding, an authored source is deliberately
captured as one exact literal Document until closeout; every offline pass
byte-compares the pair, so the temporary duplication cannot drift silently.
No generated index outlives the run that built it.

## What earns a semantic record

Agents creating or reviewing records should read and follow the
[shared memory-capture prompt](../../.pyxis/authoring/memory-capture.md). It
explains memory's role, the capture decision, evidence and ownership rules,
worked examples, and how to reuse the same instructions in other workflows.

Capture information that cannot be reliably reconstructed from code and adds
value to future work: obligations, intended constraints, reasons for choices
and rejected alternatives, attributed operator judgments, and reusable lessons.
Ask: **what future decision or action would be worse without this record?**
Keep it once in its owning contract or decision and refer to that identity.

The architecture as implemented, package descriptions, file existence, hashes,
test inventories, and ordinary change summaries are derived information.
Code additions, edits, and removals do not require new semantic records.
Formatting, lint fixes, fixture updates, and routine passing tests create no
Learning by themselves. A test failure that exposes a reusable cause or changes
future verification practice can justify a Learning; retain its evidence.
Existing decisions can continue to govern changed code. Required revision
verification remains in the code gate and explicit closeout receipts.

The current writer and merge check reject the recognized legacy generated
anchor and generic code-backfill patterns. Ordinary search, collision
suggestions, and task packs omit those records; `search --all`, `view`, and
`render` retain access to their immutable history. Recognition is deliberately
narrow: meaningful constraints and lessons are not excluded because they name
code or tests. Arbitrary prose still needs author/reviewer judgment; no keyword
filter can prove its future usefulness.

## Historical material

Session logs and transcripts are optional operational history, not authored
sources that must be represented in memory. Sessions with no durable decision
or lesson create no record. Existing substantive log imports remain available
through explicit history access (`search --all`, `view`, and `render`), but
ordinary search, collision suggestions, and context packs omit them. A
Decision or Learning that cites a log remains ordinary durable memory.

The one-time session-log cleanup admits deletion only of the exact empty
Document envelopes pinned in `internal/memory/sessionlog/retired.json`, with
their matching bodies. Both files must be removed together, their base bytes
must be unchanged, and no record may refer to them. All other records retain
the normal immutability and deletion checks. The current reconciler excludes
logs from source discovery and historical source obligations; comparison with
a trusted v1 baseline still reconstructs its original source inventory before
applying the narrower policy.

## The model

- A **record** is one claim, decision, learning, question, answer or
  revocation: `memory/<namespace>/<name>/<Kind>-<ULID>.json`. It is written
  once and never edited. Change is a new record that names what it
  `supersedes`; withdrawal is a `Revocation`. History is kept beside the
  present.
- A **subject** (`prd/stacked-prs`, `design/stacked-prs`, `code/executor`,
  `governance/principles`) is a directory. A **family** (`FR-1`, `DOD-3`,
  `D-1`) is the exclusivity key inside it: at most one active claim per
  subject and family.
- **Status** (`active`, `superseded`, `revoked`) is computed from the
  relations, never stored.
- Every claim carries its **intent** and **rationale**, what it was **learned
  from**, and the **alternatives rejected**, so later judgment can ask why.
- **Relations**: `supersedes` (history), `serves` (a decision serves a promise,
  an expectation or a need), `depends_on`, `about` (a code anchor),
  `refines`, `conflicts`, `answers`, `revokes`.
- **Consequence** (low / medium / high) is computed by `memory/policy.json`
  from what a change touches, never declared by genre.

Kinds: `Need`, `Promise`, `Expectation`, `Decision`, `Assumption`, `Policy`,
`Document` (claims, with a family); `Learning`, `Plan`, `Question`, `Answer`,
`Revocation`.

## The flow

1. **Write records on a branch.** `memory add` refuses at write time what the
   merge check would refuse later: a claim in a family that already has a
   head it does not name (`conflict-at-admission`), or a `--supersedes`
   target that is no longer the head (`effective-set-moved`). Every relation
   target must already exist and be older than the record that names it.
   Unrelated newer history does not invalidate a record's ID, but inserting
   records cannot activate a previously rejected or dangling historical act.
   Manually authored and CLI-authored graphs use the same causal rules.
   `memory collide` narrows the memory to the records a draft is about (BM25
   plus subject, family, tags, code and in-flight branches) and, with
   `--judge-cmd`, asks an agent CLI which of them the draft supersedes,
   narrows, refines, conflicts with, or leaves alone.
2. **Read the packet.** `memory packet --body` renders the pull request body
   from the branch's records: the promise delta the product side signs, the
   design delta the engineering side signs, what it supersedes, which designs
   it reopens, which new claims nothing serves, and what other branches have
   in flight on the same families. Approval is the pull request review.
3. **Check at merge.** `memory check` exits 1 on blockers: an edited or
   deleted record, an invalid file, a dangling relation, a superseded head
   that moved under the branch, two active claims in one family, competing
   Answers or Revocations, forward relations, a fork that replaces one base
   claim with multiple active candidates, or a newly supplied target that
   would silently activate a historical dangling act. Reopened designs,
   undesigned claims and concurrent work are warnings for the reviewer.
4. **Give agents their context.** `memory pack --subject prd/x --family FR-3`
   compiles the claims in scope, the decisions that serve them, the policies
   that bind, the questions already answered and still open, reopened
   designs, in-flight work, and learnings, under a token budget with every
   omission named.
5. **Ask once.** `memory ask` returns an existing answer instead of recording
   a duplicate question; `memory answer` records the answer.
6. **Reconcile offline.** `memory reconcile --check` deterministically compares
   one pinned `HEAD` against memory: exact authored-source bytes, folded PRD
   rows, the live design graph, and production code units. `--write` appends
   only lossless source Documents, exact typed PRD rows and Questions,
   explicit `**Decision:**` paragraphs, and the
   design index's exact `Serves` declarations. Design distillation waits for
   its exact source Document and exact referenced PRD claims; code paths named
   in a Decision become direct `code:<path>` tags. It never assigns nearby
   prose, undeclared coverage, or unmentioned code by inference.
   Capturing a Question never guesses or closes its Answer. A live
   `Q` row is closed only by a `Question` at `prd/<feature>#<row>` tagged
   `reconcile:prd-disposition` and `source-unit:<row-sha256>`, with the exact
   effective-PRD source and durable evidence provenance, plus exactly one
   active `Answer`. A live `R` row uses the same exact address, tags, and
   provenance on a `Decision` whose rationale and fit criterion record how the
   risk was dispositioned. Stale, duplicate, or loosely matched dispositions
   fail closed. Reconciliation reports unfinished design work separately in
   `completion` (`memory-contract-completion/v1`). An exact captured Must claim
   without authorized coverage is `undesigned`. An unchanged, captured design
   from the trusted predecessor can be `reopened` after an explicit requirement
   retirement or supersession with a valid native transition. A new Decision
   cannot claim historical coverage by backdating its ID or provenance.
   Missing source captures, unknown addresses, stale active claims, missing
   native Decisions, and missing edges for declared live coverage still block.
   Completion findings remain visible and prevent planning under the existing
   contract gate and feature closeout; they never become debt allowances.

   The original `requirements-first/v1` header remains immutable. Historical
   v3 evaluation preserves its original first-design boundary; current
   classification also supports later amendments and unmarked ledgers without
   retrofitting their bases. Trusted v1 through v6 baselines replay under their
   original classifications. Candidate enforcement requires v7; synchronize a
   checked-in policy that still names an older evaluator. Synchronization may
   remove old completion allowances but cannot add or widen integrity debt.
   During a legacy-debt burn-down, `--check --baseline
   .github/memory-reconciliation-baseline.json` requires the checked-in debt
   inventory to match the current finding addresses, counts, severities,
   and repairability exactly. A fix therefore tightens
   the baseline in the same change, preventing that debt from returning under
   a stale allowance. Legacy v1 additionally stores and checks all four input
   digests; `reconcile --baseline-generate` emits that exact v1 inventory.
   Partitioned v2 keeps a stable format/evaluator marker at the same path and
   one allowance per `.github/memory-reconciliation-baseline/<address-sha256>.json`.
   Input digests remain in the invocation's report. An ordinary change that
   preserves findings edits no policy file; disjoint debt reductions edit
   different members. `reconcile --baseline-sync --baseline-ref <trusted-commit>`
   writes only the required changes after validating the committed candidate
   and trusted base. It refuses added debt, arbitrary dirty policy, uncommitted
   nonpolicy work, missing policy, and CI base overrides. An exact retry after
   interruption resumes the same planned paths; a completed retry writes none.
   Pull requests and main pushes also pass `--baseline-ref <trusted-commit>`:
   the candidate inventory may tighten but never widen its strict-ancestor
   predecessor. If a squash merge carried an unchanged baseline alongside
   other work, `--baseline-replay-ref <original-snapshot>` can reproduce its
   inputs. Replay requires the identical trusted baseline file and all four
   matching input hashes; it cannot add allowances. Source retirement
   is checked from both that snapshot and the actual trusted base. The original snapshot must
   already be available locally; the checker never fetches it.
   Git reads also disable lazy fetching in partial clones. A tree entry whose
   blob is unavailable fails explicitly; it cannot disappear from the memory
   or policy inventory. Restore the retained inputs before rerunning the check.
   Legacy evaluator v1 and v2 snapshots are rebuilt only with their bounded historical
   classifications: v1 session-log sources and the evaluator v1/v2 pre-sourced-zsh code
   inventory. Evaluators v1 through v6 also retain mandatory code anchors and
   per-unit semantic coverage for exact historical replay only. Current code
   inventory is derived and creates no semantic-capture obligation.
   Storage v2 uses the current `memory-reconcile/v7` evaluator; trusted v2
   allowances must reproduce exactly at their own commit and configuration.
   Unknown evaluators refuse. Deploy the dual reader first, then run
   `reconcile --baseline-sync --baseline-ref <trusted-commit> --migrate-v2`
   for a separate data-only cutover. Migration preserves the full trusted
   allowance inventory and proves the legacy input binding, using bounded
   replay when necessary. It cannot combine unrelated source/code changes,
   bootstrap missing policy, or downgrade an adopted v2 lineage. The trusted
   ref may equal HEAD only for this explicit migration at the configured
   mainline. Commit the migration before using ordinary v2 synchronization.
   The baseline has one fixed path, so changing the flag cannot
   silently bootstrap a new lineage. An absent file at the trusted ref is the
   visible first bootstrap: the candidate must exactly match the current
   report and that bootstrap change may not delete any trusted-tree path.
   New, changed, and removed code units do not require invented semantic
   acknowledgments. A still-applicable decision may be reused. Explicit current
   associations on substantive records must remain valid, and stale closeout
   witnesses still fail verification until a replacement receipt binds the
   current bytes. Generated legacy anchors retain their historical digest;
   they are neither refreshed nor required for new code.
   Removing an unbackfilled source by calling the disappearance an
   improvement is rejected, whatever finding originally described the gap. A
   source-retirement exception must come from a closeout already present at the
   trusted ref and reproducible from its recorded source commit; a candidate
   cannot authorize its own deletion. CI also runs `memory check` against the
   same trusted ref so immutable records cannot be changed or removed outside
   the reconciliation inventory. The ordinary `--check` remains strict and
   still fails on every blocker.
   `--write` accepts a repeatable exact selector in the form
   `--action 'code|scope'` for bounded migrations (for example,
   `--action 'capture-document|docs/wiki/example.md'`). Every selector is validated
   against the same pinned report before the first append; an unknown action
   code, duplicate selector, or selector with no current action refuses the
   whole write. Omitting `--action` preserves the all-safe-actions behavior.
7. **Close out, then retire context.** Once a feature's mapped requirements are
   aligned and served by Decisions with explicit `code:<path>` associations,
   `memory closeout --feature x --write` seals their exact source, native-memory,
   design, and code identities. Run closeout only after the implementation and
   its memory have landed: start a closeout-only branch at the current configured
   trusted mainline commit. Both `--check` and `--write` refuse a topic-only
   `HEAD`, so the receipt's source commit remains reachable and replayable even
   when the closeout PR is squash-merged and its branch is deleted. Its bounded
   source set includes the PRD ledger,
   every indexed design that serves it, typed brainstorm and review artifacts,
   valid plan sources and their authored companions, and paired
   problem/delivery ledgers joined through exact typed metadata. Those bound
   source files may then be removed. A source owned
   by more than one feature remains required until every typed owner has a
   replay-valid closeout sealing identical bytes. `memory verify --offline`
   accepts that retirement while continuing
   to fail on code drift, deletion, newly covered-but-unsealed code, stale
   native records, broken associations, or malformed receipts.
   A changed code witness requires an explicit replacement receipt; it can
   bind the same still-applicable decisions. Changes to the governing PRD or
   design still require their own explicit semantic evolution.
8. **Retire a standalone mutable source.** `memory retire-source` accepts
   regular Markdown sources under `.pyxis/skills/<slug>/`, direct Markdown
   files under `.pyxis/authoring/` or `.pyxis/templates/`, direct Markdown
   entries under `docs/wiki/`, and `.pyxis/vendor/<slug>/README.md` files when
   they have exact active literal captures and no feature owner. Governing and
   historical document roots remain ineligible. Start at the configured mainline, keep sources and
   memory clean, and issue all related seals together with repeated `--source`
   flags and an explicit `--reason`. The writer preflights the whole batch,
   preserves both files, and recognizes an exact complete pending retry.
   Land the receipt-only PR before deleting sources in a later PR. The gate
   and offline verifier bind the original introduction, source commit, bytes,
   mode, and capture; changed or reintroduced files need new evidence.
   A candidate cannot authorize its own deletion. Frozen documents and
   feature-owned context continue to use their existing rules.

## Documents: the PRD and the design people read

Rows and decisions are records, but a PRD or a design is also narrative:
approach, scenarios, flows, diagrams. That narrative is a record too, a
`Document` on the subject it documents, with a family naming its slot (`prd`,
`design`), so one document is active per slot. Its body is markdown stored
beside the envelope as `<id>.md`, hashed into the envelope so it is as
immutable as any record. Bodies are stored and hashed with canonical LF line
endings, and `.gitattributes` preserves those bytes across Git clients. The
body embeds claims by reference, `{{FR-3}}` or `{{prd/x#FR-3}}`, and never
restates them: a claim's text has exactly one home. `memory add` refuses a
marker that names no active claim.

For a lossless import of a legacy document or template, pass
`--literal-body`. The body is stored and hashed byte-for-byte, but brace-shaped
source text such as `{{...}}` or `${{...}}` is not interpreted as a claim
embed. A literal document's sections are included as narrative in packs for
its own subject (subject to the pack budget); they do not enter other subjects'
packs through brace-shaped text. Revisions preserve the document's literal
mode.

`memory render prd/x` produces the human document: the head Document's
narrative with every marker expanded to the claim's current head (statement,
intent, rationale, fit criterion, what serves it), then the claims the
narrative does not cover, the questions, the learnings, and the document's
history. A superseded claim updates the rendered document on its own; a
document edit is `memory revise prd/x#prd --body-file next.md`, which writes
version N+1 superseding version N. The version number is the position in the
supersedes chain, never stored. `memory render prd/x --at <ref>` renders the
document as the records stood at that commit, so "the PRD as signed" is the
render at the signing PR's merge commit. Rendered documents are produced on
demand and not committed; a committed copy would be the projection pyxis had
to byte-verify. If any Document file for the requested subject and slot is
invalid, rendering fails closed instead of presenting an older revision as
current.

In a PR, the packet shows the narrative delta, a line diff against the
version it replaces, beside the promise and design deltas. The pack loads the
document sections that embed the claims in scope. Search indexes bodies, so
`collide` sees narrative too.

## Verbs

```
memory init
memory add --kind Promise --subject prd/x --family FR-3 --statement "…" --intent "…" --rationale "…" \
           [--supersedes id|subject#family] [--serves …] [--about …] [--tag code:internal/executor] [--learned-from …] \
           [--source path] [--pr N] [--commit sha] [--note "…"]
memory add --kind Document --subject prd/x --family prd --statement "Stacked PRs" --body-file prd.md [--literal-body]
memory revise prd/x#prd --body-file prd-v2.md --intent "…"    memory render prd/x [--slot prd] [--at origin/main]
memory revoke <id> --reason "…"
memory ask --subject prd/x --text "…"        memory answer <question-id> --text "…"
memory view prd/x [--since origin/main]      memory view design/x
memory why prd/x#FR-3                        memory delivery prd/x          memory status
memory search "…" [--subject prd/] [--tag t] [--code path] [-k N] [--all]
memory collide --statement "…" --subject prd/x --family FR-3 [--judge-cmd "claude -p"] [--json]
memory pack --subject prd/x [--family FR-3] [--budget 8000] [--json]
memory-gate [--base trusted-ref]
memory reconcile --check [--baseline .github/memory-reconciliation-baseline.json [--baseline-ref trusted-commit [--baseline-replay-ref original-snapshot]]] [--json]
memory reconcile --write [--action 'code|scope']… [--actor who] [--json]
memory reconcile --baseline-generate > .github/memory-reconciliation-baseline.json
memory reconcile --baseline-sync --baseline-ref trusted-commit [--migrate-v2] [--baseline-replay-ref original-snapshot] [--json]
memory closeout --feature x --check [--json]  memory closeout --feature x --write [--actor who] [--json]
memory retire-source --source .pyxis/skills/x/SKILL.md [--source .pyxis/authoring/x.md] [--source .pyxis/templates/x.md] [--source docs/wiki/x.md] [--source .pyxis/vendor/x/README.md] --reason "..." --check|--write [--actor who] [--json]
memory verify --offline [--json]
memory packet [--base origin/main] [--body] [--description "…"]
memory check [--base origin/main]
```

The actor written into a record is `--actor`, then `$MEMORY_ACTOR`, then git
`user.email`. `memory add --json` reads the fields from stdin for agents.

`memory-gate` is the shared local/hosted integrity check. It requires a clean,
committed tree and runs both `check` and the baseline ratchet against one pinned
base. Locally it uses the configured mainline (fetch it first); `--base` selects
an explicit trusted ref. GitHub supplies `PYXIS_CI_EVENT` and `PYXIS_CI_BASE`
from the event, and a missing or overridden hosted base is refused. Scheduled
runs and an implicit local run at mainline HEAD check the exact inventory;
they do not claim to validate a proposed change against itself.

Independent records keep their original IDs and bytes after a rebase, even
when another branch has added newer records. Issuance time is still ordered
along explicit relations: a successor or answer must follow its prerequisite.
Every writer and the merge check also compare historical meaning. Adding a
target or replacing a prerequisite cannot silently activate an old dangling
or rejected act; retire that invalid carrier and issue a fresh act instead.
Documents enter the admission trial with their bodies and verified hashes.
Same-family claims, the same debt address, and changes within one Go package
still require coordination; this change removes unrelated global contention.

## The judge protocol

`--judge-cmd` runs a command (split on whitespace, no shell) with JSON on
stdin — `{"draft": …, "candidates": [...], "relations": [...],
"instructions": "…"}` — and reads
`{"verdicts": [{"id": …, "relation": "supersedes|narrows|refines|compatible|conflicts|unrelated|review", "why": …}]}`
from stdout; prose around the JSON object is ignored. Without a judge a
heuristic answers: the active head of the draft's family is what it
supersedes, an in-flight record in that family is concurrent work, the top
lexical hits get `review`.

## Durable evidence with squash merges

Repository evidence is captured and checked by deterministic Go code. It
uses local Git objects and SHA-256; it makes no model or network calls.
Upgrade the memory reader and writer together before creating the new
`memory-evidence/v1` bindings. Older closed readers refuse the new field.

`memory add` and `memory reconcile --write` capture every declared source
file, literal repository path in `learned_from`, and production unit selected
by `code:<path>` tags or typed `about` relations. A unit includes every
production member; explicitly cited test files are additional evidence.
Paths, regular Git modes, file hashes, inventory version, unit membership,
and unit hashes are checked exactly. Test source identifies a test; it does
not claim that the test ran.

New source evidence normally uses **record-introduction**: the record omits
`provenance.commit` and carries a content manifest. Before merge, it must
match committed `HEAD` with the trusted base as an ancestor. After squash,
verification finds the record's first mainline introduction and reads that
snapshot. Later source edits, renames, and deletion do not change its meaning.
An explicitly selected retained historical snapshot uses **mainline-commit**;
its full commit ID must be reachable from the trusted base. A commit-only
reference proves only that retained commit's identity.

```text
memory evidence --check --base origin/main --json

# Preview a repair of explicitly selected, unlanded records.
memory evidence --base origin/main --source HEAD --record Learning-<ULID> --record Need-<ULID> --json

# Apply the same exact repair after examining its preview.
memory evidence --base origin/main --source HEAD --record Learning-<ULID> --record Need-<ULID> --write
```

Repair reads the original evidence and requires exact equality with the
selected source. Missing or changed original bytes refuse repair. It preserves
IDs, timestamps, prose, and other non-evidence fields; it refuses records
already on the trusted base. Repeating a completed repair writes nothing.
An interrupted batch reports its applied and pending IDs and can be retried.
The writer uses kernel-held exclusion with no age-based takeover. An old
directory lock must be removed only after its previous writer has stopped.

Capture and the shared gate require committed source bytes. Working files
must preserve raw Git bytes and modes: these checks do not run checkout or
clean filters, hooks, external diff drivers, or fsmonitor. Shallow history,
missing objects, changed pins, and ambiguous record introductions fail closed.
The verifier never fetches missing history itself.

The same validator runs through `memory check`, reconciliation (including
baseline generation and policy synchronization), `memory verify --offline`, and `memory-gate` before
the debt ratchet. Every new identity is checked, including immediately
superseded or revoked records. Unchanged inherited records without reproducible
evidence are reported as **legacy-unverified**; no new baseline allowance can
excuse invalid evidence. Changed code can reuse its governing knowledge;
a new semantic record is warranted only when that knowledge changes.

The shared gate and draft repair retain blocking reports as immutable files
under `.pyxis/evidence/memory/<report-sha256>.json`. Commit these failure events
with the proposal. Direct checks remain read-only unless `--retain` is passed;
clean passes create no event. No evidence command commits, pushes, or changes
merge settings. The existing hosted memory check must be required by the
repository's merge protections before deployment is considered active.

## Layout of the code

`internal/memory/record` (the schema), `store` (files in, files out),
`gitsrc` (the only git adapter), `resolve` (status and relations), `policy`,
`association` (explicit memory-to-code joins), `codeinventory` (pinned source
units), `reconcile` (offline comparison and closeout receipts), `search`,
`packet` (proposal and merge check), `pack` (agent context), `view`, `ask`, and
`cli`. `cmd/memory` is a thin main. The end-to-end suite
in `internal/memory/cli` drives every verb against real git repositories,
including two clones sharing a bare origin.
