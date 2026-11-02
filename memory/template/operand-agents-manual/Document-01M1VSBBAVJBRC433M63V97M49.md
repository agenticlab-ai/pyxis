# AGENTS.md — operating manual for agents working in this repository

*Instantiated once by `pyxis init` from `operand-agents-manual/v1`, then
**repo-owned**: the humans here extend it freely, `pyxis converge` never
rewrites it, and `pyxis doctor` advises when the shipped template has moved
on. Vendor entry files (`CLAUDE.md`, per-CLI configs) are pointers to this
file — knowledge lives here, never in a vendor surface.*

## This repository is factory-operated

This repository is managed by **pyxis**, pinned to one verified system
release. The knobs captured at install time:

- **Tracker:** `{{tracker}}` — where work items live and where the factory
  files escalations and welcome items.
- **CI mode:** `{{ci_mode}}` — where the docs gate runs: `local` (hooks
  only), `actions` (hosted CI), or `both`.
- **Default branch:** `{{branch}}` — the mainline every gate and merge-base
  computation keys on.
- **Delegated authority (argus):** `{{argus_enabled}}` — while `false`, no
  delegated decision authority exists in this repository: the argus skill
  refuses with an enablement pointer and no authority contract file exists;
  flipping the knob is a human act through the pyxis config.

Every engine tool runs through the one front door — `pyxis exec <tool> --
<argv…>` — which verifies the binary against the pinned release manifest
before every execution. **Never invoke an engine binary directly, fetch one
ad hoc, or build one from this repository**: an installed repository carries
no engine source, and nothing unpinned executes here.

## Session logs — every session, every stage, no exceptions

**Before you do anything else in a session, open a session log.** This binds
every automated author, in every CLI (Claude, Codex, Cursor, OpenCode), at
every step of the pipeline — a skill run, a plan task, a gate fix, a one-off
chore. It is a prerequisite, not a closing chore: open the log at your first
real step, write to it the moment you have to put something to a human, and
leave it behind when you stop.

**The file.** One file per session, at
`logs/<YYYY-MM-DD>-<HHMM>-<short-slug>.md` — the date and time you created it
first (take them from `date '+%Y-%m-%d-%H%M'`, local time), then a few words
naming the job, e.g. `logs/2026-07-30-1742-checkout-retry-fix.md`. Commit it
by explicit path along with your work. Entries are append-only: once you have
written one, add to it, never rewrite it.

**What belongs in it — only what a human had to decide.** If you hit a problem
and worked it out yourself, it does **not** go in the log, however annoying it
was. Solving things is the job, not a log entry. The log records only the ones
you could not settle on your own: the problem stopped you, you had to put it to
a human, and a human made the call.

These are the kinds of problem that reach that bar — but only once you have
actually escalated one:

- a skill's own steps contradict each other, the repository, or a doc, and you
  cannot tell which side is right
- a documented path, command, flag, or artifact is not there, and there is no
  safe way to guess what was meant
- a gate, hook, or tool blocks you for a reason unrelated to your change, and
  the only way past it would be to override a guard
- two rules disagree, and following either one breaks the other
- an input you were told to consume is missing, so no later step can run
  honestly
- an unknown you genuinely cannot settle — any answer you pick might be the
  wrong one, and the cost of being wrong is real
- a limit, permission, or access wall you cannot get past

**What never belongs in it:**

- anything you settled yourself — however much time it cost you
- review findings — the PRD, design, implementation, and e2e reviewers each
  write their own reports; those are not session-log entries
- bugs in the feature you are building or the bug you were asked to fix —
  that is the work itself and it belongs to the work's own artifacts
- failures caused by your own half-finished change
- opinions, ideas, and things you would do differently

The test: **did a human have to make the call? If not, it does not go in.**

**How to write it.** Plain English, short sentences, no jargon, no strings of
IDs. Keep every reference — file paths, requirement or doc IDs, proposal and
issue numbers, commands, error text — out of the prose and down on the `Refs`
line, so a human can read the problem and the decision straight through without
decoding anything. Each entry carries the time you hit the problem and the time
the human decided, says plainly what you asked, and says plainly what they
answered.

Copy this skeleton:

```markdown
# Session log — <what this session was doing>

Started: 2026-07-30 17:42 · CLI: Claude Code · Stage or skill: /plan-new

## P-1 — <the problem in one plain line>

- **Found:** 2026-07-30 17:42
- **Problem:** What went wrong, in two or three plain sentences.
- **Blocked:** What this stopped me from doing, and why I could not settle it
  myself.
- **Asked:** 2026-07-30 17:50 — what I put to the human, in plain words.
- **Decided:** 2026-07-30 17:58 — what the human decided.  (or: Still waiting)
- **How:** What was done to carry that decision out.
- **Refs:** paths, IDs, proposal numbers, commands, error text.
```

Number entries `P-1`, `P-2`, … within the file. If a decision is still
outstanding when you stop, leave it as `Still waiting` and say so in your
hand-off — an open question recorded honestly is worth more than a tidy log.

**If you escalated nothing**, still leave the file, with the header and one
line: `Nothing escalated this session.` A run that needed no human is worth
recording too, and the habit of opening the file is what makes you notice when
one does.

The log is the record of the calls a human had to make for the factory. When
one turns out to be a lesson worth keeping forever, graduate it separately into
the incident registry through `pyxis exec docgate -- add-incident` — the
registry is for permanent lessons, the log is for what needed a human today.

## The document system — rules that gate your edits

1. **Link direction is concrete → abstract, never the reverse.** PRDs and
   technical designs cite the vision and principles that govern them;
   `docs/product-vision.md` and `docs/architecture-principles.md` link to
   no concrete document and cite no requirement IDs.
2. **Mutability classes.** The vision, the principles, and incident entries
   are **append-only** (the gate enforces it; governing-document changes
   also need the `Governing-Change: operator` commit trailer — a human
   authorization an agent adds only on explicit instruction). Per-feature
   PRD ledger entries and technical designs are **immutable once merged** —
   merge is the freezing event. Frozen designs evolve by supersession (a new
   document citing the old); frozen PRDs evolve only by typed amendments.
3. **PRDs are ledgers.** A feature's PRD lives at `docs/prds/<feature>/` —
   `000-base.md` plus dense-numbered amendment entries, folded into the
   generated projection. **Never hand-edit `effective.md` or
   `effective.source.json`**; regenerate with
   `pyxis exec docgate -- fold docs/prds/<feature>`.
4. **Generated artifacts are never hand-edited.** The design index under
   `docs/tech-design/` regenerates via `pyxis exec docgate -- gen-index`;
   if a projection is wrong, fix the source and regenerate.
5. **Registries grow through coded writers.** Incidents append via
   `pyxis exec docgate -- add-incident` — never hand-written; entries are
   permanent once merged.
6. **Governing documents are the humans' to author.** The seeded skeletons
   under `docs/` await their owner; no automated author writes governing
   content — `pyxis doctor` advises while they stay unauthored.
7. **No living rollups before code ships.** `docs/prd.md` and
   `docs/techspec.md` do not exist until the first capability ships; do not
   create them early.

## Gates — what stands between an edit and a merge

- **Desk:** the git hooks are active from install (no manual step). The
  pre-commit hook runs the tree-local checks; the pre-push hook runs the
  full gate against `origin/{{branch}}` — both through the exec grammar.
- **CI:** per the `{{ci_mode}}` knob, the hosted `docs-gate` workflow runs
  the identical umbrella verb; desk and CI can never drift because both run
  one implementation.
- Run it yourself before proposing:
  `pyxis exec docgate -- gate --merge-base origin/{{branch}}`.
- A red gate names its defects and remedies; fix the source, never the
  gate's output. `pyxis doctor` reports drift, environment health, and
  advisories read-only; `pyxis converge` repairs engine-owned drift.

## Language policy

This manual sets no language policy for the repository's own code — the
repository's owners append theirs below. The factory's tooling arrives as
pinned release binaries through `pyxis`; agents never build or vendor
factory tools from within this repository.

## Default working rules

- **Work on the current branch.** Never create or switch branches unless
  asked; a proposal rides a `<type>/<slug>` feature branch onto
  `{{branch}}`.
- **Stage by explicit path — never add-all.** Parallel sessions share
  clones; add-all sweeps another session's work into your commit.
- **One concern per proposal**, conventional-commit title, and a body that
  names what closes.
- **Done is an artifact, not a claim.** Report success only on a verifiable
  artifact — the proposal exists, the gate is green — never on prose.
- **Ambiguity → escalate.** If a rule here and an instruction conflict, or
  intent is unclear, ask the operator with options; never guess a silent
  resolution.

## Extending this manual

This file is repo-owned: append the repository's own conventions below —
language policy, build commands, review rules. Keep the factory sections
above intact; they mirror gates that are mechanically enforced, so editing
their prose changes nothing but the truth of the description.
