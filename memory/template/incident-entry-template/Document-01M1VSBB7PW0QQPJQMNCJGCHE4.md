# Incident entry — template

*Version: `incident-entry-template/v1`. This documents the entry shape the
coded writer renders and `lint-incidents` checks — one schema, three
consumers (writer, lint, this file; incident-machinery FR-3). Entries are
NEVER hand-written: `docgate add-incident` allocates the ID and writes the
entry; `docgate graduate` performs the one sanctioned mutation. The
registry itself (its intro and legend) documents the semantics; this file
pins the shape.*

## The shape

```
**I<n> (YYYY-MM-DD) [tag, tag, …]**

- **Decision:** what was decided / built.
- **Broke:** how it broke (the observed failure).
- **Lesson:** the distilled takeaway — what to do instead.
- **Links:** provenance — the introducing design, the implementing PR(s), the issue(s).
```

## The rules the writer and lint enforce

- **`I<n>`** — dense, code-allocated, never renumbered (INV-35).
- **Date** — `YYYY-MM-DD`, supplied by the caller (the writer reads no clock).
- **Tags** — one or more comma-separated lowercase kebab-case tokens; the
  retrieval key ("consumers grep their tags").
- **Fields** — exactly the four, each as a `- **<Field>:**`-anchored line.
- **Links** — per AP-7: the technical design that introduced the issue and
  the change(s) that implemented it; grows (only) by graduation:
  `Graduated: AP-<n>`.

## Worked example

```
**I7 (2026-07-08) [gate, config]**

- **Decision:** The gate read its exemption lists from an ungated file.
- **Broke:** A PR added a design and its own exemption in one diff; the gate passed it.
- **Lesson:** Policy keys need the operator trailer — self-unlock is never self-serve.
- **Links:** docgate-fence-completion.md (introducing design); PR #58; issue #54.
```
