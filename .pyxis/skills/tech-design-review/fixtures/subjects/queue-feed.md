# Queue feed — technical design *(seeded fixture subject — abbreviated, six planted defects; not a real design)*

| Field | Value |
|---|---|
| **Status** | Draft *(fixture)* |
| **Serves** | prd:queue-widget/FR-1 · prd:queue-widget/FR-3 |
| **Builds-on** | — |
| **Supersedes** | — |
| **Template version** | tech-design-template/v1 |

## 2. Decision

**Decision:** The widget reads a feed file the engine maintains; a refresh
verb regenerates it on demand.

## 3. The design

### 3.1 Mechanics

The feed is a snapshot artifact; the staleness mark is decided from the
snapshot's own age header. *(planted: Serves cites FR-3 — the staleness
mark on outage — but nothing here decides outage vs mere age: a
Serves-substance gap — upstream-alignment)*

### 3.3 Invariants

- **DINV-1** — the feed is reliable. *(planted: not property-testable —
  contract-quality)*

## 4. Alternatives considered

| Alternative | Why rejected |
|---|---|
| do nothing | no feed exists |
| a feed that recomputes on every widget open per keystroke | absurd cost *(planted: a strawman nobody could choose — coherence)* |

## 6. Acceptance

| ID | Acceptance criterion | Verify | Traces to |
|---|---|---|---|
| A-1 | a seeded queue change reaches the feed within the refresh bound | Test | prd:queue-widget/FR-1 |
| A-2 | the outage path shows the marked stale state | Test | #DINV-2 *(planted: DINV-2 does not exist — coherence)* |

## Principles

- **AP-15 — honored.** Staleness decisions use exact reads. *(planted:
  §3.1 decides staleness from the snapshot's cached age header —
  cited-vs-honored breach — upstream-alignment)*
- **Deviation:** none.

*(planted: an adapter-shaped surface — the feed consumed by a widget —
with no testing-strategy subsection anywhere: feasibility-completeness)*
