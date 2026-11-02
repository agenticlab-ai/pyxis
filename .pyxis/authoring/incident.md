# /incident — capture or graduate an incident through the coded writer

The vendor-neutral authoring guide (AP-25). The registry is append-only
failure memory with one coded writer (INV-35; `prd:incident-machinery`);
an agent never hand-allocates an ID and never hand-writes or edits an
entry — the writer verbs are the only path.

## Capture

1. **Decide it is an incident** — a paid-for lesson: a decision/design that
   later broke, too specific for a principle, too expensive to relearn.
   Capture is judgment (yours or the operator's); the machinery only
   governs *how*. When in doubt, ask the operator.
2. **Assemble the four fields** — Decision (what was decided/built), Broke
   (the observed failure), Lesson (what to do instead), Links (the
   introducing design, the implementing PR(s), the issue — AP-7's link
   contract). One bounded entry, ~6 body lines; write complete sentences.
3. **Pick tags** — lowercase kebab-case, comma-separated; the subsystem
   grep keys (`docgate`, `tech-design`, `governance`, …). Reuse existing
   tags before inventing.
4. **Write through the writer** (never by hand):

   ```
   docgate add-incident \
     --date <YYYY-MM-DD> --tags <a,b,c> \
     --decision "…" --broke "…" --lesson "…" --links "…"
   ```

   The writer allocates the next dense `I<n>`, self-checks against the
   lint, and prints the ID. A refusal names the defect and writes nothing
   — fix the fields and retry.
5. **Commit and PR normally.** The gate re-checks shape and append-only-ness;
   a concurrent-allocation collision reds the second PR with the next free
   number (renumber via a fresh `add-incident` on the rebased branch).

## Graduate

When a lesson recurs and the operator distills it into a principle
(registry → principle is one-way and operator-fed):

```
docgate graduate --id <n> --principle AP-<n>
```

This appends `Graduated: AP-<n>` to the entry's `Links:` line — the one
sanctioned mutation. Double graduation is refused.

## Never

- Never hand-write, renumber, or edit an entry — including "just fixing a
  typo" (entries are append-only; a wrong entry is corrected by a new
  entry that cites it).
- Never invent the discovery: automated detection/attribution is
  engine-era (issue #53) — today a human or agent *judges*, the writer
  *records*.
- Never edit `docs/observability/incidents.md` scaffolding to change entry
  semantics — the shape is pinned by `incident-entry-template/v1`.
