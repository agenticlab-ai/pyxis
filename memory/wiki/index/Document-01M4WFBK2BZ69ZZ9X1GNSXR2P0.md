# docs/wiki — distilled knowledge, never authority

The wiki captures **descriptive** knowledge: orientation material, conventions
explainers, maps of how the document system fits together. It is the one
deliberately mutable documentation class, and it stays safe under four rules:

1. **Never authoritative.** AGENTS.md, the vision/principles, designs, and
   PRDs govern; on any conflict, the wiki is wrong by definition. Every entry
   declares this and cites its sources.
2. **Pointer-heavy, copy-light.** Entries distill shape and link to the
   authoritative document — they do not restate normative text (duplicated
   rules are the drift vector; links to immutable targets never go stale).
3. **Flat, with routing headers.** One directory, kebab-case slugs, no
   subdirectory taxonomy. Every entry opens with the house header — In one
   line · Relevant-when · Authority · Sources · Last reviewed — so agents can
   load-or-skip without reading. When this index outgrows hand maintenance it
   becomes a generated one, not a reorganization.
4. **Unnumbered sections, no entry IDs.** Numbered anchors are permanent by
   ruling; mutable pages should not carry that burden. Wiki entries are
   pointers, not citable decisions — slugs suffice.

Entries are freely edited via normal PRs; bump `Last reviewed` when you
verify one against its sources. Until stage-11 maintenance and the drift
sentinel exist, freshness is a review-time discipline.

## Index

| Entry | In one line |
|---|---|
| [document-classes](document-classes.md) | Every document class — template, evolution, current-state view, gate, mutability — one table plus a paragraph each. |
| [github-agentic-setup](github-agentic-setup.md) | Repo settings, merge-train discipline, generated-artifact hygiene, and `gh` mechanics for agent-driven PR pipelines. |
| [glossary](glossary.md) | One-line definitions of the document-system vocabulary, each pointing at its defining document. |
| [impl-review-reference](impl-review-reference.md) | What a persisted proposal review carries — the report/verdict/finding schema and the triage states (review-absent · review-stale · dispute · escalation). |
