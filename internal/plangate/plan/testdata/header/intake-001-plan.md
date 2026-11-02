# Maintenance intake — plan 001

| Field | Value |
|---|---|
| **Intake** | maintenance |
| **Plan version** | 001 |
| **Dialect** | harbormaster/fs-spec:v1 |
| **Epic title** | Repair the shared widget model |
| **Status** | Draft |
| **Template version** | plan-template/v1 |

## Tasks

### T1 — Repair widget validation

| Field | Value |
|---|---|
| **Blocked by** | — |
| **Size band** | 60–120 |
| **Mechanical** | no |
| **Size accepted** | — |
| **Overlap accepted** | — |
| **Routing** | rung=2 |
| **Pattern** | — |
| **Touches** | internal/widget/model |

**Goal:** The widget model rejects unknown fields with a named defect.
**Current behavior:** Unknown fields can enter the widget model without a named defect.
**Desired behavior:** Validation rejects each unknown field and names it in the defect.

**Scope:**
- closed validation of the widget model

**Out of scope:**
- persistence of widgets

**Acceptance criteria:**
- [ ] validating a widget with an unknown field yields a defect naming that field

**Key interfaces:**
- `Widget` — the model type callers construct

**Traces:** prd:widget/FR-1 · design:widget-design#3.1
