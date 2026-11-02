# Pyxis — Architecture Principles

**Version v0.2** transcribed the operator's
founding guidance (AP-1..AP-9); v0.2 adds the **engineering principles**
(AP-10..AP-24), distilled from a first-generation factory's full operating
history — its principle set, its incident registry, and its complete issue
tracker — and the **rewrite principles** (RW-1..RW-6). This document is
**append-only** once accepted: principles are added, never rewritten or
renumbered, so citations never go stale. It serves the product vision and, as
an abstract-layer document, links to no concrete document.

## How to use this document

- **Every technical design must cite the principles it honors or deviates
  from, by ID, with reasoning.** Deviation-with-reason is legitimate; silent
  omission is not.
- Three tiers: **product principles** (AP-1..9, the founding rules),
  **engineering principles** (AP-10..24, paid for in operation), and
  **rewrite principles** (RW-1..6, temporal — they govern the rewrite and are
  retired, never renumbered, when it completes).
  *(Note, 2026-07-09: principles accrete — the engineering tier now extends
  past AP-24. Read the ranges above as the v0.2 snapshot, not a bound.)*
- Pyxis maintains its own **append-only incident registry** (stable IDs,
  tagged, ~6 lines each: decision / broke / lesson / links). Lessons that
  recur graduate into principles here — the registry is the staging ground;
  the reference direction is one-way, registry → principle.

---

## Product principles

**AP-1 — The core is medium-independent.** Core logic is fully separated from
every integration point through adapter and interface layers. The core never
knows or cares which system holds the work items, which hosts the code, which
runs the checks, or which agent runtime implements a task. Swapping any medium
touches adapters only.

**AP-2 — The core is invocation-independent.** The engine's behavior is
identical whether it is invoked by **pull** (a scheduler tick) or **push** (a
webhook). Triggers deliver a normalized wake signal; they never carry logic.

**AP-3 — Adding an integration is following a document.** Every port ships an
adapter-authoring guide and a runnable conformance kit; a new integration point
is built by following the document, passing the kit, and it just works — with
zero core changes.

**AP-4 — Skills and hooks are portable.** All skills/hooks are authored once,
canonically, and are portable across every supported agent runtime and to user
machines. Per-runtime translation is adapter work, never a fork of the
content.

**AP-5 — Monitoring and tracing are first-class citizens.** Every action emits
structured events and carries trace context; a feature is traceable end-to-end
from raw idea through PRD, designs, tasks, workers, and PRs to verification.
Observability is designed in, never bolted on.

**AP-6 — Everything passed around is contractual.** No free-form text between
components: all cross-boundary information is schema-versioned structured
data. The data model is easily extensible (additive evolution) **and compact**
— burning fewer tokens and keeping payloads small is a design constraint, not
an optimization.

**AP-7 — Document mutability, gating, and links.**
- The product vision, the architecture principles, and incident entries are
  **append-only from the beginning**.
- Per-feature **PRDs** and **technical designs** are **immutable once the
  change that adds them is approved**: no automated author may change them.
  Gating exists at **both** the automated-check level and as constraints in
  the prompts/guards.
- The **living rollups** (current capability inventory; current technical
  state) are updated, if needed, on every change — and they do not exist
  before the first capability ships.
- **Links point from concrete to abstract, never the reverse**: PRDs and
  technical designs cite the vision and principles that govern them; the
  abstract docs link to no concrete document. Because link targets are
  append-only or immutable, **links never go stale**.
- **Incidents link to the technical design that introduced the issue and the
  change(s) that implemented it.**

**AP-8 — Per-repo extension is sanctioned and schema'd.** A repo customizes
Pyxis only through its one sanctioned, closed-schema extension surface:
repo-scoped configuration plus custom prompts for named actions/skills. No
other extension path exists.

**AP-9 — Releases are staged and observable.** Every install role auto-updates
through release channels and cohorts (a percentage of installs, or named
ones), so a release's impact is observed before it widens; a beta channel
tracks the source directly, including in-place core updates that need no
artifact rebuild. Anonymous global telemetry feeds product improvement; the
local plane never depends on the global one.

---

## Engineering principles

The axiom under all of them: **the engine is deterministic; intelligence is
invoked only at explicit judgment points.** AP-10 and AP-21 are the two halves
of that axiom; the rest keep an autonomous, concurrent, failure-prone system
honest.

**AP-10 — Codify over prompt.** If a behavior can be captured as structure or
code, codify it — never encode a mechanical rule in a prompt and hope the
model adheres. Code is predictable, enforceable, and free; prompt-encoded
rules drift, cannot be tested, and are paid for in tokens on every invocation.
Prompts are reserved for genuine judgment (AP-21).

**AP-11 — One predicate, one place.** When two components evaluate the same
condition — readiness, candidacy, drift, reap-eligibility, required checks —
they share one implementation. *Why:* twin predicates always diverge, and a
divergent pair once produced a perpetual re-fire loop burning millions of
tokens per hour on already-handled work; a local gate that omitted one CI
check let work pass locally and strand red in CI. The local quality gate
mirrors CI exactly, or it is not a mirror.

**AP-12 — Every acquisition is accounted.** Every acquired resource — a lease,
a workspace, a branch, a scratch artifact — ends in exactly one of: consumed,
explicitly released, or reaped by an audited janitor. Acting on *absence*
requires an explicit in-flight predicate: a vanished resource may be
legitimate end-of-life, not failure. A new throwaway-resource type ships with
its janitor **in the same change** — resource types added without reapers
accumulate as husks.

**AP-13 — Success is a verified artifact.** Report success only on a
verifiable artifact — the proposal exists, the ref is pushed — never on an
exit code or an agent's prose ("done" is a claim; the artifact is the proof).
A gate's PASS certifies **the exact revision that ships**: validate what will
be pushed, assert the working tree is clean, and confirm the shipped commit
matches what was validated.

**AP-14 — Verify by execution and observation.** Acceptance is observed
behavior — events, telemetry, live probes — never merged-diff review or
unit-green alone. Corollaries, each paid for: execute generated and vendored
artifacts install-free before committing them (a green merge is not a working
artifact); test from the deployment context (a hostile cwd, no ambient
credentials, the packaged layout), not only the dev checkout; a test double
must satisfy the **real dependency's contract**, not the code-under-test's
assumption about it — a stub that mirrors your own wrong guess makes a
never-working feature test green; extract scripts embedded in configuration
so they are parseable and testable.

**AP-15 — Exact, fresh reads for consequential decisions.** Any read feeding a
destructive or dispatch decision uses exact, strongly-consistent lookups —
never search indexes or cache-prone conditional reads (a stale read once made
a just-shipped task invisible, and a second worker rebuilt it). When a
stale-read defect is found, sweep **every read of that class**, not just the
one that bit. Tests must exercise the same caching path production uses — a
global freshness bypass in tests hides exactly the bug being tested for.

**AP-16 — Nothing loops or skips silently.** A safety skip that can repeat
indefinitely must count, alarm, or self-heal after N occurrences. Every
self-healing loop bounds its rounds and escalates with context when it cannot
converge — an unbounded retry against a fast-moving target loops forever. A
recovery surface handles an already-known bad state **directly**, not only as
a side-effect of some other failed action, or the known-bad case dead-ends.

**AP-17 — Fail closed on destructive, fail open on additive.** Ambiguity never
deletes, tombstones, or kills: destructive actions require positive evidence,
and a failed probe reads as "alive". At worst, failing open on the additive
side spawns redundant work — which mutual exclusion serializes cheaply.

**AP-18 — Observe, don't declare.** Behavior keys on observed state — presence
freshness, power source, load, capability probes — never on declared kinds or
roles. Declared-kind branches rot as reality changes; observations stay true
by construction.

**AP-19 — Idle costs nothing.** Intelligence fires only behind a mechanical
gate that has already secured the work; losing racers exit at ~zero cost;
an idle system spends ~nothing. Every recurring path states its steady-state
cost, and per-decision context is bounded — recurring cost that nobody stated
is how a quiet system burns a budget.

**AP-20 — Narrowest blast radius.** Mutations happen in the smallest scope
that can contain them — a worker mutates only its own workspace, never a
shared checkout — and the boundary is enforced mechanically (guards), not by
convention. A single stray mutation of shared state once silently froze a
whole node for hours.

**AP-21 — Judge once, at the richest-context site; freeze the result.** Some
decisions are genuine judgments, not codifiable rules — complexity, routing,
risk, disposition. Make each **once**, at the phase holding the most context;
record the result as structured data every later step reads verbatim; hold it
inside a mechanically-enforced envelope (clamps, floors, caps). Never re-judge
downstream with less context; never trust a judgment to stay in bounds on its
own. The complement of AP-10: AP-10 codifies *rules*, AP-21 places and
contains *judgments*.

**AP-22 — Idempotent and raceable by default.** Every cross-component
operation converges under re-runs and races: the losing racer records a no-op
(never an error), re-running an installer converges, a second sweep finds
nothing to do. Correctness never depends on "exactly one actor is running" —
in a fleet, someone is always racing you.

**AP-23 — Design for parallel authors.** Many agents edit the same system
concurrently. Shared append surfaces (usage strings, label sets, event
catalogs, dispatch tables) are **line-per-item and single-sourced**, so
concurrent additions merge cleanly; work decomposition declares the files each
task touches and sequences the overlaps. Remove conflict magnets structurally
— a recovery loop that repeatedly re-merges the same hot line is treating the
symptom.

**AP-24 — Governance is never rebuilt from the governed.** Manifests,
policies, the vision, and these principles are human-authored inputs.
Enforcement reconciles reality *toward* them; nothing regenerates them *from*
reality. A governing artifact derived from the system it governs can justify
anything the system already does.

**AP-25 — Agent-facing surfaces are vendor-neutral.** Knowledge an agent
needs to work in a repo — operating rules, authoring guides, review
procedure — lives in vendor-neutral documents; a vendor surface (`.claude/`,
`.cursor/`, a CLI config) may only point at it, adding invocation glue,
never content. AP-1 keeps the engine's mediums swappable; this keeps the
repo's agent runtime swappable: onboarding a new CLI is adding a pointer,
never porting knowledge. (Practiced from the start as the
AGENTS.md/CLAUDE.md symlink; ratified when the first authoring-skills PR
violated it and was corrected.)

**AP-26 — A medium's status certifies the medium's object, never yours.**
Every status a medium reports — "merged", "delivered", "closed", "deployed"
— is a claim scoped to that medium's own object: a proposal's merged badge
certifies its base branch, not the mainline; a queue's "delivered" certifies
the broker's hand-off, not the consumer's processing. Goal-state is accepted
only by observing the goal object itself — the commit is an ancestor of the
mainline, the record exists in the store, the traffic serves — never by
reading the upstream status. This is AP-13 and AP-14 applied at integration
boundaries, where they are easiest to skip: a chain of statuses, each true
of an intermediate object, can sum to a false belief about the end state.
*Why:* a stacked four-proposal train once showed every badge green and
"merged" while none of its content had reached the mainline — each badge was
true, about a side branch nobody wanted; the fix then stranded the same way
within the hour, and only direct ancestry checks against the mainline caught
both.

**AP-27 — Extension is by addition; a shared append point is a design
defect.** When a plan fans one "add an X" out to N sibling tasks, each
addition must land as pure addition — a new file, a new self-contained entry
— with zero edits to any line a sibling could also be editing. Git cannot
order two insertions at the same point: every shared append surface (an
umbrella enumeration, a dispatch switch, a usage block, a hand-kept index)
turns N independent branches into ~N² hand-resolved keep-both merges against
a mainline that never stops moving. The unit of concurrent extension is
therefore the **file, not the line** — sharpening AP-23, whose line-per-item
rule keeps each resolution trivial yet still parks every sibling on the same
list tail. A genuinely needed central aggregate is **derived, never
authored**: self-registration, discovery by scan or manifest, or generation
by the build — no author's diff touches the aggregate, its order is explicit
data (keys, sorted output), and a conformance test proves a new unit is
picked up with zero central edits. Distinguish ledgers from sets: where
order carries meaning (an amendment ledger) serialization *is* the design;
everywhere else the collection is a set, and sets merge by addition.
Sequencing set-like overlaps in the plan, or hand-merging them at land time,
is damage control that spends the parallelism the plan exists to create —
when honest per-task file footprints intersect on a hot file, the defect
belongs to the design that owns the file, never to the schedule. AP-22 makes
operations raceable; this makes changes raceable: mergeability never depends
on being the only author. *Why:* four sibling tasks from one plan each added
one check; every addition edited the same three central lists plus bodies in
one shared file, each merge re-conflicted the siblings still in flight, and
the hot surface grew two more entries while its replacement was being
designed — the storm outran the fix.

**AP-28 — Improvement fuel is retained in git, never discarded.** Any generated
output that carries signal the system could learn from — a gate's defect report,
a failure disposition, a review verdict, a drift snapshot — is kept as a
committed artifact in the repository, not written to a gitignored scratch path,
an ephemeral CI-run artifact, or a log that ages out. The store is git because
the fuel must be durable, diffable, attributable, and reviewable long after the
run that produced it: improvement is a slow loop, and a signal discarded at run
time cannot feed it. Retain such output as an immutable, append-only **event
record** — a dense-numbered or timestamped entry capturing the state at the
moment it was produced — never as a mutable live projection a later fix silently
overwrites, which goes stale the instant the underlying state moves. This binds
only the outputs that teach: a clean pass still materializes nothing, and routine
derivable projections regenerate on demand — the retained set is exactly the
failure-and-judgment signal that is cheapest to lose and dearest to reconstruct.
*Why:* the mechanical contract gate emits a defect report only when a contract
fails to compile, and the operator ruled that this report — and any output that
can help the system improve — is git-committed as an event record, so the
failures the factory catches accrete into the corpus it improves from instead of
evaporating with the run that found them.

**AP-29 — A gate asserts correctness; a measurement only records.** A blocking
gate — anything whose verdict can fail a change: a check, a test, a conformance
assertion — must be **hermetic**: a pure function of its declared input, so the
same input yields the same verdict on any machine, any run, under any load. A
**measurement** — wall-clock time, memory, cost, size, throughput — is a
different category: it is a property of the run's environment as much as of the
code, so it is never a correctness claim and must never be asserted as a blocking
pass/fail. Measurements are *recorded* — to a benchmark, the observability plane,
or an append-only event record — and read as trend and regression signal; they do
not gate. Encoding one as a gate — "performance testing as a blocking unit test"
— is a category error: it greens on a fast, unloaded box and reds on a slow one
for identical code, so it cannot be mirrored across the surfaces that must agree
(it breaks AP-11) and it flakes the gate for every change regardless of the
change. When a speed or size bound genuinely matters, it is a recorded figure a
human reads against a target, or a separately-classed advisory that never blocks —
never a correctness verdict. *Why:* incident I5 — the code-gate strand where a
wall-clock latency assertion over an ambient `git log` diff passed on a dev
machine and stranded required CI red under `-race` on a slow runner (#600),
un-mirrorable by construction — and the operator's ruling that performance
testing asserted as a unit test is a category error: measurements record, they
never gate.

**AP-30 — Every operand-owned value is configured, never hardcoded.** Any value
that names something the *operand repository* owns — a documentation path, the
default branch, the git remote, a source directory, the layout of a vendor
surface — resolves from the repo config with its current value as the default,
and is never written as a literal. Ownership is the oracle: pyxis's own installed
namespace (`.pyxis/`) is byte-identical in every repository, so its internal
paths and values are hardcoded freely — configuring them buys adoption nothing.
The instant a value crosses from *pyxis owns this* to *the repo owns this*, it
becomes configuration. This is the coherence property that makes "no privileged
substrate" (AP-1) true in fact rather than in claim: a value baked to the shape
of the repository it was authored in makes pyxis run correctly *only there*,
dishonoring or colliding with any adopter whose repository is laid out
differently — and repository portability is the precondition of the entire
adoption funnel, never a later polish. The property is **medium-independent, and
that is where it is lost.** It binds every surface pyxis authors, not the
engine's code alone: gates, git-hooks, and the generated CI workflow, but equally
skill prompts, authoring guides, and templates. A natural-language prompt that
bakes in an operand's doc path is exactly as hardcoded as a Go constant — and
more dangerous, because it reads as innocent prose and slips past a grep of code
literals. A surface that genuinely cannot resolve config at runtime (a static CI
trigger) is rendered from config at install; every other surface reads the config
it already holds, single-sourced so no second copy can drift (AP-11). AP-1 keeps
the engine's *mediums* swappable and AP-25 keeps the repo's *agent runtime*
swappable; AP-30 keeps the *repository itself* swappable, reading only from
AP-8's one sanctioned config surface. *Why:* the moment the engine's
target-facing paths, branch, and remote were lifted out of code into config, a
newly-authored discovery skill hardcoded the operand's `docs/` corpus paths
straight back in — through its prompt, the one medium the coherence audit treated
as prose and never scanned as it scanned code. The regression rode in on the very
class of value the engine had just been freed of, proving the boundary is a
property of *every* surface pyxis authors, not of Go alone.

**AP-31 — Build what was asked, at the scale that exists.** A commitment — a
requirement, a guarantee, a mechanism, a supported platform — enters a contract
only when a stated need asks for it. What an author or a reviewer adds on their
own judgment waits outside the contract, named and priced, until the operator
pulls it in: adding is a ruling, while removing or narrowing an unasked
commitment is always open and needs none. A need is carried at the strength it
was stated — the ordinary case at the present operating point — and is never
silently hardened into a guarantee under every failure, or widened to a scale
nobody has yet. The other principles bind what is built: they are honored for
the mechanisms that exist and are never, by themselves, a reason to build one
more. Because an omission announces itself as a failure and an excess never
does, every stage that can ask "what is missing?" must be able to ask "who asked
for this?" with the same force, and what a contract will cost to build is put in
front of the operator before it locks, while cutting is still cheap. *Why:* a
request for one scheduled job entered the requirements pipeline and left as a
contract of dozens of binding rows and a plan of thirty tasks, many of which
traced to no statement the operator had made. Every step was locally valid — a
template slot filled, a review finding closed by adding a mechanism, a standing
principle honored on a mechanism nobody had asked for — and no gate was placed
to notice the sum, because each one checked for something missing and none for
something unneeded. The operator cut the scope back by hand after the plan had
already frozen.

---

## Rewrite principles

Temporal: these govern the rewrite of the first-generation factory into Pyxis
and are retired (never renumbered) when it completes.

**RW-1 — Parity is a checklist, not a blueprint.** The predecessor's feature
inventory defines *what* Pyxis must do; it never dictates *how*. Rebuild from
requirements and invariants — port behaviors, never mechanisms, and translate
no script line-by-line.

**RW-2 — Port the lessons as constraints, not the scars as code.** Every
inherited failure lesson lands in Pyxis as a principle, a gate, a port
contract, or a test — never as copied workaround code. Pyxis starts its own
incident registry at I1; the predecessor's registry informed this document and
then stays behind.

**RW-3 — Medium workarounds die at the port boundary.** Any mechanism that
existed only to compensate for a medium's weakness — comment-as-database
grammars, label-encoded state machines, cache-defeating read tricks,
redundant lock witnesses — is absorbed into a port's contract requirements
instead of being rebuilt. If a behavior exists because the old medium was
weak, the new interface must simply not be weak there.

**RW-4 — Substrate first.** Ports, the minimal-local adapters, and the
conformance kit are built **before** any real-world adapter. The
factory-in-a-box (local repos + file tracker) is the first shippable artifact:
it is simultaneously the simplest installation and the engine's hermetic test
harness — every invariant is testable in-process before the first network
call exists.

**RW-5 — Design-first, one decision per document.** No engine code without an
approved technical design; the open architecture questions (engine state,
process shape, schema details) are decided in design documents — explicitly,
one decision per document — never silently in code.

**RW-6 — Cutover is earned per capability.** The predecessor keeps operating
while Pyxis grows. A capability replaces its predecessor only on demonstrated
evidence — conformance suites green, e2e verification against the requirement,
observed behavior in shadow or side-by-side operation — never by declaration
or calendar.
