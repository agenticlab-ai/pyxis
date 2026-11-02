# Widget capability — plan 001

| Field | Value |
|---|---|
| **Feature** | widget |
| **PRD** | prd/widget |
| **Design** | design/widget |
| **Basis** | lock/widget/v1 |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v2 |
| **Epic title** | Widget capability v1 |
| **Status** | Draft |
| **Template version** | plan-template/v2 |

## Tasks

### T1 — Implement durable widget opening

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 80–140 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Requirements** | {"schema_version":"task-requirements/v1","implementation":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"adapt","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:acceptance"}]},"rationale":"Durable opening and offline queuing compose existing behavior within one feature."},"review":{"assessment":{"reasoning":"complex","coupling":"composed","novelty":"known","verification":"composed","impact_scope":"contained","architectural_role":"feature_local","citations":[{"id":"task:goal"}]},"rationale":"Independently check durability and queue outcomes across the composed feature."}} |
| **Pattern** | — |
| **Touches** | internal/widget |

**Goal:** Widgets open durably and queue while offline.

**Current behavior:** No widget opening implementation exists.

**Desired behavior:** A widget opening survives retries and an offline opening enters a durable queue.

**Scope:**
- the durable opening and queue

**Out of scope:**
- widget sharing

**Acceptance criteria:**
- [ ] a retry preserves the opening and an offline request appears in the queue

**Key interfaces:**
- `Open` — accepts a widget opening request

**Traces:** prd:widget/FR-1 · prd:widget/NR-1 · prd:widget/NFR-1 · prd:widget/UB-1 · design:widget#D-1

## Coverage

| Requirement | Tasks |
|---|---|
| prd:widget/FR-1 | T1 |
| prd:widget/NR-1 | T1 |
| prd:widget/NFR-1 | T1 |
| prd:widget/UB-1 | T1 |
