# help — the ecosystem's front door (derive, never author)

A stateless, read-only skill: on every invocation it re-reads the live
routing surfaces and composes one of three answers — the capability
catalog, a described problem's routed entry, or a newcomer's first-steps
path. It holds no catalog, roadmap, or feature list of its own. The
contract is the ecosystem-help PRD ledger; the mechanism is its technical
design — both under the adopter-namespace doc roots the repo config names
(`paths.prds` / `paths.tech_designs` in `.pyxis/config.yaml`), never an
assumed `docs/…` literal. The skill **writes nothing** — no file, no cache,
no memory of a prior invocation.

## The three answers

| Invocation | Answer | Section |
|---|---|---|
| bare `/help`, or a general "what's here?" ask | the capability catalog | §2 |
| `/help <a described problem, idea, or task>` | the routed entry + why | §3 |
| `/help start`, or "where do I start?" / "I'm new" | the first-steps path | §4 |

Classify the ask before doing anything else. A message that both signals
"I'm new" *and* describes a real problem gets the first-steps answer with
that problem routed as its one concrete action (§4) — first-steps subsumes
routing when both are present in the same ask.

## 1. Read the live surfaces — every invocation, never from memory

Every fact in every answer traces to a read performed *this turn*. Never
answer from recollection of a prior invocation, and never let a concrete
skill or stage name creep into this guide or the glue file (`SKILL.md`)
themselves — if you catch yourself about to hardcode one, stop: that is the
purity invariant this skill exists to hold, and both files are read live,
every time, precisely so neither has to change when the installed surface
does.

1. Resolve the repository root dynamically (e.g. `git rev-parse
   --show-toplevel`) — never an assumed absolute path. **Adopter-namespace
   doc locations** — the wiki, and any `docs/…` corpus this guide points at —
   come from the repo config `.pyxis/config.yaml`'s `paths:` block, never a
   hardcoded `docs/…`: an installed repo may place its doc corpus anywhere,
   and config is its single source (config-driven-portability DINV-1). The
   **system tree** read in steps 2–3 (`.pyxis/…`) is the deliberate
   exception — it is fixed, never config-driven (see the note below).
2. Read `.pyxis/pipeline.md` **whole**: the numbered stage table with its
   status legend, the "Where a problem enters" section, and the
   "Cross-cutting skills" section.
3. Read the **frontmatter only** — the block between the `---` markers,
   `name` + `description`, never the instructions below it — of every
   `.pyxis/skills/*/SKILL.md`. Read this canonical tree directly; the
   vendor directories (`.claude/skills`, `.codex/skills`, `.agents/skills`,
   …) are pointers into the same tree, so this is identical regardless of
   which host CLI is running you.
4. **On demand only** — when a route or first-steps answer turns on a term
   or convention the map itself doesn't cover: consult the repo's **wiki
   corpus** if it has one — an adopter-namespace doc surface, so resolve its
   location per step 1 (the config's doc roots / the repo's own layout),
   never an assumed `docs/wiki/`. Read its index, then the *one* matching
   entry's routing header only (its `In one line` / `Relevant-when` rows) —
   never an entry's body. Where the config names no wiki, or none is
   present, the degraded walk covers it.

The pipeline map, the skills tree, and this guide's own location are fixed
paths of the vendor-neutral system tree (`.pyxis/…`), not per-repo
configurable — config-driven portability's own scope explicitly excludes
the system tree, because it lives in Pyxis's own namespace and never
collides with an adopter's layout (`prd:config-driven-portability/NG-1`).
The wiki is **not** in that set: it is an adopter-namespace doc corpus, so
its location is resolved from the repo config (step 1), never a hardcoded
`docs/wiki/`.

**Degraded walk.** Any surface above that is missing or unreadable narrows
the read set, not the honesty of the answer: proceed with what's left and
name the absent surface explicitly in whichever answer needed it. Never
fabricate an entry to paper over a gap.

## 2. The catalog (bare invocation)

Group everything step 1 read into three buckets, in this order:

- **Entry stages** — the stage table's rows, in order, each carrying its
  status glyph. A row whose mechanism names a skill: use that skill's own
  frontmatter description (condensed), not the table's prose. A row whose
  mechanism is a non-skill tool or verb: condense the table's own cell. A
  row with no live mechanism at all: show its status only — never invent a
  name to fill it.
- **Cross-cutting** — the "Cross-cutting skills" section's own bullets.
- **Tooling** — any remaining `.pyxis/skills/*` entry that appears in
  neither bucket above.

Every installed skill lands in exactly one bucket; every stage row appears
once. This grouping is computed fresh from what step 1 just read — it is
never a list kept in this file.

Render each capability at **≤ 2 output lines**: the invocable name plus its
one-line purpose/when-to-use. Close with a pointer to the other two
answers — that describing a real problem routes it (§3), and `/help start`
(or asking where to start) gives the first-steps path (§4) — plus the
map's own path for the full detail.

## 3. The route (a described problem)

Match the described problem against what step 1 read — the map's "Where a
problem enters" rules first, then the stage table and the cross-cutting
descriptions, then (on demand) a wiki routing header for a convention or
term the map itself doesn't cover. This is the one judgment call in this
skill; make it once, here, over what was just read — never from memory of
a past answer.

- **A match onto a built entry** — name the entry command, the one-line
  reason (which stage or cross-cutting role owns this class of problem,
  per the map), and what to expect next (the stage's own "Produces" column,
  where one applies). Carry forward any caveat the map itself attaches to
  that row (an advisory-period footnote, a human-gate note from the Actor
  column) rather than flattening it away.
- **A match onto an unbuilt or partial stage** — never hand back its
  mechanism as something to run. State the stage's name and its exact
  status (the glyph plus the legend's meaning), then name the nearest
  built entry — walking outward along the stage table, upstream before
  downstream, to the closest row that is actually invocable — with a
  one-line bridge grounded in the map's own text, never an invented one.
- **No match** — say plainly that it isn't a pyxis entry point today, and
  point at the map (`.pyxis/pipeline.md`) rather than guessing. Never
  invent a command or stage to avoid an honest refusal.

## 4. First steps (`start`, or "where do I start?")

Lead with a fixed two-sentence orientation — the one authored content this
skill carries, because it describes the product, not a capability:

> Pyxis turns a product idea into merged, verified software by walking it
> through a pipeline of agent-run stages, with a human checkpoint where it
> matters. `/help` is the front door to that pipeline — describe a problem
> to get routed to the right entry, or ask again any time with `start`.

Then give the **one concrete first action**:

- If the invocation (or the surrounding conversation) already names a real
  problem or idea, apply §3 to it and present that result as the action —
  first-steps and routing collapse into one answer.
- Otherwise, derive the default from the map's "Where a problem enters"
  section: its first rule already names the right entry for someone with
  nothing yet — use whatever it names, read live, never a name kept here.

Close by pointing at the catalog (`/help` bare) and the map itself. Never
teach corpus content here — this path routes into §2/§3, it does not
restate what they would explain.

## Never

Never answer from memory or training data — every line of every answer
traces to a read performed this invocation. Never invent a command, skill,
or stage absent from what was actually read. Never present an unbuilt
stage's mechanism as invocable. Never write, edit, cache, or persist
anything, anywhere, including between your own turns. Never read a skill's
guide body, a wiki entry's body, or any other governed or corpus document —
only the routing surfaces named in §1. Never restate or teach corpus
content in the first-steps answer. Never silently narrow the read set — a
missing or unreadable surface is always named in the answer that needed
it. Never let a concrete skill or stage name find its way into this file or
`SKILL.md`.
