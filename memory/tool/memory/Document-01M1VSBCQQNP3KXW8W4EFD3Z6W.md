# memory — the repository's software memory

Typed, immutable records under `memory/<subject>/`, one JSON file each, shared
through git like any other file. Every document people read — the PRD of a
subject, a design, the why behind a claim, what is designed and what is not,
the review packet of a branch, the context pack of a task — is rendered from
the records on each run. Nothing is stored twice, so nothing can drift, and
no index outlives the run that built it.

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

Kinds: `Need`, `Promise`, `Expectation`, `Decision`, `Assumption`, `Policy`
(claims, with a family); `Learning`, `Plan`, `Question`, `Answer`,
`Revocation`.

## The flow

1. **Write records on a branch.** `memory add` refuses at write time what the
   merge check would refuse later: a claim in a family that already has a
   head it does not name (`conflict-at-admission`), or a `--supersedes`
   target that is no longer the head (`effective-set-moved`).
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
   that moved under the branch, two active claims in one family. Reopened
   designs, undesigned claims and concurrent work are warnings for the
   reviewer.
4. **Give agents their context.** `memory pack --subject prd/x --family FR-3`
   compiles the claims in scope, the decisions that serve them, the policies
   that bind, the questions already answered and still open, reopened
   designs, in-flight work, and learnings, under a token budget with every
   omission named.
5. **Ask once.** `memory ask` returns an existing answer instead of recording
   a duplicate question; `memory answer` records the answer.

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
           [--supersedes id|subject#family] [--serves …] [--about …] [--tag code:internal/executor] [--learned-from …]
memory add --kind Document --subject prd/x --family prd --statement "Stacked PRs" --body-file prd.md
memory revise prd/x#prd --body-file prd-v2.md --intent "…"    memory render prd/x [--slot prd] [--at origin/main]
memory revoke <id> --reason "…"
memory ask --subject prd/x --text "…"        memory answer <question-id> --text "…"
memory view prd/x [--since origin/main]      memory view design/x
memory why prd/x#FR-3                        memory delivery prd/x          memory status
memory search "…" [--subject prd/] [--tag t] [--code path] [-k N] [--all]
memory collide --statement "…" --subject prd/x --family FR-3 [--judge-cmd "claude -p"] [--json]
memory pack --subject prd/x [--family FR-3] [--budget 8000] [--json]
memory packet [--base origin/main] [--body] [--description "…"]
memory check [--base origin/main]
```

The actor written into a record is `--actor`, then `$MEMORY_ACTOR`, then git
`user.email`. `memory add --json` reads the fields from stdin for agents.

## The judge protocol

`--judge-cmd` runs a command (split on whitespace, no shell) with JSON on
stdin — `{"draft": …, "candidates": [...], "relations": [...],
"instructions": "…"}` — and reads
`{"verdicts": [{"id": …, "relation": "supersedes|narrows|refines|compatible|conflicts|unrelated|review", "why": …}]}`
from stdout; prose around the JSON object is ignored. Without a judge a
heuristic answers: the active head of the draft's family is what it
supersedes, an in-flight record in that family is concurrent work, the top
lexical hits get `review`.

## Layout of the code

`internal/memory/record` (the schema), `store` (files in, files out),
`gitsrc` (the only git adapter), `resolve` (status and relations), `policy`,
`search`, `packet` (proposal and merge check), `pack` (agent context),
`view`, `ask`, and `cli`. `cmd/memory` is a thin main. The end-to-end suite
in `internal/memory/cli` drives every verb against real git repositories,
including two clones sharing a bare origin.
