# Widget — prototype findings record

| Field | Value |
|---|---|
| **In one line** | A local adapter probe found a technical challenge. |
| **Relevant-when** | Authoring or reviewing the widget draft contract. |
| **Idea** | Add a widget adapter. |
| **Prototype** | route: ambient codex codex-implementation-economy · branch: `prototype/widget` · base: `base-sha` · head: `head-sha` |
| **Date** | 2026-10-25 |
| **Outcome** | ran |
| **Template version** | prototype-record/v1 |

## What was built

A minimal adapter path that accepts a local widget.

## What was left out

Persistence and production integration.

## Findings

### L-1 — Adapter lookup needs a stable key

- **Kind**: technical challenge
- **Tried**: Looked up the adapter from the widget's display name.
- **Observed**: Two display names can vary while referring to the same adapter.
- **Answer**: open

## Structure checklist

- [x] Header fields and template version are complete.
- [x] Built and left-out behavior are stated.
- [x] The finding has an `L-n`, kind, tried, observed, and Answer field.
