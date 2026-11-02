# prototype — tech spec

The prototype step has four parts. A draft verb owns the prototype's worktree
and branch (D-1), resolves the harness and model that build the prototype
(D-2), and launches the build as a separate headless process (D-5). A guide,
exposed as a skill, tells the asking session how to run the step and write its
findings (D-2, D-3). The existing authoring and review guides gain the lines
that answer and check those findings (D-4). Nothing in the draft compiler,
draft readiness, the implement-now worker cycle, or the memory store changes.
The direction and the shapes it displaced are in the feature's design
brainstorm record, locked with version 1 under `memory/brainstorm/prototype/`.

### D-1 — One draft verb creates and removes the prototype's worktree

**Decision:** `pyxis draft prototype <feature> --idea <file>` creates a local branch `prototype/<feature>` and a worktree for it from the fetched mainline, and `pyxis draft prototype <feature> --remove` deletes both; neither form publishes anything, and the prototype stays on the local machine until the feature's draft locks or is abandoned (design-brainstorm:prototype#D-3).

**Serves:** FR-2 · UB-1 · NR-1

#### Concrete layout

The verb joins the existing `pyxis draft` verb set and its logic sits beside
the other draft lifecycle operations. `--idea` names a plain-text file holding
the idea and what the prototype should exercise and leave out; D-5 puts it in
the build's brief. The create form also takes `--ambient-backend` and
`--ambient-model`, which D-2 uses.

Create, in order, refusing before any change when a check fails:

1. The feature slug passes the rule `draft open` applies, the idea file
   exists and is not empty, and the repository has the ordinary layout
   described below. Another layout refuses and names it.
2. The feature's local draft branch exists, named by the configured draft
   branch prefix. A missing branch refuses with "open the draft first". A
   prototype therefore exists only for a feature with an open draft.
3. The build route resolves (D-2). A route that cannot be resolved or cannot
   run refuses here, before anything is created.
4. The configured mainline remote and branch are fetched, as `draft open`
   fetches them.
5. Neither the branch `prototype/<feature>` nor the target directory exists.
   An existing one refuses, names it, and prints the remove command.
6. `git worktree add --no-track -b prototype/<feature> <directory> <mainline ref>`
   runs. `--no-track` keeps the branch from recording mainline as its upstream.
7. The verb confirms the directory is a registered worktree on that branch at
   the fetched mainline commit, prints the worktree path, the branch, the
   base commit, and the route, one `key: value` line each, and hands over to
   D-5.

The directory is `<parent of the main checkout>/.pyxis-worktrees/<main checkout name>/prototype-<feature>`,
the place the interactive worktree launcher already uses. The main checkout is
resolved from the Git common directory, so the path is the same from the main
checkout and from any linked worktree. The verb supports only the ordinary
layout, in which the Git common directory is the `.git` directory inside the
main checkout. In any other layout, such as a repository created with a
separate Git directory, the main checkout cannot be resolved this way, and
both forms refuse and name the layout before changing anything: the create
form at step 1, before the fetch, and the remove form before it changes
anything (operator ruling, problem:prototype#A-6).

The branch prefix `prototype/` is fixed. It names a local, throwaway branch of
Pyxis's own and nothing the operand repository owns, so it has no config key.

Failure (UB-1): if step 6 or step 7 fails, the verb removes whatever it
created (the worktree registration, the directory, the branch), exits
non-zero, and prints the cause. It retries nothing. No step reads or writes
the draft branch or the invoking checkout's working tree, so both are
unchanged on every path. The verb relies on Git's own commands, locks and
refusals, and guarantees nothing when another process changes the
prototype's branch or directory while it runs (operator ruling,
problem:prototype#A-6).

Remove: the verb deletes the worktree, the runtime state directory D-5 keeps
beside it, and then the branch, only when the directory is a registered
worktree of this repository checked out on exactly `prototype/<feature>`. It
uses Git's own refusal for a worktree with uncommitted changes and names the
files. With no worktree and no branch present it reports that there is nothing
to remove and exits zero.

NR-1: the verb never pushes and never opens a pull request. A prototype
change can reach mainline only if someone publishes the branch by hand and the
operator merges it. The draft pull request cannot carry prototype files under
Argus attribution, whose scope admits no application code path.

Verification: Go tests against temporary repositories cover creation (branch
name, base commit equal to fetched mainline, no upstream, draft branch and
invoking checkout unchanged: DOD-7), each refusal, a repository with a
separate Git directory refused by both forms before any fetch or other Git
change, a forced failure of
worktree creation with cleanup and a reported cause (DOD-6), removal of a
prototype whose changes are committed (the worktree and the branch are both
gone afterwards), the dirty-worktree refusal, and the nothing-to-remove case.

#### Observability

The verb's output is its record: the `key: value` lines on success, the named
cause on failure. Creating and removing the worktree adds no event.

#### Security and privacy

Creating and removing the worktree performs one fetch and local Git
operations with the invoking user's access. It reads no credential and sends
nothing off the machine. The launched build is covered by D-5.

### D-2 — The verb resolves the build's harness and model: the configured ones, else the asking session's

**Decision:** The verb always launches the build as a separate headless process (D-5) and resolves its harness and model profile in this order: the optional `prototype` block of the repository's config when present; otherwise the harness and model of the session that asked, which the prototype skill passes as `--ambient-backend` and `--ambient-model` (design-brainstorm:prototype#D-4).

**Serves:** FR-1 · FR-4 · FR-6

**Builds-on:** D-1

#### Concrete layout

Config. One new block in the closed config schema, legal in the tracked
config and in the machine-local overlay:

```yaml
prototype:
  harness: codex
  profile: codex-implementation-economy
```

`harness` is one of the harness names the engine supports. `profile` names an
entry in that harness's implementation catalog under `harness-config`, which
fixes the model and its effort in the one place model identifiers already
live. The catalog is the effective one: the tracked config merged with the
machine-local overlay, loaded once per invocation by the loader the
implementation engine uses, and that one snapshot serves both route
resolution here and the launch in D-5. The block holds both keys or is absent; one key without the other, an
unknown harness, and an unknown key are config defects.

Ambient. With the block absent, the harness is `--ambient-backend`, the
convention skills already use to carry the invoking harness, and the profile
is the entry in that harness's implementation catalog whose model equals
`--ambient-model`. Harness launches take a model and an effort that the
catalog knows, so the session's model has to resolve to a catalog entry; when
several entries share the model, the one with the lowest cost rank is used.

Refusals, each at D-1's step 3 with the cause, before anything is created:

- the block absent and either ambient flag missing, or no catalog entry with
  the ambient model: the message names the `prototype` config block as the
  way to set a route;
- a configured profile the harness's catalog does not hold;
- a harness the engine cannot construct a runtime for, or one that fails its
  authentication check.

A configured route that cannot run is never replaced by the ambient route.

The route is reported as `configured <harness> <profile>` or
`ambient <harness> <profile>`.

The prototype guide and skill. Two files in the agent surface, following the
pattern of the existing skills: the guide under the authoring directory and a
skill pointer under the skills directory that holds invocation glue only.
Vendor skill directories already point at the skills directory, so no vendor
file changes. The guide's steps:

1. Confirm the feature has an open draft and read the idea: the problem
   ledger when Argus drives, otherwise the provider's words. Write the idea,
   and what the prototype should exercise and leave out, to a scratch file.
2. Run `pyxis draft prototype <feature> --idea <that file> --ambient-backend <this harness> --ambient-model <this session's model>`
   and wait for it. The session does not build the prototype itself.
3. Read the verb's report and the findings file it names, and check the
   findings against the worktree.
4. A build that could not reach a working state is a result: the findings
   say what stopped it, and it becomes a finding in the record (FR-4).
5. Write the findings record (D-3) on the draft branch, naming the route, the
   prototype branch, and its head commit.
6. Leave the worktree in place and report. The prototype is released when
   the draft ends (below).

The guide's Never list: push the prototype branch; open a pull request from
it; copy prototype files into the draft or into an implementation branch;
settle a product question from the prototype; build the prototype in the
asking session; continue after a failure the guide does not name. On a failed
run the session stops, records the cause in its report, and writes no findings
for that run (UB-1; D-1 and D-5 cover the verb's own failures).

Release: the session that takes the draft to its end on this machine removes
the prototype with `pyxis draft prototype <feature> --remove`. It does so once
it has verified that the lock landed on mainline, and right after it runs
`pyxis draft abandon`. When Argus drives, that session is the Argus drive and
the removal is journaled. When the draft is driven by hand, it is the
operator's session that confirms the lock or abandons the draft. D-1 refuses a
worktree with uncommitted changes; the session commits or discards them and
runs the removal again.

Asking (FR-1): a prototype runs when the provider says so with the idea or
invokes the skill on an open draft. The Argus drive guide gains one rule:
where the idea or an authority entry asks for a prototype, run the prototype
guide after the draft opens and before stage-1 authoring, and journal the
route, the prototype branch, and its head commit; never start a prototype
nobody asked for.

Verification: Go tests cover route resolution with the block present and
valid, with the block absent and the ambient flags given, the lowest-cost
choice among entries sharing a model, a profile that exists only in the
machine-local overlay, and each defect and refusal above (DOD-8). The guide is verified by demonstration on the first feature drafted
with a prototype request (DOD-1), and by one run on an idea chosen because it
cannot be built in the repository (DOD-3). Both ways a draft ends finish in
the same removal call, which D-1's tests cover for a clean, committed
prototype. The release passage itself is checked by inspection: once the
first prototyped feature's lock has landed, and at the first abandonment of a
prototyped draft, no prototype worktree or branch remains for that feature.

#### Observability

The verb prints the route it resolved. The findings record names the route
and is the run's durable output. The session's report names the worktree, the
branch, and the head commit.

#### Security and privacy

Route resolution reads config and the harness catalog and checks the chosen
harness's login state. It reads no secret and sends nothing off the machine.
The launched build is covered by D-5.

### D-3 — Findings live in a numbered brainstorm working file

**Decision:** A prototype's findings are written from a new prototype-record template into the next unused numbered file among the draft's brainstorm working files, each finding carrying an `L-n` anchor, its kind, what was tried, what was observed, and its answer (design-brainstorm:prototype#D-2).

**Serves:** FR-3 · FR-4

#### Concrete layout

One new template beside the existing document templates, with template
version `prototype-record/v1`. An instantiated record holds:

- A header table: In one line; Relevant-when; Idea (the problem ledger or the
  provider's words); Prototype (route, branch, base commit, head commit);
  Date; Outcome (`ran` or `did not run`); Template version.
- **What was built** and **What was left out**: a few lines each.
- **Findings**: one block per finding.

  ```
  ### L-1 — <gist>

  - **Kind**: technical challenge | product conflict
  - **Tried**: <what the prototype did>
  - **Observed**: <what happened, with the file, command, or output that shows it>
  - **Answer**: open
  ```

  A run that learned nothing states `No findings: <why>` in place of the
  blocks. A run whose Outcome is `did not run` carries at least one finding
  naming what stopped the build.

`L-n` anchors are dense from 1 across all of a feature's prototype records and
are never reused. Other draft files cite a finding in the existing brainstorm
grammar, as `brainstorm:<feature>#L-n` with the feature's slug and the
finding's number. The draft compiler captures the file at lock as a brainstorm
document, and draft readiness resolves the citation, with no change to either.

The asking session writes the record, because the file's number and the
`L-n` numbering depend on the draft's state. It transfers the findings the
build wrote (D-5) into the record after checking them against the worktree.

The Answer line starts as `open` and is rewritten while the draft is open, by
D-4. The record is not part of the contract digest, so filling an answer does
not stale a review.

Verification: by inspection of the first prototyped feature's record (DOD-1),
against the template's own structure checklist.

#### Observability

none — the record is the observation.

#### Security and privacy

The record is a draft working file and is published with the draft. The
template tells the author to quote only what the finding needs and never a
credential or a token seen during the build.

### D-4 — The authoring guides answer findings and the review guides check them

**Decision:** The stage guides that author and review a draft's contract read the draft's prototype records, answer each finding by citing the PRD row, tech-spec decision, or ruled requirement problem that handles it or by setting it aside with a reason, and treat a finding still marked `open` as a blocking review finding.

**Serves:** FR-5 · NR-2

**Builds-on:** D-3

#### Concrete layout

Each guide gains a short passage in its draft-born section. A draft with no
prototype record meets none of them.

- Feature brainstorm: product-conflict findings are put to the principal as
  questions, and what the prototype observed enters the record as facts citing
  the finding.
- PRD authoring: each product-conflict finding is answered by a requirement
  row, a non-goal, a waiting-room entry, an answered open question, or a ruled
  problem, and the finding's Answer line is rewritten to cite it.
- Design brainstorm and tech-spec authoring: each technical-challenge finding
  is answered by a decision whose text states the challenge and how its layout
  handles it, and the Answer line is rewritten to cite that decision. Because
  tasks are planned from decisions, implementation receives the learning
  through the contract it already receives.
- Any authoring stage may write `set aside — <reason>` on a finding that the
  contract will not handle.
- PRD review: a product-conflict finding whose Answer is `open` at the
  reviewed head is a blocking finding. Tech-spec review: the same for a
  technical-challenge finding.
- The Argus drive guide: before preparing the lock, confirm no finding is
  `open`; after verifying the lock on mainline, and after an abandonment,
  remove the prototype as D-2's release passage says and journal the removal.
- The pipeline map lists the prototype skill among the cross-cutting skills.

Draft readiness is unchanged, so a draft with no prototype record sees no new
condition (NR-2). An unanswered finding is caught by the two reviews; a
mechanical check waits as WR-2.

Verification: by inspection of the first prototyped feature at its locked head
(DOD-2), and by the existing draft, readiness, and review suites passing
unchanged (DOD-5).

#### Observability

Each review report names the findings it checked and their answers.

#### Security and privacy

none — guide text only.

### D-5 — The build is one launched harness process, bounded and committed by the verb

**Decision:** The verb launches one process of the harness D-2 resolved, on the resolved profile's model and effort, in the prototype's worktree, through the executor's existing harness runtime under a new prototype launch purpose; it waits for the process within the engine's default worker time bound, commits what the process left on the prototype branch, and reports the outcome (design-brainstorm:prototype#D-4).

**Serves:** FR-6 · UB-1

**Builds-on:** D-1 · D-2

#### Concrete layout

Launch. The verb constructs the resolved harness's runtime the way the
implementation engine's composition root does: through the adapter's own
constructor, given the effective catalog snapshot D-2 resolved the route from,
so the adapter validates and launches against the same profiles. The
constructor function that lives in that composition root today moves to a
shared internal package, unchanged in behavior, and both roots call it. The
verb then launches with the prototype worktree as the working directory,
`prototype/<feature>` as the head branch, the model and effort of the resolved
profile, and the runtime state directory described next. The implement-now
worker cycle is not entered: there is no tracker item, claim, code gate,
review, proposal, or task-session record.

Runtime state. A harness launch runs with its home and login directories
redirected into an isolated state directory that the host must prepare. Before
the launch the verb prepares that state for the prototype worktree and the
resolved harness with the runtime's existing task-state preparation, which
creates the directory beside the worktree and snapshots the selected harness's
login into it, and passes it as the launch's runtime state directory. The
verb owns this directory: D-1's remove deletes it with the worktree.

Purpose and brief. The harness port gains one launch purpose, `prototype`,
beside `implementation` and `review`. The shared brief assembler gives it its
own short protocol in place of the delivery protocol, because that protocol
promises a CI gate and a published proposal. The assembled brief is, in
order: the contents of the idea file, marked as data; the build rules (the
smallest version that exercises the idea end to end, no tests beyond what
proves the path, no polishing, stop at the first working path or when the
obstacle that prevents one is understood); and the protocol:

- The process runs non-interactively in an isolated worktree on the named
  branch, and nothing it produces is published.
- It leaves its changes in the working tree. The verb commits them after it
  exits. It performs no Git mutation, as an implementation launch does not.
- It writes its findings, one block per finding with kind, tried, and
  observed, to `.pyxis/scratch/prototype-findings.md` in the worktree, and
  says there when the build could not reach a working state and why.

Every harness adapter assembles its brief through the shared assembler and
branches only on the review purpose, so a prototype launch uses each
adapter's implementation invocation. The shared runtime's other two places
that branch on purpose admit the new one as well: launch access gives it the
confinement an implementation launch has, and it receives no memory pack. No
adapter gains prototype-specific code.

Bound. The verb awaits the process under the engine's default per-worker
wall-clock bound. On expiry it kills the process and treats the run as failed
with the cause `timed out`.

Commit. The verb commits only when the runtime acknowledges that the
process tree has stopped: a normal exit, an abnormal exit, or a kill the
runtime confirms. It then stages and commits everything in the worktree on
the prototype branch (the scratch findings file is ignored by Git and stays
on disk), and prints, after D-1's lines: the model and effort; the outcome,
`finished` or `failed: <cause>`; the head commit; the findings file's path,
or `findings: missing`; and the usage the adapter reported.

Failure (UB-1). A launch error, a process that exits abnormally, and a
time-out each end the run with a non-zero exit and the cause. The verb
retries nothing and does not switch to another route. Where the runtime
acknowledged the stop, the worktree, the committed partial work, and the
process log stay in place for inspection. Where the runtime reports that it
could not prove the process tree stopped, from a launch or from an await,
the verb stages nothing, commits nothing, and removes nothing: it leaves the
worktree, the branch, and the runtime state exactly as they are and reports
that cause. In every failed run the asking session writes no findings.

Verification (DOD-8). Go tests with a stub harness runtime cover a launch on
the resolved model and effort in the prototype worktree, for a configured
route and for an ambient one; the assembled brief for the prototype purpose;
the commit after exit; the reported lines; a time-out with an acknowledged
kill; a launch error; and a runtime that reports it could not prove the
process tree stopped, after which no file is staged, no commit exists, and
nothing has been removed. Two cases go past the stub. One prepares runtime
state for a fixture harness with a file-based login and no API token in the
environment, and runs the shared launch-access and launch-environment
functions against it, showing the state directory and the login snapshot
reach the launch. The other puts a profile only in the machine-local overlay
and shows that route resolution and the real adapter's catalog check agree on
its model and effort. The existing harness conformance suites pass unchanged.

#### Observability

The process's standard output and error are kept under the ignored local
logs directory, grouped with the engine's other agent logs. The verb's report
carries the route, model, effort, outcome, head commit, and usage.

#### Security and privacy

The launched process runs with a snapshot of the resolved harness's own
login inside its isolated runtime state, and with the confinement the runtime
already applies to implementation launches, in the prototype's worktree. The idea and the repository code the process reads go
to that harness's provider, as they do for implementation runs; the operator
chooses the provider by configuring a route, and otherwise it is the provider
of the session already running. No credential is passed in the brief, and the
brief treats the idea file as data.

## Principles

- **AP-1** honored: the verb reaches a harness only through the runtime port
  and the adapters' existing constructors.
- **AP-10** honored: creating and removing a worktree, resolving the route,
  and launching the build are mechanical, so they are code (D-1, D-2, D-5).
  Deviation, with reason: whether every finding was answered is checked by
  the reviews and not by code (D-4), because a mechanical check is a
  mechanism no stated need asks for yet (AP-31, WR-2).
- **AP-12** honored by explicit release: the verb that creates the prototype
  removes it, the session that takes the draft to its end calls it (D-2), and
  one feature holds at most one. There is no periodic reaper; the lock lands
  on the code host, where no local command runs.
- **AP-13** honored: the verb reports success only after confirming the
  worktree, its branch, and its base commit, and reports the build by its
  exit and its commit, not by the process's own claim.
- **AP-16** honored: the build is bounded in time, and a route that cannot
  run is reported, never skipped in favor of another.
- **AP-17** honored: removal acts only on a registered worktree on the exact
  prototype branch and refuses a dirty one.
- **AP-20** honored: the prototype mutates only its own worktree, and the
  verb does not checkpoint a tree whose writers it cannot prove stopped.
- **AP-25** honored: the knowledge lives in a vendor-neutral guide.
- **AP-30** honored: the remote, mainline branch, draft prefix, and the
  prototype's harness and profile come from the repository's config, and the
  profile keeps model identifiers in their one home, the harness catalog. The
  prototype branch prefix is Pyxis's own.
- **AP-31** honored: the launched build exists because the operator asked for
  it (FR-6). No budget meter, requirement-based routing, new file class, or
  readiness condition is added.

## Task estimate

Five tasks: the worktree verb with its tests (D-1); the prototype config
block and route resolution (D-2); the launched build, with the prototype
launch purpose (D-5); the prototype guide, skill pointer, and record template
(D-2, D-3); the passages in the existing guides and the pipeline map (D-4).
