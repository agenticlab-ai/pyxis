# logs/ — session logs

One file per agent session, named for when it started:
`<YYYY-MM-DD>-<HHMM>-<short-slug>.md` — for example
`2026-07-30-1742-e2e-budget-fix.md`.

Each file records **only the problems that session could not settle on its
own** — the ones serious enough that it had to stop, put the question to a
human, and wait for a decision. A contradictory skill step, a documented path
that isn't there, two rules where following either one breaks the other, an
unknown too costly to guess at, a wall it could not get past. Every entry says
when the problem was hit, what was asked, when the human decided, and what
they decided — in plain English, with all the paths, IDs and commands kept on
a separate `Refs` line so the story reads straight through.

**Not in here:** anything the agent worked out for itself, however long it
took — solving things is the job, not a log entry. Nor findings from
`/prd-review`, `/tech-design-review`, `/impl-review`, or `/e2e-review` (they
write their own reports), nor bugs in the feature being built or the bug being
fixed (that is the work itself).

The test is simple: **did a human have to make the call?** If not, it does not
go in.

A session that escalated nothing still leaves a file, with its header and one
line: `Nothing escalated this session.`

The binding rule and the entry skeleton live in `AGENTS.md` → "Session logs —
every session, every stage, no exceptions". Writing one is mandatory for every
session, in every CLI, at every stage of the pipeline.

A decision worth keeping forever graduates separately into
`docs/observability/incidents.md` via `/incident`.
