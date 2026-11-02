# Architecture Principles

**Seeded by `pyxis init` from `architecture-principles-skeleton/v1` —
structure and guidance only; the principles are the owner's to author.**
This is the repository's citable principle registry: the rules its designs
are judged against, each under a permanent ID.

Standing rules, in force from the moment this file is seeded:

- **Append-only, from the beginning.** Principles are added under fresh IDs,
  never rewritten, renumbered, or deleted — the gate enforces this even for
  the operator (a change to this file also carries the
  `Governing-Change: operator` commit trailer), so citations never go stale.
  A principle that proves wrong is answered by a new entry that says so and
  supersedes it in force, not by an edit.
- **Citable by ID, honored or deviated from — never ignored.** Every
  technical design in this repository cites the principles it honors or
  deviates from, by ID, with reasoning. Deviation-with-reason is legitimate;
  silent omission is not.
- **IDs are permanent.** `AP-1, AP-2, …` — dense from 1, never reused; the
  shipped design template's Principles section expects this form.
- **The abstract layer links downward to nothing.** No PRD, design, or
  requirement-ID citations here; concrete documents cite the principles,
  never the reverse. The gate scans for violations.
- **Fed by incidents, one way.** The incident registry stages paid-for
  lessons; a lesson that recurs graduates into a principle here, and the
  registry entry gains the graduation pointer. This file never links back
  down.

*Guidance — how to author this document. The seeded lines above and this
guidance block are part of the permanent record and stay in place; author by
**appending** entries below the authoring line at the bottom, never by
editing what is already committed. The fenced scaffold shows the entry form:
copy it beneath the line and fill the `<angle-bracket>` points — the first
entries usually transcribe the founding rules the owner already holds; later
ones arrive as incident lessons graduate. While nothing follows the
authoring line, `pyxis doctor` reports this document as an unauthored
skeleton.*

```
**AP-1 — <the principle's name, as a short imperative>.** <the rule, in two
to five sentences: what it demands, what it forbids, and — once it has been
paid for — the failure that taught it>

**AP-2 — <name>.** <rule>
```

---

*Authored content appends below this line.*
