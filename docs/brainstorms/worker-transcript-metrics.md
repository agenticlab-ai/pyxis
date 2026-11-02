# Worker transcript metrics — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Go beneath the per-phase spend aggregate to per-turn, per-tool-use transcript signals — durations, token usage, context growth — as the substrate for pipeline, harness, and prompt optimization. |
| **Relevant-when** | brainstorming or contracting measurement below phase granularity — per-turn or per-tool-use signals, run/tool wall-clock duration, context-growth measurement, worker-run (as opposed to authoring-phase) attribution; checking prior rulings about what pipeline-measurement deliberately excluded, or about who owns worker-run usage capture |
| **Idea** | ok, as part of the measurement and observability, beside just capturing the harness information, we want to go deeper and store metrics on the worker transcripts as well. information such as total harness run duration, tool uses and their duration, each turn token usage, total token usage and every other metrics that can be captured so we can optimize the pipeline (e.g. how to make the planning to generate smaller amoutn of work), or harness optimization and prompting, context build up, etc |
| **Operator** | @mdarabi |
| **Date(s)** | 2026-08-05 |
| **Status** | gated: ready-for-prd *(after gating, the record changes only through the introducing PR's review loop — decisions added or superseded with review anchors — and freezes entirely at merge)* |
| **Tracking** | — |
| **Feeds** | docs/prds/worker-transcript-metrics/ |
| **Prior records** | brainstorm:pipeline-measurement |
| **Template version** | brainstorm-record/v1 |

## Orientation

*Sources read before the first question (routing surfaces first), and every
overlap surfaced — merged capability, prior refusals, prior records.*

| Source | Why loaded |
|---|---|
| docs/product-vision.md | routing surface; §7 capability row "Observability & spend", §10 "How we measure progress" — confirms the plane is in-vision |
| docs/prds/README.md · docs/prds/ ledger list | routing surface; located the owning area PRD and confirmed which feature ledgers exist |
| docs/prds/observability-and-spend.md | area PRD for this plane; OBS-1..OBS-6 parity corpus, OBS-2 is the closest existing requirement |
| docs/brainstorms/pipeline-measurement.md | prior record on the adjacent feature; its D-5 boundary, O-1..O-8 refusals, and T-1 term ruling bear directly on this idea |
| docs/prds/pipeline-measurement/effective.md | merged contract of the adjacent feature; NG rows |
| docs/tech-design/pipeline-measurement-v2.md | §3.6 the record schema + store shape that this would extend or sit beside |
| docs/tech-design/implement-now-routed-execution.md | frozen; §3.4 already assigns per-sublaunch usage capture to the routed plane — a live ownership overlap |
| docs/tech-design/routing-evidence-v1.md | frozen; §2 makes a git evidence ref the authority for routed attempt/outcome evidence |
| docs/domain-model.md §3.7 | entity catalog; TokenUsageRecord is the planned (unbuilt) per-turn entity |
| docs/wiki/glossary.md | term-collision check for "phase", "turn", "spend", "telemetry" |
| internal/phasemeter/** (record, adapter/claudecode) | built capability — what is actually captured today vs. designed |

**Overlaps surfaced:**

- **pipeline-measurement deliberately drew its line *above* this** — its `T-1` defines the *phase run record* as "per-phase, aggregated over the phase's turns" and explicitly names what to **avoid**: "*TokenUsageRecord* (OBS-2's per-turn record — this is per-phase…)". So per-turn granularity was not overlooked; it was ruled out of that feature by name (F-3).
- **…but parked it as a waiting room, not a permanent refusal** — its `O-6` puts "the rest of the OBS plane" (OBS-1, OBS-4, OBS-5, OBS-6) in the waiting room: "our own plane but not v1". OBS-2 is notably absent from both the O-6 waiting list and the permanent non-goals O-1..O-5 (F-4).
- **The requirement already exists, unbuilt** — OBS-2 requires "one record per session turn-end: pass vs worker attribution, end reason…, input/output/cache token quartet, **turn and tool-use counts**", and the domain model plans `TokenUsageRecord` for it (verdict Generalize). Both are unrealized in pyxis (F-1, F-2).
- **A live ownership collision on worker runs** — `implement-now-routed-execution` (frozen) §3.4 already assigns per-sublaunch usage capture to the routed plane, and its metrics line names "tokens/cost/turns"; `routing-evidence-v1` §2 makes a git evidence ref the *authority* for that evidence and calls runtime journals "projections" (F-8, F-9). Any worker-transcript measurement must settle whether it feeds, reads, or duplicates that plane.
- **The stated purpose is a permanent non-goal of the adjacent feature** — "optimize the pipeline / planning / prompting" is the optimizer loop, which pipeline-measurement's `O-2` permanently assigns elsewhere (F6/VNEXT-15). The idea's *purpose* is legitimate; it is the *consumer* of what this feature would produce, not this feature (F-7).
- **Not covered** — no merged feature realizes per-turn or per-tool-use capture, so this is not a duplicate-contract case.

## Framings

*2–3 genuinely distinct framings — different theses of what the feature is —
then the operator's pick or hybrid as the Destination.*

- **A · OBS-2 realized — the per-turn spend record.** The thesis: this is the known, unbuilt parity requirement finally built. One record per turn-end (token quartet, turn and tool-use *counts*, pass/worker attribution, end reason), with the existing per-phase record becoming a rollup over it. Deepens the axis pyxis already measures. Implications: smallest new surface, schema already sketched in the domain model, and it closes a named parity gap — but OBS-2 specifies *counts*, not durations, so run/tool timing and context growth arrive as extensions rather than as the point.
- **B · The transcript profiler — timing and shape as the new axis.** The thesis: tokens are already measured; what is missing is **time and shape** — wall-clock run duration, per-tool-use duration, latency distribution, and the context-growth curve. A profiler that answers "where did the time and the context go", not "where did the tokens go". Implications: this is the axis the stated optimization targets actually live on (prompting, harness choice, context buildup); it needs per-event timestamps, which reopens a per-harness capability matrix like the one pipeline-measurement had to resolve (P-1).
- **C · The universal transcript record — one normalized schema, any harness.** The thesis: the feature is the **normalization layer**, not any metric list: one versioned, harness-neutral transcript record behind an adapter port carrying every capturable signal (turns, tokens, tools, timings, context size, end reason), which consumers — phase rollup, routing, the future optimizer — read. Implications: the honest shape if the goal really is "every other metric that can be captured", and it makes new signals additive rather than a new system each time; but it is the largest scope and runs directly at the sprawl that pipeline-measurement's D-5 was written to prevent.

**Destination:** the **B-on-C hybrid** — *timing and shape* is the thesis (run and per-tool-use wall-clock duration, context growth), carried on **one versioned, harness-neutral transcript record** (C's discipline) so further signals are additive rather than a new system each time; OBS-2's per-turn token quartet and tool-use counts ride that same record as a subset rather than a competing record class. Operator adopted the recommendation (t1).

## Decisions

*One block per ruling. All five fields are mandatory (DINV-1); ≤ 6 lines each.*

### D-1 — the destination framing (B-on-C hybrid: timing-led, one record)

- **Question**: What is this feature fundamentally — OBS-2's per-turn spend record realized (A), a timing/shape profiler (B), a metric-agnostic normalization layer (C), or a hybrid?
- **Answer**: The **B-on-C hybrid**: the new value is *time and shape* — total run duration, per-tool-use duration, context growth — carried on one versioned harness-neutral transcript record, with OBS-2's token quartet and turn/tool-use counts falling out of that same record.
- **Reasoning**: Tokens at phase granularity already exist and per-turn material is already parsed then discarded (F-5), so A alone adds the least new information and its count-only fields are not the axis the stated optimization targets live on. The targets named — planning producing smaller units of work, prompting, harness choice, context buildup — are predominantly time-and-shape questions (B). C's one-record discipline is the guard against the open-ended "every other metric" appetite sprawling, which is exactly what the adjacent feature's D-5 was written to prevent (F-3, F-4): new signals become additive schema, not a new system.
- **Anchor**: 2026-08-05 · t1
- **Status**: live

### D-2 — the measured unit and the two required levels

- **Question**: What exactly is measured, and at what granularity — what is the thing a record describes?
- **Answer**: The unit is a **harness run**: one worker executing one task in one role — **implementation or review** — e.g. "implement task ABC". Within a run the harness's model, effort and configuration are **constant**. **Both** levels are required: the **aggregate** for the run (total token usage, total execution time) *and* the **breakdown** beneath it (per tool use: how long it took, what token usage it drove). The breakdown exists specifically to improve the harness.
- **Reasoning**: Operator's framing (t2). The aggregate answers "what did this run cost and how long did it take"; the breakdown answers "where inside the run did the time and the context go" — the latter being the signal harness and prompt optimization actually need, and the thing nothing in the corpus captures today (F-6). Role (implementation vs review) is a property of the run because a worker is bound to one harness in one role.
- **Anchor**: 2026-08-05 · t2
- **Status**: live

### D-3 — what one stored record describes (per-process **and** a task rollup)

- **Question**: One process emits one transcript. Should one stored record describe one process, the whole task, or both?
- **Answer**: **Both.** A task produces one record per process *plus* one stored task-level rollup — for the worked example, four records for task ABC. The operator **overrode** the recommended per-process-only shape (where the task total would be summed at read time).
- **Reasoning**: Both reads are then direct and need no arithmetic — "how good is the harness at this job" reads a single process record, "what did this task cost" reads the rollup. The operator accepted the restatement risk surfaced with the option (a stored rollup repeats figures its parts already carry). That risk is not dismissed: it is carried to the design stage as P-1.
- **Anchor**: 2026-08-05 · t3
- **Status**: live

### D-4 — capture maximally for machines; presentation is a later feature; the schema is the only constraint

- **Question**: Who is the primary reader, and what bounds which metrics are captured?
- **Answer**: **Capture everything capturable, in as much detail as possible, without regard to how it will be used.** The primary consumer is a **machine processing records at scale**; a presentation for human users is explicitly a **later, separate feature**, not this one. The single hard constraint is that the output **must have a schema** so machines can process it at scale without issue.
- **Reasoning**: Operator's ruling (t4). Deciding what to capture from anticipated use would discard signals whose value is not yet known — the point of the substrate is that the analysis has not been invented yet. Bounding therefore moves off the metric list and onto the *structure*: a closed, versioned schema is what makes maximal capture safe to consume. This **broadens D-1 rather than contradicting it** — D-1's reasoning already named the one-record discipline as the guard that lets the "every other metric" appetite be additive rather than a new system per signal; this ruling confirms that guard is sufficient and declines to bound the metric set itself.
- **Anchor**: 2026-08-05 · t4
- **Status**: live

### D-5 — v1 covers all four supported harnesses, including the ephemeral one

- **Question**: Which harnesses must v1 capture, given that three are readable after the run but Cursor CLI persists nothing and can only be captured at launch?
- **Answer**: **All four** — Claude Code, Codex, OpenCode **and** Cursor CLI. This knowingly accepts a change on the executor's launch path to redirect and retain Cursor's stream, and with it a **deliberate departure** from the adjacent feature's `O-3` posture that measurement stays execution-agnostic.
- **Reasoning**: The schema *is* the deliverable (D-4), and a schema that has never met the ephemeral/streaming case is not demonstrably harness-neutral — it is only demonstrably file-shaped. Meeting that case after the schema freezes is expensive under additive-only evolution (AP-6), so the hard source belongs in v1 where it can still constrain the design. Consistent with the operator's own precedent on the adjacent feature, where a "one harness plus a seam" recommendation was overridden in favour of concrete four-harness coverage (F-4 context; pipeline-measurement D-6).
- **Anchor**: 2026-08-05 · t5
- **Status**: live

### D-6 — the bet and its observable success (complete, honest capture)

- **Question**: What is the bet, and how would you know v1 actually worked?
- **Answer**: **Complete, honest capture.** For every run, on every supported harness, a **schema-valid record exists**, and **every figure is either measured or explicitly declared unavailable** — never a silent zero, never a silently omitted field. The observable check is a real multi-harness batch of runs producing one record each with zero silent gaps: where a harness cannot expose something, the record *says so*.
- **Reasoning**: This is the property the feature itself controls. Judging v1 by whether the data produced an optimization would make it answer for analyses it explicitly does not own (O-3) and could fail it for a downstream feature that was never built. The bar also inherits an already-proven in-repo discipline — phasemeter's honest-degradation floor and its silent-zero guard (F-5) — rather than inventing a new standard. The operator accepted that this bar says nothing about the data being *useful*; usefulness is the later consumer's to demonstrate.
- **Anchor**: 2026-08-05 · t6
- **Status**: live

### D-7 — the primary jobs (cost attribution · in-run breakdown · rework exposure)

- **Question**: What are the primary jobs this data does — what does a consumer come to it to find out?
- **Answer**: Three. **J1** — attribute a run's total cost and duration to a task (the aggregate: total tokens, total execution time per harness run). **J2** — expose *where inside a run* the time, tokens and context went, per tool use with its duration and the tokens it drove, so the harness and its prompts can be improved. **J3** — expose rework across a task's processes (that a ci-fix or review-fix process happened at all, and what it cost) so planning slice size can be judged.
- **Reasoning**: These are the operator's own stated purposes, put back as jobs and confirmed (t7). J1 is the aggregate half of D-2 and J2 the breakdown half; J3 is what makes the D-3 task rollup worth storing separately — rework is only visible across a task's processes, never inside one. Note each job is a *question the data answers*, not an analysis this feature performs (O-3).
- **Anchor**: 2026-08-05 · t7
- **Status**: live

### D-8 — the anti-segments

- **Question**: Who does this deliberately *not* serve?
- **Answer**: Two. **The human wanting a readable view** — actively anti, not merely deferred: where a design choice trades machine-processability for human readability, this feature refuses it; readability is the later presentation feature's problem (O-1). **The built application's runtime** — observable-by-construction's disjoint plane; this measures the factory's workers, not the software they build.
- **Reasoning**: Confirmed (t7). The first anti-segment is the one doing real work — it is what stops "capture everything at maximum detail" (D-4) quietly degrading into "capture what looks good in a table", which is the failure mode a maximal-capture substrate is most likely to suffer. The second carries forward the adjacent feature's own plane boundary (its O-5) rather than reopening a settled split.
- **Anchor**: 2026-08-05 · t7
- **Status**: live

### D-9 — "everything" means measurements and identifiers, never free-text content

- **Question**: Does D-4's "capture everything capturable" include transcript **content** — prompt text, tool arguments, tool output, code — or only measurements about it? *(A genuine gap this record had not decided or parked; surfaced at PRD synthesis and put to the operator once rather than filled in.)*
- **Answer**: **Metrics plus structural identifiers, never free text.** Captured: every measurement, plus identifiers such as tool name, exit status and target path. **Not** captured: command strings, tool output bodies, prompt text, or written code.
- **Reasoning**: This preserves J2 — "the Bash call at turn 7 took 4.2s and put 48KB into context" is answerable — while a measurements-only variant would gut it (a spike with no attributable operation says nothing about *what* to improve). Capturing content would make this prompt logging, which the adjacent record parked in its waiting room rather than build (its O-6), would make secret redaction a hard requirement, would raise the contract to T3 on privacy surface, and would multiply volume against P-4. The operator accepted the cost: the records cannot reconstruct *why* a call was made.
- **Anchor**: 2026-08-05 · t9
- **Status**: live

## Facts

*Looked up, never asked — each with its source.*

- **F-1**: OBS-2 requires "one record per session turn-end: pass vs worker attribution, end reason (clean stop vs killed), input/output/cache token quartet, turn and tool-use counts; captured mechanically by a session-end hook" — a harbormaster-parity requirement, unbuilt in pyxis — source: docs/prds/observability-and-spend.md OBS-2
- **F-2**: The domain model plans `TokenUsageRecord` — "Per-turn spend: kind pass/worker, end stop/killed, token quartet, turns, tool_uses. Additive-schema discipline", origin `token-usage.jsonl` (Stop hook), verdict **Generalize** — planned, not built — source: docs/domain-model.md §3.7
- **F-3**: pipeline-measurement's `T-1` fixes its record as "per-phase, aggregated over the phase's turns" and names *TokenUsageRecord* (per-turn) as a term to **avoid** — per-turn was excluded by name, not by oversight — source: docs/brainstorms/pipeline-measurement.md T-1
- **F-4**: That record's `O-6` waiting room covers OBS-1, OBS-4, OBS-5, OBS-6; OBS-2 appears in neither the waiting room nor the permanent non-goals O-1..O-5 — source: docs/brainstorms/pipeline-measurement.md O-1..O-8
- **F-5**: `internal/phasemeter/adapter/claudecode` already parses per-turn model + usage (`turnUsage`) and then **aggregates it away** into one per-phase per-model quartet — the per-turn material is already read today and discarded by the schema — source: internal/phasemeter/adapter/claudecode/claudecode.go; internal/phasemeter/record/record.go
- **F-6**: The only duration captured today is `Efficiency.LatencySeconds`, derived from **git timestamps**, not harness wall-clock; no tool-use capture of any kind exists — source: internal/phasemeter/record/record.go; internal/phasemeter/efficiency/latency.go
- **F-7**: The optimizer / self-improvement loop is a **permanent** non-goal of pipeline-measurement (O-2 / PRD NG-2), assigned to F6/VNEXT-15 — the stated purpose here is that loop's input — source: docs/brainstorms/pipeline-measurement.md O-2; docs/prds/pipeline-measurement/effective.md NG-2
- **F-8**: `implement-now-routed-execution` (frozen, merged #811) §3.4 already assigns per-sublaunch usage capture — "captures actual launch/usage", attempt "aggregating its sublaunch usage" — and its metrics list names "tokens/cost/turns" — source: docs/tech-design/implement-now-routed-execution.md §3.4, §5
- **F-9**: `routing-evidence-v1` §2 makes a git evidence ref the **authority** for routed evidence and designates "runtime journals" as projections referencing canonical event IDs — source: docs/tech-design/routing-evidence-v1.md §2
- **F-10**: Neither routing-evidence-v1 nor its launch-capture upstream is implemented: no `routing_resolved` / `routing-evidence` / `refs/evidence` code exists, and no plan covers the routing PRD's FR-20/FR-21; the launch-capture tasks (002-plan T4–T7, issues #929–#932) are open — source: origin/main tree; docs/plans/planner-driven-model-routing/; gh issue list
- **F-11**: The product vision carries "Observability & spend — first-class events, traces, and spend attribution" as a capability and §10 computes progress "from the observability plane", so this idea sits inside the vision's scope — source: docs/product-vision.md §7, §10
- **F-12**: The frozen routed design **corroborates** the operator's "config stays the same within a run": one admitted native profile owns an attempt, and "every fresh AgentRuntime sublaunch under that owner … uses the same admitted native profile and launch controls" — source: docs/tech-design/implement-now-routed-execution.md §3.4
- **F-13**: That design distinguishes two nested units by name — the **attempt** (owns the claim and "the whole bounded Worker lifecycle through final review: initial implementation, CI self-heal, and review-finding remediation") and the **sublaunch** (each fresh AgentRuntime launch under it, with a distinct `sublaunch_id`) — so "one task's run" and "one process's run" are different spans in the live vocabulary — source: docs/tech-design/implement-now-routed-execution.md §3.4
- **F-14**: Role is already a first-class routed concept — the composition "resolves `role=implementation`" and the impl-review reviewer is launched under its own route — matching the operator's implementation-or-review split — source: docs/tech-design/implement-now-routed-execution.md §2, §3.1
- **F-15**: **Claude Code and Codex persist their own transcripts on disk, unprompted** — `~/.claude/projects/<project>/<session-id>.jsonl` and `~/.codex/sessions/<yyyy>/<mm>/<dd>/rollout-<ts>-<id>.jsonl` — so a reader can capture them *after* the run — source: internal/phasemeter/adapter/claudecode/claudecode.go; internal/phasemeter/adapter/codex/codex.go
- **F-16**: **Cursor CLI persists no discoverable transcript at all** (confirmed empirically: `~/.cursor/projects/<project>` holds only MCP-server cache metadata, never a chat/session record). Its accounting record is the *live* NDJSON stream from `--print --output-format stream-json`, which **the caller must redirect to a file at launch time** or the data is gone permanently. That stream does carry a tool-call taxonomy (`readToolCall` / `writeToolCall` / `editToolCall` / `shellToolCall` / `listToolCall` / `searchToolCall`), so tool-level detail is obtainable *if* captured — source: internal/phasemeter/adapter/cursor/cursor.go
- **F-17**: F-15 and F-16 together mean maximal capture is **not** uniformly an after-the-fact read: for at least one supported harness it requires arranging capture **at launch**, which is a change on the executor's launch path, not a pure reader — source: derived from F-15, F-16

## Fog

*Type is mandatory.*

- **G-1** (non-blocking — blocks: the PRD's non-goal/boundary section, not the destination): **The boundary against the frozen routed-execution plane is unruled.** That design already claims per-sublaunch usage capture and names "tokens/cost/turns" among its metrics (F-8), while `routing-evidence-v1` makes a git evidence ref the authority for routed evidence and calls runtime journals "projections referencing canonical event IDs" (F-9). Under D-4 this feature captures those figures anyway, so the two planes will hold overlapping numbers. Whether the routed plane *references* these records, *duplicates* them, or *supersedes* part of this capture was put to the operator twice and left unruled; it does not block the destination (which is settled) but the PRD must resolve it as a boundary/non-goal row. Neither plane is implemented (F-10), so nothing is broken today.

## Out of scope

*Operator refusals with durable reasons — the seed of the PRD's NG/WR rows.*

- **O-1**: **Any presentation / view / dashboard for human users** — explicitly a later, separate feature; this one captures and stores, it does not display (from D-4)
- **O-2**: **Deciding which metrics are worth capturing from anticipated use** — refused as a selection principle: capture is maximal and use-agnostic, because discarding a signal whose value is not yet known is the failure this substrate exists to avoid (from D-4)
- **O-3**: **The optimizer / self-improvement loop itself** — the analyses this data is *for* (planning slice size, harness and prompt tuning) consume these records; building the adaptation loop is a separate feature, already assigned by the adjacent contract to F6/VNEXT-15 (from D-4's purpose framing; boundary inherited from pipeline-measurement O-2 / PRD NG-2, F-7)

## Candidate glossary terms

*Terms born or sharpened this session, for the domain glossary.*

- **T-1**: **harness run** — one worker executing one task in one role (implementation or review) on one harness, within which model, effort and configuration are constant (D-2) · avoid: *session* (a harness's own notion, which may not align), *phase* (pipeline-measurement's authoring-stage bracket — a different span entirely, F-3)
- **T-2**: **process record** — the record describing a single launched agent process, one per transcript, written when that process exits (D-3) · avoid: *sublaunch record* (the routed design's word for the same span — keep the term but do not assume its plane owns the record, G-1), *turn record* (a turn is a level *inside* this record, not the record)
- **T-3**: **task rollup** — the stored record aggregating a task's process records into one total (D-3) · avoid: *attempt record* (the routed design's span name, same caveat as T-2), *phase run record* (pipeline-measurement's per-phase record — a different unit, F-3)

## Parked technical constraints

*Surfaced technical matter conveyed to stages 3–4, never resolved here.*

- **P-1**: **Rollup ↔ parts consistency.** D-3 stores a task-level rollup whose figures the per-process records already carry, so the design must decide how divergence is prevented or detected — and must not let the rollup become a figure that is *recorded but never measured*. The in-repo precedent to follow or deliberately depart from is phasemeter's reconciliation tiers (exact / harness-precision / unavailable) and its silent-zero guard — context: D-3, F-5
- **P-2**: **Attributing token usage to a tool use.** D-2 requires per-tool-use token figures, but tokens are consumed per model turn, not per tool call. The design must fix what the figure means — e.g. the tokens a tool's *result* injected into context (a large file read or command output), versus an apportionment of the enclosing turn — and say so explicitly rather than leave a number whose meaning is ambiguous — context: D-2
- **P-3**: **Per-harness capability matrix for timing and tool events.** Durations and per-tool events require timestamped transcript events; whether each of the four supported harnesses (Claude Code, Codex, Cursor CLI, OpenCode) exposes them, and by what channel, is a design input — the same shape of unknown that pipeline-measurement had to resolve as its own P-1, and where its honest-degradation floor (declare unavailable, never a silent zero) would apply — context: D-1, D-2, F-6
- **P-4**: **Volume and retention under maximal capture.** D-4 takes every capturable signal at full detail for every process of every task; "machine-processable at scale" therefore has to survive the volume that produces. Storage form, retention, compaction and whether records are ever pruned are design calls this record does not make — context: D-4
- **P-5**: **What "has a schema, processable at scale" concretely obliges.** At minimum a closed, versioned schema with stable field semantics and additive-only evolution (AP-6; the repo's Go rules forbid `map[string]any` cross-boundary contracts and require config/policy parsed closed) — and a form a consumer can process incrementally rather than by loading a whole run into memory. The design fixes the concrete shape — context: D-4
- **P-6**: **Launch-time capture for harnesses that persist nothing.** Per F-16/F-17, at least Cursor CLI requires its stream be redirected at launch or the data is unrecoverable — so the capture mechanism is not uniformly an after-the-fact reader and may touch the executor's launch path. How that is arranged without coupling measurement to execution policy is a design call, and D-5 makes it mandatory rather than optional — context: F-16, F-17, D-2, D-5

## Gate

*Exactly one entry, ever (DINV-4).*

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-08-05 |
| **Operator** | @mdarabi |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered (§2.1 D-1 · §2.2 D-4 + D-8 · §2.3 D-7 · §3.1 D-6 · §3.4 O-1..O-3 · appetite D-5), zero blocking fog (G-1 is non-blocking) |
