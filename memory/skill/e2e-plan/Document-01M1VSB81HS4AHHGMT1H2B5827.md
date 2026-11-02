---
name: e2e-plan
description: Author an ordinary plan-new plan/v1 from an approved e2e design doc — one task per scenario, plus a harness task only when the feature's e2e harness does not already exist, every scenario task blocked-by an emitted harness task. Hands the plan to plan-new's existing pipeline unchanged; refuses on an unapproved doc. Use once a feature's e2e design (docs/e2e/<feature>/design.md) has been reviewed and approved.
---

Follow `.pyxis/authoring/e2e-plan.md` — the vendor-neutral guide (AP-25:
this file is invocation glue only; all authoring knowledge lives there).
Read that guide fully and execute its steps, including its Never list.
