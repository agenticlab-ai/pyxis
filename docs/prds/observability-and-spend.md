# PRD — Observability & Spend

**Status: draft v0.1 (baseline inventory — as built in harbormaster) · 2026-07-01**
Part of the [Pyxis PRD set](README.md); serves the [product vision](../product-vision.md). Northstar category: C8 (observability — a
first-class plane, per AP-5). Spans every pipeline stage.

## Scope

Seeing what happened — in the process (state transitions, health), in the spend
(tokens, per pass/worker), and in the operator surfaces that make an unattended
fleet trustworthy. Users: the **operator** (monitoring, audits), the **factory**
(self-diagnosis, alarms).

## Features

### OBS-1 — Domain event stream
**What:** Every state transition is one structured, append-only event.
**Requirements (as built):**
- One JSON record per transition (~60 kinds in stable families: gate decisions, lease lifecycle, merges, recovery loops, health, scheduling, maintenance); every kind documented in an operator reference (what fires it, key fields, required action).
- Additive schema evolution — readers default missing fields, old rows never break.
- Bounded retention (rotation + short archive) — observability never grows unbounded.
**Origin:** events.jsonl; events-reference.md.

### OBS-2 — Token-spend records
**What:** Every unit of intelligence spend is attributed.
**Requirements (as built):**
- One record per session turn-end: pass vs worker attribution, end reason (clean stop vs killed), input/output/cache token quartet, turn and tool-use counts; captured mechanically by a session-end hook.
**Origin:** token-usage.jsonl; supervisor-spend-attribution.md.

### OBS-3 — Pass reports (the audit row)
**What:** Each orchestrator pass leaves one durable, structured audit record.
**Requirements (as built):**
- Per pass: spawned / succeeded / hollow / failed / skipped (with reasons) + budget; `succeeded` is **artifact-verified** (the shipped proposal was confirmed to exist) — the row spend/reconciliation tooling aggregates.
**Origin:** pass_report (#475).

### OBS-4 — Health alarms & escalation
**What:** Silent failure modes become loud, deduplicated operator signals.
**Requirements (as built):**
- Detectors over the event history: work starvation (ready-but-unclaimed streaks), triage stuck, claim churn (acquire-without-fire cycles), consecutive gate failures, runaway worker turns, whole-fleet silence.
- Alarms escalate as deduped operator issues; recovered conditions clear.
**Origin:** doctor checks; fleet-hardening-system1.md.

### OBS-5 — Live monitoring surfaces
**What:** The operator can watch the fleet in real time without SSH archaeology.
**Requirements (as built):**
- In-container dashboard (pool/watchdog state), host-native log tailing, and the read-only fleet status command (per-member idle-vs-working spend, active leases, flagged proposals — FLEET-4).
- Observability files live host-bound (bind-mounted out of containers) so standard tools work on them.
**Origin:** node-container.sh monitor/logs; rig-status.

### OBS-6 — Prompt logging & provenance *(partial)*
**What:** The process side of observability — what humans asked, with provenance.
**Requirements (as built):**
- Session prompts logged locally with source provenance and secret redaction, never git-tracked (a data invariant); a periodic review digests them — the substrate for the northstar metrics (intervention tracking is the designed next step).
- Status: shipped as governance v1 pilot; fleet-wide migration was still in flight in harbormaster.
**Origin:** factory-governance.md §7; prompt-log.sh.

## Inherited metrics (the point of this plane)

From the northstar — computable from these streams once intervention tracking
lands: **zero-touch rate** (primary), **interventions per feature** (the
factory's backlog-priority function), **design-share of engineer time**, with
guardrails (bypass rate, drift MTTR, fleet compliance %, cost per shipped
feature). *Pyxis elevates this plane further: contractual data + end-to-end
trace context are architecture principles (AP-5, AP-6).*
