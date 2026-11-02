# Worker transcript metrics — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The solution shape for capturing per-process transcript metrics — where the capture seam sits, how a schema survives four structurally different harnesses, and whether this plane or the routed-evidence plane owns worker-run usage. |
| **Relevant-when** | designing or changing worker-run measurement capture — the record schema, the per-harness capture seam, in-flight derivation for a harness that persists nothing, the reconciliations against launch and required-figure sources; checking prior design rulings or refused shapes about where transcript capture attaches, or about the boundary against the routed-execution and routing-evidence planes |
| **Basis** | prd:worker-transcript-metrics *(merged ledger, PR #1035, merge commit 0c7b61b8; folded effective.md read)* |
| **Authority** | @mdarabi |
| **Date(s)** | 2026-08-05 |
| **Status** | gated: needs-more-product *(after gating, the record changes only through the introducing PR's review loop — rulings added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | — |
| **Feeds** | — |
| **Prior records** | brainstorm:worker-transcript-metrics · brainstorm:pipeline-measurement |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

*Sources read before the first question (routing surfaces first), and every overlap surfaced before exploration.*

| Source | Why loaded |
|---|---|
| docs/prds/worker-transcript-metrics/effective.md | the admitted basis — the merged, folded contract this design serves |
| docs/tech-design/index.md | routing surface; checked which live designs' `Serves` touch this basis — **none does** — and which touch its adjacents |
| docs/tech-design/implement-now-routed-execution.md | live; §3.4 already claims per-sublaunch usage capture and its §5 metrics name tokens/cost/turns — the Q1 collision |
| docs/tech-design/routing-evidence-v1.md | live; §2 makes a git evidence ref the authority for routed evidence and designates runtime journals as projections |
| docs/tech-design/pipeline-measurement-v2.md | live; the per-phase record and store this design sits beneath, and the source of the four shipped harness adapters |
| docs/brainstorms/worker-transcript-metrics.md | the gated stage-1 record — its D-1..D-9 rulings and P-1..P-6 parked constraints are this session's inherited inputs |
| docs/brainstorms/pipeline-measurement.md | the adjacent feature's stage-1 record — its P-1..P-7 constraints anticipate several of this design's questions |
| docs/reviews/worker-transcript-metrics-000.md | the basis's evidence package — its 18 overlay corrections record which contract properties are load-bearing and why |
| docs/architecture-principles.md | AP-1 (the hexagon), AP-6 (additive schema), AP-25 (authored once, wired per CLI), AP-29 (gate vs measurement) |
| docs/observability/incidents.md | I5 (a blocking gate must be hermetic), I8 (a fixture that cannot disagree certifies the opposite) |
| internal/phasemeter/** | built capability — the four shipped harness adapters, the spend port, the record schema and store this design must sit beside or extend |
| internal/executor/** | built capability — the runtime adapters, worker lifecycle, and event stream the capture seam would attach to |

**Overlaps surfaced (before exploration):**

- **No live design serves this basis.** The design index shows no `Serves` entry for `prd:worker-transcript-metrics`, so this is new design ground rather than a re-litigation of a decided question.
- **But two live designs already claim adjacent ground, and the basis knows it.** `implement-now-routed-execution` §3.4 states every sublaunch "captures actual launch/usage" and its attempt appends one terminal outcome "aggregating its sublaunch usage", with §5 naming tokens/cost/turns among its metrics; `routing-evidence-v1` §2 makes an immutable git evidence ref the **authority** for routed evidence and calls runtime journals "projections referencing canonical event IDs". The basis carries this as **Q1**, solution-blocking and unresolved.
- **The basis merged with that hole open.** Its §8 readiness row "Zero solution-blocking TBDs or open questions" is unchecked, and both gate sign-off tables (§2.6 problem, §4.4 solution) merged still `*(pending)*`. Merge froze the ledger in that state, so Q1 can now close only by typed amendment — it is not this session's to rule (guide step 8; DINV-8).
- **A shipped adapter set already reads three of the four harnesses.** `internal/phasemeter/adapter/{claudecode,codex,cursor,opencode}` implement a spend port over exactly the sources this design must read, including the empirical findings about which harness persists what. This is reusable prior art, not a competing claim: those adapters aggregate to a per-phase figure the basis explicitly is not (stage-1 F-3, F-5).

## Design questions

*The open solution-space questions in scope. Each carries a panel-log line.*

| # | Question | Status |
|---|---|---|
| DQ-1 | Given two live designs already claim per-process usage capture on an authoritative evidence ref, does this design own worker-run transcript capture, feed those planes, or stand down from part of it? | **closed — bounced to the contract (D-1)**; it is a requirement question, not a solution shape |
| DQ-2 | Where does the capture seam attach, given three harnesses persist a readable transcript and one must be consumed in flight? | **not opened** — the session gated before exploration; this is the frontier a resumed session starts from |
| DQ-3 | What shape lets one versioned record span four structurally different harnesses while keeping every figure measured-or-declared? | **not opened** — as DQ-2 |

## Panel log

*Per `DQ-n`: seam call · action · why. An unlogged call is invalid (DINV-7).*

| DQ | Seam call | Action | Why |
|---|---|---|---|
| DQ-1 | no seam | **skipped** | The question resolved as a *requirement* bounce, not a solution shape: the authority ruled the ownership boundary belongs in the contract (D-1), so no interface, port, document class, or module boundary was opened or replaced here and there was nothing to diverge alternatives over. Recorded per DINV-7 — a skipped call is still a logged call |
| DQ-2 | *(not judged)* | **not reached** | The session gated `needs-more-product` before this question opened, so no seam call was made. Logged rather than omitted, per DINV-7 |
| DQ-3 | *(not judged)* | **not reached** | As DQ-2 |

## Decisions

*One block per ruling. All fields mandatory; a ruling needs a live authority answer from this session.*

### D-1 — the ownership boundary is bounced to the contract, not ruled here

- **Question**: Given the routed plane ships only launch-fidelity capture (`LaunchCapture{NativeModel, NativeEffort, Controls}`, unpersisted — F-8, F-9) and its evidence ref does not exist (F-10), should this session rule the boundary as a design decision and let an amendment follow, bounce to `/prd-amend` first, or park Q1 as fog and design around it?
- **Answer**: **Bounce first.** Gate this session `needs-more-product` and close Q1 by typed amendment on `prd:worker-transcript-metrics`; the design brainstorm resumes at DQ-2 once the contract states its own boundary.
- **Reasoning**: The authority took the slower path knowing the overlap had narrowed to a factual, disjoint split — and accepted the recorded downside that this is arguably ceremony for a narrow question. What it buys: the contract states its boundary *before* a design rests on it, §8's failing readiness row closes, and the record carries no declared irregularity. The rejected alternative (rule here, amend after) would have had a design session decide requirement content, which is what DINV-8 exists to prevent; parking as fog would only defer the same decision to a gate that blocking fog forbids anyway.
- **Anchor**: 2026-08-05 · t2
- **Status**: live

## Alternatives

*Every candidate shape — panel-born or hand-authored.*

none — the one design question opened (DQ-1) resolved as a requirement bounce with no seam to diverge over, so no panel fired and no shape was scored. The solution space is unexplored by design, not by omission.

## Seam map

none — no seam was placed or moved this session. The seams this design will eventually own (the capture port, the record schema, the reconciliation boundary) are the resumed session's work, once the contract's own boundary is settled.

## Facts

*Looked up, never asked — each with its source.*

- **F-1**: No live design's `Serves` names `prd:worker-transcript-metrics` — this basis has no incumbent design — source: docs/tech-design/index.md
- **F-2**: `implement-now-routed-execution` (live) §3.4: every AgentRuntime sublaunch "captures actual launch/usage" and the attempt "appends its single terminal routed outcome … aggregating its sublaunch usage"; §5 names "tokens/cost/turns" among its metrics — source: docs/tech-design/implement-now-routed-execution.md
- **F-3**: `routing-evidence-v1` (live) §2: routing evidence is an immutable content-addressed event family on a dedicated git ref, and "Git is both transport and authority. Proposals, reports, tracker text, runtime journals, and local caches are projections referencing canonical event IDs" — source: docs/tech-design/routing-evidence-v1.md
- **F-4**: *(corrected 2026-08-05 against origin/main 727849b — an earlier reading of this record said "neither is implemented", which was true when the basis merged and is no longer true.)* **The native-routing half of `implement-now-routed-execution` is now implemented**: both routing plans are complete (001-plan T1–T4 #822–#825, 002-plan T1–T9 #926–#934 all closed, T4–T9 landing the same day the basis merged), shipping `internal/nativerouting` and `internal/executor/port/nativerouting.go` — source: origin/main tree; gh issue list
- **F-5**: The basis merged with Q1 solution-blocking and unresolved, its §8 readiness row unchecked, and both gate sign-off tables `*(pending)*` — source: docs/prds/worker-transcript-metrics/000-base.md §7.2, §8, §2.6, §4.4
- **F-6**: Four harness adapters already read the exact sources this design needs, and encode the empirical capability findings (Claude Code and Codex persist transcripts; OpenCode exposes `export`; Cursor persists nothing and must be consumed in flight) — source: internal/phasemeter/adapter/{claudecode,codex,cursor,opencode}
- **F-7**: Those adapters implement a *spend* port that aggregates to a per-phase per-model quartet — the granularity the basis explicitly is not — so they are reusable mechanism, not a competing record — source: internal/phasemeter/spend, internal/phasemeter/record/record.go
- **F-8**: What shipped as capture is **launch fidelity, not usage**: `port.LaunchCapture` carries exactly `NativeModel`, `NativeEffort` and `Controls` — no tokens, no duration, no turns, no tool events — and its purpose is proving a process launched with the profile the resolver selected — source: internal/executor/port/nativerouting.go
- **F-9**: `LaunchCapture` is adapter-level only — it appears in no worker, dispatch, or report code, so nothing persists it — source: origin/main tree
- **F-10**: `routing-evidence-v1` remains wholly unimplemented: no `routing_resolved`, `routing-evidence`, or `refs/evidence` code anywhere in `internal/` or `cmd/`; the git evidence ref its §2 calls "the authority" does not exist, and the routing PRD's FR-20/FR-21 are covered by no plan — source: origin/main tree; docs/plans/
- **F-11**: `AgentRuntime.TierMap` is still present in the executor port (3 references) despite `implement-now-routed-execution` §2 stating it "is removed" — the design leads the code on that seam too — source: internal/executor/port/agentruntime.go

## Fog

*Type is mandatory.*

none — the one blocking gap is a *basis* defect, so it is carried as P-1 (a parked product gap with a bounce disposition) rather than as fog. Q2/Q3/Q4 of the basis remain the basis's own open questions and are not re-parked here.

## Out of scope

none — no scope refusal was ruled this session.

## Candidate glossary terms

none — no term was coined or sharpened this session; the three terms this feature needed landed with the basis.

## Parked product gaps

*Basis defects found while designing — parked or bounced, never patched (DINV-8).*

- **P-1** *(disposition: **bounced** — the authority ruled `/prd-amend` closes it before designing resumes; cites D-1)*: **Q1 — the ownership boundary against the routed-execution and routing-evidence planes — is unresolved in the frozen basis.** The basis declares it solution-blocking (§7.2 Q1), leaves §8's readiness row unchecked, and merged with both gate sign-off tables pending, so the contract froze without the boundary its own gate says must close. Two live designs claim adjacent ground (F-2, F-3), though verification narrowed the collision sharply: what the routed plane actually ships is launch fidelity, carrying no field this contract also carries (F-8), unpersisted (F-9), and its evidence ref does not exist (F-10). The boundary is therefore a narrow factual split rather than a contest — but it still decides what the feature's requirements *are*, which is requirement content, and a frozen ledger changes only by typed amendment — basis anchors: `prd:worker-transcript-metrics` §7.2 Q1 · §8 readiness row · §2.6 · §4.4

## Gate

| Field | Value |
|---|---|
| **Verdict** | needs-more-product |
| **Date** | 2026-08-05 |
| **Authority** | @mdarabi |
| **Covering design** | — |
| **Coverage check** | — *(required only for `ready-for-design`)* |
| **Amend-route** | `/prd-amend` on **prd:worker-transcript-metrics** — close §7.2 Q1 by stating the ownership boundary against `implement-now-routed-execution` and `routing-evidence-v1`, which clears §8's failing readiness row. Evidence anchors: **P-1** (the gap, with its basis anchors) and **D-1** (the authority's bounce ruling). The amendment's `Learned-from` is `operator-decision:design-brainstorm:worker-transcript-metrics-tech-design#D-1`. This session resumes at DQ-2 once it merges |
