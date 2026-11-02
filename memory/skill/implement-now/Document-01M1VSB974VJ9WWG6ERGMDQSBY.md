---
name: implement-now
description: Drive one frozen, projected plan through the pyxis-native local executor. By default, start the plan-scoped self-drive watcher, which polls durable tracker state and invokes bounded implementation sweeps as human merges unlock work; run one explicit scoped sweep only when the operator asks for once mode. Use when a plan needs building locally ("implement this plan", "run the executor", "/implement-now") or its self-driving execution needs resuming.
---

Follow `.pyxis/authoring/implement-now.md` — the vendor-neutral guide (AP-25:
this file is invocation glue only; all execution knowledge lives there).
Read that guide fully and execute its steps, including its Never list. The
one-sweep binary is `cmd/implement-now`; the default driver is `pyxis watch`.
The data contracts are `docs/tech-design/implement-now-routed-execution.md`
and `docs/tech-design/wake-watch-v2.md`.
