# Widget — plan 001

| Field | Value |
|---|---|
| **Feature** | widget |
| **PRD** | docs/prds/widget |
| **Design** | docs/tech-design/widget.md |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Widget delivery |
| **Status** | Approved |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Build the widget

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/widget |

**Goal:** The widget works.
**Current behavior:** No widget exists.
**Desired behavior:** A working widget is delivered.

**Scope:**
- widget behavior

**Out of scope:**
- a second feature

**Acceptance criteria:**
- [ ] the widget works

**Key interfaces:**
- the widget entry point

**Traces:** prd:widget/FR-1

### T2 — Verify the widget

| Field | Value |
|---|---|
| **Blocked by** | T1 |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | — |
| **Pattern** | — |
| **Touches** | internal/widget |

**Goal:** The widget has delivery evidence.
**Current behavior:** Delivery is unverified.
**Desired behavior:** Checks verify the widget.

**Scope:**
- delivery checks

**Out of scope:**
- a second feature

**Acceptance criteria:**
- [ ] the checks pass

**Key interfaces:**
- the check result

**Traces:** prd:widget/FR-1

## Coverage

| Requirement | Tasks |
|---|---|
| prd:widget/FR-1 | T1, T2 |
