# Prototype an idea before its contract — design-brainstorm record

| Field | Value |
|---|---|
| **In one line** | The prototype step is a skill the asking session follows, a command that owns the prototype's worktree, and a findings record among the draft's brainstorm files. |
| **Relevant-when** | Designing or changing how a prototype is built, where its findings live, or how its worktree is created and removed; checking refused shapes for a prototype worker or a new draft working-file class. |
| **Basis** | draft:prototype/prd.md |
| **Authority** | Argus as stage-3 decision authority for the operator (@mdarabi) · authority-contract/v1, tier-2 |
| **Date(s)** | 2026-10-10 |
| **Status** | gated: ready-for-design |
| **Tracking** | — |
| **Feeds** | `drafts/prototype/tech-spec.md` |
| **Prior records** | brainstorm:prototype |
| **Template version** | design-brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `drafts/prototype/prd.md` | The requirement basis. |
| `drafts/prototype/brainstorm/001.md` | Its fog items G-1 and G-2 and parked constraints are this session's inputs. |
| `internal/draft/compile/document.go` | Which working files a draft may hold and how each compiles. |
| `internal/draft/lifecycle/open.go` | How the existing draft verb creates a branch from mainline. |
| `internal/executor/workspace/workspace.go` | Where the engine already places worktrees. |
| `.pyxis/authoring/implement-now.md` | The worker cycle a prototype would have to bypass. |
| `docs/architecture-principles.md` | AP-6, AP-10, AP-12, AP-13, AP-17, AP-20, AP-25, AP-27, AP-31. |
| `docs/observability/incidents.md` | I13, on closing gaps by adding mechanisms. |

**Overlaps surfaced:** none. No live design decides how to build before a contract.

## Design questions

- **DQ-1**: Who builds the prototype: the session that asked for it, or a worker the engine launches?
- **DQ-2**: Where in the feature's draft do the findings live?
- **DQ-3**: What creates and removes the prototype's worktree and branch, and how long are they kept?
- **DQ-4**: How does a Pyxis command choose and start the harness and model that build the prototype?

## Panel log

- **DQ-1**: opens/replaces — fired — a launched worker would open a new executor mode; three shapes under distinct constraints, generated in this session under the sequential fallback.
- **DQ-2**: opens/replaces — fired — a dedicated file class would open a new draft document class; three shapes, sequential fallback.
- **DQ-3**: extends — skipped — it adds one verb to the existing draft verb set; two hand-authored shapes recorded.
- **DQ-4**: opens/replaces — forced — the operator's rulings in the lock review fixed the direction (a command launches a headless run; configuration first, else the session's own harness and model; not implement-now); two shapes recorded under it, sequential fallback.

## Decisions

### D-1 — Who builds the prototype?

- **Question**: Does the asking session build it, or does the engine launch a worker for it?
- **Answer**: The asking session builds it, following a prototype guide exposed as a skill (ALT-1).
- **Reasoning**: It serves FR-1 and FR-4 with no engine change and runs on every supported agent CLI. A launched worker needs an executor mode that stops before the gate, review, and proposal; nothing at today's scale asks for that (AP-31), and it waits as WR-3.
- **Anchor**: 2026-10-10 · Argus assumption (delivery:prototype#E-10)
- **Status**: superseded by D-4 — the operator ruled that a Pyxis command launches the build

### D-2 — Where do the findings live?

- **Question**: Which working file holds a prototype's findings?
- **Answer**: A numbered file among the draft's brainstorm working files, written from a new prototype-record template (ALT-4).
- **Reasoning**: The compiler already captures every numbered brainstorm file at lock, and the existing brainstorm citation grammar already resolves anchors in them, so FR-3 and FR-5 need no compiler or readiness change.
- **Anchor**: 2026-10-10 · Argus assumption (delivery:prototype#E-10)
- **Status**: live

### D-3 — What owns the prototype's worktree?

- **Question**: Who creates and removes the worktree and branch, and when?
- **Answer**: One draft verb creates them from mainline and removes them. They stay on the local machine, unpublished, until the feature's draft locks or is abandoned (ALT-7).
- **Reasoning**: Creating a worktree is mechanical, so it is code, and code makes FR-2 and UB-1 testable (AP-10). The provider may want to look at the prototype while the contract is written, so it outlives the findings record.
- **Anchor**: 2026-10-10 · Argus assumption (delivery:prototype#E-10)
- **Status**: live

### D-4 — How is the build chosen and started?

- **Question**: What decides the harness and model that build a prototype, and what starts the build?
- **Answer**: The draft verb always launches the build as one headless process through the executor's harness runtime, under a prototype launch purpose. It takes the harness and model profile from a `prototype` config block when present, and otherwise from the session's own harness and model, matched to a catalog profile (ALT-9).
- **Reasoning**: The operator ruled the direction (problem:prototype#A-2, problem:prototype#A-3). Reusing the runtime port and the harness catalog adds one launch purpose and one config block, and no adapter code.
- **Anchor**: 2026-10-10 · review — operator rulings; the catalog-matching rule and the time bound are Argus assumptions (delivery:prototype#E-21)
- **Status**: live

## Alternatives

### ALT-1 — The asking session builds it

- **Shape**: A vendor-neutral guide and a skill pointer. The session that was asked creates the worktree, builds in it, and writes the findings.
- **Trade-offs**: No engine change; works on every agent CLI. The build spends the asking session's context, and its size is bounded by the guide, not by a meter.
- **Principles**: AP-25 honored (knowledge in a vendor-neutral guide); AP-31 honored (no mechanism nobody asked for).
- **Incidents**: I13 — matched: the shape that adds least.
- **Verdict**: chosen — serves FR-1 and FR-4 at today's scale. 2026-10-10 · review: superseded by D-4; ALT-9 is chosen.
- **From**: DQ-1 · panel

### ALT-2 — The engine launches a prototype worker

- **Shape**: A command composes the executor's workspace and agent runtime, launches a worker with a prototype charter and a budget, and collects its findings.
- **Trade-offs**: Isolates the build's context and enforces a budget. Needs a worker mode without gate, review, or proposal, about three tasks.
- **Principles**: AP-20 honored; AP-31 strained (isolation and a budget nobody asked for).
- **Incidents**: I13 — matched: a mechanism added on the author's judgment.
- **Verdict**: rejected — waits as WR-3 until a prototype does not fit one session. 2026-10-10 · review: hybridized into ALT-9, without a budget meter or requirement-based routing.
- **From**: DQ-1 · panel

### ALT-3 — Reuse the direct issue path with publication off

- **Shape**: File a tracker issue for the prototype and run the existing one-issue implementation with a flag that stops before publishing.
- **Trade-offs**: Reuses isolation and routing. Requires a tracker issue and a task capsule for throwaway work, and bends a path whose contract is a verified proposal.
- **Principles**: AP-13 strained (that path reports success on a verified proposal, and a run that stops before publishing has none).
- **Incidents**: none checked beyond I13 in `docs/observability/incidents.md`.
- **Verdict**: rejected — it changes an existing contract to save little.
- **From**: DQ-1 · panel

### ALT-4 — A numbered brainstorm working file

- **Shape**: The findings record is the next numbered file under the draft's brainstorm directory, from its own template, with its own anchor family.
- **Trade-offs**: No compiler, readiness, or memory change. The file shares a directory with two other record classes; its header names its class.
- **Principles**: AP-27 honored (extension by addition); AP-31 honored.
- **Incidents**: I13 — matched: no new gate.
- **Verdict**: chosen.
- **From**: DQ-2 · panel

### ALT-5 — A dedicated prototype file class

- **Shape**: A new directory of numbered prototype files in the draft, with its own citation stem and memory subject.
- **Trade-offs**: Cleaner listing. Changes the compiler's closed file set, numbering, link resolution, and the memory subject space.
- **Principles**: AP-31 strained.
- **Incidents**: I13 — matched.
- **Verdict**: rejected — cost without a served row.
- **From**: DQ-2 · panel

### ALT-6 — Findings written straight into the problems file

- **Shape**: Each finding becomes a requirement problem.
- **Trade-offs**: Reuses the ruling flow. A problem names an existing or reserved requirement, and a technical challenge found before the PRD has none.
- **Principles**: AP-6 strained (a record used outside its contract).
- **Incidents**: none checked beyond I13 in `docs/observability/incidents.md`.
- **Verdict**: rejected — a finding may still be answered by a ruled problem, which FR-5 allows.
- **From**: DQ-2 · panel

### ALT-7 — A draft verb owns the worktree

- **Shape**: One verb creates the prototype's branch and worktree from mainline, and the same verb with a remove flag deletes them.
- **Trade-offs**: Deterministic and testable; one more verb to maintain.
- **Principles**: AP-10 honored; AP-12 honored by explicit release; AP-17 honored (removal needs positive evidence); AP-20 honored.
- **Incidents**: none checked beyond I13 in `docs/observability/incidents.md`.
- **Verdict**: chosen.
- **From**: DQ-3 · hand-authored

### ALT-8 — The guide tells the session to run git itself

- **Shape**: The guide lists the git commands to create and remove the worktree.
- **Trade-offs**: No code. Branch names, the base, and cleanup drift between sessions, and FR-2 and UB-1 cannot be tested.
- **Principles**: AP-10 strained (a mechanical rule left in a prompt).
- **Incidents**: none checked beyond I13 in `docs/observability/incidents.md`.
- **Verdict**: rejected.
- **From**: DQ-3 · hand-authored

### ALT-9 — The verb launches the build through the harness runtime

- **Shape**: The draft verb resolves a harness and a catalog profile, from a `prototype` config block or else from the session's own harness and model, and launches one process through the executor's runtime port under a new prototype launch purpose with its own brief. It waits under the engine's worker time bound and commits the result.
- **Trade-offs**: One path for every prototype; the driver's context is not spent on the build. Adds a launch purpose, a config block, and a rule for matching a session's model to a catalog profile.
- **Principles**: AP-1 honored (through the port and registered adapters); AP-30 honored (harness and profile from config); AP-31 honored (the operator asked for it).
- **Incidents**: I13 — matched: only what the ruling asks is added; no budget meter and no routing by requirements.
- **Verdict**: chosen — by D-4.
- **From**: DQ-4 · panel

### ALT-10 — With nothing configured, the asking session builds

- **Shape**: The verb launches a process only when a `prototype` config block exists; otherwise it reports an ambient route and the asking session builds the prototype itself.
- **Trade-offs**: Needs no way to learn the session's model. Two paths to guide and test, and the driver's context is spent on the unconfigured build.
- **Principles**: AP-31 honored; AP-10 strained (the unconfigured path leaves the build's rules in a prompt).
- **Incidents**: none checked beyond I13 in `docs/observability/incidents.md`.
- **Verdict**: rejected — the operator chose a headless run in both cases (problem:prototype#A-3).
- **From**: DQ-4 · panel

## Seam map

- **S-1**: the draft verb set — extends — behind it: creating and removing the prototype's branch and worktree — feeds: the tech spec's worktree decision, serving FR-2, UB-1, NR-1.
- **S-2**: the agent surface (skills and authoring guides) — extends — behind it: the prototype guide and the lines other guides gain — feeds: the build and answering decisions, serving FR-1, FR-4, FR-5.
- **S-3**: the document templates — extends — behind it: the prototype-record template — feeds: the findings decision, serving FR-3.
- **S-4**: the draft's brainstorm working files — unchanged — behind it: capture at lock and citation resolution — feeds: NR-2.
- **S-5**: the executor's harness runtime port — extends — behind it: a prototype launch purpose and its brief protocol — feeds: the launched-build decision, serving FR-6.
- **S-6**: the repository config schema — extends — behind it: the `prototype` block naming a harness and a catalog profile — feeds: the route decision, serving FR-6.

## Facts

- **F-1**: The draft compiler admits numbered files under the brainstorm and reviews directories and captures each brainstorm file as a literal document at lock — source: `internal/draft/compile/document.go`.
- **F-2**: Draft readiness resolves a brainstorm citation by finding its anchor in any of the feature's brainstorm documents — source: `internal/draft/readiness/links.go`.
- **F-3**: The executor places its worktrees in a sibling directory of the parent checkout — source: `internal/executor/workspace/workspace.go`.
- **F-4**: A branch created from a remote-tracking ref records that ref as its upstream unless tracking is declined — source: observed on `draft/prototype` after `pyxis draft open` (delivery:prototype#E-2).
- **F-5**: A new throwaway resource type ships with the means to release it — source: `docs/architecture-principles.md` AP-12.
- **F-6**: Every harness adapter assembles its brief through one shared function and branches only on the review purpose — source: `internal/executor/runtime/brief.go`; the four adapters under `internal/adapters/harness/`.
- **F-7**: A harness launch carries a model and an effort, and the routing layer accepts only pairs the harness catalog holds — source: `internal/nativerouting/profile.go`; `internal/executor/port/agentruntime.go`.
- **F-8**: Skills carry the invoking harness to a command as `--ambient-backend` — source: `.pyxis/authoring/implement-now.md`.

## Fog

none

## Out of scope

- **O-1**: No executor mode, budget meter, or launched worker for prototypes — nothing at today's scale asks for one (from D-1). Superseded by O-4.
- **O-2**: No new draft working-file class, citation stem, or readiness condition (from D-2).
- **O-3**: No automatic removal of a prototype at lock; the lock lands on the code host, away from the local machine (from D-3).
- **O-4**: No budget meter, no routing by task requirements, and no run through the implement-now worker cycle for prototypes — the operator asked for control of the harness and model, and nothing more (from D-4).

## Candidate glossary terms

none

## Parked product gaps

none

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-design |
| **Date** | 2026-10-10 |
| **Authority** | Argus under `sign-gate-verdict` (authority-contract/v1, tier-2) for the operator; delivery:prototype#E-11 |
| **Covering design** | — |
| **Coverage check** | pass — core four covered, zero blocking fog |
| **Amend route** | — |
