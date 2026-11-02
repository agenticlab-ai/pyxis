# Prototype an idea before its contract — brainstorm record

| Field | Value |
|---|---|
| **In one line** | Let whoever hands Pyxis an idea ask for a throwaway prototype first, so its technical challenges and product conflicts are known before the contract is written. |
| **Relevant-when** | Brainstorming or contracting prototyping, spikes, or any step that builds before a feature's contract locks; checking prior rulings on what happens to prototype code. |
| **Idea** | `docs/problems/prototype/problem.md` (the operator's dictated message, verbatim) |
| **Operator** | Argus as decision principal for the operator (@mdarabi) · authority-contract/v1, tier-2 |
| **Date(s)** | 2026-10-10 |
| **Status** | gated: ready-for-prd |
| **Tracking** | — |
| **Feeds** | `drafts/prototype/prd.md` |
| **Prior records** | — |
| **Template version** | brainstorm-record/v1 |

## Orientation

| Source | Why loaded |
|---|---|
| `docs/product-vision.md` | The pipeline grammar, the autonomy levels, and what counts as a stage result. |
| `AGENTS.md` | The standing rule on when implementation may happen. |
| `docs/observability/incidents.md` (I13) | The most recent record of a contract written before anything was built. |
| `docs/prds/design-solution-shape/effective.md` | The nearest existing "throwaway" concept. |
| `docs/prds/plan-new/effective.md` | The one existing mention of a prototype snippet. |
| `.pyxis/authoring/implement-now.md` | How Pyxis implements in isolated worktrees today. |
| `.pyxis/templates/draft-problems-template.md` | How a draft handles a requirement problem found late. |

**Overlaps surfaced:** none covers the idea. The solution shape (prd:design-solution-shape/FR-5) is a paper projection of a design and is never executed. The direct issue path in implement-now builds one bounded task and lands it. The design brainstorm scores alternatives on paper. Nothing builds before a contract in order to learn.

## Framings

- **Evidence for the draft** — the prototype is a throwaway build whose only product is what was learned; the feature is then contracted and implemented in the usual way. Keeps every existing gate; costs one extra build.
- **First commit of the implementation** — the prototype's code becomes the starting point of the real work. Saves the second build; puts unreviewed code with no acceptance criteria under the feature and breaks F-2.
- **Standalone tool** — a command that prototypes any idea and prints a report, tied to no draft. Simple to use; the learning has no route into the contract.

**Destination:** Evidence for the draft. The operator's words separate the prototype from "the real implementation" and ask for the learning to be applied in the feature draft, which the other two framings do not deliver (delivery:prototype#E-4).

## Decisions

### D-1 — What is a prototype in Pyxis?

- **Question**: Is the prototype evidence for the draft, the start of the implementation, or a tool outside the pipeline?
- **Answer**: Evidence for the draft. Pyxis builds a throwaway version of the idea to learn from; the feature is still contracted and implemented in the usual way.
- **Reasoning**: The idea names "the real implementation" apart from the prototype and asks to "apply that learning" in the feature draft. F-2 keeps landed code behind a locked contract.
- **Anchor**: 2026-10-10 · operator intake message; ruled by Argus assumption (delivery:prototype#E-4)
- **Status**: live

### D-2 — Who is it for, and who is it not for?

- **Question**: Which idea providers does a prototype serve?
- **Answer**: Whoever hands Pyxis an idea and cannot yet judge how hard it is or what it collides with; Argus drives on their behalf. It is not for a change whose solution is already understood.
- **Reasoning**: "when we give an idea we can also prototype it first" leaves the choice with the idea's provider. A prototype costs a build, and an understood change gains nothing from it.
- **Anchor**: 2026-10-10 · Argus assumption (delivery:prototype#E-5)
- **Status**: live

### D-3 — What is the bet, and how would it show?

- **Question**: What should a prototype change, observably?
- **Answer**: Building a small throwaway version first surfaces the obstacles and conflicts that otherwise appear after the contract locks. It shows when a prototyped feature's locked contract answers every finding its prototype raised.
- **Reasoning**: "it gives more robust input for the real implementation". F-4 and F-5 are the late discovery this targets.
- **Anchor**: 2026-10-10 · Argus assumption (delivery:prototype#E-5)
- **Status**: live

### D-4 — What is this worth?

- **Question**: Which tier and what budget?
- **Answer**: Tier T1. A budget of four plan tasks, and the operator's one review of the contract.
- **Reasoning**: The operator called it a "simple idea". The worst realistic outcome today is one session of model work spent on findings nobody uses, which is reversible.
- **Anchor**: 2026-10-10 · Argus assumption (delivery:prototype#E-5)
- **Status**: superseded by D-11 — the operator's ruling in D-10 added two tasks

### D-5 — When does a prototype happen?

- **Question**: Is a prototype a required step, and where does it sit?
- **Answer**: It happens only when the idea's provider asks for one, and before the feature's PRD is written.
- **Reasoning**: "we can also prototype it first": optional, and first.
- **Anchor**: 2026-10-10 · operator intake message
- **Status**: live

### D-6 — Where is it built?

- **Question**: Where does the prototype's code live while it is built?
- **Answer**: In a Git worktree of its own, apart from the feature's draft and from mainline.
- **Reasoning**: "pyxis will go implement it in a work tree" (read per F-1).
- **Anchor**: 2026-10-10 · operator intake message
- **Status**: live

### D-7 — What does it produce?

- **Question**: What comes out of a prototype?
- **Answer**: Two kinds of finding: technical challenges, and product conflicts that need thinking through. Each states what was tried and what was observed.
- **Reasoning**: "try to understand what are the challenges technical challenges where the product conflicts that we need to think through". The observation is what separates a finding from an opinion.
- **Anchor**: 2026-10-10 · operator intake message; the observation clause is an Argus assumption (delivery:prototype#E-6)
- **Status**: live

### D-8 — How is the learning applied?

- **Question**: How do findings reach the feature draft and the implementation?
- **Answer**: Each finding is answered in the feature's draft, by a PRD row, a tech-spec decision, or a ruled requirement problem, or it is set aside with a reason. Implementation receives the learning through the locked contract.
- **Reasoning**: "you apply that learning when you're going … through the feature draft and the actual implementation". The idea names no channel, so the existing one is assumed.
- **Anchor**: 2026-10-10 · operator intake message; the channel is an Argus assumption (delivery:prototype#E-6)
- **Status**: live

### D-9 — What if the prototype does not work?

- **Question**: Is a prototype that never runs a failed step?
- **Answer**: No. It still reports; not reaching a working state is a finding, with what stopped it.
- **Reasoning**: The purpose is to understand the challenges, and the hardest one is often what stopped the build.
- **Anchor**: 2026-10-10 · Argus assumption (delivery:prototype#E-6)
- **Status**: live

### D-10 — Who chooses the harness and model that build a prototype?

- **Question**: Does the session that asked build the prototype, or does Pyxis choose what executes it?
- **Answer**: A Pyxis command chooses and launches it as a separate headless run: on the harness and model the repository's configuration names, and on the current session's harness and model when none is configured. Prototypes do not run through the implement-now flow.
- **Reasoning**: The operator wants to control what executes a prototype. Implement-now workers must pass the full code gate and publish a proposal, which throwaway code should not.
- **Anchor**: 2026-10-10 · review — operator rulings problem:prototype#A-2 and problem:prototype#A-3
- **Status**: live

### D-11 — What is this worth, after D-10?

- **Question**: Does the tier or the budget change with the added requirement?
- **Answer**: Tier T1 stays. The budget rises to five plan tasks.
- **Reasoning**: The launched build adds a config block with route resolution and the launch itself, two tasks. The worst realistic outcome is still reversible. The operator was told the price before the change was applied.
- **Anchor**: 2026-10-10 · review — Argus assumption (delivery:prototype#E-21)
- **Status**: live

### D-12 — Can a prototype run without a feature draft?

- **Question**: Is the prototype step available outside an open feature draft?
- **Answer**: Not in this first version. A prototype exists only inside a feature draft, where its findings are stored.
- **Reasoning**: Storing findings outside a draft is new scope with no consumer named yet.
- **Anchor**: 2026-10-10 · review — operator ruling problem:prototype#A-3
- **Status**: live

## Facts

- **F-1**: The idea arrived as dictated speech. "pixies" and "pixie" are read as Pyxis, "work tree" as a Git worktree, and "future draft" as the feature draft — source: `docs/problems/prototype/problem.md`; reading recorded at delivery:prototype#E-3.
- **F-2**: Implementation follows a locked feature draft or an approved existing document contract — source: `AGENTS.md`, "What this repo is".
- **F-3**: The pipeline grammar's design-direction result is "alternatives and a ruling proportional to uncertainty", and implementation takes the executable plan and frozen charters as its input — source: `docs/product-vision.md` §4.
- **F-4**: The maintenance-jobs contract locked at 45 rows and eight decisions with ten design findings each closed by adding a mechanism; the operator cut its scope after the plan froze, about 25 hours after the first ask, with none of the capability implemented — source: `docs/observability/incidents.md` I13.
- **F-5**: A draft carries a problems file because design and review sessions find wrong, missing, or unbuildable requirements after the PRD is written, and a locked feature can be reopened with the cause `gap-lock-should-have-caught` — source: `.pyxis/templates/draft-problems-template.md`; `pyxis draft reopen` usage.
- **F-6**: A draft's working files are a closed set: the PRD, the tech spec, problems, events, and numbered files under `brainstorm/` and `reviews/` — source: `internal/draft/compile/document.go`.
- **F-7**: Implementation workers already run in isolated per-task worktrees — source: `.pyxis/authoring/implement-now.md`.
- **F-8**: A plan charter may carry "a marked prototype/design snippet" — source: prd:plan-new/FR-13.

## Fog

- **G-1** (non-blocking — blocks: the tech spec's decision on where the prototype lives): how long the prototype's worktree and branch are kept after its findings are written, and who removes them.
- **G-2** (non-blocking — blocks: the tech spec's decision on who builds): whether the session that asked for the prototype builds it, or a separate worker does.

## Out of scope

- **O-1**: Prototype code is never merged, proposed for merge, or used as the starting commit of the implementation — landed code follows a locked contract (F-2), and a prototype has no acceptance criteria and no review (from D-1).
- **O-2**: A prototype is never required, and no gate fails because a draft has none — the provider chooses it (from D-5).
- **O-3**: A prototype settles no product question; it raises them, and rulings stay with the operator or with Argus under its grant (from D-7, D-8).
- **O-4**: Argus starting a prototype that nobody asked for is deferred — the idea places the choice with the provider (from D-5).
- **O-5**: Prototypes do not run through the implement-now flow — its workers must pass the full code gate and publish a proposal (from D-10).
- **O-6**: A prototype outside an open feature draft is deferred — its findings have no home outside a draft yet (from D-12).

## Candidate glossary terms

- **T-1**: prototype — a throwaway build of an idea, made to learn its technical challenges and product conflicts before the feature's contract is written; never merged · avoid: spike, proof of concept, MVP
- **T-2**: prototype finding — one thing a prototype taught, of kind technical challenge or product conflict, with what was tried and what was observed · avoid: issue, bug, lesson

## Parked technical constraints

- **P-1**: A prototype's findings need a home among the draft's existing working-file classes, or the draft compiler changes — context: F-6.
- **P-2**: The draft pull request admits no application code path under Argus attribution, so prototype code cannot ride it — context: `.pyxis/authoring/argus.md`, "Attribution, always".
- **P-3**: A draft branch cannot be published until its contract is complete and reviewed, so findings written before the PRD stay local until then — context: delivery:prototype#E-2.

## Gate

| Field | Value |
|---|---|
| **Verdict** | ready-for-prd |
| **Date** | 2026-10-10 |
| **Operator** | Argus under `sign-gate-verdict` (authority-contract/v1, tier-2) for the operator; delivery:prototype#E-7 |
| **Covering source** | — |
| **Coverage check** | pass — all six inputs covered, zero blocking fog |
